package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bucketeer/internal/store"
)

/* ---- The wire shapes ---- */

// picgoResponse is the JSON every PicGo client reads: what succeeded and
// the URLs to paste, or a message saying what went wrong.
type picgoResponse struct {
	Success bool     `json:"success"`
	Result  []string `json:"result"`
	// Message explains a failure. PicGo's own server returns errors in this
	// field, and the plugins show it to the user.
	Message string `json:"message,omitempty"`
}

// picgoListBody is the JSON body of a path-based request.
type picgoListBody struct {
	List []string `json:"list"`
}

// uploadImage is one image a request brought.
type uploadImage struct {
	Name string
	Body []byte
	// FromPath marks a path-based upload, which the app reads from disk
	// rather than out of the body.
	FromPath string
}

// writeJSON marshals v and answers with it.
func writeJSON(w http.ResponseWriter, status int, v any) {
	body, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "could not encode the response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	w.Write(body)
	w.Write([]byte("\n"))
}

/* ---- multipart ---- */

// readMultipart takes every `files` part out of a request. A part without a
// name gets one from its content type, as a screenshot pasted without a
// file name would.
func readMultipart(r *http.Request) ([]uploadImage, error) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, errors.New("invalid multipart upload request")
	}
	if mediaType != "multipart/form-data" {
		return nil, errors.New("invalid multipart upload request")
	}
	reader := multipart.NewReader(r.Body, params["boundary"])

	var images []uploadImage
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.New("invalid multipart upload request")
		}
		// Only the field the protocol names carries images; a stray part
		// from a curious client is ignored rather than uploaded.
		if part.FormName() != "files" {
			part.Close()
			continue
		}
		body, err := io.ReadAll(io.LimitReader(part, store.MaxUploadMemory+1))
		part.Close()
		if err != nil {
			return nil, errors.New("could not read the image")
		}
		if len(body) > store.MaxUploadMemory {
			return nil, fmt.Errorf("images are limited to %d MiB", store.MaxUploadMemory>>20)
		}
		if len(body) == 0 {
			continue
		}
		name := filepath.Base(part.FileName())
		if name == "" || name == "." || name == ".." {
			name = imageNameFor(body, "image")
		}
		images = append(images, uploadImage{Name: name, Body: body})
		if len(images) > store.MaxImagesPerRequest {
			return nil, store.ErrTooManyUploads
		}
	}
	if len(images) == 0 {
		return nil, store.ErrNoUploads
	}
	return images, nil
}

/* ---- JSON list of paths ---- */

// readJSONList takes {"list": ["/path/image.png"]}: paths on this machine,
// which is what Typora's desktop plugin sends. Only image files up to the
// same bound are accepted.
func readJSONList(r *http.Request) ([]uploadImage, error) {
	// A list of paths is a request to read files; the body is bounded by
	// how long a list of paths can be, not by the images themselves.
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return nil, errors.New("could not read the request")
	}
	var parsed picgoListBody
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, errors.New("expected JSON {\"list\": [\"/path/to/image.png\"]}")
	}
	if len(parsed.List) == 0 {
		return nil, store.ErrNoUploads
	}
	if len(parsed.List) > store.MaxImagesPerRequest {
		return nil, store.ErrTooManyUploads
	}

	images := make([]uploadImage, 0, len(parsed.List))
	for _, path := range parsed.List {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		image, err := statLocalImage(path)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	if len(images) == 0 {
		return nil, store.ErrNoUploads
	}
	return images, nil
}

// statLocalImage checks a path-based image without reading it: the sink
// reads the file when the request's turn comes, so a long list does not
// hold the service's memory while it waits.
func statLocalImage(path string) (uploadImage, error) {
	if !filepath.IsAbs(path) {
		return uploadImage{}, fmt.Errorf("%s is not an absolute path", filepath.Base(path))
	}
	info, err := os.Stat(path)
	if err != nil {
		return uploadImage{}, fmt.Errorf("could not read %s", filepath.Base(path))
	}
	if info.IsDir() {
		return uploadImage{}, fmt.Errorf("%s is a folder", filepath.Base(path))
	}
	if info.Size() > store.MaxUploadMemory {
		return uploadImage{}, fmt.Errorf("%s is larger than %d MiB", filepath.Base(path), store.MaxUploadMemory>>20)
	}
	return uploadImage{Name: filepath.Base(path), FromPath: path}, nil
}

/* ---- Naming ---- */

// imageKindOf reads the first bytes of an image and names its kind, so a
// nameless paste still gets a working extension.
func imageKindOf(body []byte) string {
	switch {
	case len(body) >= 8 && string(body[:8]) == "\x89PNG\r\n\x1a\n":
		return "png"
	case len(body) >= 3 && body[0] == 0xFF && body[1] == 0xD8 && body[2] == 0xFF:
		return "jpg"
	case len(body) >= 6 && (string(body[:6]) == "GIF87a" || string(body[:6]) == "GIF89a"):
		return "gif"
	case len(body) >= 12 && string(body[8:12]) == "WEBP":
		return "webp"
	case len(body) >= 4 && string(body[:4]) == "RIFF":
		return "webp"
	}
	return ""
}

// imageNameFor builds a name for an image that arrived without one: what
// its bytes say it is, or png when they say nothing. The moment it arrived
// keeps two pastes apart even before UniqueKey has had its say.
func imageNameFor(body []byte, base string) string {
	kind := imageKindOf(body)
	if kind == "" {
		kind = "png"
	}
	return fmt.Sprintf("%s-%s.%s", base, time.Now().Format("20060102-150405.000"), kind)
}
