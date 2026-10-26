package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// errSecretNotFound says the store does not hold the key.
var errSecretNotFound = errors.New("secret not found")

// fileSecrets keeps credentials in a JSON file beside the app's other data,
// mode 0600. It never reads or writes the system credential store.
//
// The file holds one JSON object mapping key to base64 value. A whole-file
// read-modify-write under a mutex is correct for the sizes involved.
type fileSecrets struct {
	path string
	mu   sync.Mutex
}

// newFileSecrets builds the store rooted at dir, next to connections.json.
func newFileSecrets(dir string) *fileSecrets {
	return &fileSecrets{path: filepath.Join(dir, "secrets.json")}
}

// load reads the whole store. A missing file is an empty store.
func (s *fileSecrets) load() (map[string][]byte, error) {
	out := map[string][]byte{}
	b, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(s.path), err)
	}
	var encoded map[string]string
	if err := json.Unmarshal(b, &encoded); err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(s.path), err)
	}
	for k, v := range encoded {
		raw, err := base64.StdEncoding.DecodeString(v)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.Base(s.path), err)
		}
		out[k] = raw
	}
	return out, nil
}

// save writes the whole store atomically, mode 0600.
func (s *fileSecrets) save(items map[string][]byte) error {
	encoded := make(map[string]string, len(items))
	for k, v := range items {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}
	b, err := json.MarshalIndent(encoded, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Set stores a secret.
func (s *fileSecrets) Set(key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load()
	if err != nil {
		return err
	}
	items[key] = value
	return s.save(items)
}

// Get reads a secret, or store.ErrSecretNotFound when it is not there.
func (s *fileSecrets) Get(key string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load()
	if err != nil {
		return nil, err
	}
	value, ok := items[key]
	if !ok {
		return nil, fmt.Errorf("read %s: %w", key, errSecretNotFound)
	}
	return value, nil
}

// Delete drops a secret; deleting one that is not there is fine.
func (s *fileSecrets) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	items, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := items[key]; !ok {
		return nil
	}
	delete(items, key)
	return s.save(items)
}
