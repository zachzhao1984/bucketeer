package main

import "errors"

// readSecret returns one small secret from the app's local file store.
func (s *Service) readSecret(key string) ([]byte, error) {
	s.mu.Lock()
	secrets := s.secrets
	s.mu.Unlock()
	if secrets == nil {
		return nil, errors.New("no credential store")
	}
	return secrets.Get(key)
}

// writeSecret stores one small secret in the app's local file store.
func (s *Service) writeSecret(key string, value []byte) error {
	s.mu.Lock()
	secrets := s.secrets
	s.mu.Unlock()
	if secrets == nil {
		return errors.New("no credential store")
	}
	return secrets.Set(key, value)
}
