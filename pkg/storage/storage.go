// Package storage stores application files behind a driver interface, so the
// local filesystem today can become object storage later without changing
// call sites.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when a stored file does not exist.
var ErrNotFound = errors.New("storage: file not found")

// ErrUnsafePath is returned for a path that escapes the disk root.
var ErrUnsafePath = errors.New("storage: unsafe path")

// Disk is a place files are kept.
type Disk interface {
	Put(ctx context.Context, relPath string, r io.Reader) (int64, error)
	Get(ctx context.Context, relPath string) (io.ReadCloser, error)
	Delete(ctx context.Context, relPath string) error
	Exists(ctx context.Context, relPath string) (bool, error)
	// Path resolves a stored file to an absolute filesystem path.
	Path(relPath string) (string, error)
}

// LocalDisk stores files under a root directory.
type LocalDisk struct {
	root string
}

// NewLocal opens (creating if needed) a disk rooted at dir.
func NewLocal(dir string) (*LocalDisk, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, fmt.Errorf("storage: creating %s: %w", abs, err)
	}
	return &LocalDisk{root: abs}, nil
}

// Root returns the disk's base directory.
func (d *LocalDisk) Root() string { return d.root }

// resolve turns a relative path into an absolute one, refusing anything that
// would escape the root.
//
// This is the single choke point for traversal: every other method goes
// through it, so "../../etc/passwd" cannot be reached even if a caller passes
// an attacker-controlled string.
func (d *LocalDisk) resolve(relPath string) (string, error) {
	clean := path.Clean("/" + strings.ReplaceAll(relPath, "\\", "/"))
	if clean == "/" {
		return "", ErrUnsafePath
	}

	full := filepath.Join(d.root, filepath.FromSlash(clean))

	// Belt and braces: confirm the result is still inside the root after
	// symlinks and Join's own cleaning.
	rel, err := filepath.Rel(d.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrUnsafePath
	}
	return full, nil
}

// Path resolves a stored file to an absolute filesystem path.
func (d *LocalDisk) Path(relPath string) (string, error) { return d.resolve(relPath) }

// Put writes r to relPath, creating parent directories as needed.
func (d *LocalDisk) Put(ctx context.Context, relPath string, r io.Reader) (int64, error) {
	full, err := d.resolve(relPath)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return 0, err
	}

	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	n, err := io.Copy(f, r)
	if err != nil {
		// Do not leave a half-written file behind.
		_ = os.Remove(full)
		return 0, err
	}
	return n, nil
}

// Get opens a stored file for reading.
func (d *LocalDisk) Get(ctx context.Context, relPath string) (io.ReadCloser, error) {
	full, err := d.resolve(relPath)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(full)
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrNotFound
	}
	return f, err
}

// Delete removes a stored file. Removing a missing file is not an error.
func (d *LocalDisk) Delete(ctx context.Context, relPath string) error {
	full, err := d.resolve(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Exists reports whether a stored file is present.
func (d *LocalDisk) Exists(ctx context.Context, relPath string) (bool, error) {
	full, err := d.resolve(relPath)
	if err != nil {
		return false, err
	}

	_, err = os.Stat(full)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

var _ Disk = (*LocalDisk)(nil)
