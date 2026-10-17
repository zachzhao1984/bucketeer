package store

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// These tests run against a real S3-compatible endpoint. They are skipped
// unless S3_ENDPOINT is set, so `go test ./...` stays offline:
//
//	S3_ENDPOINT=http://127.0.0.1:9000 S3_KEY=… S3_SECRET=… go test ./internal/store/ -run Integration -v
//
// The endpoint must accept path-style requests, as MinIO does.

// integrationSession opens a session against the configured endpoint, and
// returns it with a bucket that is created and removed for the test.
func integrationSession(t *testing.T) (*Session, string) {
	t.Helper()

	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("set S3_ENDPOINT to run the storage integration tests")
	}

	dir := t.TempDir()
	profiles := NewProfileStore(filepath.Join(dir, "connections.json"), newFakeSecrets())
	profile, err := profiles.Create(Input{
		Name:        "integration",
		Provider:    string(ProviderMinIO),
		Endpoint:    endpoint,
		Region:      "us-east-1",
		AccessKeyID: os.Getenv("S3_KEY"),
		SecretKey:   os.Getenv("S3_SECRET"),
		PathStyle:   true,
	})
	if err != nil {
		t.Fatalf("create the connection: %v", err)
	}

	ctx := context.Background()
	session, err := profiles.Open(ctx, profile.ID)
	if err != nil {
		t.Fatalf("open the connection: %v", err)
	}

	// A bucket unique to this run, so tests do not collide.
	bucket := "bucketeer-test-" + strings.ToLower(newID())
	if err := session.CreateBucket(ctx, bucket); err != nil {
		t.Fatalf("create the bucket: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if listing, err := session.Search(cleanupCtx, bucket, "", "", 10_000); err == nil {
			keys := make([]string, 0, len(listing.Items))
			for _, item := range listing.Items {
				keys = append(keys, item.Key)
			}
			if len(keys) > 0 {
				_, _ = session.Remove(cleanupCtx, bucket, keys)
			}
		}
		_ = session.DeleteBucket(cleanupCtx, bucket)
	})
	return session, bucket
}

// uploadTestFile writes a file of the given size and uploads it.
func uploadTestFile(t *testing.T, session *Session, bucket, key string, size int) {
	t.Helper()
	src := filepath.Join(t.TempDir(), filepath.Base(key))
	body := make([]byte, size)
	for i := range body {
		body[i] = byte('a' + i%26)
	}
	if err := os.WriteFile(src, body, 0o644); err != nil {
		t.Fatalf("write %s: %v", src, err)
	}
	err := session.Upload(context.Background(),
		UploadOptions{Bucket: bucket, Key: key, Src: src, Size: int64(size)},
		func(Progress) {})
	if err != nil {
		t.Fatalf("upload %s: %v", key, err)
	}
}

func TestIntegrationBucketsAndObjects(t *testing.T) {
	session, bucket := integrationSession(t)
	ctx := context.Background()

	// The bucket shows up in the endpoint's listing.
	buckets, err := session.Buckets(ctx)
	if err != nil {
		t.Fatalf("list buckets: %v", err)
	}
	var found bool
	for _, b := range buckets {
		if b.Name == bucket {
			found = true
		}
	}
	if !found {
		t.Fatalf("the created bucket %q is missing from the listing", bucket)
	}

	// A folder, a file inside it, and a file at the root.
	if _, err := session.MakeFolder(ctx, bucket, "", "images"); err != nil {
		t.Fatalf("make folder: %v", err)
	}
	uploadTestFile(t, session, bucket, "images/photo.png", 4096)
	uploadTestFile(t, session, bucket, "root.txt", 128)

	root, err := session.List(ctx, bucket, "")
	if err != nil {
		t.Fatalf("list root: %v", err)
	}
	if root.Folders != 1 || root.Files != 1 {
		t.Errorf("root listing = %d folders, %d files; want 1 and 1", root.Folders, root.Files)
	}
	if root.Items[0].Folder != true || root.Items[0].Name != "images" {
		t.Errorf("the folder should sort first, got %+v", root.Items[0])
	}

	inFolder, err := session.List(ctx, bucket, "images/")
	if err != nil {
		t.Fatalf("list images/: %v", err)
	}
	if inFolder.Files != 1 || inFolder.Folders != 0 {
		t.Errorf("images/ listing = %d files, %d folders; want 1 and 0", inFolder.Files, inFolder.Folders)
	}
	if inFolder.Bytes != 4096 {
		t.Errorf("images/ bytes = %d, want 4096", inFolder.Bytes)
	}
}

func TestIntegrationUploadDownloadRoundTrip(t *testing.T) {
	session, bucket := integrationSession(t)
	ctx := context.Background()

	// Larger than the multipart threshold, so this exercises parts.
	const size = 6 << 20
	uploadTestFile(t, session, bucket, "big.bin", size)

	info, err := session.StatObject(ctx, bucket, "big.bin")
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size != size {
		t.Fatalf("size = %d, want %d", info.Size, size)
	}

	// A download reports progress and writes every byte.
	dest := filepath.Join(t.TempDir(), "big.bin")
	var last Progress
	if _, err := session.Download(ctx, DownloadOptions{Bucket: bucket, Key: "big.bin", Dest: dest},
		func(p Progress) { last = p }); err != nil {
		t.Fatalf("download: %v", err)
	}
	written, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat the download: %v", err)
	}
	if written.Size() != size {
		t.Errorf("downloaded %d bytes, want %d", written.Size(), size)
	}
	if last.Total != size || last.Done != size {
		t.Errorf("final progress = %d/%d, want %d/%d", last.Done, last.Total, size, size)
	}

	// A ranged read, which is what a media preview does.
	body, _, err := session.OpenObject(ctx, bucket, "big.bin", 100, 199)
	if err != nil {
		t.Fatalf("open range: %v", err)
	}
	defer body.Close()
	buf := make([]byte, 100)
	n, err := body.Read(buf)
	if err != nil && n == 0 {
		t.Fatalf("read range: %v", err)
	}
	if n != 100 {
		t.Errorf("read %d bytes of the range, want 100", n)
	}
}

func TestIntegrationSearchStatAndPresign(t *testing.T) {
	session, bucket := integrationSession(t)
	ctx := context.Background()

	uploadTestFile(t, session, bucket, "docs/notes.md", 512)
	uploadTestFile(t, session, bucket, "docs/report.pdf", 512)
	uploadTestFile(t, session, bucket, "media/clip.mp4", 512)

	found, err := session.Search(ctx, bucket, "", "notes", 100)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if found.Files != 1 || found.Items[0].Key != "docs/notes.md" {
		t.Errorf("search for %q = %+v, want only docs/notes.md", "notes", found.Items)
	}

	// A search limited to a prefix finds only that subtree.
	inDocs, err := session.Search(ctx, bucket, "docs/", "", 100)
	if err != nil {
		t.Fatalf("search docs/: %v", err)
	}
	if inDocs.Files != 2 {
		t.Errorf("docs/ search found %d files, want 2", inDocs.Files)
	}

	stat, err := session.Stat(ctx, bucket, 0)
	if err != nil {
		t.Fatalf("stat bucket: %v", err)
	}
	if stat.Objects != 3 || stat.Bytes != 3*512 {
		t.Errorf("bucket stat = %d objects, %d bytes; want 3 and %d", stat.Objects, stat.Bytes, 3*512)
	}
	if stat.Partial {
		t.Error("a small bucket should not report a partial scan")
	}
	// docs/ holds two thirds of the bytes, so it should lead.
	if len(stat.TopFolders) == 0 || stat.TopFolders[0].Name != "docs" {
		t.Errorf("top folders = %+v, want docs first", stat.TopFolders)
	}

	url, err := session.Presign(ctx, bucket, "docs/notes.md", time.Hour)
	if err != nil {
		t.Fatalf("presign: %v", err)
	}
	if !strings.Contains(url, "X-Amz-Signature") {
		t.Errorf("presigned URL has no signature: %s", url)
	}
}

func TestIntegrationRenameMoveAndDelete(t *testing.T) {
	session, bucket := integrationSession(t)
	ctx := context.Background()

	uploadTestFile(t, session, bucket, "root.txt", 64)
	uploadTestFile(t, session, bucket, "docs/keep.md", 64)

	// Rename keeps the file in place.
	if err := session.Rename(ctx, bucket, "root.txt", "renamed.txt"); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if _, err := session.StatObject(ctx, bucket, "root.txt"); err == nil {
		t.Error("the old key still exists after a rename")
	}
	if _, err := session.StatObject(ctx, bucket, "renamed.txt"); err != nil {
		t.Errorf("the new key is missing: %v", err)
	}

	// Move into a folder.
	if err := session.MoveToFolder(ctx, bucket, []string{"renamed.txt"}, "docs/"); err != nil {
		t.Fatalf("move: %v", err)
	}
	if _, err := session.StatObject(ctx, bucket, "docs/renamed.txt"); err != nil {
		t.Errorf("the moved key is missing: %v", err)
	}

	// Deleting a folder takes its contents with it.
	deleted, err := session.Remove(ctx, bucket, []string{"docs/"})
	if err != nil {
		t.Fatalf("delete folder: %v", err)
	}
	if deleted < 2 {
		t.Errorf("deleted %d objects, want at least 2", deleted)
	}
	after, err := session.Search(ctx, bucket, "", "", 100)
	if err != nil {
		t.Fatalf("list after delete: %v", err)
	}
	if after.Files != 0 {
		t.Errorf("the bucket still holds %d files after deleting the folder", after.Files)
	}
}

func TestIntegrationMissingObjectIsAnError(t *testing.T) {
	session, bucket := integrationSession(t)
	ctx := context.Background()

	if _, err := session.StatObject(ctx, bucket, "no/such/key.txt"); err == nil {
		t.Fatal("stat of a missing object succeeded")
	} else if !strings.Contains(err.Error(), "not found") {
		t.Errorf("error = %q, want it to say the object was not found", err)
	}
}

// TestDescribeKeepsProviderMessages checks the error translation, which
// needs no endpoint.
func TestDescribeKeepsProviderMessages(t *testing.T) {
	if got := describe(nil); got != nil {
		t.Errorf("describe(nil) = %v, want nil", got)
	}
	plain := errors.New("dial tcp: connection refused")
	if got := describe(plain); !errors.Is(got, plain) {
		t.Errorf("describe passed through %v, want the original error", got)
	}
}

// TestProgressReaderReportsPeriodically checks the progress plumbing
// without an endpoint.
func TestProgressReaderReportsPeriodically(t *testing.T) {
	data := bytes.Repeat([]byte("x"), 1000)
	var calls int
	var last Progress
	r := progressReader(context.Background(), bytes.NewReader(data), int64(len(data)), func(p Progress) {
		calls++
		last = p
	})
	if _, err := r.Read(make([]byte, 10)); err != nil {
		t.Fatalf("read: %v", err)
	}
	// The first read reports, so a slow transfer shows progress at once.
	if calls != 1 {
		t.Errorf("reported %d times after one read, want 1", calls)
	}
	if last.Done != 10 || last.Total != int64(len(data)) {
		t.Errorf("progress = %d/%d, want 10/%d", last.Done, last.Total, len(data))
	}
}
