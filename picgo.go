package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"bucketeer/internal/store"
)

// The PicGo-compatible upload service: a small HTTP server on the loopback
// that puts the images notebooks and editors bring into the bucket the app
// has open, and answers with the URLs to paste back.
//
// The protocol is PicGo's server protocol, which Obsidian's and Typora's
// image-upload plugins already speak: a POST to /upload whose body is
// either multipart form-data with one or more `files` parts, or JSON
// {"list": ["/path/to/image.png"]}; and a reply of
// {"success": true, "result": ["https://…"]}.

// DefaultPicGoPort is the port the service listens on, which PicGo's own
// clients expect by default.
const DefaultPicGoPort = 36677

// picgoSecretKey is the credential-store key of the secret a request must
// carry, so a web page in the user's browser cannot upload into the bucket
// by guessing the port.
const picgoSecretKey = "picgo-http-token"

// PicGoConfig is the part of the settings this service reads.
type PicGoConfig struct {
	Enabled bool   `json:"enabled"`
	Port    int    `json:"port"`
	Prefix  string `json:"prefix"`
	// ProfileID is the saved connection the uploads go to, "" for whichever
	// the app has open.
	ProfileID string `json:"profileId"`
	// PublicBases maps a connection's id to the origin its objects are
	// served at, as https://cdn.example.com. A connection without one gets
	// presigned links, which expire.
	PublicBases map[string]string `json:"publicBases,omitempty"`
}

// ServiceEnabled reports whether the service wants to run.
func (c PicGoConfig) ServiceEnabled() bool { return c.Enabled }

// defaultPicGo is what the settings hold before anyone opened the page.
func defaultPicGo() PicGoConfig {
	return PicGoConfig{Port: DefaultPicGoPort}
}

// normalizePicGo clamps the config the settings file may hold.
func normalizePicGo(c PicGoConfig) PicGoConfig {
	if c.Port <= 0 || c.Port > 65535 {
		c.Port = DefaultPicGoPort
	}
	c.Prefix = strings.Trim(strings.TrimSpace(c.Prefix), "/")
	return c
}

// PicGoState is what the settings page shows: the configuration as the
// service is running it, and, when it is not running, why.
type PicGoState struct {
	Config PicGoConfig `json:"config"`
	// Token is the secret in the URL, and URL the address a client pastes.
	Token string `json:"token,omitempty"`
	URL   string `json:"url,omitempty"`
	Err   string `json:"error,omitempty"`
}

/* ---- The server ---- */

// picgoServer is one running HTTP service. The app builds a new one each
// time the settings change, replacing the old, so the port and the prefix
// always match what the page shows.
type picgoServer struct {
	svc    *Service
	queue  *store.PicGoQueue
	config PicGoConfig
	token  string
	http   *http.Server
	addr   string
	wg     sync.WaitGroup
}

// startPicGo builds and starts the service from the settings, stopping the
// one that ran before. It returns the running service's state, whatever the
// outcome, so the page can show why nothing is listening.
func (s *Service) startPicGo() PicGoState {
	s.mu.Lock()
	config := normalizePicGo(s.settings.PicGo)
	s.settings.PicGo = config
	old := s.picgo
	s.mu.Unlock()

	if old != nil {
		old.stop()
	}
	if !config.Enabled {
		s.mu.Lock()
		s.picgo = nil
		s.mu.Unlock()
		return PicGoState{Config: config}
	}

	token, err := s.picgoToken()
	if err != nil {
		return PicGoState{Config: config, Err: "Could not read the upload secret: " + err.Error()}
	}

	server := &picgoServer{svc: s, queue: s.picgoQueue, config: config, token: token}
	addr, err := server.listen()
	if err != nil {
		if isAddrInUse(err) {
			return PicGoState{Config: config, Err: fmt.Sprintf("Port %d is already in use: another program has it, or the service is already running there.", config.Port)}
		}
		return PicGoState{Config: config, Err: "Could not listen: " + err.Error()}
	}

	server.addr = addr
	s.mu.Lock()
	s.picgo = server
	s.mu.Unlock()
	return PicGoState{
		Config: config,
		Token:  token,
		URL:    server.uploadURL(),
	}
}

// stop shuts the service down and waits for its handler to return.
func (p *picgoServer) stop() {
	if p == nil || p.http == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	p.http.Shutdown(ctx)
	p.wg.Wait()
}

// listen binds the loopback and wires the routes.
func (p *picgoServer) listen() (string, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/upload", p.handleUpload)
	mux.HandleFunc("/upload/heartbeat", p.handleHeartbeat)
	// Anything else says what the service is, so a person who opens the
	// port in a browser sees something that explains itself.
	mux.HandleFunc("/", p.handleRoot)

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p.config.Port)))
	if err != nil {
		return "", err
	}
	p.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		// The upload itself may be slow on a hotel network; the bound is a
		// guard, not a stopwatch.
		ReadTimeout: 5 * time.Minute,
		IdleTimeout: 2 * time.Minute,
	}
	go func() {
		if err := p.http.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("picgo: serve: %v", err)
		}
	}()
	return listener.Addr().String(), nil
}

// uploadURL is the address a client pastes into its plugin: the endpoint,
// with the secret already in it, so the setup is one copy.
func (p *picgoServer) uploadURL() string {
	if p == nil {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d/upload?secret=%s", p.config.Port, p.token)
}

// isAddrInUse reports whether an error is the port being taken.
func isAddrInUse(err error) bool {
	if err == nil {
		return false
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return strings.Contains(opErr.Err.Error(), "address already in use")
	}
	return strings.Contains(err.Error(), "address already in use")
}

/* ---- The routes ---- */

// handleRoot says what the port is for.
func (p *picgoServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintf(w, "Bucketeer image upload service (PicGo-compatible)\n\nPOST /upload with multipart form-data (files=@image.png)\nor JSON {\"list\": [\"/path/to/image.png\"]}.\n")
}

// handleHeartbeat answers the check some clients make before uploading.
func (p *picgoServer) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !p.authorized(r) {
		p.fail(w, http.StatusUnauthorized, "Unauthorized: the upload URL secret does not match. Copy the upload URL from Bucketeer's settings again.")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprint(w, `{"success":true}`)
}

// handleUpload is the one route a client needs.
func (p *picgoServer) handleUpload(w http.ResponseWriter, r *http.Request) {
	// A browser page cannot send a POST across origins without a preflight
	// it would fail; answering it keeps that guarantee true rather than
	// hoping for it.
	w.Header().Set("Access-Control-Allow-Origin", "http://127.0.0.1")
	w.Header().Set("Vary", "Origin")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-PicGo-Secret")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST, OPTIONS")
		p.fail(w, http.StatusMethodNotAllowed, "POST an image to /upload.")
		return
	}
	if !p.authorized(r) {
		p.fail(w, http.StatusUnauthorized, "Unauthorized: the upload URL secret does not match. Copy the upload URL from Bucketeer's settings again.")
		return
	}

	images, err := readUpload(r)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, store.ErrTooManyUploads) {
			status = http.StatusRequestEntityTooLarge
		}
		p.fail(w, status, err.Error())
		return
	}

	jobs := make([]store.UploadJob, 0, len(images))
	for _, image := range images {
		jobs = append(jobs, store.UploadJob{Name: image.Name, Body: image.Body, SrcPath: image.FromPath})
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()
	result, err := p.svc.picgoQueue.Submit(ctx, store.UploadRequest{
		ProfileID: p.config.ProfileID,
		Prefix:    p.config.Prefix,
		Images:    jobs,
	})
	if err != nil {
		p.fail(w, http.StatusRequestTimeout, "The upload did not finish in time.")
		return
	}
	p.reply(w, result)
}

// authorized checks the secret, which may arrive as ?secret=, as an
// X-PicGo-Secret header, or as a bearer token.
func (p *picgoServer) authorized(r *http.Request) bool {
	if p.token == "" {
		return false
	}
	if got := r.URL.Query().Get("secret"); got != "" {
		return constantTimeEqual(got, p.token)
	}
	if got := r.Header.Get("X-PicGo-Secret"); got != "" {
		return constantTimeEqual(got, p.token)
	}
	auth := r.Header.Get("Authorization")
	if got, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return constantTimeEqual(got, p.token)
	}
	return false
}

// readUpload takes the images out of a request: multipart form-data first,
// then the JSON list of file paths, then an empty body, which some clients
// mean as "upload the clipboard".
func readUpload(r *http.Request) ([]uploadImage, error) {
	switch {
	case strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data"):
		return readMultipart(r)
	case strings.HasPrefix(r.Header.Get("Content-Type"), "application/json"):
		return readJSONList(r)
	default:
		// A body without a type is how a client says "the clipboard".
		return nil, nil
	}
}

// fail answers with PicGo's error shape and a message a person can act on.
func (p *picgoServer) fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, picgoResponse{Success: false, Message: message})
}

// reply answers with PicGo's shape: the URLs of what landed, and, when a
// request was partly successful, the full outcome.
func (p *picgoServer) reply(w http.ResponseWriter, result store.UploadResult) {
	if result.OK {
		writeJSON(w, http.StatusOK, picgoResponse{Success: true, Result: result.URLs})
		return
	}
	message := "One or more uploads failed."
	for _, d := range result.Details {
		if d.Error != "" {
			message = d.Error
			break
		}
	}
	writeJSON(w, http.StatusOK, picgoResponse{Success: false, Result: result.URLs, Message: message})
}

/* ---- Token ---- */

// picgoToken reads the request secret, creating one on first use. It lives
// in the credential store beside the access keys, not in the settings file,
// so it does not travel with a backup the user copies around.
func (s *Service) picgoToken() (string, error) {
	if got, err := s.readSecret(picgoSecretKey); err == nil && len(got) > 0 {
		return string(got), nil
	}
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	if err := s.writeSecret(picgoSecretKey, []byte(token)); err != nil {
		return "", err
	}
	return token, nil
}

// RotatePicGoToken replaces the secret, which invalidates every URL copied
// so far.
func (s *Service) RotatePicGoToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	if err := s.secrets.Set(picgoSecretKey, []byte(token)); err != nil {
		return "", err
	}
	// Re-key the running service without touching the listening port.
	s.mu.Lock()
	server := s.picgo
	s.mu.Unlock()
	if server != nil {
		server.token = token
	}
	return token, nil
}

// constantTimeEqual compares two secrets without leaking how much matched.
func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var v byte
	for i := 0; i < len(a); i++ {
		v |= a[i] ^ b[i]
	}
	return v == 0
}

/* ---- The app's controls ---- */

// PicGoStatus reports the service as it is running, for the settings page.
func (s *Service) PicGoStatus() PicGoState {
	s.mu.Lock()
	server := s.picgo
	settings := s.settings.PicGo
	connected := s.session != nil
	s.mu.Unlock()

	if server != nil {
		return PicGoState{
			Config: server.config,
			Token:  server.token,
			URL:    server.uploadURL(),
			Err:    s.picgoConfigError(server.config, connected),
		}
	}
	return PicGoState{Config: normalizePicGo(settings)}
}

// picgoConfigError says what the page should warn about while the service
// runs: a connection that is not open, or one without a public base, whose
// links would expire.
func (s *Service) picgoConfigError(config PicGoConfig, connected bool) string {
	profile, err := s.picgoProfile()
	if err != nil {
		return err.Error()
	}
	_ = connected
	if profile.Bucket == "" {
		return "The connection " + profile.Name + " has no bucket, so uploads have nowhere to go."
	}
	return ""
}

// SavePicGoConfig stores the service's configuration and applies it,
// starting, stopping or restarting the listener as the change needs. It
// returns the service as it now runs.
func (s *Service) SavePicGoConfig(config PicGoConfig) PicGoState {
	config = normalizePicGo(config)
	s.mu.Lock()
	s.settings.PicGo = config
	stored := s.settings
	s.mu.Unlock()
	if err := s.SetSettings(stored); err != nil {
		state := s.PicGoStatus()
		state.Err = "Could not save the settings: " + err.Error()
		return state
	}
	return s.startPicGo()
}

// connectToPicGo wires the queue's sink to the app's sessions while a
// connection is open, and unwires it when one closes. The Connect and
// Disconnect paths call it.
func (s *Service) connectToPicGo() {
	s.mu.Lock()
	open := s.session != nil
	s.mu.Unlock()
	if open {
		s.picgoQueue.SetSink(s.performPicGoUploads)
		return
	}
	s.picgoQueue.SetSink(nil)
}
