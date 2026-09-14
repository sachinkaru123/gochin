package router

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func staticRouter(t *testing.T) (*Router, string) {
	t.Helper()

	dir := t.TempDir()
	write := func(name, body string) {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("app.css", "body{color:red}")
	write("js/app.js", "console.log(1)")
	write(".env", "SECRET=do-not-serve")
	write("nested/.git/config", "[core]")

	// A secret one level above the served directory, to prove traversal fails.
	if err := os.WriteFile(filepath.Join(filepath.Dir(dir), "outside.txt"), []byte("TOP SECRET"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := New()
	r.Static("/assets", dir)
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}
	return r, dir
}

func getStatic(r *Router, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestStaticServesFiles(t *testing.T) {
	r, _ := staticRouter(t)

	rec := getStatic(r, "/assets/app.css")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Body.String(); got != "body{color:red}" {
		t.Errorf("body = %q", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}

	if rec := getStatic(r, "/assets/js/app.js"); rec.Code != http.StatusOK {
		t.Errorf("nested file status = %d, want 200", rec.Code)
	}
}

// The files that actually get leaked in the wild.
func TestStaticRefusesDotfilesAndListings(t *testing.T) {
	r, _ := staticRouter(t)

	cases := map[string]string{
		"dotfile":            "/assets/.env",
		"nested dotdir":      "/assets/nested/.git/config",
		"directory":          "/assets/js",
		"directory slash":    "/assets/js/",
		"root directory":     "/assets/",
		"traversal":          "/assets/../outside.txt",
		"encoded traversal":  "/assets/%2e%2e/outside.txt",
		"deep traversal":     "/assets/../../etc/passwd",
		"double dot segment": "/assets/js/../../outside.txt",
	}

	for name, target := range cases {
		t.Run(name, func(t *testing.T) {
			rec := getStatic(r, target)
			if rec.Code == http.StatusOK {
				t.Errorf("%s returned 200 and served: %q", target, rec.Body.String())
			}
			if bytes.Contains(rec.Body.Bytes(), []byte("TOP SECRET")) ||
				bytes.Contains(rec.Body.Bytes(), []byte("do-not-serve")) {
				t.Errorf("%s leaked file contents", target)
			}
		})
	}
}

func TestStaticMissingFileIs404(t *testing.T) {
	r, _ := staticRouter(t)
	if rec := getStatic(r, "/assets/nope.css"); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
}

// --- uploads ---------------------------------------------------------------

func multipartBody(t *testing.T, field, filename string, content []byte) (*bytes.Buffer, string) {
	t.Helper()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf, w.FormDataContentType()
}

// A 1x1 PNG, so DetectContentType reports image/png.
var pngBytes = []byte{
	0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a,
	0x00, 0x00, 0x00, 0x0d, 'I', 'H', 'D', 'R',
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4, 0x89,
}

// recordingDisk captures what would be written, so a test can assert on the
// path without touching the filesystem.
type recordingDisk struct {
	paths []string
}

func (d *recordingDisk) Put(_ context.Context, relPath string, r io.Reader) (int64, error) {
	d.paths = append(d.paths, relPath)
	n, err := io.Copy(io.Discard, r)
	return n, err
}

func uploadRouter(t *testing.T, rules UploadRules, onFile func(*UploadedFile) error) *Router {
	t.Helper()

	r := New(WithMaxUploadBytes(1 << 20))
	r.Post("/upload", func(c *Context) error {
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}
		if err := file.Validate(rules); err != nil {
			return err
		}
		if onFile != nil {
			if err := onFile(file); err != nil {
				return err
			}
		}
		return c.JSON(http.StatusCreated, map[string]string{
			"client_name":  file.ClientName,
			"sniffed_type": file.SniffedType,
		})
	})
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}
	return r
}

func postUpload(r *Router, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestUploadAcceptsValidImage(t *testing.T) {
	r := uploadRouter(t, ImageRules(1<<20), nil)

	body, ct := multipartBody(t, "file", "photo.png", pngBytes)
	rec := postUpload(r, body, ct)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("image/png")) {
		t.Errorf("expected the sniffed type to be image/png, got %s", rec.Body.String())
	}
}

// A script renamed to .jpg passes the extension check and must still be
// rejected on its sniffed content.
func TestUploadRejectsSpoofedContentType(t *testing.T) {
	r := uploadRouter(t, ImageRules(1<<20), nil)

	body, ct := multipartBody(t, "file", "payload.jpg", []byte("<?php system($_GET['c']); ?>"))
	rec := postUpload(r, body, ct)

	if rec.Code == http.StatusCreated {
		t.Fatal("a PHP script renamed to .jpg was accepted")
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422 (body %s)", rec.Code, rec.Body.String())
	}
}

func TestUploadRejectsDisallowedExtension(t *testing.T) {
	r := uploadRouter(t, ImageRules(1<<20), nil)

	body, ct := multipartBody(t, "file", "notes.txt", []byte("hello world"))
	if rec := postUpload(r, body, ct); rec.Code == http.StatusCreated {
		t.Error("a .txt file was accepted under image rules")
	}
}

func TestUploadRejectsOversizedFile(t *testing.T) {
	r := uploadRouter(t, UploadRules{MaxBytes: 100}, nil)

	body, ct := multipartBody(t, "file", "big.png", bytes.Repeat([]byte("x"), 500))
	rec := postUpload(r, body, ct)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413 (body %s)", rec.Code, rec.Body.String())
	}
}

// The client filename must never reach the stored path.
func TestUploadIgnoresClientFilenameForStorage(t *testing.T) {
	var stored string
	disk := &recordingDisk{}

	r := New(WithMaxUploadBytes(1 << 20))
	r.Post("/upload", func(c *Context) error {
		file, err := c.FormFile("file")
		if err != nil {
			return err
		}
		stored, err = file.Store(c.Ctx(), disk, "avatars")
		if err != nil {
			return err
		}
		return c.NoContent(http.StatusCreated)
	})
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	body, ct := multipartBody(t, "file", "../../../etc/cron.d/evil.png", pngBytes)
	if rec := postUpload(r, body, ct); rec.Code != http.StatusCreated {
		t.Fatalf("status = %d (body %s)", rec.Code, rec.Body.String())
	}

	if stored == "" {
		t.Fatal("no path was returned")
	}
	for _, bad := range []string{"..", "etc", "cron.d", "evil"} {
		if bytes.Contains([]byte(stored), []byte(bad)) {
			t.Errorf("stored path %q still contains %q from the client filename", stored, bad)
		}
	}
	if filepath.Ext(stored) != ".png" {
		t.Errorf("stored path %q lost its extension", stored)
	}
}

func TestUploadMissingFieldIsBadRequest(t *testing.T) {
	r := uploadRouter(t, UploadRules{}, nil)

	body, ct := multipartBody(t, "other", "x.png", pngBytes)
	if rec := postUpload(r, body, ct); rec.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", rec.Code)
	}
}
