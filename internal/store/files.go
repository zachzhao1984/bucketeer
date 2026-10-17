package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// copyToFile streams r into a file at dest, creating parent directories.
// It writes to a temporary file next to dest and renames it into place, so
// an interrupted download never leaves a half-written file under the
// destination's name.
func copyToFile(ctx context.Context, r io.Reader, dest string, total int64, progress func(Progress)) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(dest), err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".download-*")
	if err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(dest), err)
	}
	name := tmp.Name()
	defer func() {
		if name != "" {
			os.Remove(name)
		}
	}()

	reader := progressReader(ctx, r, total, progress)
	if _, err := io.Copy(tmp, reader); err != nil {
		tmp.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("write %s: %w", filepath.Base(dest), err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(dest), err)
	}
	if err := os.Rename(name, dest); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(dest), err)
	}
	name = "" // renamed; the deferred cleanup must not remove it
	return nil
}

// openFile opens a local file for reading.
func openFile(path string) (*os.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return f, nil
}

// UniqueLocalPath returns a path in dir for the file name, adding " 2",
// " 3" and so on before the extension when something already has that
// name.
func UniqueLocalPath(dir, name string) string {
	name = filepath.Base(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "download"
	}
	dest := filepath.Join(dir, name)
	if !exists(dest) {
		return dest
	}
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for i := 2; i < 10_000; i++ {
		dest = filepath.Join(dir, fmt.Sprintf("%s %d%s", base, i, ext))
		if !exists(dest) {
			return dest
		}
	}
	return dest
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ErrCanceled reports that a transfer was canceled by the user.
var ErrCanceled = errors.New("canceled")
