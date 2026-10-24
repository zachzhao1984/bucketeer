package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// The PicGo queue: what the HTTP service's requests hand to the app, and
// how the app performs them. The protocol itself lives in the main package;
// this is the half that touches storage.

// MaxUploadMemory is the largest body a request may bring, which bounds
// what the service holds in memory at once.
const MaxUploadMemory = 32 << 20

// MaxImagesPerRequest is the most images one request may carry, as the
// reference server bounds its own.
const MaxImagesPerRequest = 20

// ErrTooManyUploads is what a request past the bound gets.
var ErrTooManyUploads = errors.New("a request can contain at most 20 images")

// ErrNoUploads is what a request without an image gets.
var ErrNoUploads = errors.New("no image files were provided")

// uploadJob is one image a request brought, held in memory until the app
// writes it.
type UploadJob struct {
	// Name is the file name the client gave, or one derived from its type.
	Name string
	// Body is the image's bytes. When SrcPath is set, the app reads the
	// image from there instead: a path-based request carries no bytes.
	Body    []byte
	SrcPath string
}

// UploadRequest is what a PicGo request asks the app to do: put these
// images in the bucket, under the prefix, and report the public URL of
// each.
type UploadRequest struct {
	ProfileID string
	Bucket    string
	Prefix    string
	Images    []UploadJob
}

// UploadOutcome is what happened to one image.
type UploadOutcome struct {
	Name  string `json:"name"`
	Key   string `json:"key,omitempty"`
	URL   string `json:"url,omitempty"`
	Error string `json:"error,omitempty"`
	Bytes int64  `json:"bytes,omitempty"`
}

// UploadResult is the whole request's outcome. Its first two fields are the
// shape PicGo clients read.
type UploadResult struct {
	OK      bool            `json:"success"`
	URLs    []string        `json:"result"`
	Details []UploadOutcome `json:"-"`
}

// ImageKey builds the object key of an uploaded image: the prefix, a folder
// for the month, and the file's own name. The month keeps a notebook's
// pictures together the way the reference server does.
func ImageKey(prefix, name string, at time.Time) string {
	prefix = strings.Trim(prefix, "/")
	parts := make([]string, 0, 4)
	if prefix != "" {
		parts = append(parts, prefix)
	}
	if !at.IsZero() {
		parts = append(parts, at.Format("2006/01"))
	}
	parts = append(parts, name)
	return strings.Join(parts, "/")
}

// SanitizeImageName makes a file name safe to use as a key segment: the
// base name only, no path separators, never empty.
func SanitizeImageName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || name == "." || name == ".." {
		return "", errors.New("an image needs a name")
	}
	if len(name) > 200 {
		// Shorten the stem and keep the extension, so the object still
		// renders as the image it is.
		ext := ""
		if dot := strings.LastIndexByte(name, '.'); dot > 0 && len(name)-dot <= 20 {
			ext, name = name[dot:], name[:dot]
		}
		name = name[:200-len(ext)] + ext
	}
	return name, nil
}

// NewTestID is a random id for a test that needs a unique bucket or key
// name. It is the generator the connections use, exported for the tests
// that run against storage.
func NewTestID() string { return newID() }

/* ---- The queue ---- */

// picgoWaiter is what one request waits on.
type picgoWaiter struct {
	request UploadRequest
	done    chan UploadResult
}

// PicGoQueue holds the images PicGo requests bring and hands them to the
// app one request at a time, so the window can show the HTTP service's
// uploads among its own, through the same transfer list.
type PicGoQueue struct {
	mu      sync.Mutex
	pending []picgoWaiter
	sink    func(UploadRequest) UploadResult
	closed  bool
}

// NewPicGoQueue builds a queue.
func NewPicGoQueue() *PicGoQueue { return &PicGoQueue{} }

// SetSink tells the queue what performs the uploads. The app sets it while
// a connection is open and clears it when one closes.
func (q *PicGoQueue) SetSink(fn func(UploadRequest) UploadResult) {
	q.mu.Lock()
	q.sink = fn
	q.mu.Unlock()
	q.pump()
}

// Close fails everything still waiting, as when the service stops.
func (q *PicGoQueue) Close() {
	q.mu.Lock()
	q.closed = true
	pending := q.pending
	q.pending = nil
	q.mu.Unlock()
	for _, w := range pending {
		w.done <- failedResult(w.request.Images, "the upload service is stopping")
	}
}

// Submit hands a request to the queue and waits for its result. The HTTP
// handler calls it on its own goroutine, so a slow upload holds its client
// rather than the service.
func (q *PicGoQueue) Submit(ctx context.Context, req UploadRequest) (UploadResult, error) {
	w := picgoWaiter{request: req, done: make(chan UploadResult, 1)}
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return failedResult(req.Images, "the upload service is stopping"), nil
	}
	q.pending = append(q.pending, w)
	q.mu.Unlock()
	q.pump()

	select {
	case res := <-w.done:
		return res, nil
	case <-ctx.Done():
		// The client went away. Take the request back unless the app is
		// already performing it; then its result is simply dropped.
		q.mu.Lock()
		for i, other := range q.pending {
			if other.done == w.done {
				q.pending = append(q.pending[:i], q.pending[i+1:]...)
				q.mu.Unlock()
				return UploadResult{}, ctx.Err()
			}
		}
		q.mu.Unlock()
		return UploadResult{}, ctx.Err()
	}
}

// HasSink reports whether requests would be performed, for tests.
func (q *PicGoQueue) HasSink() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.sink != nil
}

// pump hands the oldest pending request to the sink, one at a time.
func (q *PicGoQueue) pump() {
	for {
		q.mu.Lock()
		sink := q.sink
		if sink == nil || len(q.pending) == 0 {
			q.mu.Unlock()
			return
		}
		next := q.pending[0]
		q.pending = q.pending[1:]
		q.mu.Unlock()

		next.done <- sink(next.request)
	}
}

// failedResult builds a result that says the same thing about every image.
func failedResult(images []UploadJob, message string) UploadResult {
	details := make([]UploadOutcome, 0, len(images))
	for _, img := range images {
		details = append(details, UploadOutcome{Name: img.Name, Error: message})
	}
	return UploadResult{OK: false, Details: details}
}

/* ---- Performing ---- */

// ImageWriter is what the upload service needs of a session: put bytes at a
// key. A *Session does it against storage; a test does it into a map.
type ImageWriter interface {
	UploadWriter(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) error
}

// UploadImage writes one image and returns its size.
func UploadImage(ctx context.Context, w ImageWriter, bucket, key string, body []byte) (int64, error) {
	size := int64(len(body))
	if err := w.UploadWriter(ctx, bucket, key, bytes.NewReader(body), size, ""); err != nil {
		return 0, err
	}
	return size, nil
}

// KeyFinder reports whether a bucket holds a key. It is what UniqueKey
// asks, and an interface so tests can stand in for the endpoint.
type KeyFinder interface {
	StatObject(ctx context.Context, bucket, key string) (ObjectInfo, error)
}

// UniqueKey returns key, or key-2, key-3…, the first name the bucket does
// not hold, so pasting the same screenshot twice keeps both. It is a free
// function because a test's fake cannot override a promoted method.
func UniqueKey(ctx context.Context, finder KeyFinder, bucket, key string) string {
	if _, err := finder.StatObject(ctx, bucket, key); err != nil {
		return key
	}
	dot := strings.LastIndexByte(key, '.')
	slash := strings.LastIndexByte(key, '/')
	base, ext := key, ""
	if dot > slash+1 {
		base, ext = key[:dot], key[dot:]
	}
	for n := 2; n < 1000; n++ {
		candidate := fmt.Sprintf("%s-%d%s", base, n, ext)
		if _, err := finder.StatObject(ctx, bucket, candidate); err != nil {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d%s", base, time.Now().Unix(), ext)
}
