package router

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
)

// UploadedFile is one file from a multipart request.
//
// ClientName is attacker-controlled and is never used to build a path: it is
// kept only so a caller can display or record what the user called it.
type UploadedFile struct {
	ClientName string
	Size       int64
	// DeclaredType is the client's Content-Type, which cannot be trusted.
	DeclaredType string
	// SniffedType is detected from the file's own first bytes.
	SniffedType string

	header *multipart.FileHeader
}

// UploadRules constrains an accepted upload.
type UploadRules struct {
	MaxBytes int64
	// AllowedTypes are matched against the sniffed media type, e.g.
	// "image/png". Empty allows anything.
	AllowedTypes []string
	// AllowedExtensions are matched against the client filename's extension,
	// lowercase and including the dot. Empty allows anything.
	AllowedExtensions []string
}

// ImageRules is a ready-made rule set for image uploads.
func ImageRules(maxBytes int64) UploadRules {
	return UploadRules{
		MaxBytes:          maxBytes,
		AllowedTypes:      []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		AllowedExtensions: []string{".jpg", ".jpeg", ".png", ".gif", ".webp"},
	}
}

// FormFile reads one uploaded file from a multipart request.
func (c *Context) FormFile(field string) (*UploadedFile, error) {
	maxBytes := c.router.maxUpload
	if maxBytes <= 0 {
		maxBytes = 10 << 20
	}

	// Bound the request before reading any of it.
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)

	if err := c.Request.ParseMultipartForm(4 << 20); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return nil, NewError(http.StatusRequestEntityTooLarge, "upload is too large").Wrap(err)
		}
		return nil, BadRequestf("could not read the uploaded file").Wrap(err)
	}

	file, header, err := c.Request.FormFile(field)
	if err != nil {
		return nil, BadRequestf("no file was uploaded under %q", field).Wrap(err)
	}
	defer file.Close()

	// Sniff the real type from the content, since the declared one is just
	// whatever the client typed into the multipart part.
	head := make([]byte, 512)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, BadRequestf("could not read the uploaded file").Wrap(err)
	}

	return &UploadedFile{
		ClientName:   filepath.Base(header.Filename),
		Size:         header.Size,
		DeclaredType: header.Header.Get("Content-Type"),
		SniffedType:  mediaType(http.DetectContentType(head[:n])),
		header:       header,
	}, nil
}

// Validate applies rules to an uploaded file.
func (f *UploadedFile) Validate(rules UploadRules) error {
	if rules.MaxBytes > 0 && f.Size > rules.MaxBytes {
		return NewError(http.StatusRequestEntityTooLarge, "upload is too large")
	}

	if len(rules.AllowedExtensions) > 0 {
		ext := strings.ToLower(filepath.Ext(f.ClientName))
		if !containsFold(rules.AllowedExtensions, ext) {
			return Unprocessablef("files of type %q are not accepted", ext).
				WithCode("unsupported_file_type")
		}
	}

	// Checked against the sniffed type on purpose: a .php renamed to .jpg
	// passes the extension check but fails here.
	if len(rules.AllowedTypes) > 0 && !containsFold(rules.AllowedTypes, f.SniffedType) {
		return Unprocessablef("file content is %s, which is not accepted", f.SniffedType).
			WithCode("unsupported_file_type")
	}

	return nil
}

// storer is the subset of a storage disk that uploads need, declared here so
// pkg/router does not import pkg/storage.
type storer interface {
	Put(ctx context.Context, relPath string, r io.Reader) (int64, error)
}

// Store writes the upload into dir on the given disk under a generated name,
// returning the stored relative path.
//
// The name is random rather than derived from the client filename: that
// filename is the path-traversal and overwrite vector, and reusing it is the
// most common upload vulnerability.
func (f *UploadedFile) Store(ctx context.Context, disk storer, dir string) (string, error) {
	src, err := f.header.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	name, err := randomFilename(extensionFor(f))
	if err != nil {
		return "", err
	}

	relPath := strings.Trim(dir, "/") + "/" + name
	if dir == "" {
		relPath = name
	}

	if _, err := disk.Put(ctx, relPath, src); err != nil {
		return "", err
	}
	return relPath, nil
}

// Open returns a reader over the uploaded content.
func (f *UploadedFile) Open() (multipart.File, error) { return f.header.Open() }

// extensionFor picks a safe extension, preferring one implied by the sniffed
// type over anything the client supplied.
func extensionFor(f *UploadedFile) string {
	switch f.SniffedType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	case "text/plain":
		return ".txt"
	}

	ext := strings.ToLower(filepath.Ext(f.ClientName))
	// Only a conservative, self-evidently inert set is echoed back.
	if ext != "" && isSafeExtension(ext) {
		return ext
	}
	return ".bin"
}

func isSafeExtension(ext string) bool {
	for _, r := range ext[1:] {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		if !isLower && !isDigit {
			return false
		}
	}
	return len(ext) > 1 && len(ext) <= 6
}

func randomFilename(ext string) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf) + ext, nil
}

// mediaType strips any parameters from a content type.
func mediaType(ct string) string {
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		return strings.TrimSpace(ct[:i])
	}
	return strings.TrimSpace(ct)
}

func containsFold(list []string, want string) bool {
	for _, v := range list {
		if strings.EqualFold(v, want) {
			return true
		}
	}
	return false
}
