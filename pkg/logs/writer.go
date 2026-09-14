package logs

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// dailyWriter appends to a file named after the current date, reopening when
// the date rolls over.
//
// Rotation by filename rather than by size keeps this dependency-free: there
// is no index shuffling and no half-written rename to recover from.
type dailyWriter struct {
	mu      sync.Mutex
	dir     string
	prefix  string
	day     string
	file    *os.File
	nowFunc func() time.Time
}

func newDailyWriter(dir, prefix string) (*dailyWriter, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("logs: creating %s: %w", dir, err)
	}
	return &dailyWriter{dir: dir, prefix: prefix, nowFunc: time.Now}, nil
}

// Write is called from every request goroutine, so the whole rotate-and-write
// sequence is guarded.
func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.rotateLocked(); err != nil {
		return 0, err
	}
	return w.file.Write(p)
}

func (w *dailyWriter) rotateLocked() error {
	day := w.nowFunc().Format("2006-01-02")
	if w.file != nil && day == w.day {
		return nil
	}

	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	path := filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, day))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("logs: opening %s: %w", path, err)
	}

	w.file = f
	w.day = day
	return nil
}

// Path returns the file currently being written to.
func (w *dailyWriter) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return filepath.Join(w.dir, fmt.Sprintf("%s-%s.log", w.prefix, w.nowFunc().Format("2006-01-02")))
}

func (w *dailyWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// multiWriter fans writes out to several destinations, tolerating a failure
// of one: losing the file must not also lose stdout.
type multiWriter struct {
	writers []interface{ Write([]byte) (int, error) }
}

func (m multiWriter) Write(p []byte) (int, error) {
	var firstErr error
	for _, w := range m.writers {
		if _, err := w.Write(p); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return len(p), firstErr
}
