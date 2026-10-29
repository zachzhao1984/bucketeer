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

// The providers the app has presets for, in the order the connection
// form lists them: the services the reference client offers, each with
// the endpoint and region its own documentation gives.
const (
	ProviderR2     Provider = "r2"
	ProviderAWS    Provider = "aws"
	ProviderB2     Provider = "b2"
	ProviderWasabi Provider = "wasabi"
	ProviderMinIO  Provider = "minio"
	ProviderOSS    Provider = "oss"
	ProviderCOS    Provider = "cos"
	ProviderSpaces Provider = "spaces" // kept for saved connections created before this menu changed
	ProviderOther  Provider = "other"
)

// Preset is a provider's default connection settings, and the hints the
// connection form shows beside them.
type Preset struct {
	ID    Provider `json:"id"`
	Label string   `json:"label"`
	// EndpointHint is what the endpoint field shows while it is empty: a
	// real host for the services whose host is fixed, or a template in
	// angle brackets when it carries the account.
	EndpointHint string `json:"endpointHint"`
	Region       string `json:"region"`
	RegionHint   string `json:"regionHint"`
	// EndpointPattern, when set, derives the endpoint from the region:
	// "{region}" is replaced by the region the form holds. The services
	// whose host carries their region work this way, so a person who
	// knows only the region gets a working endpoint.
	EndpointPattern string `json:"endpointPattern,omitempty"`
	PathStyle       bool   `json:"pathStyle"`
	Note            string `json:"note"`
}

// EndpointFor returns the endpoint a preset's pattern makes for a region,
// or "" when the preset has no pattern.
func (p Preset) EndpointFor(region string) string {
	if p.EndpointPattern == "" || region == "" {
		return ""
	}
	return strings.ReplaceAll(p.EndpointPattern, "{region}", region)
}

// Presets are the providers the app offers, in menu order.
var Presets = []Preset{
	{
		ID:           ProviderR2,
		Label:        "Cloudflare R2",
		EndpointHint: "https://<account-id>.r2.cloudflarestorage.com",
		Region:       "auto",
		Note:         "Create an S3 API token in the R2 dashboard; the region is auto. For a bucket-scoped token, enter the bucket below.",
	},
	{
		ID:              ProviderAWS,
		Label:           "Amazon S3",
		EndpointHint:    "https://s3.us-east-1.amazonaws.com",
		EndpointPattern: "https://s3.{region}.amazonaws.com",
		Region:          "us-east-1",
		RegionHint:      "as in us-east-1, eu-west-2",
		Note:            "An IAM user's access key works. The region above is the bucket's own, and the endpoint follows it.",
	},
	{
		ID:              ProviderB2,
		Label:           "Backblaze B2",
		EndpointHint:    "https://s3.us-west-004.backblazeb2.com",
		EndpointPattern: "https://s3.{region}.backblazeb2.com",
		Region:          "us-west-004",
		RegionHint:      "as in us-west-004",
		Note:            "Both the endpoint and the region are on the bucket's B2 details page, under its S3-compatible address.",
	},
	{
		ID:              ProviderWasabi,
		Label:           "Wasabi",
		EndpointHint:    "https://s3.us-east-1.wasabisys.com",
		EndpointPattern: "https://s3.{region}.wasabisys.com",
		Region:          "us-east-1",
		RegionHint:      "as in us-east-1, eu-central-1",
		Note:            "The region is the one in the endpoint's host name; Wasabi's keys are plain access and secret keys.",
	},
	{
		ID:              ProviderOSS,
		Label:           "Alibaba Cloud OSS",
		EndpointHint:    "https://s3.oss-cn-hangzhou.aliyuncs.com",
		EndpointPattern: "https://s3.{region}.aliyuncs.com",
		Region:          "oss-cn-hangzhou",
		RegionHint:      "as in oss-cn-hangzhou",
		Note:            "Use the S3-compatible endpoint, s3.oss-{region}.aliyuncs.com; the region keeps its oss- prefix. A RAM user's access key works.",
	},
	{
		ID:              ProviderCOS,
		Label:           "Tencent Cloud COS",
		EndpointHint:    "https://cos.ap-guangzhou.myqcloud.com",
		EndpointPattern: "https://cos.{region}.myqcloud.com",
		Region:          "ap-guangzhou",
		RegionHint:      "as in ap-guangzhou",
		Note:            "The region is the bucket's, as in ap-guangzhou, and the endpoint follows it. Use a CAM user's SecretId and SecretKey; include the APPID suffix in the bucket name.",
	},
	{
		ID:           ProviderOther,
		Label:        "Other S3-compatible",
		EndpointHint: "https://storage.example.com",
		Region:       "us-east-1",
		Note:         "Any endpoint that speaks the S3 API. Turn on path-style requests if the service asks for them.",
	},
}

// Legacy profiles remain editable without being offered for new connections.
var legacyMinIOPreset = Preset{
	ID: ProviderMinIO, Label: "MinIO", EndpointHint: "http://localhost:9000",
	Region: "us-east-1", PathStyle: true,
	Note: "Self-hosted MinIO answers path-style requests, so the path-style switch below is already on.",
}

var legacySpacesPreset = Preset{
	ID: ProviderSpaces, Label: "DigitalOcean Spaces",
	EndpointHint: "https://nyc3.digitaloceanspaces.com", Region: "nyc3",
	Note: "The region is the endpoint's first label, as in nyc3.",
}

// PresetFor returns the preset of provider, and whether one exists.
func PresetFor(p Provider) (Preset, bool) {
	if p == ProviderMinIO {
		return legacyMinIOPreset, true
	}
	if p == ProviderSpaces {
		return legacySpacesPreset, true
	}
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

// SecretStore keeps small secrets, such as access keys, outside profile metadata.
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
	if in.Region == "" {
		in.Region = preset.Region
	}
	if in.Endpoint == "" {
		// The services whose host carries the region make the endpoint
		// from it; the others have one fixed host, or a template the
		// person fills in because it carries their account.
		if derived := preset.EndpointFor(in.Region); derived != "" {
			in.Endpoint = derived
		} else if !strings.Contains(preset.EndpointHint, "<") {
			in.Endpoint = preset.EndpointHint
		}
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
	// Deleting stale credentials must not undo removal of the profile.
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
