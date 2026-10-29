package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMenuBarTargetPersists(t *testing.T) {
	s := newTestService(t)
	settings := s.GetSettings()
	settings.MenuBarUpload = MenuBarTarget{ProfileID: "saved-connection", Bucket: "images"}
	if err := s.SetSettings(settings); err != nil {
		t.Fatal(err)
	}
	reopened, err := newServiceAt(s.dataDir, &memSecrets{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got := reopened.GetSettings().MenuBarUpload; got != settings.MenuBarUpload {
		t.Fatalf("target = %+v", got)
	}
}

func TestMenuBarUploadRequiresSavedTarget(t *testing.T) {
	s := newTestService(t)
	for _, target := range []MenuBarTarget{{}, {ProfileID: "missing", Bucket: "images"}} {
		if n, err := s.UploadMenuBarPaths(context.Background(), target, []string{"file.png"}); err == nil || n != 0 {
			t.Fatalf("target %+v: queued=%d err=%v", target, n, err)
		}
	}
	if len(s.ListTransfers()) != 0 {
		t.Fatal("invalid target queued transfers")
	}
}

func TestMenuBarUploadUsesDefaultConnectionWithoutChangingBrowser(t *testing.T) {
	requests := make(chan string, 4)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		if r.Method != http.MethodPut {
			t.Errorf("unexpected request %s %s", r.Method, r.URL)
			w.WriteHeader(400)
			return
		}
		requests <- r.URL.Path
		w.Header().Set("ETag", `"d41d8cd98f00b204e9800998ecf8427e"`)
	}))
	defer server.Close()
	s := newTestService(t)
	input := localInput()
	input.Endpoint = server.URL
	target, err := s.SaveConnection("", input)
	if err != nil {
		t.Fatal(err)
	}
	input.Name = "Browser connection"
	input.Endpoint = "http://127.0.0.1:1"
	browser, err := s.SaveConnection("", input)
	if err != nil {
		t.Fatal(err)
	}
	session, err := s.profiles.Open(context.Background(), browser.ID)
	if err != nil {
		t.Fatal(err)
	}
	s.session = session
	dir := t.TempDir()
	folder := filepath.Join(dir, "photos")
	if err := os.Mkdir(folder, 0700); err != nil {
		t.Fatal(err)
	}
	paths := []string{filepath.Join(dir, "file.txt"), folder}
	for _, p := range []string{paths[0], filepath.Join(folder, "image.png")} {
		if err := os.WriteFile(p, []byte("upload contents"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.UploadMenuBarPaths(context.Background(), MenuBarTarget{ProfileID: target.ID, Bucket: "default-bucket"}, paths)
	if err != nil || n != 2 {
		t.Fatalf("queued=%d err=%v", n, err)
	}
	if got := s.CurrentConnection().Profile.ID; got != browser.ID {
		t.Fatalf("browser changed to %s", got)
	}
	seen := map[string]bool{}
	for range 2 {
		select {
		case p := <-requests:
			seen[p] = true
		case <-time.After(5 * time.Second):
			t.Fatal("default endpoint did not receive uploads")
		}
	}
	for _, key := range []string{"/default-bucket/file.txt", "/default-bucket/photos/image.png"} {
		if !seen[key] {
			t.Fatalf("missing %s, received %v", key, seen)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		done := true
		for _, tr := range s.ListTransfers() {
			if !tr.Finished() {
				done = false
			} else if strings.Contains(string(tr.State), "failed") {
				t.Fatal(tr.Error)
			}
		}
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("transfers did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
}
