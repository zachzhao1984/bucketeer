package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"bucketeer/internal/store"
)

// The performing side of the upload service: what the queue's requests
// become once the app has them. The public URL of an uploaded image is the
// part Obsidian and Typora paste into the note, so it is built with care:
// the profile's public base URL when it has one, else a presigned link.

// profilePublicBase returns the public base URL of a connection: the origin
// its bucket is served at, which a person sets once in the connection form.
// Empty means presigned links.
func (s *Service) profilePublicBase(profileID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.settings.PicGo.PublicBases[profileID]
}

// SetPublicBase records the public base URL of a connection, checked before
// it is stored.
func (s *Service) SetPublicBase(profileID, raw string) (string, error) {
	base, err := PublicBaseURLFromEnv(raw)
	if err != nil {
		return "", err
	}
	s.mu.Lock()
	if s.settings.PicGo.PublicBases == nil {
		s.settings.PicGo.PublicBases = map[string]string{}
	}
	if base == "" {
		delete(s.settings.PicGo.PublicBases, profileID)
	} else {
		s.settings.PicGo.PublicBases[profileID] = base
	}
	settings := s.settings
	s.mu.Unlock()
	// The service owns its own settings; persisting here keeps the page's
	// writes and the service's from racing through a channel.
	if err := s.SetSettings(settings); err != nil {
		return "", err
	}
	return base, nil
}

// picgoProfile returns the connection uploads go to: the one the settings
// name, or the one the app has open. Without either there is nothing to
// upload into.
func (s *Service) picgoProfile() (*store.Profile, error) {
	s.mu.Lock()
	want := s.settings.PicGo.ProfileID
	session := s.session
	s.mu.Unlock()
	if session != nil {
		p := session.Profile
		if want == "" || want == p.ID {
			return &p, nil
		}
	}
	if want != "" {
		if p, err := s.profiles.Get(want); err == nil {
			return &p, nil
		}
		return nil, errors.New("the connection the upload service points at no longer exists")
	}
	return nil, errors.New("no active connection. Connect a profile first.")
}

// publicImageURL is the URL a note gets: the profile's public base URL
// joined with the key, or a presigned link that works for the settings'
// lifetime. R2 and friends serve an object at base/key, so the join is
// plain string work with a slash between.
func (s *Service) publicImageURL(profile *store.Profile, key string) (string, error) {
	if base := s.profilePublicBase(profile.ID); base != "" {
		return joinURL(base, key), nil
	}
	s.mu.Lock()
	hours := s.settings.PresignTTLHours
	s.mu.Unlock()
	session, err := s.currentSession()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return session.Presign(ctx, profile.Bucket, key, time.Duration(hours)*time.Hour)
}

// joinURL joins a base URL and an object key with exactly one slash.
func joinURL(base, key string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(key, "/")
}

// performPicGoUploads is the sink the queue calls. It opens (or reuses) the
// connection the settings name, writes every image, and reports a URL per
// image, in the order they arrived.
func (s *Service) performPicGoUploads(req store.UploadRequest) store.UploadResult {
	profile, err := s.picgoProfile()
	if err != nil {
		return failedAll(req.Images, err.Error())
	}
	if profile.Bucket == "" {
		return failedAll(req.Images, errors.New("this connection has no bucket: set one in Bucketeer, in Settings → Connections").Error())
	}
	if len(req.Images) == 0 {
		return failedAll(req.Images, store.ErrNoUploads.Error())
	}

	// A request names its profile explicitly, so a request for another
	// connection must open that one rather than write through the wrong
	// credentials.
	session, err := s.sessionFor(profile)
	if err != nil {
		return failedAll(req.Images, err.Error())
	}

	details := make([]store.UploadOutcome, 0, len(req.Images))
	urls := make([]string, 0, len(req.Images))
	ok := true
	now := time.Now()

	for _, image := range req.Images {
		outcome := s.uploadOneImage(session, profile, req.Prefix, image, now)
		details = append(details, outcome)
		if outcome.Error != "" {
			ok = false
			continue
		}
		urls = append(urls, outcome.URL)
	}

	return store.UploadResult{OK: ok, URLs: urls, Details: details}
}

// uploadOneImage writes one image and builds its URL. An error is recorded
// per image, so one bad file does not sink the rest of the request.
func (s *Service) uploadOneImage(session *store.Session, profile *store.Profile, prefix string, image store.UploadJob, now time.Time) store.UploadOutcome {
	outcome := store.UploadOutcome{Name: image.Name}

	body := image.Body
	if image.SrcPath != "" {
		data, err := readUploadFile(image.SrcPath)
		if err != nil {
			outcome.Error = err.Error()
			return outcome
		}
		body = data
	}
	if len(body) == 0 {
		outcome.Error = "the image is empty"
		return outcome
	}
	if len(body) > MaxImageBytes {
		outcome.Error = fmt.Sprintf("images are limited to %d MiB", MaxImageBytes>>20)
		return outcome
	}

	name, err := store.SanitizeImageName(image.Name)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}

	key := store.ImageKey(prefix, name, now)
	key = store.UniqueKey(context.Background(), session, profile.Bucket, key)

	size, err := store.UploadImage(context.Background(), session, profile.Bucket, key, body)
	if err != nil {
		outcome.Error = err.Error()
		return outcome
	}
	outcome.Key, outcome.Bytes = key, size

	url, err := s.publicImageURL(profile, key)
	if err != nil {
		outcome.Error = "uploaded, but the public URL is unavailable: " + err.Error()
		return outcome
	}
	outcome.URL = url
	return outcome
}

// sessionFor returns a session for the profile, reusing the one the window
// opened when it already is the connection the upload goes through, so a
// notebook's rapid uploads share it.
func (s *Service) sessionFor(profile *store.Profile) (*store.Session, error) {
	s.mu.Lock()
	current := s.session
	closed := s.closed
	s.mu.Unlock()
	if closed {
		return nil, errors.New("the app is closing")
	}
	if current != nil && current.Profile.ID == profile.ID {
		return current, nil
	}
	// A different profile is opened for this request alone; storage calls
	// carry no state, so the session needs no closing beyond the request.
	return s.profiles.Open(context.Background(), profile.ID)
}

// readUploadFile reads a path-based image with the same bounds the request
// bodies have.
func readUploadFile(path string) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("could not read %s", filepath.Base(path))
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a folder", filepath.Base(path))
	}
	if info.Size() > MaxImageBytes {
		return nil, fmt.Errorf("%s is larger than %d MiB", filepath.Base(path), MaxImageBytes>>20)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read %s", filepath.Base(path))
	}
	return data, nil
}

// failedAll builds a result that says the same thing about every image.
func failedAll(images []store.UploadJob, message string) store.UploadResult {
	details := make([]store.UploadOutcome, 0, len(images))
	for _, image := range images {
		details = append(details, store.UploadOutcome{Name: image.Name, Error: message})
	}
	return store.UploadResult{OK: false, Details: details}
}

// PublicBaseURLFromEnv is a custom domain written as https://cdn.example.com,
// with no path, credentials or query: the shape a public base must have to
// be joined with a key. Empty means the profile has none.
func PublicBaseURLFromEnv(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", errors.New("the public base URL must be an http or https address")
	}
	if u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("the public base URL is a bare origin, as https://cdn.example.com")
	}
	return u.Scheme + "://" + u.Host, nil
}

// picgoQueueSinkIsNil reports whether the queue has nowhere to hand
// requests, for a test.
func (s *Service) picgoQueueSinkIsNil() bool {
	return !s.picgoQueue.HasSink()
}
