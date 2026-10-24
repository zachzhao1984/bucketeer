package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"bucketeer/internal/store"
)

// TestPicGoLiveUploads runs the real service against real storage and
// uploads through HTTP exactly as Typora (a JSON list of paths) and
// Obsidian (multipart form-data) do. It is skipped unless S3_ENDPOINT names
// an endpoint, so `go test ./...` stays offline.
func TestPicGoLiveUploads(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set S3_ENDPOINT to run the upload service against storage")
	}
	svc := newTestService(t)
	// A bucket of its own per run: an earlier run's leftovers would change
	// the counts, and two parallel runs would fight over one name.
	bucket := "bucketeer-picgo-" + strings.ToLower(store.NewTestID())

	// A first connection opens without a bucket and creates the one the
	// upload connection names, as the app's new-bucket sheet does.
	admin, err := svc.SaveConnection("", store.Input{
		Name: "Admin", Provider: "minio", Endpoint: endpoint, Region: "us-east-1",
		AccessKeyID: os.Getenv("S3_KEY"), SecretKey: os.Getenv("S3_SECRET"), PathStyle: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Connect(t.Context(), admin.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.CreateBucket(t.Context(), bucket); err != nil {
		t.Log("create:", err)
	}
	svc.Disconnect()

	profile, err := svc.SaveConnection("", store.Input{
		Name: "Uploads", Provider: "minio", Endpoint: endpoint, Region: "us-east-1",
		AccessKeyID: os.Getenv("S3_KEY"), SecretKey: os.Getenv("S3_SECRET"), PathStyle: true,
		Bucket: bucket,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Connect(t.Context(), profile.ID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		svc.Disconnect()
		if listing, err := svc.SearchObjects(t.Context(), bucket, "", "", 100); err == nil {
			keys := []string{}
			for _, item := range listing.Items {
				keys = append(keys, item.Key)
			}
			if len(keys) > 0 {
				svc.DeleteObjects(t.Context(), bucket, keys)
			}
		}
		svc.DeleteBucket(t.Context(), bucket)
	})

	config := defaultPicGo()
	config.Enabled = true
	config.Port = freePort(t)
	config.Prefix = "shots"
	state := svc.SavePicGoConfig(config)
	if state.Err != "" {
		t.Fatalf("the service did not start: %s", state.Err)
	}

	// Typora: a JSON list of local paths.
	dir := t.TempDir()
	image := dir + "/paste.png"
	os.WriteFile(image, pngBytes(), 0o644)
	resp, err := http.Post(
		"http://127.0.0.1:"+itoa(config.Port)+"/upload?secret="+state.Token,
		"application/json", strings.NewReader(`{"list":["`+image+`"]}`))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var typora picgoResponse
	if err := json.Unmarshal(raw, &typora); err != nil {
		t.Fatalf("the JSON reply was not PicGo's shape: %s", raw)
	}
	if !typora.Success || len(typora.Result) != 1 {
		t.Fatalf("the JSON request got %s", raw)
	}
	if !strings.Contains(typora.Result[0], "/shots/") {
		t.Errorf("the JSON request's URL ignores the prefix: %s", typora.Result[0])
	}

	// Obsidian: multipart form-data with a files field.
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("files", "from-obsidian.png")
	part.Write(pngBytes())
	w.Close()
	resp2, err := http.Post(
		"http://127.0.0.1:"+itoa(config.Port)+"/upload?secret="+state.Token,
		w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatal(err)
	}
	raw2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	var obsidian picgoResponse
	if err := json.Unmarshal(raw2, &obsidian); err != nil {
		t.Fatalf("the multipart reply was not PicGo's shape: %s", raw2)
	}
	if !obsidian.Success || len(obsidian.Result) != 1 {
		t.Fatalf("the multipart request got %s", raw2)
	}

	// The two URLs name different objects, and both serve the image.
	if typora.Result[0] == obsidian.Result[0] {
		t.Error("both uploads reported the same URL")
	}
	for _, url := range []string{typora.Result[0], obsidian.Result[0]} {
		got, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET %s: %v", url, err)
		}
		got.Body.Close()
		if got.StatusCode != http.StatusOK {
			t.Errorf("GET %s = %d, want the image", url, got.StatusCode)
		}
	}

	// Both objects are in the bucket, under the prefix and the month.
	deep, err := svc.SearchObjects(t.Context(), bucket, "shots/", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(deep.Items) != 2 {
		t.Fatalf("%d objects under shots/, want 2", len(deep.Items))
	}
	month := time.Now().Format("2006/01")
	for _, item := range deep.Items {
		if !strings.Contains(item.Key, month) {
			t.Errorf("%s is not under the month's folder %s", item.Key, month)
		}
	}
}
