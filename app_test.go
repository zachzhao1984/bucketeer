package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo"

	"bucketeer/internal/store"
)

// memSecrets is a credential store that keeps secrets in memory, so the
// service can be tested without a system keychain.
type memSecrets struct {
	mu     sync.Mutex
	values map[string][]byte
}

func (m *memSecrets) Set(key string, value []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.values == nil {
		m.values = map[string][]byte{}
	}
	m.values[key] = append([]byte(nil), value...)
	return nil
}

func (m *memSecrets) Get(key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.values[key]
	if !ok {
		return nil, errors.New("no such secret")
	}
	return append([]byte(nil), v...), nil
}

func (m *memSecrets) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.values, key)
	return nil
}

// newTestService returns a service rooted at temporary directories.
func newTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	service, err := newServiceAt(dir, &memSecrets{})
	if err != nil {
		t.Fatalf("newServiceAt: %v", err)
	}
	t.Cleanup(service.Close)
	return service
}

// testInput is the connection the integration tests use.
func testInput() store.Input {
	return store.Input{
		Name:        "Integration",
		Provider:    string(store.ProviderMinIO),
		Endpoint:    os.Getenv("S3_ENDPOINT"),
		Region:      "us-east-1",
		AccessKeyID: os.Getenv("S3_KEY"),
		SecretKey:   os.Getenv("S3_SECRET"),
		PathStyle:   true,
	}
}

// localInput is a complete, non-empty connection for tests that do not
// talk to a server.
func localInput() store.Input {
	return store.Input{
		Name:        "Local",
		Provider:    string(store.ProviderMinIO),
		Endpoint:    "http://localhost:9000",
		Region:      "us-east-1",
		AccessKeyID: "minioadmin",
		SecretKey:   "minioadmin",
		PathStyle:   true,
	}
}

// requireEndpoint skips a test that needs a real S3 server.
func requireEndpoint(t *testing.T) {
	t.Helper()
	if os.Getenv("S3_ENDPOINT") == "" {
		t.Skip("set S3_ENDPOINT to run the service integration tests")
	}
}

func TestServiceSettingsRoundTrip(t *testing.T) {
	service := newTestService(t)

	// The defaults are sensible before anything is saved.
	got := service.GetSettings()
	if got.ViewMode != "grid" || !got.ConfirmDelete || got.PresignTTLHours != 24 {
		t.Errorf("default settings = %+v, want the documented defaults", got)
	}

	// A bad view mode and an out-of-range lifetime are corrected.
	if err := service.SetSettings(Settings{ViewMode: "diagonal", PresignTTLHours: -5, ConfirmDelete: false}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	got = service.GetSettings()
	if got.ViewMode != "grid" {
		t.Errorf("ViewMode = %q, want it corrected to grid", got.ViewMode)
	}
	if got.PresignTTLHours != 24 {
		t.Errorf("PresignTTLHours = %d, want it corrected to 24", got.PresignTTLHours)
	}
	if got.ConfirmDelete {
		t.Error("ConfirmDelete was not saved as false")
	}

	// And it survives a restart of the service.
	dir := t.TempDir()
	first, err := newServiceAt(dir, &memSecrets{})
	if err != nil {
		t.Fatalf("newServiceAt: %v", err)
	}
	defer first.Close()
	if err := first.SetSettings(Settings{ViewMode: "list", ConfirmDelete: true, PresignTTLHours: 72}); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	second, err := newServiceAt(dir, &memSecrets{})
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer second.Close()
	got = second.GetSettings()
	if got.ViewMode != "list" || got.PresignTTLHours != 72 {
		t.Errorf("settings after reopen = %+v, want the saved ones", got)
	}
}

func TestServiceWithoutAConnection(t *testing.T) {
	service := newTestService(t)

	if service.CurrentConnection().Connected {
		t.Error("a fresh service reports itself connected")
	}
	ctx := context.Background()

	// Every storage call explains that nothing is connected, rather than
	// panicking on a nil session.
	if _, err := service.ListBuckets(ctx); err == nil {
		t.Error("ListBuckets succeeded without a connection")
	}
	if _, err := service.ListObjects(ctx, "b", ""); err == nil {
		t.Error("ListObjects succeeded without a connection")
	}
	if _, err := service.DeleteObjects(ctx, "b", []string{"k"}); err == nil {
		t.Error("DeleteObjects succeeded without a connection")
	}
	if _, err := service.PresignObject(ctx, "b", "k", 1); err == nil {
		t.Error("PresignObject succeeded without a connection")
	}

	// The listing helpers return empty slices, not nil, so the frontend
	// never has to guard against null.
	if list := service.ListTransfers(); list == nil {
		t.Error("ListTransfers returned nil")
	}
	if list, err := service.ListConnections(); err != nil || list == nil {
		t.Errorf("ListConnections = %v, %v; want an empty slice", list, err)
	}
	if got := service.Providers(); len(got) == 0 {
		t.Error("Providers returned nothing")
	}
}

func TestServiceConnectionLifecycle(t *testing.T) {
	service := newTestService(t)

	// Saving validates the input.
	if _, err := service.SaveConnection("", store.Input{Name: ""}); err == nil {
		t.Error("SaveConnection accepted a nameless connection")
	}

	profile, err := service.SaveConnection("", localInput())
	if err != nil {
		t.Fatalf("SaveConnection: %v", err)
	}
	if profile.ID == "" {
		t.Error("the saved connection has no id")
	}

	list, err := service.ListConnections()
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(list) != 1 || list[0].ID != profile.ID {
		t.Fatalf("ListConnections = %+v, want the saved connection", list)
	}

	// An update keeps the stored secret when the form sends none.
	updated, err := service.SaveConnection(profile.ID, store.Input{
		Name:        "Renamed",
		Provider:    string(store.ProviderMinIO),
		Endpoint:    "http://localhost:9000",
		Region:      "us-east-1",
		AccessKeyID: "key",
		PathStyle:   true,
	})
	if err != nil {
		t.Fatalf("SaveConnection update: %v", err)
	}
	if updated.Name != "Renamed" || !updated.HasSecret {
		t.Errorf("updated connection = %+v, want a new name and its secret", updated)
	}

	if err := service.DeleteConnection(profile.ID); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	if list, _ := service.ListConnections(); len(list) != 0 {
		t.Errorf("ListConnections = %+v after deleting, want empty", list)
	}
	if err := service.DeleteConnection(profile.ID); err == nil {
		t.Error("deleting an unknown connection succeeded")
	}
}

// TestServiceTestConnectionReportsFailureAsAResult checks that an
// unreachable endpoint is reported to the form, not raised as an error:
// the form shows the message, it does not crash.
func TestServiceTestConnectionReportsFailureAsAResult(t *testing.T) {
	service := newTestService(t)

	// A port nothing is listening on.
	input := localInput()
	input.Endpoint = "http://127.0.0.1:1"
	result, err := service.TestConnection(input)
	if err != nil {
		t.Fatalf("TestConnection returned an error instead of a result: %v", err)
	}
	if result.OK {
		t.Error("TestConnection reported success against a dead endpoint")
	}
	if result.Message == "" {
		t.Error("TestConnection reported no message for the failure")
	}

	// An incomplete form is a real error, since nothing was attempted.
	if _, err := service.TestConnection(store.Input{Name: "x"}); err == nil {
		t.Error("TestConnection accepted an empty endpoint")
	}
}

// TestServiceAgainstStorage drives the calls the window makes against a
// real S3-compatible endpoint.
func TestServiceAgainstStorage(t *testing.T) {
	requireEndpoint(t)
	service := newTestService(t)
	ctx := context.Background()

	// The connection form's Test button.
	result, err := service.TestConnection(testInput())
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if !result.OK {
		t.Fatalf("TestConnection failed: %s", result.Message)
	}
	if !strings.Contains(result.Message, "bucket") {
		t.Errorf("message = %q, want it to mention buckets", result.Message)
	}

	// Saving and connecting.
	profile, err := service.SaveConnection("", testInput())
	if err != nil {
		t.Fatalf("SaveConnection: %v", err)
	}
	info, err := service.Connect(ctx, profile.ID)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if !info.Connected || info.Profile == nil || info.Host == "" {
		t.Fatalf("Connect returned %+v", info)
	}

	bucket := "bucketeer-svc-" + strings.ToLower(profile.ID)
	if err := service.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	defer func() {
		if listing, err := service.SearchObjects(ctx, bucket, "", "", 10_000); err == nil {
			keys := make([]string, 0, len(listing.Items))
			for _, item := range listing.Items {
				keys = append(keys, item.Key)
			}
			if len(keys) > 0 {
				_, _ = service.DeleteObjects(ctx, bucket, keys)
			}
		}
		_ = service.DeleteBucket(ctx, bucket)
	}()

	if _, err := service.CreateFolder(ctx, bucket, "", "images"); err != nil {
		t.Fatalf("CreateFolder: %v", err)
	}

	// Uploading through the queue, then waiting for it to finish.
	src := filepath.Join(t.TempDir(), "photo.png")
	if err := os.WriteFile(src, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	queued, err := service.UploadPaths(ctx, bucket, "images", []string{src})
	if err != nil {
		t.Fatalf("UploadPaths: %v", err)
	}
	if queued != 1 {
		t.Fatalf("UploadPaths queued %d, want 1", queued)
	}
	waitForTransfers(t, service)

	listing, err := service.ListObjects(ctx, bucket, "images/")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	if listing.Files != 1 {
		t.Fatalf("images/ holds %d files, want 1", listing.Files)
	}

	// Metadata, a share link, and the text preview path.
	obj, err := service.ObjectInfo(ctx, bucket, "images/photo.png")
	if err != nil {
		t.Fatalf("ObjectInfo: %v", err)
	}
	if obj.Size != 2048 {
		t.Errorf("size = %d, want 2048", obj.Size)
	}
	url, err := service.PresignObject(ctx, bucket, "images/photo.png", 1)
	if err != nil {
		t.Fatalf("PresignObject: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("share link has no signature: %s", url)
	}
	if data, err := service.ReadImage(ctx, bucket, "images/photo.png"); err != nil || len(data) != 2048 {
		t.Errorf("ReadImage: bytes=%d err=%v", len(data), err)
	}
	if preview, err := service.ReadText(ctx, bucket, "images/photo.png"); err != nil || !preview.Binary {
		t.Errorf("ReadText should recognize the binary fixture: %+v, %v", preview, err)
	}

	// The dashboard summary.
	stat, err := service.BucketStat(ctx, bucket, 0)
	if err != nil {
		t.Fatalf("BucketStat: %v", err)
	}
	if stat.Objects != 1 {
		t.Errorf("stat counted %d objects, want 1", stat.Objects)
	}

	// Renaming and moving.
	if err := service.RenameObject(ctx, bucket, "images/photo.png", "renamed.png"); err != nil {
		t.Fatalf("RenameObject: %v", err)
	}
	if moved, err := service.MoveObjects(ctx, bucket, []string{"images/renamed.png"}, ""); err != nil || moved != 1 {
		t.Fatalf("MoveObjects = %d, %v; want 1", moved, err)
	}
	root, err := service.ListObjects(ctx, bucket, "")
	if err != nil {
		t.Fatalf("ListObjects root: %v", err)
	}
	if root.Files != 1 {
		t.Errorf("the root holds %d files after the move, want 1", root.Files)
	}

	// Deleting, and the bucket.
	if deleted, err := service.DeleteObjects(ctx, bucket, []string{"renamed.png"}); err != nil || deleted != 1 {
		t.Fatalf("DeleteObjects = %d, %v; want 1", deleted, err)
	}

	// The images/ folder marker is still there, so the bucket is not
	// empty and the server refuses to remove it.
	if err := service.DeleteBucket(ctx, bucket); err == nil {
		t.Error("DeleteBucket removed a bucket that still held an object")
	}

	// Deleting the folder takes its marker with it, and then the bucket
	// goes.
	if deleted, err := service.DeleteObjects(ctx, bucket, []string{"images/"}); err != nil || deleted != 1 {
		t.Fatalf("DeleteObjects(images/) = %d, %v; want 1", deleted, err)
	}
	if err := service.DeleteBucket(ctx, bucket); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
}

// waitForTransfers blocks until the service has no transfers running.
func waitForTransfers(t *testing.T, service *Service) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		transfers := service.ListTransfers()
		active := 0
		for _, transfer := range transfers {
			switch transfer.State {
			case "done":
			case "failed", "canceled":
				t.Fatalf("transfer %s of %s: %s", transfer.State, transfer.Name, transfer.Error)
			default:
				active++
			}
		}
		if active == 0 && len(transfers) > 0 {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("timed out waiting for the transfers to finish")
}

func TestR2ConnectionWithFile(t *testing.T) {
	b, err := os.ReadFile("r2.json")
	if err != nil {
		t.Skip("r2.json not found")
	}
	var cfg struct {
		Endpoint        string `json:"Endpoint"`
		Bucket          string `json:"Bucket"`
		AccessKeyID     string `json:"Access Key ID"`
		SecretAccessKey string `json:"Secret Access Key"`
	}
	if err := json.Unmarshal(b, &cfg); err != nil {
		t.Fatalf("unmarshal r2.json: %v", err)
	}
	if cfg.Endpoint == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" {
		t.Skip("incomplete r2.json")
	}

	service := newTestService(t)

	// 1. Without bucket, TestConnection should explain bucket scope
	noBucket := store.Input{
		Name:        "R2 No Bucket",
		Provider:    string(store.ProviderR2),
		Endpoint:    cfg.Endpoint,
		Region:      "auto",
		AccessKeyID: cfg.AccessKeyID,
		SecretKey:   cfg.SecretAccessKey,
	}
	resNoBucket, err := service.TestConnection(noBucket)
	if err != nil {
		t.Fatalf("TestConnection unexpected err: %v", err)
	}
	if resNoBucket.OK {
		t.Log("Note: token unexpectedly had account-level ListBuckets permission")
	} else if !strings.Contains(strings.ToLower(resNoBucket.Message), "bucket") {
		t.Errorf("expected guidance on bucket field, got: %s", resNoBucket.Message)
	}

	// 2. With bucket, TestConnection should succeed
	withBucket := noBucket
	withBucket.Name = "R2 With Bucket"
	withBucket.Bucket = cfg.Bucket
	resWithBucket, err := service.TestConnection(withBucket)
	if err != nil {
		t.Fatalf("TestConnection with bucket err: %v", err)
	}
	if !resWithBucket.OK {
		t.Fatalf("TestConnection with bucket failed: %s", resWithBucket.Message)
	}
	t.Logf("TestConnection with bucket succeeded: %s", resWithBucket.Message)

	// 3. Save connection and Connect
	p, err := service.SaveConnection("", withBucket)
	if err != nil {
		t.Fatalf("SaveConnection: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	info, err := service.Connect(ctx, p.ID)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	if !info.Connected {
		t.Fatal("session reports not connected")
	}

	// 4. ListBuckets returns the configured bucket
	buckets, err := service.ListBuckets(ctx)
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if len(buckets) == 0 || buckets[0].Name != cfg.Bucket {
		t.Fatalf("ListBuckets want [%s], got %+v", cfg.Bucket, buckets)
	}

	// 5. ListObjects on the bucket succeeds
	listing, err := service.ListObjects(ctx, cfg.Bucket, "")
	if err != nil {
		t.Fatalf("ListObjects: %v", err)
	}
	t.Logf("ListObjects on %s: %d items", cfg.Bucket, len(listing.Items))
}

func TestSeedR2Connection(t *testing.T) {
	// Only meaningful when the app ships an r2.json.
	dir, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		t.Skip("no resources path")
	}
	if _, err := os.Stat(filepath.Join(dir, "r2.json")); err != nil {
		t.Skip("r2.json not shipped")
	}

	service := newTestService(t)
	if !service.seedR2Connection() {
		t.Fatal("seedR2Connection reported nothing was seeded")
	}

	profile := service.r2Profile()
	if profile == nil {
		t.Fatal("r2Profile found nothing after seeding")
	}
	if profile.Provider != store.ProviderR2 {
		t.Errorf("provider = %s, want r2", profile.Provider)
	}
	if profile.Bucket == "" {
		t.Error("seeded profile has no bucket")
	}
	if !profile.HasSecret {
		t.Error("seeded profile has no secret")
	}

	// Idempotent: seeding again neither errors nor duplicates.
	if service.seedR2Connection() {
		t.Fatal("seedR2Connection ran twice")
	}
	list, err := service.ListConnections()
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	count := 0
	for _, p := range list {
		if p.Endpoint == profile.Endpoint && p.AccessKeyID == profile.AccessKeyID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("seeded connection appears %d times, want 1", count)
	}
}

// TestSettingsMigrateAppearance checks that a settings file from before the
// three-way appearance choice still reads: a pinned dark keeps its pin, and
// everything else follows the system.
func TestSettingsMigrateAppearance(t *testing.T) {
	dir := t.TempDir()
	old := `{"viewMode":"grid","showThumbnails":true,"confirmDelete":true,"presignTtlHours":24,"darkMode":true}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	svc, err := newServiceAt(dir, &memSecrets{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	if got := svc.GetSettings().Appearance; got != "dark" {
		t.Fatalf("a pinned dark file migrated to %q, want dark", got)
	}
	svc.Close()

	plain := `{"viewMode":"grid","showThumbnails":true,"confirmDelete":true,"presignTtlHours":24,"darkMode":false}`
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(plain), 0o600); err != nil {
		t.Fatal(err)
	}
	svc2, err := newServiceAt(dir, &memSecrets{})
	if err != nil {
		t.Fatal(err)
	}
	defer svc2.Close()
	if got := svc2.GetSettings().Appearance; got != "system" {
		t.Fatalf("a light file migrated to %q, want system", got)
	}
}

// TestFileSecretsRoundTrip checks the file credential store: set, get,
// delete, persistence across instances, and owner-only permissions.
func TestFileSecretsRoundTrip(t *testing.T) {
	dir := t.TempDir()
	secrets := newFileSecrets(dir)

	if _, err := secrets.Get("profile/x/secret"); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("a missing key returned %v, want not-found", err)
	}
	if err := secrets.Set("profile/x/secret", []byte("wNHc1td")); err != nil {
		t.Fatal(err)
	}
	if err := secrets.Set("picgo-http-token", []byte("tok-9")); err != nil {
		t.Fatal(err)
	}

	// A second store over the same directory sees what the first wrote.
	again := newFileSecrets(dir)
	if got, err := again.Get("profile/x/secret"); err != nil || string(got) != "wNHc1td" {
		t.Fatalf("Get = %q, %v", got, err)
	}

	info, err := os.Stat(filepath.Join(dir, "secrets.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("secrets.json mode = %v, want 0600", perm)
	}

	if err := again.Delete("profile/x/secret"); err != nil {
		t.Fatal(err)
	}
	if _, err := again.Get("profile/x/secret"); !errors.Is(err, errSecretNotFound) {
		t.Fatalf("after Delete, Get returned %v", err)
	}
}

// TestKeychainImport moves the credentials an older build kept in the
// keychain into the file store and clears the keychain entries.
func TestKeychainImport(t *testing.T) {
	old := legacySecrets
	t.Cleanup(func() { legacySecrets = old })
	keychain := &memSecrets{values: map[string][]byte{
		"profile/definitely-not-a-real-id/secret": []byte("stale"),
		"picgo-http-token":                        []byte("tok-9"),
	}}
	legacySecrets = keychain

	dir := t.TempDir()
	svc, err := newServiceAt(dir, newFileSecrets(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	// The connection exists before the import: only live profiles' keys
	// are imported, and the secret key carries this profile's id. An old
	// build wrote this key to the keychain, not to the file, so the file
	// copy is taken back out before the import runs.
	p, err := svc.profiles.Create(store.Input{
		Name: "Acme", Provider: "other", Endpoint: "https://s3.example.com",
		AccessKeyID: "AKIA", SecretKey: "wNHc1td",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.secrets.Delete(p.SecretKey()); err != nil {
		t.Fatal(err)
	}
	if err := keychain.Set(p.SecretKey(), []byte("wNHc1td")); err != nil {
		t.Fatal(err)
	}
	svc.migrateKeychain()

	// Both live keys are in the file and gone from the keychain; the
	// keychain entry of a profile that no longer exists is left behind —
	// deleting an unknown profile's key is not this store's business.
	for _, key := range []string{p.SecretKey(), "picgo-http-token"} {
		if got, err := svc.secrets.Get(key); err != nil {
			t.Fatalf("%s did not import: %v", key, err)
		} else if len(got) == 0 {
			t.Fatalf("%s imported empty", key)
		}
		if _, err := keychain.Get(key); err == nil {
			t.Fatalf("%s is still in the keychain", key)
		}
	}
}
