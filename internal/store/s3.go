package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Entry is a folder or an object in a listing.
type Entry struct {
	// Key is the full object key, or the prefix of a folder, ending in
	// "/".
	Key string `json:"key"`
	// Name is the last path segment, without the folder's trailing "/".
	Name string `json:"name"`
	// Folder marks a common prefix instead of an object.
	Folder bool `json:"folder"`
	// Size in bytes, zero for a folder.
	Size int64 `json:"size"`
	// LastModified is the object's modification time; zero for a folder.
	LastModified time.Time `json:"lastModified"`
	// StorageClass and ETag describe the object; empty for a folder.
	StorageClass string `json:"storageClass,omitempty"`
	ETag         string `json:"etag,omitempty"`
	// ContentType is what the server reports, "" when it did not say.
	ContentType string `json:"contentType,omitempty"`
}

// Listing is the contents of one prefix.
type Listing struct {
	Prefix string  `json:"prefix"`
	Folder string  `json:"folder"`
	Items  []Entry `json:"items"`
	// Truncated is true when the listing stopped at the requested limit
	// and more objects remain.
	Truncated bool `json:"truncated"`
	// Total is the number of entries returned.
	Total int `json:"total"`
	// Folders, Files and Bytes summarize the entries returned.
	Folders int   `json:"folders"`
	Files   int   `json:"files"`
	Bytes   int64 `json:"bytes"`
}

// ObjectInfo is an object's metadata.
type ObjectInfo struct {
	Key          string            `json:"key"`
	Name         string            `json:"name"`
	Folder       string            `json:"folder"`
	Size         int64             `json:"size"`
	ContentType  string            `json:"contentType"`
	ETag         string            `json:"etag"`
	LastModified time.Time         `json:"lastModified"`
	StorageClass string            `json:"storageClass"`
	Metadata     map[string]string `json:"metadata"`
}

// Stat is a bucket's size and object count, and its largest objects.
type Stat struct {
	Bucket  string       `json:"bucket"`
	Bytes   int64        `json:"bytes"`
	Objects int64        `json:"objects"`
	Folders int64        `json:"folders"`
	ByClass []ClassTotal `json:"byClass"`
	// ByKind is the same bytes and objects, grouped by what the objects
	// are: images, video, audio, documents, archives, other.
	ByKind  []KindTotal `json:"byKind"`
	Largest []Entry     `json:"largest"`
	// TopFolders are the largest top-level folders, biggest first.
	TopFolders []FolderTotal `json:"topFolders"`
	// Recent counts the objects modified in the last week, and
	// LastModified is the newest of them.
	Recent       int64     `json:"recent"`
	LastModified time.Time `json:"lastModified"`
	// Partial is true when the scan stopped before the bucket ended, so
	// the totals are a lower bound.
	Partial bool `json:"partial"`
}

// KindTotal is the size of the objects of one kind.
type KindTotal struct {
	Kind    string `json:"kind"`
	Bytes   int64  `json:"bytes"`
	Count   int64  `json:"count"`
	Largest int64  `json:"largest"`
}

// ClassTotal is the size of the objects of one storage class.
type ClassTotal struct {
	Class string `json:"class"`
	Bytes int64  `json:"bytes"`
	Count int64  `json:"count"`
}

// FolderTotal is the size of one top-level folder.
type FolderTotal struct {
	Prefix  string `json:"prefix"`
	Name    string `json:"name"`
	Bytes   int64  `json:"bytes"`
	Objects int64  `json:"objects"`
}

// Progress reports how far a transfer has come.
type Progress struct {
	// Done is the number of bytes transferred.
	Done int64 `json:"done"`
	// Total is the object's size, or the sum of the batch's sizes.
	Total int64 `json:"total"`
}

// Session is a live connection to one S3-compatible endpoint.
type Session struct {
	Profile Profile
	client  *minio.Client
}

// Open connects to the endpoint of a profile.
func (s *ProfileStore) Open(ctx context.Context, id string) (*Session, error) {
	p, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	secret, err := s.Secret(id)
	if err != nil {
		return nil, err
	}

	endpoint := p.Endpoint
	secure := true
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		endpoint = u.Host
		secure = u.Scheme != "http"
	} else {
		return nil, fmt.Errorf("the endpoint %q is not a URL", p.Endpoint)
	}

	lookup := minio.BucketLookupAuto
	if p.PathStyle {
		lookup = minio.BucketLookupPath
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(p.AccessKeyID, secret, ""),
		Secure:       secure,
		Region:       p.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", p.Host(), err)
	}
	return &Session{Profile: p, client: client}, nil
}

// Bucket describes one bucket.
type Bucket struct {
	Name    string    `json:"name"`
	Created time.Time `json:"created"`
	Region  string    `json:"region"`
}

// Buckets lists the endpoint's buckets.
func (s *Session) Buckets(ctx context.Context) ([]Bucket, error) {
	infos, err := s.client.ListBuckets(ctx)
	if err != nil {
		return nil, describe(err)
	}
	out := make([]Bucket, 0, len(infos))
	for _, b := range infos {
		out = append(out, Bucket{Name: b.Name, Created: b.CreationDate, Region: b.BucketRegion})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// CreateBucket makes a new bucket.
func (s *Session) CreateBucket(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("give the bucket a name")
	}
	return describe(s.client.MakeBucket(ctx, name, minio.MakeBucketOptions{Region: s.Profile.Region}))
}

// DeleteBucket removes an empty bucket.
func (s *Session) DeleteBucket(ctx context.Context, name string) error {
	return describe(s.client.RemoveBucket(ctx, name))
}

// ListPageSize is the most objects one listing returns.
const ListPageSize = 5000

// List returns the contents of a prefix, one level deep. Objects whose key
// ends in "/" are reported as folders, and their descendants are not
// listed, so a folder a bucket holds as an empty object still appears.
func (s *Session) List(ctx context.Context, bucket, prefix string) (Listing, error) {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	out := Listing{Prefix: prefix, Folder: FolderName(prefix)}
	seq := s.client.ListObjectsIter(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
		MaxKeys:   ListPageSize,
	})
	seenFolders := map[string]bool{}
	for obj := range seq {
		if obj.Err != nil {
			return Listing{}, describe(obj.Err)
		}
		name := strings.TrimPrefix(obj.Key, prefix)
		if name == "" {
			continue // the folder's own marker object
		}
		if strings.HasSuffix(name, "/") {
			// A folder held as an empty object.
			if seenFolders[obj.Key] {
				continue
			}
			seenFolders[obj.Key] = true
			out.Items = append(out.Items, Entry{Key: obj.Key, Name: strings.TrimSuffix(name, "/"), Folder: true})
			continue
		}
		out.Items = append(out.Items, Entry{
			Key:          obj.Key,
			Name:         name,
			Size:         obj.Size,
			LastModified: obj.LastModified,
			StorageClass: obj.StorageClass,
			ETag:         strings.Trim(obj.ETag, `"`),
			ContentType:  obj.ContentType,
		})
	}

	sortEntries(out.Items)
	for _, e := range out.Items {
		if e.Folder {
			out.Folders++
		} else {
			out.Files++
			out.Bytes += e.Size
		}
	}
	out.Total = len(out.Items)
	out.Truncated = out.Total >= ListPageSize
	return out, nil
}

// sortEntries puts folders first, then sorts each group by name, the way a
// file manager does.
func sortEntries(items []Entry) {
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if a.Folder != b.Folder {
			return a.Folder
		}
		return strings.ToLower(a.Name) < strings.ToLower(b.Name)
	})
}

// Search returns every object under prefix whose key contains query,
// case-insensitively. An empty query lists everything under the prefix.
func (s *Session) Search(ctx context.Context, bucket, prefix, query string, limit int) (Listing, error) {
	if limit <= 0 {
		limit = 500
	}
	needle := strings.ToLower(strings.TrimSpace(query))
	out := Listing{Prefix: prefix, Folder: FolderName(prefix)}
	seq := s.client.ListObjectsIter(ctx, bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	for obj := range seq {
		if obj.Err != nil {
			return Listing{}, describe(obj.Err)
		}
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(obj.Key), needle) {
			continue
		}
		if len(out.Items) >= limit {
			out.Truncated = true
			break
		}
		out.Items = append(out.Items, Entry{
			Key:          obj.Key,
			Name:         path.Base(obj.Key),
			Size:         obj.Size,
			LastModified: obj.LastModified,
			StorageClass: obj.StorageClass,
			ETag:         strings.Trim(obj.ETag, `"`),
			ContentType:  obj.ContentType,
		})
	}
	sortEntries(out.Items)
	for _, e := range out.Items {
		if e.Folder {
			out.Folders++
		} else {
			out.Files++
			out.Bytes += e.Size
		}
	}
	out.Total = len(out.Items)
	return out, nil
}

// Stat sums a bucket's objects. It walks the whole bucket, so it is slow
// on a large one; limit caps the objects scanned, and sets Partial.
func (s *Session) Stat(ctx context.Context, bucket string, limit int) (Stat, error) {
	if limit <= 0 {
		limit = 250_000
	}
	out := Stat{Bucket: bucket}
	classes := map[string]*ClassTotal{}
	kinds := map[string]*KindTotal{}
	folders := map[string]*FolderTotal{}
	var scanned int

	seq := s.client.ListObjectsIter(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
	for obj := range seq {
		if obj.Err != nil {
			return Stat{}, describe(obj.Err)
		}
		if strings.HasSuffix(obj.Key, "/") {
			out.Folders++
			continue
		}
		if scanned >= limit {
			out.Partial = true
			break
		}
		scanned++
		out.Objects++
		out.Bytes += obj.Size

		class := obj.StorageClass
		if class == "" {
			class = "STANDARD"
		}
		c, ok := classes[class]
		if !ok {
			c = &ClassTotal{Class: class}
			classes[class] = c
		}
		c.Bytes += obj.Size
		c.Count++

		kind := KindOf(obj.Key)
		k, ok := kinds[kind]
		if !ok {
			k = &KindTotal{Kind: kind}
			kinds[kind] = k
		}
		k.Bytes += obj.Size
		k.Count++
		k.Largest = max(k.Largest, obj.Size)

		if obj.LastModified.After(out.LastModified) {
			out.LastModified = obj.LastModified
		}
		if time.Since(obj.LastModified) < 7*24*time.Hour {
			out.Recent++
		}

		if top, _, ok := strings.Cut(obj.Key, "/"); ok && top != "" {
			key := top + "/"
			f, ok := folders[key]
			if !ok {
				f = &FolderTotal{Prefix: key, Name: top}
				folders[key] = f
			}
			f.Bytes += obj.Size
			f.Objects++
		}

		out.Largest = append(out.Largest, Entry{
			Key:          obj.Key,
			Name:         path.Base(obj.Key),
			Size:         obj.Size,
			LastModified: obj.LastModified,
			StorageClass: obj.StorageClass,
			ETag:         strings.Trim(obj.ETag, `"`),
		})
	}

	for _, c := range classes {
		out.ByClass = append(out.ByClass, *c)
	}
	sort.Slice(out.ByClass, func(i, j int) bool { return out.ByClass[i].Bytes > out.ByClass[j].Bytes })

	for _, k := range kinds {
		out.ByKind = append(out.ByKind, *k)
	}
	sort.Slice(out.ByKind, func(i, j int) bool { return out.ByKind[i].Bytes > out.ByKind[j].Bytes })

	sort.Slice(out.Largest, func(i, j int) bool { return out.Largest[i].Size > out.Largest[j].Size })
	if len(out.Largest) > 25 {
		out.Largest = out.Largest[:25]
	}

	// Attach the top-level folders, biggest first, for the dashboard.
	out.ByClass = out.ByClass[:len(out.ByClass):len(out.ByClass)]
	out.TopFolders = make([]FolderTotal, 0, len(folders))
	for _, f := range folders {
		out.TopFolders = append(out.TopFolders, *f)
	}
	sort.Slice(out.TopFolders, func(i, j int) bool { return out.TopFolders[i].Bytes > out.TopFolders[j].Bytes })
	if len(out.TopFolders) > 12 {
		out.TopFolders = out.TopFolders[:12]
	}
	return out, nil
}

// StatObject returns one object's metadata.
func (s *Session) StatObject(ctx context.Context, bucket, key string) (ObjectInfo, error) {
	info, err := s.client.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, describe(err)
	}
	meta := map[string]string{}
	for k, v := range info.Metadata {
		if len(v) == 0 {
			continue
		}
		meta[strings.ToLower(k)] = v[0]
	}
	return ObjectInfo{
		Key:          info.Key,
		Name:         path.Base(info.Key),
		Folder:       FolderName(strings.TrimSuffix(info.Key, path.Base(info.Key))),
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         strings.Trim(info.ETag, `"`),
		LastModified: info.LastModified,
		StorageClass: info.StorageClass,
		Metadata:     meta,
	}, nil
}

// FolderName returns the display name of a prefix: its last segment.
func FolderName(prefix string) string {
	p := strings.Trim(prefix, "/")
	if p == "" {
		return ""
	}
	return path.Base(p)
}

// MakeFolder creates an empty folder. Object storage has no directories,
// so this writes the zero-byte marker object that makes one appear.
func (s *Session) MakeFolder(ctx context.Context, bucket, prefix, name string) (string, error) {
	name = strings.Trim(strings.TrimSpace(name), "/")
	if name == "" {
		return "", errors.New("give the folder a name")
	}
	if strings.Contains(name, "/") {
		return "", errors.New("a folder name cannot contain a slash")
	}
	key := prefix + name + "/"
	_, err := s.client.PutObject(ctx, bucket, key, strings.NewReader(""), 0, minio.PutObjectOptions{
		ContentType: "application/x-directory",
		// The marker is empty, so there is nothing to checksum. Sending it
		// with the streaming signature would frame it as chunked, which
		// some S3-compatible servers reject for a zero-byte body.
		DisableContentSha256: true,
	})
	if err != nil {
		return "", describe(err)
	}
	return key, nil
}

// Rename copies an object to a new key and removes the old one.
func (s *Session) Rename(ctx context.Context, bucket, from, to string) error {
	if from == to {
		return nil
	}
	if _, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: bucket, Object: to},
		minio.CopySrcOptions{Bucket: bucket, Object: from},
	); err != nil {
		return describe(err)
	}
	if err := s.client.RemoveObject(ctx, bucket, from, minio.RemoveObjectOptions{}); err != nil {
		return describe(err)
	}
	return nil
}

// MoveToFolder moves each key under prefix, keeping its base name.
func (s *Session) MoveToFolder(ctx context.Context, bucket string, keys []string, prefix string) error {
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	for _, key := range keys {
		to := prefix + path.Base(key)
		if to == key {
			continue
		}
		if err := s.Rename(ctx, bucket, key, to); err != nil {
			return err
		}
	}
	return nil
}

// Remove deletes objects and folders. A folder key ending in "/" is
// deleted with everything under it.
func (s *Session) Remove(ctx context.Context, bucket string, keys []string) (int, error) {
	var deleted int
	for _, key := range keys {
		if strings.HasSuffix(key, "/") {
			n, err := s.removePrefix(ctx, bucket, key)
			if err != nil {
				return deleted, err
			}
			deleted += n
			continue
		}
		if err := s.client.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{}); err != nil {
			return deleted, describe(err)
		}
		deleted++
	}
	return deleted, nil
}

// removePrefix deletes every object under prefix, in batches.
func (s *Session) removePrefix(ctx context.Context, bucket, prefix string) (int, error) {
	ch := make(chan minio.ObjectInfo)
	// sent counts the objects handed to the client; the error channel only
	// reports the ones that failed, so what was deleted is the difference.
	sent := 0
	go func() {
		defer close(ch)
		for obj := range s.client.ListObjectsIter(ctx, bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
			if obj.Err != nil {
				return
			}
			select {
			case ch <- obj:
				sent++
			case <-ctx.Done():
				return
			}
		}
	}()

	var failures int
	var firstErr error
	for err := range s.client.RemoveObjects(ctx, bucket, ch, minio.RemoveObjectsOptions{}) {
		if err.Err == nil {
			continue
		}
		failures++
		if firstErr == nil {
			firstErr = describe(err.Err)
		}
	}
	deleted := sent - failures
	if deleted < 0 {
		deleted = 0
	}
	return deleted, firstErr
}

// DownloadOptions describes a download.
type DownloadOptions struct {
	Bucket string
	Key    string
	// Dest is the local path to write.
	Dest string
}

// Download writes an object to a local file, reporting progress as it
// goes. The progress callback is called from the transferring goroutine.
func (s *Session) Download(ctx context.Context, opts DownloadOptions, progress func(Progress)) (int64, error) {
	obj, err := s.client.GetObject(ctx, opts.Bucket, opts.Key, minio.GetObjectOptions{})
	if err != nil {
		return 0, describe(err)
	}
	defer obj.Close()

	info, err := obj.Stat()
	if err != nil {
		return 0, describe(err)
	}
	if err := copyToFile(ctx, obj, opts.Dest, info.Size, progress); err != nil {
		return 0, err
	}
	return info.Size, nil
}

// UploadOptions describes an upload.
type UploadOptions struct {
	Bucket string
	Key    string
	// Src is the local file to read.
	Src string
	// Size is the file's size; -1 to have it detected.
	Size int64
}

// Upload writes a local file to an object, using a multipart upload when
// the file is large.
func (s *Session) Upload(ctx context.Context, opts UploadOptions, progress func(Progress)) error {
	f, err := openFile(opts.Src)
	if err != nil {
		return err
	}
	defer f.Close()

	size := opts.Size
	if size < 0 {
		st, err := f.Stat()
		if err != nil {
			return err
		}
		size = st.Size()
	}

	put := minio.PutObjectOptions{
		ContentType: contentTypeFor(opts.Key),
		PartSize:    multipartPartSize(size),
	}
	if size == 0 {
		// An empty object has nothing to checksum, and the streaming
		// signature would frame it as chunked, which some S3-compatible
		// servers reject for a zero-byte body.
		put.DisableContentSha256 = true
	}
	_, err = s.client.PutObject(ctx, opts.Bucket, opts.Key, progressReader(ctx, f, size, progress), size, put)
	if err != nil {
		return describe(err)
	}
	return nil
}

// Presign returns a URL that grants temporary access to an object.
func (s *Session) Presign(ctx context.Context, bucket, key string, ttl time.Duration) (string, error) {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	u, err := s.client.PresignedGetObject(ctx, bucket, key, ttl, url.Values{})
	if err != nil {
		return "", describe(err)
	}
	return u.String(), nil
}

// OpenObject returns a reader over an object, and its metadata. The
// caller closes it. start and end select a byte range; a negative end
// reads to the end of the object, as HTTP ranges do.
func (s *Session) OpenObject(ctx context.Context, bucket, key string, start, end int64) (io.ReadCloser, ObjectInfo, error) {
	opts := minio.GetObjectOptions{}
	if start > 0 || end >= 0 {
		if err := opts.SetRange(start, end); err != nil {
			return nil, ObjectInfo{}, err
		}
	}
	obj, err := s.client.GetObject(ctx, bucket, key, opts)
	if err != nil {
		return nil, ObjectInfo{}, describe(err)
	}
	info, err := obj.Stat()
	if err != nil {
		obj.Close()
		return nil, ObjectInfo{}, describe(err)
	}
	return obj, ObjectInfo{
		Key:          info.Key,
		Name:         path.Base(info.Key),
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         strings.Trim(info.ETag, `"`),
		LastModified: info.LastModified,
	}, nil
}

// describe turns an S3 error into one that reads well in the interface,
// keeping the provider's own message.
func describe(err error) error {
	if err == nil {
		return nil
	}
	var resp minio.ErrorResponse
	if errors.As(err, &resp) {
		switch resp.StatusCode {
		case http.StatusForbidden:
			return fmt.Errorf("access denied: %s", firstLine(resp.Message))
		case http.StatusNotFound:
			return fmt.Errorf("not found: %s", firstLine(resp.Message))
		case http.StatusNotImplemented:
			return fmt.Errorf("not supported by this endpoint: %s", firstLine(resp.Message))
		}
		if resp.Message != "" {
			return errors.New(firstLine(resp.Message))
		}
	}
	return err
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "the storage endpoint returned an error"
	}
	return s
}

// contentTypeFor guesses an object's content type from its key.
func contentTypeFor(key string) string {
	switch strings.ToLower(path.Ext(key)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".avif":
		return "image/avif"
	case ".heic":
		return "image/heic"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".m4a":
		return "audio/mp4"
	case ".flac":
		return "audio/flac"
	case ".ogg":
		return "audio/ogg"
	case ".pdf":
		return "application/pdf"
	case ".json":
		return "application/json"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".js":
		return "text/javascript"
	case ".txt", ".md", ".csv", ".log", ".yaml", ".yml", ".toml", ".ini", ".conf":
		return "text/plain; charset=utf-8"
	case ".zip":
		return "application/zip"
	case ".gz":
		return "application/gzip"
	case ".tar":
		return "application/x-tar"
	}
	return "application/octet-stream"
}

// multipartPartSize picks a part size: minio-go uploads in one request
// below the multipart threshold, and in parts of this size above it.
func multipartPartSize(size int64) uint64 {
	const (
		minPart = 16 << 20
		maxPart = 256 << 20
	)
	switch {
	case size <= 64<<20:
		return minPart
	case size >= 16<<30:
		return maxPart
	default:
		// Aim for about 1000 parts, rounded to a power of two MiB.
		p := uint64(1) << 20
		for p*1000 < uint64(size) && p < maxPart {
			p <<= 1
		}
		if p < minPart {
			p = minPart
		}
		return p
	}
}

// progressReader reports how many bytes have been read, at most about
// every ProgressInterval.
func progressReader(ctx context.Context, r io.Reader, total int64, progress func(Progress)) io.Reader {
	if progress == nil {
		return r
	}
	return &countingReader{ctx: ctx, r: r, total: total, progress: progress}
}

type countingReader struct {
	ctx      context.Context
	r        io.Reader
	total    int64
	done     int64
	last     time.Time
	progress func(Progress)
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.done += int64(n)
	if now := time.Now(); now.Sub(c.last) >= ProgressInterval || err == io.EOF {
		c.last = now
		c.progress(Progress{Done: c.done, Total: c.total})
	}
	return n, err
}

// ProgressInterval is how often transfer progress is reported.
const ProgressInterval = 200 * time.Millisecond

// KindOf classifies a key by its extension, so the dashboard can group
// the bytes by what they are. It is the storage layer's own copy of the
// rule the interface draws with, because a bucket's breakdown is computed
// where the objects are listed.
func KindOf(key string) string {
	base := key
	if i := strings.LastIndex(key, "/"); i >= 0 {
		base = key[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return "file"
	}
	switch strings.ToLower(base[dot:]) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".bmp", ".ico", ".tiff", ".heic", ".svg":
		return "image"
	case ".mp4", ".mov", ".webm", ".mkv", ".avi", ".m4v", ".ogv":
		return "video"
	case ".mp3", ".wav", ".m4a", ".flac", ".ogg", ".oga", ".aac", ".opus":
		return "audio"
	case ".zip", ".gz", ".tar", ".tgz", ".bz2", ".xz", ".7z", ".rar", ".zst":
		return "archive"
	case ".pdf", ".doc", ".docx", ".xls", ".xlsx", ".ppt", ".pptx", ".rtf", ".epub",
		".txt", ".md", ".markdown", ".csv", ".tsv", ".log", ".json", ".yaml", ".yml",
		".toml", ".ini", ".conf", ".env", ".srt", ".vtt", ".js", ".mjs", ".cjs",
		".ts", ".tsx", ".jsx", ".go", ".rs", ".py", ".rb", ".java", ".kt", ".c",
		".h", ".cc", ".cpp", ".hpp", ".cs", ".swift", ".php", ".sh", ".bash",
		".zsh", ".sql", ".css", ".scss", ".html", ".htm", ".xml", ".vue",
		".svelte", ".lua", ".dart":
		return "document"
	default:
		return "file"
	}
}

// Recent returns the newest objects in a bucket, newest first, up to
// limit. It walks the bucket, so it is only worth asking for a small
// number.
func (s *Session) Recent(ctx context.Context, bucket string, limit int) (Listing, error) {
	if limit <= 0 {
		limit = 40
	}
	out := Listing{Prefix: ""}
	seq := s.client.ListObjectsIter(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
	for obj := range seq {
		if obj.Err != nil {
			return Listing{}, describe(obj.Err)
		}
		if strings.HasSuffix(obj.Key, "/") {
			continue
		}
		out.Items = append(out.Items, Entry{
			Key:          obj.Key,
			Name:         path.Base(obj.Key),
			Size:         obj.Size,
			LastModified: obj.LastModified,
			StorageClass: obj.StorageClass,
			ETag:         strings.Trim(obj.ETag, `"`),
			ContentType:  obj.ContentType,
		})
		if len(out.Items) >= limit*8 {
			break // enough to sort; a bucket is not walked in full
		}
	}
	sort.Slice(out.Items, func(i, j int) bool { return out.Items[i].LastModified.After(out.Items[j].LastModified) })
	if len(out.Items) > limit {
		out.Items = out.Items[:limit]
	}
	out.Total = len(out.Items)
	return out, nil
}
