package router

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Static serves the files in dir under urlPrefix.
//
//	r.Static("/assets", "public")   // GET /assets/app.css -> public/app.css
//
// Directory listings and dotfiles are refused, and responses carry
// X-Content-Type-Options: nosniff.
func (rt *Router) Static(urlPrefix, dir string) {
	prefix := joinPath(urlPrefix)
	fs := http.FileServer(safeFileSystem{root: http.Dir(dir)})
	stripped := http.StripPrefix(prefix, fs)

	// Registered against the root, not through Handle: assets must not be
	// pushed under the configured API prefix (/api/assets/... would be wrong).
	//
	// A trailing multi-wildcard captures the whole subtree. Build() skips
	// synthesizing 405s for these, so nothing here shadows other routes.
	rt.handle(http.MethodGet, joinPath(prefix, "{path...}"), func(c *Context) error {
		// Reject dotfiles before touching the filesystem: .env and .git are
		// the files that actually matter here.
		if hasDotSegment(c.Param("path")) {
			return NotFoundf("not found")
		}

		c.Header().Set("X-Content-Type-Options", "nosniff")

		// The file server writes the response itself, bypassing
		// Context.WriteHeader, so capture the status it chose. Without this
		// every static 404 would be logged as a 200.
		rec := &statusRecorder{ResponseWriter: c.Writer, status: http.StatusOK}
		stripped.ServeHTTP(rec, c.Request)

		c.wroteHeader = true
		c.status = rec.status
		return nil
	}, nil, prefix)
}

// statusRecorder remembers the status written by a wrapped handler.
type statusRecorder struct {
	http.ResponseWriter
	status  int
	written bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.written {
		r.status = status
		r.written = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.written = true
	return r.ResponseWriter.Write(b)
}

// safeFileSystem refuses directories and dotfiles.
//
// http.Dir already blocks traversal above the root, but it happily serves
// directory listings and hidden files, which is how .env and .git/config get
// exposed.
type safeFileSystem struct {
	root http.FileSystem
}

func (fs safeFileSystem) Open(name string) (http.File, error) {
	if hasDotSegment(name) {
		return nil, os.ErrNotExist
	}

	f, err := fs.root.Open(name)
	if err != nil {
		return nil, err
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	if info.IsDir() {
		// No directory listings, and no implicit index.html either: serving
		// one is a decision an application should make explicitly.
		_ = f.Close()
		return nil, os.ErrNotExist
	}

	return f, nil
}

// hasDotSegment reports whether any path segment starts with a dot.
func hasDotSegment(name string) bool {
	for _, seg := range strings.Split(filepath.ToSlash(name), "/") {
		if strings.HasPrefix(seg, ".") && seg != "" {
			return true
		}
	}
	return false
}
