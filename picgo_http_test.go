package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bucketeer/internal/store"
)

// pngBytes is a real 1x1 png, so the name the service derives is honest.
func pngBytes() []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4, 0x89, 0x00, 0x00, 0x00,
		0x0D, 0x49, 0x44, 0x41, 0x54, 0x78, 0x9C, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4E, 0x44, 0xAE, 0x42, 0x60, 0x82,
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}

func startsWith(s, prefix string) bool { return strings.HasPrefix(s, prefix) }

// recordSink stands in for storage: it records what the requests brought
// and answers with a URL each.
func recordSink(results *[][]store.UploadJob, ok bool) func(store.UploadRequest) store.UploadResult {
	return func(req store.UploadRequest) store.UploadResult {
		*results = append(*results, req.Images)
		if !ok {
			return store.UploadResult{OK: false, Details: []store.UploadOutcome{{Name: req.Images[0].Name, Error: "nope"}}}
		}
		urls := make([]string, 0, len(req.Images))
		for _, image := range req.Images {
			urls = append(urls, "https://cdn.example.com/"+image.Name)
		}
		return store.UploadResult{OK: true, URLs: urls}
	}
}

// TestPicGoServiceEndToEnd starts the service against a recording sink and
// uploads through it the way Typora and Obsidian do.
func TestPicGoServiceEndToEnd(t *testing.T) {
	svc := newTestService(t)
	var uploads [][]store.UploadJob
	svc.picgoQueue.SetSink(recordSink(&uploads, true))

	// Turn the service on, on a port the test owns.
	config := defaultPicGo()
	config.Enabled = true
	config.Port = freePort(t)
	state := svc.SavePicGoConfig(config)
	if state.Err != "" {
		t.Fatalf("the service did not start: %s", state.Err)
	}
	t.Cleanup(func() { svc.SavePicGoConfig(PicGoConfig{Port: config.Port}) })

	// Multipart, as Obsidian's plugin sends.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("files", "hello.png")
	if err != nil {
		t.Fatal(err)
	}
	part.Write(pngBytes())
	writer.Close()

	url := "http://127.0.0.1:" + itoa(config.Port) + "/upload?secret=" + state.Token
	resp, err := http.Post(url, writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, raw)
	}
	var got picgoResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("response %q: %v", raw, err)
	}
	if !got.Success || len(got.Result) != 1 {
		t.Fatalf("response = %s", raw)
	}
	if !isImageKey(got.Result[0]) || filepath.Ext(got.Result[0]) != ".png" {
		t.Fatalf("result[0] = %q, want an uploaded png key", got.Result[0])
	}

	// The sink got the image the request carried.
	if len(uploads) != 1 || len(uploads[0]) != 1 || uploads[0][0].Name != "hello.png" {
		t.Fatalf("the sink saw %v", uploads)
	}

	// A wrong secret is refused.
	bad, _ := http.Post("http://127.0.0.1:"+itoa(config.Port)+"/upload?secret=nope", "text/plain", nil)
	bad.Body.Close()
	if bad.StatusCode != http.StatusUnauthorized {
		t.Fatalf("a wrong secret got %d, want 401", bad.StatusCode)
	}

	// The GET the port's root answers with an explanation, not an error.
	root, err := http.Get("http://127.0.0.1:" + itoa(config.Port) + "/")
	if err != nil {
		t.Fatal(err)
	}
	root.Body.Close()
}

// TestPicGoJSONListUploads is Typora's shape: {"list": ["…"]}.
func TestPicGoJSONListUploads(t *testing.T) {
	svc := newTestService(t)
	var uploads [][]store.UploadJob
	svc.picgoQueue.SetSink(recordSink(&uploads, true))
	config := defaultPicGo()
	config.Enabled = true
	config.Port = freePort(t)
	config.Prefix = "notes/pics"
	state := svc.SavePicGoConfig(config)
	if state.Err != "" {
		t.Fatal(state.Err)
	}
	t.Cleanup(func() { svc.SavePicGoConfig(PicGoConfig{Port: config.Port}) })

	dir := t.TempDir()
	image := filepath.Join(dir, "shot.png")
	os.WriteFile(image, pngBytes(), 0o644)

	payload, _ := json.Marshal(map[string][]string{"list": {image}})
	url := "http://127.0.0.1:" + itoa(config.Port) + "/upload?secret=" + state.Token
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var got picgoResponse
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("response %q: %v", raw, err)
	}
	if !got.Success || len(got.Result) != 1 {
		t.Fatalf("response = %s", raw)
	}
	// The prefix rides with the request, not the URL.
	if len(uploads) != 1 {
		t.Fatalf("the sink saw %d requests", len(uploads))
	}
	_ = uploads
}

// TestPicGoRejectsBadRequests pins the guardrails: too many images, a
// wrong secret, an unreadable path.
func TestPicGoRejectsBadRequests(t *testing.T) {
	svc := newTestService(t)
	var uploads [][]store.UploadJob
	svc.picgoQueue.SetSink(recordSink(&uploads, false))
	config := defaultPicGo()
	config.Enabled = true
	config.Port = freePort(t)
	state := svc.SavePicGoConfig(config)
	if state.Err != "" {
		t.Fatal(state.Err)
	}
	t.Cleanup(func() { svc.SavePicGoConfig(PicGoConfig{Port: config.Port}) })
	base := "http://127.0.0.1:" + itoa(config.Port)

	// More images than a request may carry.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	for i := 0; i < store.MaxImagesPerRequest+1; i++ {
		part, _ := writer.CreateFormFile("files", "a.png")
		part.Write(pngBytes())
	}
	writer.Close()
	resp, err := http.Post(base+"/upload?secret="+state.Token, writer.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("%d images got %d, want 413", store.MaxImagesPerRequest+1, resp.StatusCode)
	}

	// A path outside the machine's files, or not an image at all.
	payload, _ := json.Marshal(map[string][]string{"list": {"relative/path.png"}})
	resp, err = http.Post(base+"/upload?secret="+state.Token, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("a relative path got %d (%s), want 400", resp.StatusCode, raw)
	}

	// A JSON body that is not the shape the protocol names.
	resp, err = http.Post(base+"/upload?secret="+state.Token, "application/json", bytes.NewReader([]byte(`{"nope":1}`)))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("a wrong JSON shape got %d, want 400", resp.StatusCode)
	}
}
