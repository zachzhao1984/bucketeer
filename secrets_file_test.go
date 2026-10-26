package main

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"bucketeer/internal/store"
)

func TestStartupUsesLocalCredentialsOnly(t *testing.T) {
	dir := t.TempDir()
	svc, err := newServiceIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	profiles, err := svc.ListConnections()
	if err != nil {
		t.Fatal(err)
	}
	if len(profiles) != 0 || svc.Connected() {
		t.Fatal("a fresh installation must start without preloaded connections")
	}

	profile, err := svc.SaveConnection("", store.Input{
		Name: "Local test", Provider: "other", Endpoint: "https://s3.example.com",
		AccessKeyID: "test-access-id", SecretKey: "test-secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	token, err := svc.picgoToken()
	if err != nil {
		t.Fatal(err)
	}
	if token == "" {
		t.Fatal("the upload token is empty")
	}
	svc.Close()

	reopened, err := newServiceIn(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	secret, err := reopened.profiles.Secret(profile.ID)
	if err != nil || secret != "test-secret" {
		t.Fatalf("local profile key did not survive restart: %v", err)
	}
	stored, err := reopened.readSecret(picgoSecretKey)
	if err != nil || string(stored) != token {
		t.Fatalf("local upload token did not survive restart: %v", err)
	}
}

// MyGo includes resources regardless of .gitignore. Treat this directory
// as a public-asset allowlist so a private test config cannot silently ship.
func TestPackagedResourcesArePublicAssets(t *testing.T) {
	allowed := map[string]bool{
		"icon.png":       true,
		"icon Small.png": true,
	}
	err := filepath.WalkDir("resources", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), ".") {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel("resources", path)
		if err != nil {
			return err
		}
		if !allowed[rel] || entry.Type()&fs.ModeSymlink != 0 {
			t.Errorf("unreviewed resource would be bundled: %s; keep local credentials outside resources", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
