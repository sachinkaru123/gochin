package router

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJoinPath(t *testing.T) {
	cases := []struct {
		parts []string
		want  string
	}{
		{[]string{"/api/", "/v1/", "/users"}, "/api/v1/users"},
		{[]string{"", "/users"}, "/users"},
		{[]string{"/api", ""}, "/api"},
		{[]string{"/api", "/"}, "/api"},
		{[]string{"/api/v1", "users/{id}"}, "/api/v1/users/{id}"},
		{[]string{"/api//v1", "/users"}, "/api/v1/users"},
		{[]string{"", ""}, "/"},
		{[]string{"/api", "/v1", "/users", "/{id}"}, "/api/v1/users/{id}"},
	}
	for _, tc := range cases {
		if got := joinPath(tc.parts...); got != tc.want {
			t.Errorf("joinPath(%q) = %q, want %q", tc.parts, got, tc.want)
		}
	}
}

func TestValidatePath(t *testing.T) {
	valid := []string{"/users", "/users/{id}", "/files/{path...}", "/{$}", "/a/b/c"}
	for _, p := range valid {
		if err := validatePath(p); err != nil {
			t.Errorf("validatePath(%q) unexpected error: %v", p, err)
		}
	}

	invalid := []string{
		"users",        // no leading slash
		"/b_{bucket}",  // wildcard not a whole segment
		"/{id}/{id}",   // duplicate wildcard
		"/{a...}/more", // multi wildcard not final
		"/{$}/more",    // {$} not final
		"/{}",          // unnamed
		"/../etc",      // traversal
	}
	for _, p := range invalid {
		if err := validatePath(p); err == nil {
			t.Errorf("validatePath(%q) expected an error, got nil", p)
		}
	}
}

func newTestRouter(t *testing.T) *Router {
	t.Helper()
	r := New(WithPrefix("/api"))
	r.Group("/v1").Get("/users/{id}", func(c *Context) error {
		return c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})
	r.Group("/v1").Post("/users", func(c *Context) error {
		return c.NoContent(http.StatusCreated)
	})
	r.Root().Get("/health", func(c *Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})
	if err := r.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func do(r *Router, method, target string, body string) *httptest.ResponseRecorder {
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, target, nil)
	} else {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRoutingAndParams(t *testing.T) {
	r := newTestRouter(t)

	rec := do(r, http.MethodGet, "/api/v1/users/42", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["id"] != "42" {
		t.Errorf("param id = %q, want 42", got["id"])
	}
}

func TestUnprefixedRoot(t *testing.T) {
	r := newTestRouter(t)
	if rec := do(r, http.MethodGet, "/health", ""); rec.Code != http.StatusOK {
		t.Errorf("/health status = %d, want 200", rec.Code)
	}
	if rec := do(r, http.MethodGet, "/api/health", ""); rec.Code != http.StatusNotFound {
		t.Errorf("/api/health status = %d, want 404", rec.Code)
	}
}

func TestJSONNotFound(t *testing.T) {
	r := newTestRouter(t)
	rec := do(r, http.MethodGet, "/api/v1/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want JSON", ct)
	}
	var env errorEnvelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Code != "not_found" {
		t.Errorf("code = %q, want not_found", env.Error.Code)
	}
}

// The catch-all "/" pattern suppresses ServeMux's built-in 405, so Build()
// must synthesize the method complement. This is the regression test for it.
func TestSynthesizedMethodNotAllowed(t *testing.T) {
	r := newTestRouter(t)
	rec := do(r, http.MethodPatch, "/api/v1/users/42", "")

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	allow := rec.Header().Get("Allow")
	for _, want := range []string{"GET", "HEAD", "OPTIONS"} {
		if !strings.Contains(allow, want) {
			t.Errorf("Allow = %q, missing %s", allow, want)
		}
	}
	if strings.Contains(allow, "PATCH") {
		t.Errorf("Allow = %q, should not advertise PATCH", allow)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want JSON", ct)
	}
}

func TestSynthesizedOptions(t *testing.T) {
	r := newTestRouter(t)
	rec := do(r, http.MethodOptions, "/api/v1/users/42", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow = %q, want GET", allow)
	}
}

func TestMiddlewareOrdering(t *testing.T) {
	var order []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(c *Context) error {
				order = append(order, name)
				return next(c)
			}
		}
	}

	r := New()
	r.Use(mk("global"))
	g := r.Group("/a", mk("group"))
	g.Get("/b", func(c *Context) error {
		order = append(order, "handler")
		return c.NoContent(http.StatusOK)
	}, mk("route"))
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	do(r, http.MethodGet, "/a/b", "")

	want := []string{"global", "group", "route", "handler"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Errorf("order = %v, want %v", order, want)
	}
}

// Sibling groups must not share a middleware backing array.
func TestSiblingGroupsDoNotShareMiddleware(t *testing.T) {
	var hits []string
	mk := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(c *Context) error {
				hits = append(hits, name)
				return next(c)
			}
		}
	}

	r := New()
	parent := r.Group("/p")
	a := parent.Group("/a", mk("a"))
	b := parent.Group("/b", mk("b"))

	ok := func(c *Context) error { return c.NoContent(http.StatusOK) }
	a.Get("/x", ok)
	b.Get("/x", ok)
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	hits = nil
	do(r, http.MethodGet, "/p/b/x", "")
	if len(hits) != 1 || hits[0] != "b" {
		t.Errorf("group /b middleware = %v, want [b] (leaked from sibling?)", hits)
	}
}

func TestErrorMapping(t *testing.T) {
	sentinel := errors.New("domain failure")

	r := New()
	r.MapError(func(err error) *HTTPError {
		if errors.Is(err, sentinel) {
			return Conflictf("already exists")
		}
		return nil
	})
	r.Get("/boom", func(c *Context) error { return sentinel })
	r.Get("/plain", func(c *Context) error { return errors.New("unmapped") })
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	if rec := do(r, http.MethodGet, "/boom", ""); rec.Code != http.StatusConflict {
		t.Errorf("mapped error status = %d, want 409", rec.Code)
	}
	// An unmapped error must not leak its message to the client.
	rec := do(r, http.MethodGet, "/plain", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("unmapped error status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "unmapped") {
		t.Errorf("internal error detail leaked: %s", rec.Body.String())
	}
}

func TestBindRejectsUnknownFieldsAndOversizedBodies(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
	}

	r := New(WithMaxBodyBytes(64))
	r.Post("/p", func(c *Context) error {
		var p payload
		if err := c.Bind(&p); err != nil {
			return err
		}
		return c.JSON(http.StatusOK, p)
	})
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	if rec := do(r, http.MethodPost, "/p", `{"name":"ada"}`); rec.Code != http.StatusOK {
		t.Errorf("valid body status = %d, want 200", rec.Code)
	}
	if rec := do(r, http.MethodPost, "/p", `{"nmae":"typo"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field status = %d, want 400", rec.Code)
	}
	if rec := do(r, http.MethodPost, "/p", `{"name":"`+strings.Repeat("x", 200)+`"}`); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body status = %d, want 413", rec.Code)
	}
	if rec := do(r, http.MethodPost, "/p", `{bad json`); rec.Code != http.StatusBadRequest {
		t.Errorf("malformed body status = %d, want 400", rec.Code)
	}
}

func TestDuplicateRoutePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic on duplicate route registration")
		}
	}()
	r := New()
	h := func(c *Context) error { return nil }
	r.Get("/dup", h)
	r.Get("/dup", h)
}

func TestUseAfterRoutePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("expected a panic when Use follows a route registration")
		}
	}()
	r := New()
	g := r.Group("/g")
	g.Get("/x", func(c *Context) error { return nil })
	g.Use(func(next Handler) Handler { return next })
}

func TestRouteIntrospectionNamesTheHandler(t *testing.T) {
	r := New()
	r.Get("/users", namedHandler)
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	routes := r.Routes(false)
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1 (synthetic entries should be hidden)", len(routes))
	}
	if !strings.Contains(routes[0].Handler, "namedHandler") {
		t.Errorf("handler = %q, want it to mention namedHandler", routes[0].Handler)
	}

	if all := r.Routes(true); len(all) <= 1 {
		t.Errorf("got %d routes with synthetic included, want more than 1", len(all))
	}
}

func namedHandler(c *Context) error { return c.NoContent(http.StatusOK) }

// Context values must not survive into the next request that reuses the
// pooled Context.
func TestContextPoolIsReset(t *testing.T) {
	r := New()
	r.Get("/first", func(c *Context) error {
		c.Set("leak", "value")
		c.SetRequestID("req-1")
		return c.NoContent(http.StatusOK)
	})
	r.Get("/second", func(c *Context) error {
		if v, ok := c.Get("leak"); ok {
			t.Errorf("value %v leaked from a previous request", v)
		}
		if c.RequestID() != "" {
			t.Errorf("request id %q leaked from a previous request", c.RequestID())
		}
		return c.NoContent(http.StatusOK)
	})
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 50; i++ {
		do(r, http.MethodGet, "/first", "")
		do(r, http.MethodGet, "/second", "")
	}
}
