package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/egoist/mygo"
)

// The window and the command line have to read the same credential-store
// entries, but MyGo names the store after the build: the identifier
// (`dev.bucketeer.app`), its development form (`dev.bucketeer.app.dev`),
// or the app's name when it was run as a plain binary (`Bucketeer`,
// `bucketeer-dev`, the executable's own name). A command line started any
// of those ways must find the key the window wrote, wherever it is.
//
// sharedSecrets reads and writes under the service the app uses when it
// runs as the packaged app, and falls back to the others, so a key written
// by any build is readable by all of them.

// secretServices are the credential-store services the app's keys can be
// under, most likely first.
func secretServices() []string {
	out := []string{appIdentifier}
	if exe, err := os.Executable(); err == nil && exe != "" {
		// mygo names the store after the executable when the binary is not
		// packaged, so a `go run` build and an installed copy differ.
		base := nameWithoutExt(exe)
		if base != "" && base != appIdentifier {
			out = append(out, base)
		}
	}
	out = append(out,
		appIdentifier+".dev",
		// The names this app has been built under, so keys written before
		// the identifier was fixed still open.
		"Bucketeer", "Bucketeer Dev", "bucketeer-dev", "bucketeer",
	)
	return dedupe(out)
}

// nameWithoutExt returns a path's file name, without a Windows extension.
func nameWithoutExt(path string) string {
	base := path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			base = path[i+1:]
			break
		}
	}
	for _, ext := range []string{".exe", ".test"} {
		if len(base) > len(ext) && base[len(base)-len(ext):] == ext {
			return base[:len(base)-len(ext)]
		}
	}
	return base
}

// dedupe drops empty and repeated names, keeping the order.
func dedupe(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// sharedSecrets is the credential store the app and its command line share.
type sharedSecrets struct{}

// Set stores a secret under every service, so a key written now opens from
// any way the program is started.
func (sharedSecrets) Set(key string, value []byte) error {
	services := secretServices()
	var firstErr error
	for i, service := range services {
		err := mygo.Secrets.Service(service).Set(key, value)
		if err != nil && i == 0 {
			firstErr = err
		}
	}
	return firstErr
}

// Get reads the secret from the first service that has it.
func (sharedSecrets) Get(key string) ([]byte, error) {
	var lastErr error
	for _, service := range secretServices() {
		value, err := mygo.Secrets.Service(service).Get(key)
		if err == nil {
			return value, nil
		}
		if !errors.Is(err, mygo.ErrSecretNotFound) {
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("read %s: %w", key, mygo.ErrSecretNotFound)
}

// Delete removes the secret wherever it was stored.
func (sharedSecrets) Delete(key string) error {
	var firstErr error
	for _, service := range secretServices() {
		if err := mygo.Secrets.Service(service).Delete(key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
