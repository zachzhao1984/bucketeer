// Package store keeps S3 connection profiles, talks to S3-compatible
// storage, and runs transfers.
//
// It knows nothing about MyGo: the app layer binds it to the frontend and
// supplies the credential store, so the package stays testable on its own.
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// Provider names an S3-compatible storage service.
type Provider string

// The providers the app has presets for.
const (
	ProviderR2     Provider = "r2"
	ProviderAWS    Provider = "aws"
	ProviderMinIO  Provider = "minio"
	ProviderB2     Provider = "b2"
	ProviderWasabi Provider = "wasabi"
	ProviderSpaces Provider = "spaces"
	ProviderOther  Provider = "other"
)

// Preset is a provider's default connection settings, and the hints the
// connection form shows beside them.
type Preset struct {
	ID           Provider `json:"id"`
	Label        string   `json:"label"`
	Endpoint     string   `json:"endpoint"`
	EndpointHint string   `json:"endpointHint"`
	Region       string   `json:"region"`
	RegionHint   string   `json:"regionHint"`
	PathStyle    bool     `json:"pathStyle"`
	Note         string   `json:"note"`
}

// Presets are the providers the app offers, in menu order.
var Presets = []Preset{
	{
		ID:           ProviderR2,
		Label:        "Cloudflare R2",
		EndpointHint: "https://<account-id>.r2.cloudflarestorage.com",
		Region:       "auto",
		Note:         "Create an S3 API token in the R2 dashboard; region is auto. For bucket-scoped tokens, enter the Bucket below.",
	},
	{
		ID:           ProviderAWS,
		Label:        "Amazon S3",
		EndpointHint: "s3.us-east-1.amazonaws.com",
		Region:       "us-east-1",
		Note:         "Leave the endpoint empty to reach the bucket's own region.",
	},
	{
		ID:           ProviderMinIO,
		Label:        "MinIO",
		EndpointHint: "http://localhost:9000",
		Region:       "us-east-1",
		PathStyle:    true,
		Note:         "Self-hosted MinIO answers path-style requests.",
	},
	{
		ID:           ProviderB2,
		Label:        "Backblaze B2",
		EndpointHint: "https://s3.us-west-004.backblazeb2.com",
		Region:       "us-west-004",
		Note:         "The endpoint and region come from the bucket's B2 details.",
	},
	{
		ID:           ProviderWasabi,
		Label:        "Wasabi",
		EndpointHint: "https://s3.us-east-1.wasabisys.com",
		Region:       "us-east-1",
		Note:         "Match the region in the endpoint's host name.",
	},
	{
		ID:           ProviderSpaces,
		Label:        "DigitalOcean Spaces",
		EndpointHint: "https://nyc3.digitaloceanspaces.com",
		Region:       "nyc3",
		Note:         "The region is the endpoint's first label, as in nyc3.",
	},
	{
		ID:           ProviderOther,
		Label:        "Other S3-compatible",
		EndpointHint: "https://storage.example.com",
		Region:       "us-east-1",
		Note:         "Any endpoint that speaks the S3 API.",
	},
}

// PresetFor returns the preset of provider, and whether one exists.
func PresetFor(p Provider) (Preset, bool) {
	if i := slices.IndexFunc(Presets, func(preset Preset) bool { return preset.ID == p }); i >= 0 {
		return Presets[i], true
	}
	return Preset{}, false
}

// Profile is a saved connection to an S3-compatible storage. It holds no
// secret: the secret access key lives in the system's credential store,
// under SecretKey.
type Profile struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Provider    Provider  `json:"provider"`
	Endpoint    string    `json:"endpoint"`
	Region      string    `json:"region"`
	Bucket      string    `json:"bucket,omitempty"`
	AccessKeyID string    `json:"accessKeyId"`
	PathStyle   bool      `json:"pathStyle"`
	HasSecret   bool      `json:"hasSecret"`
	CreatedAt   time.Time `json:"createdAt"`
}

// SecretKey is the credential-store key of the profile's secret access
// key.
func (p Profile) SecretKey() string { return "profile/" + p.ID + "/secret" }

// Initials returns up to two letters for the profile's avatar.
func (p Profile) Initials() string {
	fields := strings.Fields(p.Name)
	switch len(fields) {
	case 0:
		return "?"
	case 1:
		return strings.ToUpper(firstRunes(fields[0], 2))
	default:
		return strings.ToUpper(firstRunes(fields[0], 1) + firstRunes(fields[len(fields)-1], 1))
	}
}

func firstRunes(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

// Host returns the profile's endpoint without its scheme, for display.
func (p Profile) Host() string {
	h := p.Endpoint
	if i := strings.Index(h, "://"); i >= 0 {
		h = h[i+3:]
	}
	return strings.TrimSuffix(h, "/")
}

// SecretStore keeps small secrets, such as access keys, in the system's
// credential store.
type SecretStore interface {
	Set(key string, value []byte) error
	Get(key string) ([]byte, error)
	Delete(key string) error
}

// ErrProfileNotFound is what the profile store returns for an unknown id.
var ErrProfileNotFound = errors.New("no such connection")

// ProfileStore reads and writes the app's connections, and their secrets.
// The file holds everything but the secrets, which go to the credential
// store.
type ProfileStore struct {
	mu      sync.Mutex
	path    string
	secrets SecretStore
	items   []Profile
	loaded  bool
}

// NewProfileStore returns a store keeping its connections in the file at
// path, and their secrets in secrets.
func NewProfileStore(path string, secrets SecretStore) *ProfileStore {
	return &ProfileStore{path: path, secrets: secrets}
}

// file is what the connections file holds.
type profileFile struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// load reads the file, once. A missing file is not an error.
func (s *ProfileStore) load() error {
	if s.loaded {
		return nil
	}
	s.loaded = true
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read connections: %w", err)
	}
	var f profileFile
	if err := json.Unmarshal(b, &f); err != nil {
		return fmt.Errorf("read connections: %w", err)
	}
	s.items = f.Profiles
	return nil
}

// save writes the file atomically, so an interrupted write cannot lose
// every connection.
func (s *ProfileStore) save() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return fmt.Errorf("save connections: %w", err)
	}
	b, err := json.MarshalIndent(profileFile{Version: 1, Profiles: s.items}, "", "  ")
	if err != nil {
		return fmt.Errorf("save connections: %w", err)
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".profiles-*.json")
	if err != nil {
		return fmt.Errorf("save connections: %w", err)
	}
	name := tmp.Name()
	defer os.Remove(name)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("save connections: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("save connections: %w", err)
	}
	if err := os.Chmod(name, 0o600); err != nil {
		return fmt.Errorf("save connections: %w", err)
	}
	if err := os.Rename(name, s.path); err != nil {
		return fmt.Errorf("save connections: %w", err)
	}
	return nil
}

// List returns every connection, oldest first.
func (s *ProfileStore) List() ([]Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return nil, err
	}
	return slices.Clone(s.items), nil
}

// Get returns one connection.
func (s *ProfileStore) Get(id string) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return Profile{}, err
	}
	for _, p := range s.items {
		if p.ID == id {
			return p, nil
		}
	}
	return Profile{}, ErrProfileNotFound
}

// Secret returns the secret access key of a connection.
func (s *ProfileStore) Secret(id string) (string, error) {
	p, err := s.Get(id)
	if err != nil {
		return "", err
	}
	b, err := s.secrets.Get(p.SecretKey())
	if err != nil {
		return "", fmt.Errorf("read the access key of %s: %w", p.Name, err)
	}
	return string(b), nil
}

// Input is what the connection form sends.
type Input struct {
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Endpoint    string `json:"endpoint"`
	Region      string `json:"region"`
	Bucket      string `json:"bucket,omitempty"`
	AccessKeyID string `json:"accessKeyId"`
	// SecretKey is empty to keep the stored one, on an update.
	SecretKey string `json:"secretKey"`
	PathStyle bool   `json:"pathStyle"`
}

// normalize trims the input and fills in the preset's defaults.
func (in Input) normalize() Input {
	in.Name = strings.TrimSpace(in.Name)
	in.Endpoint = strings.TrimSpace(in.Endpoint)
	in.Region = strings.TrimSpace(in.Region)
	in.Bucket = strings.TrimSpace(strings.Trim(in.Bucket, "/"))
	in.AccessKeyID = strings.TrimSpace(in.AccessKeyID)
	in.SecretKey = strings.TrimSpace(in.SecretKey)

	// If the user included a bucket path in the endpoint URL (e.g. https://.../mybucket),
	// extract it into in.Bucket and clean the endpoint.
	if strings.Contains(in.Endpoint, "://") {
		if u, err := url.Parse(in.Endpoint); err == nil && u.Path != "" && u.Path != "/" {
			bucket := strings.Trim(u.Path, "/")
			if in.Bucket == "" && bucket != "" && !strings.Contains(bucket, "/") {
				in.Bucket = bucket
			}
			u.Path = ""
			in.Endpoint = u.String()
		}
	}

	preset, ok := PresetFor(Provider(in.Provider))
	if !ok {
		preset = Presets[len(Presets)-1]
		in.Provider = string(preset.ID)
	}
	if in.Endpoint == "" && !strings.Contains(preset.EndpointHint, "<") {
		in.Endpoint = preset.EndpointHint
	}
	if in.Region == "" {
		in.Region = preset.Region
	}
	return in
}

// validate reports what the form got wrong.
func (in Input) validate(hasSecret bool) error {
	if in.Name == "" {
		return errors.New("give the connection a name")
	}
	if in.Endpoint == "" {
		return errors.New("enter the endpoint")
	}
	if !strings.Contains(in.Endpoint, "://") {
		return errors.New("the endpoint needs a scheme, as in https://")
	}
	if in.AccessKeyID == "" {
		return errors.New("enter the access key ID")
	}
	if !hasSecret && in.SecretKey == "" {
		return errors.New("enter the secret access key")
	}
	return nil
}

// Create saves a new connection and returns it.
func (s *ProfileStore) Create(in Input) (Profile, error) {
	in = in.normalize()
	if err := in.validate(false); err != nil {
		return Profile{}, err
	}

	p := Profile{
		ID:          newID(),
		Name:        in.Name,
		Provider:    Provider(in.Provider),
		Endpoint:    in.Endpoint,
		Region:      in.Region,
		Bucket:      in.Bucket,
		AccessKeyID: in.AccessKeyID,
		PathStyle:   in.PathStyle,
		HasSecret:   true,
		CreatedAt:   time.Now(),
	}
	if err := s.secrets.Set(p.SecretKey(), []byte(in.SecretKey)); err != nil {
		return Profile{}, fmt.Errorf("store the access key: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		s.secrets.Delete(p.SecretKey())
		return Profile{}, err
	}
	s.items = append(s.items, p)
	if err := s.save(); err != nil {
		s.items = s.items[:len(s.items)-1]
		s.secrets.Delete(p.SecretKey())
		return Profile{}, err
	}
	return p, nil
}

// Update changes a connection, keeping its secret unless the input
// carries a new one.
func (s *ProfileStore) Update(id string, in Input) (Profile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return Profile{}, err
	}
	i := slices.IndexFunc(s.items, func(p Profile) bool { return p.ID == id })
	if i < 0 {
		return Profile{}, ErrProfileNotFound
	}

	old := s.items[i]
	in = in.normalize()
	if err := in.validate(old.HasSecret); err != nil {
		return Profile{}, err
	}

	p := old
	p.Name = in.Name
	p.Provider = Provider(in.Provider)
	p.Endpoint = in.Endpoint
	p.Region = in.Region
	p.Bucket = in.Bucket
	p.AccessKeyID = in.AccessKeyID
	p.PathStyle = in.PathStyle
	if in.SecretKey != "" {
		if err := s.secrets.Set(p.SecretKey(), []byte(in.SecretKey)); err != nil {
			return Profile{}, fmt.Errorf("store the access key: %w", err)
		}
		p.HasSecret = true
	}

	s.items[i] = p
	if err := s.save(); err != nil {
		s.items[i] = old
		return Profile{}, err
	}
	return p, nil
}

// Delete forgets a connection and its secret.
func (s *ProfileStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.load(); err != nil {
		return err
	}
	i := slices.IndexFunc(s.items, func(p Profile) bool { return p.ID == id })
	if i < 0 {
		return ErrProfileNotFound
	}
	removed := s.items[i]
	s.items = slices.Delete(s.items, i, i+1)
	if err := s.save(); err != nil {
		s.items = slices.Insert(s.items, i, removed)
		return err
	}
	// The secret is gone from the file either way; a stale keychain entry
	// is harmless, so a failure here does not fail the delete.
	_ = s.secrets.Delete(removed.SecretKey())
	return nil
}

// newID returns a short random identifier.
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand does not fail on the platforms the app runs on.
		panic(fmt.Sprintf("store: random id: %v", err))
	}
	return hex.EncodeToString(b[:])
}
