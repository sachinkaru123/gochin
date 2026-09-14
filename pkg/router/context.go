package router

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Context carries one request/response pair plus the helpers handlers use.
//
// A Context is pooled and reused across requests: never retain one (or
// anything derived from its Request) after the handler returns.
type Context struct {
	Writer  http.ResponseWriter
	Request *http.Request

	router      *Router
	status      int
	wroteHeader bool
	requestID   string
	values      map[string]any
}

var ctxPool = sync.Pool{
	New: func() any { return new(Context) },
}

func (rt *Router) acquire(w http.ResponseWriter, r *http.Request) *Context {
	c := ctxPool.Get().(*Context)
	c.Writer = w
	c.Request = r
	c.router = rt
	c.status = 0
	c.wroteHeader = false
	c.requestID = ""
	return c
}

func (c *Context) release() {
	c.Writer = nil
	c.Request = nil
	c.router = nil
	c.values = nil
	ctxPool.Put(c)
}

// Ctx returns the request's context, which carries cancellation and deadlines
// down into the ORM and any outbound calls.
func (c *Context) Ctx() context.Context { return c.Request.Context() }

// SetRequest replaces the underlying request, e.g. to attach a new context.
func (c *Context) SetRequest(r *http.Request) { c.Request = r }

// Header returns the response header map.
func (c *Context) Header() http.Header { return c.Writer.Header() }

// Written reports whether the response head has already been sent.
func (c *Context) Written() bool { return c.wroteHeader }

// Status returns the status already written, or 0.
func (c *Context) Status() int { return c.status }

// RequestID returns the id assigned by the RequestID middleware, if any.
func (c *Context) RequestID() string { return c.requestID }

// SetRequestID records the request id so error envelopes can include it.
func (c *Context) SetRequestID(id string) { c.requestID = id }

// Set stores a per-request value for downstream middleware and handlers.
func (c *Context) Set(key string, value any) {
	if c.values == nil {
		c.values = make(map[string]any, 4)
	}
	c.values[key] = value
}

// Get retrieves a value stored by Set.
func (c *Context) Get(key string) (any, bool) {
	v, ok := c.values[key]
	return v, ok
}

// Param returns a path wildcard, e.g. "id" for the pattern "/users/{id}".
func (c *Context) Param(name string) string { return c.Request.PathValue(name) }

// ParamInt parses a path wildcard as an int64, returning a 400 HTTPError when
// it is missing or malformed.
func (c *Context) ParamInt(name string) (int64, error) {
	raw := c.Request.PathValue(name)
	if raw == "" {
		return 0, BadRequestf("missing path parameter %q", name)
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, BadRequestf("path parameter %q must be an integer", name)
	}
	return v, nil
}

// Query returns a query string value, or def when absent or empty.
func (c *Context) Query(name string, def ...string) string {
	if v := c.Request.URL.Query().Get(name); v != "" {
		return v
	}
	if len(def) > 0 {
		return def[0]
	}
	return ""
}

// QueryInt returns a query string value parsed as an int, falling back to def
// when absent or malformed.
func (c *Context) QueryInt(name string, def int) int {
	raw := c.Request.URL.Query().Get(name)
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return def
	}
	return v
}

// Bind decodes a JSON request body into dst.
//
// The body is capped by the router's MaxBodyBytes and unknown fields are
// rejected, so a typo in a client payload surfaces as a 400 rather than being
// silently dropped.
func (c *Context) Bind(dst any) error {
	if c.Request.Body == nil {
		return BadRequestf("request body is empty")
	}

	limit := c.router.maxBody
	if limit > 0 {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	}

	dec := json.NewDecoder(c.Request.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return NewError(http.StatusRequestEntityTooLarge, "request body too large").Wrap(err)
		}
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			return BadRequestf("malformed JSON at byte %d", syntaxErr.Offset).Wrap(err)
		}
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return BadRequestf("field %q must be of type %s", typeErr.Field, typeErr.Type).Wrap(err)
		}
		return BadRequestf("%s", err.Error()).Wrap(err)
	}

	// A second value means the client sent more than one JSON document.
	if err := dec.Decode(&struct{}{}); err == nil {
		return BadRequestf("request body must contain a single JSON object")
	}

	return nil
}

// WriteHeader sends the response head exactly once; later calls are ignored
// and reported, which prevents the "superfluous WriteHeader" class of bug.
func (c *Context) WriteHeader(status int) {
	if c.wroteHeader {
		return
	}
	c.wroteHeader = true
	c.status = status
	c.Writer.WriteHeader(status)
}

// JSON writes v as a JSON response body.
func (c *Context) JSON(status int, v any) error {
	c.Header().Set("Content-Type", "application/json; charset=utf-8")
	c.WriteHeader(status)
	// Encode straight to the wire rather than buffering via json.Marshal.
	return json.NewEncoder(c.Writer).Encode(v)
}

// String writes a plain text response body.
func (c *Context) String(status int, format string, a ...any) error {
	c.Header().Set("Content-Type", "text/plain; charset=utf-8")
	c.WriteHeader(status)
	_, err := fmt.Fprintf(c.Writer, format, a...)
	return err
}

// HTML writes an HTML response body.
func (c *Context) HTML(status int, body []byte) error {
	c.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.WriteHeader(status)
	_, err := c.Writer.Write(body)
	return err
}

// NoContent replies with a status and an empty body.
func (c *Context) NoContent(status int) error {
	c.WriteHeader(status)
	return nil
}

// Redirect replies with a redirect to url.
func (c *Context) Redirect(status int, url string) error {
	http.Redirect(c.Writer, c.Request, url, status)
	c.wroteHeader = true
	c.status = status
	return nil
}

// ClientIP returns the best-effort client address, honouring X-Forwarded-For
// only when the router is configured to trust proxy headers.
func (c *Context) ClientIP() string {
	if c.router != nil && c.router.trustProxy {
		if xff := c.Request.Header.Get("X-Forwarded-For"); xff != "" {
			if i := strings.IndexByte(xff, ','); i >= 0 {
				return strings.TrimSpace(xff[:i])
			}
			return strings.TrimSpace(xff)
		}
		if rip := c.Request.Header.Get("X-Real-Ip"); rip != "" {
			return strings.TrimSpace(rip)
		}
	}
	host, _, err := net.SplitHostPort(c.Request.RemoteAddr)
	if err != nil {
		return c.Request.RemoteAddr
	}
	return host
}
