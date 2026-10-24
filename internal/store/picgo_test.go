package store

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

// fakeWriter records what an endpoint would have taken, and says which keys
// are already there.
type fakeWriter struct {
	put    map[string][]byte
	exists map[string]bool
}

// UploadWriter records the bytes under the key, like an endpoint would.
func (f *fakeWriter) UploadWriter(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error {
	body, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	if f.put == nil {
		f.put = map[string][]byte{}
	}
	f.put[key] = body
	f.exists[key] = true
	return nil
}

// StatObject reports only what the fake was told exists.
func (f *fakeWriter) StatObject(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	if f.exists[key] {
		return ObjectInfo{Key: key}, nil
	}
	return ObjectInfo{}, errors.New("not found")
}

// TestImageKey puts the prefix, the month, and the name together, with no
// doubled slashes.
func TestImageKey(t *testing.T) {
	at := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	cases := []struct{ prefix, name, want string }{
		{"", "a.png", "2026/03/a.png"},
		{"pics", "a.png", "pics/2026/03/a.png"},
		{"pics/", "a.png", "pics/2026/03/a.png"},
		{"/pics/", "a.png", "pics/2026/03/a.png"},
		{"a/b", "c/d.png", "a/b/2026/03/c/d.png"},
	}
	for _, c := range cases {
		if got := ImageKey(c.prefix, c.name, at); got != c.want {
			t.Errorf("ImageKey(%q, %q) = %q, want %q", c.prefix, c.name, got, c.want)
		}
	}
	if got := ImageKey("x", "a.png", time.Time{}); got != "x/a.png" {
		t.Errorf("ImageKey with no time = %q", got)
	}
}

// TestSanitizeImageName keeps the client's name but not its directory.
func TestSanitizeImageName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a.png", "a.png"},
		{"/etc/passwd", "passwd"},
		{"..\\..\\x.png", "x.png"},
		{"  spaced.png  ", "spaced.png"},
	}
	for _, c := range cases {
		got, err := SanitizeImageName(c.in)
		if err != nil || got != c.want {
			t.Errorf("SanitizeImageName(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
	for _, bad := range []string{"", ".", "..", "/"} {
		if _, err := SanitizeImageName(bad); err == nil {
			t.Errorf("SanitizeImageName(%q) was accepted", bad)
		}
	}
	// A very long name is shortened but keeps its extension.
	long := strings.Repeat("a", 300) + ".png"
	got, err := SanitizeImageName(long)
	if err != nil || len(got) > 200 || !strings.HasSuffix(got, ".png") {
		t.Errorf("a long name became %q, %v", got, err)
	}
}

// TestUniqueKey keeps both of two pastes of the same screenshot.
func TestUniqueKey(t *testing.T) {
	fake := &fakeWriter{exists: map[string]bool{"2026/03/shot.png": true}}
	got := UniqueKey(context.Background(), fake, "b", "2026/03/shot.png")
	if got != "2026/03/shot-2.png" {
		t.Fatalf("UniqueKey = %q, want the next free name", got)
	}
	if got := UniqueKey(context.Background(), fake, "b", "2026/03/free.png"); got != "2026/03/free.png" {
		t.Fatalf("UniqueKey = %q, want the name itself when it is free", got)
	}
}

// TestPicGoQueueRunsRequestsOneAtATime checks the queue's order and that a
// sink that has not been set yet holds requests rather than dropping them.
func TestPicGoQueueRunsRequestsOneAtATime(t *testing.T) {
	queue := NewPicGoQueue()
	done := make(chan []string, 2)

	go func() {
		// The sink arrives late: the first request is already waiting.
		time.Sleep(50 * time.Millisecond)
		queue.SetSink(func(req UploadRequest) UploadResult {
			var names []string
			for _, img := range req.Images {
				names = append(names, img.Name)
			}
			done <- names
			return UploadResult{OK: true, URLs: []string{"https://x/" + req.Images[0].Name}}
		})
	}()

	go func() {
		res, err := queue.Submit(context.Background(), UploadRequest{
			Images: []UploadJob{{Name: "first.png", Body: []byte("a")}},
		})
		if err != nil || !res.OK {
			done <- []string{"FAILED"}
		}
	}()
	time.Sleep(10 * time.Millisecond)
	go func() {
		res, _ := queue.Submit(context.Background(), UploadRequest{
			Images: []UploadJob{{Name: "second.png", Body: []byte("b")}},
		})
		_ = res
	}()

	first := <-done
	second := <-done
	if len(first) != 1 || first[0] != "first.png" || len(second) != 1 || second[0] != "second.png" {
		t.Fatalf("the queue ran %v then %v", first, second)
	}
}

// TestPicGoQueueCloseFailsPending checks that stopping the service tells the
// waiting client rather than hanging it.
func TestPicGoQueueCloseFailsPending(t *testing.T) {
	queue := NewPicGoQueue()
	result := make(chan UploadResult, 1)
	go func() {
		res, err := queue.Submit(context.Background(), UploadRequest{
			Images: []UploadJob{{Name: "a.png"}},
		})
		if err == nil {
			result <- res
		} else {
			result <- UploadResult{OK: false}
		}
	}()
	time.Sleep(20 * time.Millisecond)
	queue.Close()
	select {
	case res := <-result:
		if res.OK {
			t.Fatal("a closed queue reported success")
		}
	case <-time.After(time.Second):
		t.Fatal("Close left a request waiting")
	}
}

// TestUploadImageAndUniqueKeyTogether uploads the same bytes twice and
// checks the second got another name.
func TestUploadImageAndUniqueKeyTogether(t *testing.T) {
	fake := &fakeWriter{exists: map[string]bool{}}
	first := UniqueKey(context.Background(), fake, "b", "2026/03/shot.png")
	fake.exists[first] = true
	second := UniqueKey(context.Background(), fake, "b", "2026/03/shot.png")
	if first == second {
		t.Fatalf("both pastes landed at %q", first)
	}
	if !strings.Contains(second, "-2") {
		t.Fatalf("the second name is %q", second)
	}
}
