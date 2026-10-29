package store

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeSecrets is a credential store that keeps secrets in memory, and can
// be told to fail.
type fakeSecrets struct {
	mu      sync.Mutex
	values  map[string][]byte
	setErr  error
	getErr  error
	deleted []string
}

func newFakeSecrets() *fakeSecrets {
	return &fakeSecrets{values: map[string][]byte{}}
}

func (f *fakeSecrets) Set(key string, value []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.setErr != nil {
		return f.setErr
	}
	f.values[key] = append([]byte(nil), value...)
	return nil
}

func (f *fakeSecrets) Get(key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.getErr != nil {
		return nil, f.getErr
	}
	v, ok := f.values[key]
	if !ok {
		return nil, errors.New("no such secret")
	}
	return append([]byte(nil), v...), nil
}

func (f *fakeSecrets) Delete(key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.deleted = append(f.deleted, key)
	delete(f.values, key)
	return nil
}

func (f *fakeSecrets) has(key string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	_, ok := f.values[key]
	return ok
}

func newTestStore(t *testing.T) (*ProfileStore, *fakeSecrets, string) {
	t.Helper()
	dir := t.TempDir()
	secrets := newFakeSecrets()
	path := filepath.Join(dir, "connections.json")
	return NewProfileStore(path, secrets), secrets, path
}

func testInput() Input {
	return Input{
		Name:        "Acme Media",
		Provider:    string(ProviderR2),
		Endpoint:    "https://abc123.r2.cloudflarestorage.com",
		Region:      "auto",
		AccessKeyID: "AKIAEXAMPLE",
		SecretKey:   "s3cr3t-value",
	}
}

func TestCreateProfileStoresSecretApartFromFile(t *testing.T) {
	s, secrets, path := newTestStore(t)

	p, err := s.Create(testInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("Create returned an empty id")
	}
	if !p.HasSecret {
		t.Error("HasSecret is false after creating a profile with a secret")
	}

	// The secret lives in the credential store, not in the file.
	if got := string(secrets.values[p.SecretKey()]); got != "s3cr3t-value" {
		t.Errorf("stored secret = %q, want %q", got, "s3cr3t-value")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read connections file: %v", err)
	}
	if strings.Contains(string(raw), "s3cr3t-value") {
		t.Errorf("the connections file holds the secret access key:\n%s", raw)
	}

	// A second store reading the same file sees the profile.
	other := NewProfileStore(path, secrets)
	list, err := other.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 || list[0].Name != "Acme Media" {
		t.Fatalf("List = %+v, want one profile named Acme Media", list)
	}

	got, err := other.Secret(p.ID)
	if err != nil {
		t.Fatalf("Secret: %v", err)
	}
	if got != "s3cr3t-value" {
		t.Errorf("Secret = %q, want %q", got, "s3cr3t-value")
	}
}

func TestCreateFillsPresetDefaults(t *testing.T) {
	s, _, _ := newTestStore(t)

	p, err := s.Create(Input{
		Name:        "Local MinIO",
		Provider:    string(ProviderMinIO),
		AccessKeyID: "minioadmin",
		SecretKey:   "minioadmin",
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if p.Endpoint != "http://localhost:9000" {
		t.Errorf("endpoint = %q, want the MinIO preset's", p.Endpoint)
	}
	if p.Region != "us-east-1" {
		t.Errorf("region = %q, want the MinIO preset's", p.Region)
	}
}

func TestCreateKeepsAnExplicitPathStyleChoice(t *testing.T) {
	s, _, _ := newTestStore(t)

	// "Other" does not default to path style, but the user asked for it.
	p, err := s.Create(Input{
		Name:        "Custom",
		Provider:    string(ProviderOther),
		Endpoint:    "https://storage.example.com",
		Region:      "us-east-1",
		AccessKeyID: "key",
		SecretKey:   "secret",
		PathStyle:   true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !p.PathStyle {
		t.Error("PathStyle was not kept")
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	cases := []struct {
		name  string
		mut   func(*Input)
		wants string
	}{
		{"no name", func(in *Input) { in.Name = "  " }, "name"},
		{"no endpoint", func(in *Input) { in.Endpoint = "" }, "endpoint"},
		{"endpoint without scheme", func(in *Input) { in.Endpoint = "example.com" }, "scheme"},
		{"no access key", func(in *Input) { in.AccessKeyID = "" }, "access key ID"},
		{"no secret", func(in *Input) { in.SecretKey = "" }, "secret access key"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, _, _ := newTestStore(t)
			in := testInput()
			tc.mut(&in)
			_, err := s.Create(in)
			if err == nil {
				t.Fatal("Create accepted invalid input")
			}
			if !strings.Contains(err.Error(), tc.wants) {
				t.Errorf("error = %q, want it to mention %q", err, tc.wants)
			}
			// Nothing should have been written.
			list, err := s.List()
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(list) != 0 {
				t.Errorf("a rejected profile was saved: %+v", list)
			}
		})
	}
}

func TestCreateRollsBackWhenTheSecretCannotBeStored(t *testing.T) {
	s, secrets, path := newTestStore(t)
	secrets.setErr = errors.New("keychain is locked")

	if _, err := s.Create(testInput()); err == nil {
		t.Fatal("Create succeeded although the secret could not be stored")
	}

	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("a profile was saved without its secret: %+v", list)
	}
	if _, err := os.Stat(path); err == nil {
		t.Error("the connections file was written although the profile was not created")
	}
}

func TestUpdateKeepsTheSecretUnlessANewOneIsGiven(t *testing.T) {
	s, secrets, _ := newTestStore(t)
	p, err := s.Create(testInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// An update without a secret keeps the stored one.
	in := testInput()
	in.Name = "Acme Media (renamed)"
	in.SecretKey = ""
	updated, err := s.Update(p.ID, in)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Acme Media (renamed)" {
		t.Errorf("name = %q, want the new one", updated.Name)
	}
	if !updated.HasSecret {
		t.Error("HasSecret is false after an update that kept the secret")
	}
	if got := string(secrets.values[p.SecretKey()]); got != "s3cr3t-value" {
		t.Errorf("secret = %q, want it unchanged", got)
	}

	// An update with a secret replaces it.
	in.SecretKey = "rotated-value"
	if _, err := s.Update(p.ID, in); err != nil {
		t.Fatalf("Update with a new secret: %v", err)
	}
	if got := string(secrets.values[p.SecretKey()]); got != "rotated-value" {
		t.Errorf("secret = %q, want the rotated one", got)
	}
}

func TestUpdateUnknownProfile(t *testing.T) {
	s, _, _ := newTestStore(t)
	_, err := s.Update("nope", testInput())
	if !errors.Is(err, ErrProfileNotFound) {
		t.Fatalf("Update = %v, want ErrProfileNotFound", err)
	}
}

func TestDeleteForgetsTheProfileAndItsSecret(t *testing.T) {
	s, secrets, _ := newTestStore(t)
	p, err := s.Create(testInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Delete(p.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List = %+v, want it empty", list)
	}
	if secrets.has(p.SecretKey()) {
		t.Error("the secret was left in the credential store")
	}
	if !slicesContains(secrets.deleted, p.SecretKey()) {
		t.Errorf("Delete did not ask the credential store to forget %q", p.SecretKey())
	}

	if err := s.Delete(p.ID); !errors.Is(err, ErrProfileNotFound) {
		t.Errorf("second Delete = %v, want ErrProfileNotFound", err)
	}
}

func slicesContains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

func TestListOnAMissingFileIsEmpty(t *testing.T) {
	s, _, _ := newTestStore(t)
	list, err := s.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("List = %+v, want it empty", list)
	}
}

func TestListReportsACorruptFile(t *testing.T) {
	s, _, path := newTestStore(t)
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.List(); err == nil {
		t.Fatal("List accepted a corrupt connections file")
	}
}

func TestProfilesSurviveAReopen(t *testing.T) {
	s, secrets, path := newTestStore(t)
	first, err := s.Create(testInput())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	in := testInput()
	in.Name = "Second"
	in.Endpoint = "https://second.example.com"
	if _, err := s.Create(in); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	reopened := NewProfileStore(path, secrets)
	list, err := reopened.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("List returned %d profiles, want 2", len(list))
	}
	if list[0].ID != first.ID {
		t.Errorf("the profiles changed order: first is %q, want %q", list[0].ID, first.ID)
	}
}

func TestProfileDisplayHelpers(t *testing.T) {
	p := Profile{Name: "Acme Media", Endpoint: "https://abc123.r2.cloudflarestorage.com/"}
	if got := p.Initials(); got != "AM" {
		t.Errorf("Initials = %q, want %q", got, "AM")
	}
	if got := p.Host(); got != "abc123.r2.cloudflarestorage.com" {
		t.Errorf("Host = %q", got)
	}
	if got := (Profile{Name: "backups"}).Initials(); got != "BA" {
		t.Errorf("Initials = %q, want %q", got, "BA")
	}
	if got := (Profile{}).Initials(); got != "?" {
		t.Errorf("Initials of a nameless profile = %q, want %q", got, "?")
	}
}

func TestSecretKeyIsUniquePerProfile(t *testing.T) {
	a := Profile{ID: "aaa"}
	b := Profile{ID: "bbb"}
	if a.SecretKey() == b.SecretKey() {
		t.Fatal("two profiles share a credential-store key")
	}
}

// TestPresetsAreTheOfferedServices pins the list the connection form
// shows, in its order, and the endpoint each pattern makes.
func TestPresetsAreTheOfferedServices(t *testing.T) {
	want := []struct {
		id    Provider
		label string
	}{
		{ProviderR2, "Cloudflare R2"},
		{ProviderAWS, "Amazon S3"},
		{ProviderB2, "Backblaze B2"},
		{ProviderWasabi, "Wasabi"},
		{ProviderOSS, "Alibaba Cloud OSS"},
		{ProviderCOS, "Tencent Cloud COS"},
		{ProviderOther, "Other S3-compatible"},
	}
	if len(Presets) != len(want) {
		t.Fatalf("%d presets, want %d", len(Presets), len(want))
	}
	for i, w := range want {
		if Presets[i].ID != w.id || Presets[i].Label != w.label {
			t.Errorf("preset %d = %q (%q), want %q (%q)", i, Presets[i].Label, Presets[i].ID, w.label, w.id)
		}
	}
}

// TestPresetEndpoints pins what each service's own documentation says the
// endpoint and region look like.
func TestPresetEndpoints(t *testing.T) {
	cases := []struct {
		provider Provider
		region   string
		endpoint string
	}{
		{ProviderAWS, "eu-west-2", "https://s3.eu-west-2.amazonaws.com"},
		{ProviderB2, "us-west-004", "https://s3.us-west-004.backblazeb2.com"},
		{ProviderWasabi, "eu-central-1", "https://s3.eu-central-1.wasabisys.com"},
		{ProviderOSS, "oss-cn-hangzhou", "https://s3.oss-cn-hangzhou.aliyuncs.com"},
		{ProviderCOS, "ap-guangzhou", "https://cos.ap-guangzhou.myqcloud.com"},
	}
	for _, c := range cases {
		preset, ok := PresetFor(c.provider)
		if !ok {
			t.Fatalf("no preset for %q", c.provider)
		}
		if got := preset.EndpointFor(c.region); got != c.endpoint {
			t.Errorf("%s in %s = %q, want %q", c.provider, c.region, got, c.endpoint)
		}
		// The default region must make the default endpoint, or the form
		// would show one and save another.
		if got := preset.EndpointFor(preset.Region); got != preset.EndpointHint {
			t.Errorf("%s default endpoint = %q, want its hint %q", c.provider, got, preset.EndpointHint)
		}
	}
	// The services whose host carries the account have no pattern: their
	// endpoint is typed, not derived.
	for _, p := range []Provider{ProviderR2, ProviderMinIO, ProviderOther} {
		preset, _ := PresetFor(p)
		if preset.EndpointPattern != "" {
			t.Errorf("%q should have no endpoint pattern", p)
		}
	}
}

// TestCreateDerivesTheEndpointFromTheRegion checks that a person who types
// only a region gets a working endpoint, which is what the form promises
// for the services whose host carries it.
func TestCreateDerivesTheEndpointFromTheRegion(t *testing.T) {
	store, _, _ := newTestStore(t)
	profile, err := store.Create(Input{
		Name:        "Tencent",
		Provider:    string(ProviderCOS),
		Region:      "ap-beijing",
		AccessKeyID: "AKIDEXAMPLE",
		SecretKey:   "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://cos.ap-beijing.myqcloud.com"; profile.Endpoint != want {
		t.Errorf("endpoint = %q, want %q", profile.Endpoint, want)
	}
}

func TestLegacySpacesProfileRemainsEditable(t *testing.T) {
	profiles, _, _ := newTestStore(t)
	original, err := profiles.Create(Input{
		Name: "Old Spaces", Provider: string(ProviderSpaces),
		Endpoint: "https://sfo3.digitaloceanspaces.com", Region: "sfo3",
		AccessKeyID: "example", SecretKey: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := profiles.Update(original.ID, Input{
		Name: "Renamed Spaces", Provider: string(original.Provider),
		Endpoint: original.Endpoint, Region: original.Region,
		AccessKeyID: original.AccessKeyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Provider != ProviderSpaces || updated.Endpoint != original.Endpoint || updated.Region != original.Region {
		t.Fatalf("legacy connection changed: provider=%q endpoint=%q region=%q", updated.Provider, updated.Endpoint, updated.Region)
	}
}

func TestPresetForEveryProvider(t *testing.T) {
	for _, preset := range Presets {
		got, ok := PresetFor(preset.ID)
		if !ok {
			t.Errorf("PresetFor(%q) reported no preset", preset.ID)
			continue
		}
		if got.Label != preset.Label {
			t.Errorf("PresetFor(%q).Label = %q, want %q", preset.ID, got.Label, preset.Label)
		}
		if preset.Label == "" {
			t.Errorf("preset %q has no label", preset.ID)
		}
	}
	if _, ok := PresetFor("nonesuch"); ok {
		t.Error("PresetFor accepted an unknown provider")
	}
}
