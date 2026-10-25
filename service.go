package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"bucketeer/internal/store"
)

// Service is the app's storage: connections, buckets, objects, transfers
// and previews. Views call it on their own goroutines, so the state it
// shares is guarded by mu; results come back through the app's window.
type Service struct {
	mu       sync.Mutex
	profiles *store.ProfileStore
	manager  *store.Manager
	// secrets is the credential store, held so the app's own tokens (such
	// as the upload service's) live beside the access keys.
	secrets       store.SecretStore
	session       *store.Session
	settings      Settings
	closed        bool
	connectionSeq uint64
	// dataDir is where the settings file lives, fixed when the service is
	// built so a service can be rooted at a temporary directory in tests.
	dataDir string
	// picgoQueue is what the image-upload service hands its requests to,
	// and picgo the one running HTTP service, nil when it is off.
	picgoQueue *store.PicGoQueue
	picgo      *picgoServer
	// onTransfers is told that the transfer list changed. It runs on a
	// transfer goroutine, so it only marks the interface for a redraw.
	onTransfers func()
}

// Settings are the app's preferences, kept in PathUserData.
type Settings struct {
	// ViewMode is "grid" or "list".
	ViewMode string `json:"viewMode"`
	// ShowThumbnails asks the grid to load image previews.
	ShowThumbnails bool `json:"showThumbnails"`
	// ConfirmDelete asks before deleting anything.
	ConfirmDelete bool `json:"confirmDelete"`
	// PresignTTLHours is how long a share link stays valid.
	PresignTTLHours int `json:"presignTtlHours"`
	// Appearance is "system", "light" or "dark": whether the interface
	// follows the system's appearance or pins one of its own.
	Appearance string `json:"appearance"`
	// DarkMode is the appearance switch of earlier settings files, kept so
	// they still read. New files carry Appearance.
	DarkMode bool `json:"darkMode,omitempty"`
	// PicGo configures the image-upload service, kept beside the rest of
	// the preferences because it is one.
	PicGo PicGoConfig `json:"picgo"`
}

func defaultSettings() Settings {
	return Settings{
		ViewMode:        "grid",
		ShowThumbnails:  true,
		ConfirmDelete:   true,
		PresignTTLHours: 24,
		Appearance:      "system",
		PicGo:           defaultPicGo(),
	}
}

// SessionInfo describes the connection the app is browsing.
type SessionInfo struct {
	Connected bool
	Profile   *store.Profile
	Host      string
}

// TestResult is what a connection test found.
type TestResult struct {
	OK      bool
	Message string
	Buckets []string
	// Took is how long the round trip took.
	Took time.Duration
}

// newService builds the app's service, reading the saved connections and
// settings from the app's data directory.
func newService() (*Service, error) {
	dataDir, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return nil, fmt.Errorf("find the app's data directory: %w", err)
	}
	secrets := newFileSecrets(dataDir)
	svc, err := newServiceAt(dataDir, secrets)
	if err != nil {
		return nil, err
	}
	// Preload the Cloudflare R2 connection the app ships with (r2.json),
	// so the app starts with a connection ready to use.
	svc.seedR2Connection()
	return svc, nil
}

// newServiceAt builds a service rooted at the given directory, keeping
// its secrets in the given store. Tests use it to run the service against
// a temporary directory and an in-memory credential store.
func newServiceAt(dataDir string, secrets store.SecretStore) (*Service, error) {
	s := &Service{
		profiles:   store.NewProfileStore(filepath.Join(dataDir, "connections.json"), secrets),
		settings:   defaultSettings(),
		dataDir:    dataDir,
		secrets:    secrets,
		picgoQueue: store.NewPicGoQueue(),
	}
	if err := s.loadSettings(dataDir); err != nil {
		return nil, err
	}
	s.manager = store.NewManager(3, s.transfersChanged)
	return s, nil
}

// migrateKeychain moves the credentials an older build kept in the system
// keychain into the file store: one last read per key that is there (the
// only authorization prompt left), then the keychain entries go. A key the
// user declines to hand over stays in the keychain and the import retries
// next launch.
func (s *Service) migrateKeychain() {
	keys := []string{picgoSecretKey}
	if profiles, err := s.profiles.List(); err == nil {
		for _, p := range profiles {
			keys = append(keys, p.SecretKey())
		}
	}
	for _, key := range keys {
		if _, err := s.secrets.Get(key); err == nil {
			continue // already in the file
		}
		value, err := legacySecrets.Get(key)
		if err != nil {
			continue // not there, or the user said no this time
		}
		if err := s.secrets.Set(key, value); err != nil {
			continue // retry next launch rather than drop the key
		}
		_ = legacySecrets.Delete(key)
	}
}

// setTransferListener records what to tell when the transfer list changes.
func (s *Service) setTransferListener(fn func()) {
	s.mu.Lock()
	s.onTransfers = fn
	s.mu.Unlock()
}

// transfersChanged tells the app that the transfer list changed. It runs
// on a transfer goroutine, so it reads the listener under the lock and
// calls it outside.
func (s *Service) transfersChanged() {
	s.mu.Lock()
	fn := s.onTransfers
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// legacySecrets is where older builds kept credentials; the file store
// imports from it at startup. A variable so tests can stand in for the
// keychain.
var legacySecrets store.SecretStore = secretStore{}

// secretStore is the credential store the window and the command line
// used to share; see secrets.go.
type secretStore = sharedSecrets

// appIdentifier is the app's bundle identifier, which MyGo names the
// credential store after.
const appIdentifier = "dev.bucketeer.app"

// r2Config is the shape of the optional r2.json the app ships with, which
// preloads a Cloudflare R2 connection so the app starts ready to use.
type r2Config struct {
	Endpoint        string `json:"Endpoint"`
	Bucket          string `json:"Bucket"`
	AccessKeyID     string `json:"Access Key ID"`
	SecretAccessKey string `json:"Secret Access Key"`
}

// readR2Config returns the Cloudflare R2 connection the app ships with,
// from r2.json in the app's resources, or nothing when there is none.
func readR2Config() (r2Config, bool) {
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return r2Config{}, false
	}
	b, err := os.ReadFile(filepath.Join(dir, "r2.json"))
	if err != nil {
		return r2Config{}, false
	}
	var cfg r2Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return r2Config{}, false
	}
	if cfg.Endpoint == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		return r2Config{}, false
	}
	return cfg, true
}

// seedR2Connection saves the Cloudflare R2 connection from r2.json, when
// the app ships one and it is not saved yet. It is idempotent: a saved
// connection to the same endpoint with the same access key is left alone,
// so the user's edits and renames are kept.
func (s *Service) seedR2Connection() bool {
	cfg, ok := readR2Config()
	if !ok {
		return false
	}
	list, err := s.profiles.List()
	if err != nil {
		return false
	}
	for _, p := range list {
		if p.Endpoint == cfg.Endpoint && p.AccessKeyID == cfg.AccessKeyID {
			return false // already saved
		}
	}

	name := "Cloudflare R2"
	if cfg.Bucket != "" {
		name = "Cloudflare R2 · " + cfg.Bucket
	}
	if _, err := s.profiles.Create(store.Input{
		Name:        name,
		Provider:    string(store.ProviderR2),
		Endpoint:    cfg.Endpoint,
		Region:      "auto",
		Bucket:      cfg.Bucket,
		AccessKeyID: cfg.AccessKeyID,
		SecretKey:   cfg.SecretAccessKey,
		PathStyle:   false,
	}); err != nil {
		return false
	}
	return true
}

// r2Profile returns the preloaded Cloudflare R2 connection, if one is
// saved, matching the r2.json the app ships with.
func (s *Service) r2Profile() *store.Profile {
	cfg, ok := readR2Config()
	if !ok {
		return nil
	}
	list, err := s.profiles.List()
	if err != nil {
		return nil
	}
	for i := range list {
		if list[i].Endpoint == cfg.Endpoint && list[i].AccessKeyID == cfg.AccessKeyID {
			return &list[i]
		}
	}
	return nil
}

// settingsPath is where the settings file lives.
func settingsPath(dataDir string) string { return filepath.Join(dataDir, "settings.json") }

func (s *Service) loadSettings(dataDir string) error {
	b, err := os.ReadFile(settingsPath(dataDir))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read settings: %w", err)
	}
	settings := defaultSettings()
	if err := json.Unmarshal(b, &settings); err != nil {
		// A settings file the app cannot read should not stop it from
		// starting; the defaults are good enough.
		return nil
	}
	// A file from before the three-way appearance choice carries no
	// appearance key; its dark-mode switch is the pin to migrate. Files
	// the app writes always carry the key.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(b, &probe); err == nil {
		if _, ok := probe["appearance"]; !ok {
			if settings.DarkMode {
				settings.Appearance = "dark"
			} else {
				settings.Appearance = "system"
			}
		}
	}
	s.mu.Lock()
	s.settings = normalizeSettings(settings)
	s.mu.Unlock()
	return nil
}

func normalizeSettings(settings Settings) Settings {
	if settings.ViewMode != "grid" && settings.ViewMode != "list" {
		settings.ViewMode = "grid"
	}
	if settings.PresignTTLHours < 1 {
		settings.PresignTTLHours = 24
	}
	if settings.PresignTTLHours > 24*7 {
		settings.PresignTTLHours = 24 * 7
	}
	switch settings.Appearance {
	case "system", "light", "dark":
	default:
		// A settings file from before the three-way choice: a pinned dark
		// appearance keeps its pin, the rest follow the system.
		if settings.DarkMode {
			settings.Appearance = "dark"
		} else {
			settings.Appearance = "system"
		}
	}
	settings.DarkMode = settings.Appearance == "dark"
	settings.PicGo = normalizePicGo(settings.PicGo)
	return settings
}

// dataDir returns the app's data directory.
func dataDir() (string, error) { return mygo.App.Path(mygo.PathUserData) }

// Close stops the transfers and the image-upload service, so a late
// progress report cannot reach an interface that is gone.
func (s *Service) Close() {
	s.mu.Lock()
	picgo := s.picgo
	queue := s.picgoQueue
	s.mu.Unlock()
	if picgo != nil {
		picgo.stop()
	}
	if queue != nil {
		queue.Close()
	}
	s.mu.Lock()
	s.closed = true
	s.connectionSeq++
	manager := s.manager
	s.session = nil
	s.onTransfers = nil
	s.mu.Unlock()
	if manager != nil {
		manager.Close()
	}
}

// currentSession returns the open session, or an error explaining that
// nothing is connected.
func (s *Service) currentSession() (*store.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return nil, errors.New("connect to a storage endpoint first")
	}
	return s.session, nil
}

// Connected reports whether a storage endpoint is open.
func (s *Service) Connected() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.session != nil
}

/* ---- Connections ---- */

// Providers returns the provider presets the connection form offers.
func (s *Service) Providers() []store.Preset { return store.Presets }

// ListConnections returns the saved connections.
func (s *Service) ListConnections() ([]store.Profile, error) {
	list, err := s.profiles.List()
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []store.Profile{}
	}
	return list, nil
}

// SaveConnection creates a connection, or updates the one with the given
// id when it is not empty.
func (s *Service) SaveConnection(id string, in store.Input) (store.Profile, error) {
	if strings.TrimSpace(id) == "" {
		return s.profiles.Create(in)
	}
	return s.profiles.Update(id, in)
}

// DeleteConnection forgets a connection.
func (s *Service) DeleteConnection(id string) error {
	s.mu.Lock()
	if s.session != nil && s.session.Profile.ID == id {
		s.session = nil
	}
	s.mu.Unlock()
	return s.profiles.Delete(id)
}

// TestConnection checks credentials against the endpoint without saving
// them.
func (s *Service) TestConnection(in store.Input) (TestResult, error) {
	if in.Endpoint == "" || in.AccessKeyID == "" {
		return TestResult{}, errors.New("enter the endpoint, the access key ID and the secret access key")
	}
	if !strings.Contains(in.Endpoint, "://") {
		in.Endpoint = "https://" + in.Endpoint
	}

	// Test uses the credentials from the form, not a stored secret, so it
	// goes through a throwaway store whose secret is in memory.
	dir, err := os.MkdirTemp("", "s3test")
	if err != nil {
		return TestResult{}, err
	}
	defer os.RemoveAll(dir)

	temp := store.NewProfileStore(filepath.Join(dir, "test.json"), &memorySecrets{})
	created, err := temp.Create(in)
	if err != nil {
		return TestResult{}, err
	}

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	session, err := temp.Open(ctx, created.ID)
	if err != nil {
		return TestResult{OK: false, Message: err.Error()}, nil
	}

	if created.Bucket != "" {
		// If a bucket is specified, test access directly against that bucket.
		listing, err := session.List(ctx, created.Bucket, "")
		if err != nil {
			return TestResult{OK: false, Message: fmt.Sprintf("Bucket “%s”: %v", created.Bucket, err)}, nil
		}
		return TestResult{
			OK:      true,
			Message: fmt.Sprintf("Connected to bucket “%s” (%d items found).", created.Bucket, len(listing.Items)),
			Buckets: []string{created.Bucket},
			Took:    time.Since(started),
		}, nil
	}

	buckets, err := session.Buckets(ctx)
	if err != nil {
		msg := err.Error()
		if strings.Contains(strings.ToLower(msg), "access denied") {
			msg = "Access denied when listing all buckets. If your API token is scoped to a specific bucket (e.g. Cloudflare R2), enter the bucket name in the Bucket field."
		}
		return TestResult{OK: false, Message: msg}, nil
	}
	names := make([]string, 0, len(buckets))
	for _, b := range buckets {
		names = append(names, b.Name)
	}
	return TestResult{
		OK:      true,
		Message: fmt.Sprintf("Connected. %s.", plural(len(names), "bucket", "buckets")),
		Buckets: names,
		Took:    time.Since(started),
	}, nil
}

// memorySecrets keeps a secret for the lifetime of one call, for testing
// credentials the user has not saved yet.
type memorySecrets struct {
	mu     sync.Mutex
	values map[string][]byte
}

func (m *memorySecrets) Set(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.values == nil {
		m.values = map[string][]byte{}
	}
	m.values[key] = append([]byte(nil), value...)
	return nil
}

func (m *memorySecrets) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	if !ok {
		return nil, errors.New("no such secret")
	}
	return v, nil
}

func (m *memorySecrets) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}

// Connect opens a saved connection and returns what the app is browsing.
func (s *Service) Connect(ctx context.Context, id string) (SessionInfo, error) {
	s.mu.Lock()
	s.connectionSeq++
	generation := s.connectionSeq
	s.mu.Unlock()
	session, err := s.profiles.Open(ctx, id)
	if err != nil {
		return SessionInfo{}, err
	}
	// Fail fast with a clear message: if a bucket was configured, verify that bucket;
	// otherwise verify ListBuckets.
	if session.Profile.Bucket != "" {
		if _, err := session.List(ctx, session.Profile.Bucket, ""); err != nil {
			return SessionInfo{}, fmt.Errorf("connect to bucket %q: %w", session.Profile.Bucket, err)
		}
	} else {
		if _, err := session.Buckets(ctx); err != nil {
			return SessionInfo{}, err
		}
	}
	s.mu.Lock()
	if s.closed || generation != s.connectionSeq || ctx.Err() != nil {
		s.mu.Unlock()
		return SessionInfo{}, context.Canceled
	}
	s.session = session
	s.mu.Unlock()
	s.connectToPicGo()
	return info(session), nil
}

// Disconnect closes the open connection.
func (s *Service) Disconnect() SessionInfo {
	s.mu.Lock()
	s.connectionSeq++
	s.session = nil
	s.mu.Unlock()
	s.connectToPicGo()
	return SessionInfo{}
}

// CurrentConnection reports what the app is connected to, if anything.
func (s *Service) CurrentConnection() SessionInfo {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.session == nil {
		return SessionInfo{}
	}
	return info(s.session)
}

func info(s *store.Session) SessionInfo {
	p := s.Profile
	return SessionInfo{Connected: true, Profile: &p, Host: p.Host()}
}

/* ---- Buckets ---- */

// ListBuckets returns the endpoint's buckets.
func (s *Service) ListBuckets(ctx context.Context) ([]store.Bucket, error) {
	session, err := s.currentSession()
	if err != nil {
		return nil, err
	}
	buckets, err := session.Buckets(ctx)
	if err != nil {
		// If listing all buckets is denied but this connection is configured for a specific bucket,
		// return that bucket so the user can browse it seamlessly.
		if session.Profile.Bucket != "" && strings.Contains(strings.ToLower(err.Error()), "access denied") {
			return []store.Bucket{{Name: session.Profile.Bucket}}, nil
		}
		return nil, err
	}
	if buckets == nil {
		buckets = []store.Bucket{}
	}
	return buckets, nil
}

// CreateBucket makes a bucket.
func (s *Service) CreateBucket(ctx context.Context, name string) error {
	session, err := s.currentSession()
	if err != nil {
		return err
	}
	return session.CreateBucket(ctx, name)
}

// DeleteBucket removes an empty bucket.
func (s *Service) DeleteBucket(ctx context.Context, name string) error {
	session, err := s.currentSession()
	if err != nil {
		return err
	}
	return session.DeleteBucket(ctx, name)
}

/* ---- Objects ---- */

// ListObjects returns the contents of a prefix, one level deep.
func (s *Service) ListObjects(ctx context.Context, bucket, prefix string) (store.Listing, error) {
	session, err := s.currentSession()
	if err != nil {
		return store.Listing{}, err
	}
	return session.List(ctx, bucket, prefix)
}

// SearchObjects returns every object under a prefix matching a query.
func (s *Service) SearchObjects(ctx context.Context, bucket, prefix, query string, limit int) (store.Listing, error) {
	session, err := s.currentSession()
	if err != nil {
		return store.Listing{}, err
	}
	return session.Search(ctx, bucket, prefix, query, limit)
}

// RecentObjects returns the newest objects in a bucket, newest first, for
// the Recent files section.
func (s *Service) RecentObjects(ctx context.Context, bucket string, limit int) (store.Listing, error) {
	session, err := s.currentSession()
	if err != nil {
		return store.Listing{}, err
	}
	return session.Recent(ctx, bucket, limit)
}

// ObjectInfo returns one object's metadata.
func (s *Service) ObjectInfo(ctx context.Context, bucket, key string) (store.ObjectInfo, error) {
	session, err := s.currentSession()
	if err != nil {
		return store.ObjectInfo{}, err
	}
	return session.StatObject(ctx, bucket, key)
}

// CreateFolder makes a folder in a prefix.
func (s *Service) CreateFolder(ctx context.Context, bucket, prefix, name string) (string, error) {
	session, err := s.currentSession()
	if err != nil {
		return "", err
	}
	return session.MakeFolder(ctx, bucket, prefix, name)
}

// RenameObject renames an object, keeping it in the same folder.
func (s *Service) RenameObject(ctx context.Context, bucket, key, newName string) error {
	session, err := s.currentSession()
	if err != nil {
		return err
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return errors.New("give the file a name")
	}
	if strings.Contains(newName, "/") {
		return errors.New("a file name cannot contain a slash")
	}
	parent := ""
	if i := strings.LastIndex(key, "/"); i >= 0 {
		parent = key[:i+1]
	}
	return session.Rename(ctx, bucket, key, parent+newName)
}

// MoveObjects moves objects into a destination prefix.
func (s *Service) MoveObjects(ctx context.Context, bucket string, keys []string, destPrefix string) (int, error) {
	session, err := s.currentSession()
	if err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, errors.New("select something to move")
	}
	if err := session.MoveToFolder(ctx, bucket, keys, destPrefix); err != nil {
		return 0, err
	}
	return len(keys), nil
}

// DeleteObjects deletes objects and folders, reporting how many objects
// went.
func (s *Service) DeleteObjects(ctx context.Context, bucket string, keys []string) (int, error) {
	session, err := s.currentSession()
	if err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, errors.New("select something to delete")
	}
	return session.Remove(ctx, bucket, keys)
}

// BucketStat sums a bucket's storage.
func (s *Service) BucketStat(ctx context.Context, bucket string, limit int) (store.Stat, error) {
	session, err := s.currentSession()
	if err != nil {
		return store.Stat{}, err
	}
	return session.Stat(ctx, bucket, limit)
}

// PresignObject returns a temporary link to an object.
func (s *Service) PresignObject(ctx context.Context, bucket, key string, hours int) (string, error) {
	session, err := s.currentSession()
	if err != nil {
		return "", err
	}
	if hours <= 0 {
		s.mu.Lock()
		hours = s.settings.PresignTTLHours
		s.mu.Unlock()
	}
	return session.Presign(ctx, bucket, key, time.Duration(hours)*time.Hour)
}

// MaxPreviewBytes caps how much of an object the app reads into memory
// for a preview.
const MaxPreviewBytes = 1 << 20

// MaxImageBytes is the largest image a preview or an upload reads.
const MaxImageBytes = 20 << 20

// TextPreview is a text file's content, and whether it was truncated.
type TextPreview struct {
	Text      string
	Truncated bool
	Size      int64
	// Binary is true when the content does not look like text.
	Binary bool
}

// ReadText returns the start of an object, for the text preview. Reading
// through Go rather than a presigned URL avoids needing CORS on the
// bucket.
func (s *Service) ReadText(ctx context.Context, bucket, key string) (TextPreview, error) {
	body, obj, err := s.readObject(ctx, bucket, key, MaxPreviewBytes)
	if err != nil {
		return TextPreview{}, err
	}
	if looksBinary(body) {
		return TextPreview{Binary: true, Size: obj.Size}, nil
	}
	return TextPreview{
		Text:      string(body),
		Truncated: obj.Size > int64(len(body)),
		Size:      obj.Size,
	}, nil
}

// ReadImage returns an object's bytes, for a picture the app draws
// itself: a thumbnail in a grid of files, or the preview of an image the
// user opened. Objects larger than MaxPreviewBytes are refused rather
// than read into memory.
func (s *Service) ReadImage(ctx context.Context, bucket, key string) ([]byte, error) {
	body, obj, err := s.readObject(ctx, bucket, key, MaxImageBytes)
	if err != nil {
		return nil, err
	}
	if obj.Size > MaxImageBytes {
		return nil, fmt.Errorf("%s is too large to preview", obj.Name)
	}
	return body, nil
}

// readObject reads the start of an object, up to limit bytes, and returns
// what it read with the object's metadata.
func (s *Service) readObject(ctx context.Context, bucket, key string, limit int64) ([]byte, store.ObjectInfo, error) {
	session, err := s.currentSession()
	if err != nil {
		return nil, store.ObjectInfo{}, err
	}
	body, obj, err := session.OpenObject(ctx, bucket, key, 0, limit-1)
	if err != nil {
		return nil, store.ObjectInfo{}, err
	}
	defer body.Close()
	b, err := io.ReadAll(io.LimitReader(body, limit))
	if err != nil {
		return nil, store.ObjectInfo{}, fmt.Errorf("read %s: %w", obj.Name, err)
	}
	return b, obj, nil
}

// looksBinary reports whether b holds bytes a text view cannot show,
// judging by the first few kilobytes.
func looksBinary(b []byte) bool {
	sample := b
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	for _, c := range sample {
		if c == 0 {
			return true
		}
	}
	return false
}

/* ---- Transfers ---- */

// PickUploadFiles asks where to upload from and returns the chosen paths.
func (s *Service) PickUploadFiles(parent *mygo.Window) ([]string, error) {
	return mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:   parent,
		Title:    "Choose files to upload",
		Message:  "The files are uploaded to the folder you are browsing.",
		Multiple: true,
	})
}

// PickUploadFolder asks for a folder to upload, whose files go under a
// prefix named after it.
func (s *Service) PickUploadFolder(parent *mygo.Window) ([]string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    parent,
		Title:     "Choose a folder to upload",
		Directory: true,
		Multiple:  true,
	})
	return paths, err
}

// PickDownloadDir asks where to save downloads.
func (s *Service) PickDownloadDir(parent *mygo.Window) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent:    parent,
		Title:     "Choose where to save",
		Directory: true,
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

// UploadPaths uploads local files into a prefix. It queues the work and
// returns how many files are on their way.
func (s *Service) UploadPaths(ctx context.Context, bucket, prefix string, paths []string) (int, error) {
	session, err := s.currentSession()
	if err != nil {
		return 0, err
	}
	if len(paths) == 0 {
		return 0, errors.New("choose some files to upload")
	}
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var queued int
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return queued, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		if info.IsDir() {
			// Uploading a folder means uploading the files inside it,
			// keeping the folder's name as a prefix.
			base := filepath.Base(path)
			err := filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() {
					return nil
				}
				rel, err := filepath.Rel(path, p)
				if err != nil {
					return err
				}
				key := prefix + base + "/" + filepath.ToSlash(rel)
				fi, err := d.Info()
				if err != nil {
					return err
				}
				s.manager.EnqueueUpload(session, bucket, key, p, fi.Size())
				queued++
				return nil
			})
			if err != nil {
				return queued, fmt.Errorf("read %s: %w", base, err)
			}
			continue
		}
		s.manager.EnqueueUpload(session, bucket, prefix+filepath.Base(path), path, info.Size())
		queued++
	}
	return queued, nil
}

// DownloadResult reports what a download request queued.
type DownloadResult struct {
	Queued int
	// Truncated is true when a folder held more objects than the app will
	// queue in one go, so the rest were left alone.
	Truncated bool
}

// MaxFolderDownload is the most objects one folder download queues.
const MaxFolderDownload = 10_000

// DownloadObjects downloads objects into a directory.
func (s *Service) DownloadObjects(ctx context.Context, bucket string, keys []string, dir string) (DownloadResult, error) {
	session, err := s.currentSession()
	if err != nil {
		return DownloadResult{}, err
	}
	if len(keys) == 0 {
		return DownloadResult{}, errors.New("select something to download")
	}
	if dir == "" {
		return DownloadResult{}, errors.New("choose where to save the files")
	}

	var result DownloadResult
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			// A folder downloads as the files under it, keeping the folder
			// structure below the chosen directory.
			n, truncated, err := s.enqueueFolderDownload(ctx, session, bucket, key, dir)
			result.Queued += n
			result.Truncated = result.Truncated || truncated
			if err != nil {
				return result, err
			}
			continue
		}
		dest := store.UniqueLocalPath(dir, store.DownloadName(key))
		s.manager.EnqueueDownload(session, bucket, key, dest)
		result.Queued++
	}
	return result, nil
}

// enqueueFolderDownload queues every object under a folder prefix, and
// reports whether the folder held more than the app would queue.
func (s *Service) enqueueFolderDownload(ctx context.Context, session *store.Session, bucket, prefix, dir string) (int, bool, error) {
	listing, err := session.Search(ctx, bucket, prefix, "", MaxFolderDownload)
	if err != nil {
		return 0, false, err
	}
	base := store.FolderName(prefix)
	var queued int
	for _, entry := range listing.Items {
		rel := strings.TrimPrefix(entry.Key, prefix)
		dest := filepath.Join(dir, base, filepath.FromSlash(rel))
		s.manager.EnqueueDownload(session, bucket, entry.Key, dest)
		queued++
	}
	if queued == 0 {
		return 0, false, fmt.Errorf("%s is empty", base)
	}
	return queued, listing.Truncated, nil
}

// SaveObjectAs asks where to save one object and downloads it there.
func (s *Service) SaveObjectAs(parent *mygo.Window, bucket, key string) (string, error) {
	session, err := s.currentSession()
	if err != nil {
		return "", err
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
		Parent:      parent,
		Title:       "Save file",
		DefaultPath: store.DownloadName(key),
	})
	if err != nil || path == "" {
		return "", err
	}
	s.manager.EnqueueDownload(session, bucket, key, path)
	return path, nil
}

// ListTransfers returns every transfer, newest first.
func (s *Service) ListTransfers() []store.Transfer {
	s.mu.Lock()
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return nil
	}
	return manager.List()
}

// CancelTransfer stops a transfer.
func (s *Service) CancelTransfer(id string) {
	s.mu.Lock()
	manager := s.manager
	s.mu.Unlock()
	if manager != nil {
		manager.Cancel(id)
	}
}

// ClearFinishedTransfers forgets the transfers that have stopped.
func (s *Service) ClearFinishedTransfers() {
	s.mu.Lock()
	manager := s.manager
	s.mu.Unlock()
	if manager != nil {
		manager.ClearFinished()
	}
}

// ActiveTransfers reports how many transfers are still going.
func (s *Service) ActiveTransfers() int {
	s.mu.Lock()
	manager := s.manager
	s.mu.Unlock()
	if manager == nil {
		return 0
	}
	return manager.Active()
}

/* ---- Settings and the platform ---- */

// GetSettings returns the app's preferences.
func (s *Service) GetSettings() Settings {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings
}

// SetSettings saves the app's preferences.
func (s *Service) SetSettings(settings Settings) error {
	settings = normalizeSettings(settings)

	s.mu.Lock()
	dir := s.dataDir
	s.mu.Unlock()
	if dir == "" {
		var err error
		if dir, err = dataDir(); err != nil {
			return err
		}
	}

	b, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(settingsPath(dir), append(b, '\n'), 0o600); err != nil {
		return fmt.Errorf("save settings: %w", err)
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	return nil
}

// AppInfo describes the app and the machine it runs on.
type AppInfo struct {
	Name      string
	Version   string
	Platform  string
	DataDir   string
	SecretOK  bool
	SecretWhy string
}

// Info returns the app's version and where it keeps its data.
func (s *Service) Info() AppInfo {
	out := AppInfo{
		Name:     "Bucketeer",
		Version:  "0.1.0",
		Platform: runtime.GOOS,
	}
	if name := mygo.App.Name(); name != "" && !strings.HasSuffix(name, ".test") {
		out.Name = name
	}
	if ver := mygo.App.Version(); ver != "" {
		out.Version = ver
	}
	s.mu.Lock()
	out.DataDir = s.dataDir
	s.mu.Unlock()
	if out.DataDir == "" {
		if dir, err := dataDir(); err == nil {
			out.DataDir = dir
		}
	}
	// Credentials live in the app's own data directory now, so no system
	// credential store is required.
	out.SecretOK = true
	return out
}
