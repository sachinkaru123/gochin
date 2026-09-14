package router

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type benchPayload struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

func benchRouter(tb testing.TB, mw ...Middleware) *Router {
	tb.Helper()

	// Keep log output off the measurement.
	r := New(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	r.Use(mw...)
	r.Get("/users/{id}", func(c *Context) error {
		return c.JSON(http.StatusOK, benchPayload{ID: 1, Name: "Ada", Email: "ada@example.com"})
	})
	r.Post("/users", func(c *Context) error {
		var p benchPayload
		if err := c.Bind(&p); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, p)
	})
	if err := r.Build(); err != nil {
		tb.Fatal(err)
	}
	return r
}

// discardWriter avoids httptest.ResponseRecorder's buffer growth showing up
// as router allocations.
type discardWriter struct{ header http.Header }

func (d *discardWriter) Header() http.Header {
	if d.header == nil {
		d.header = make(http.Header)
	}
	return d.header
}
func (d *discardWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *discardWriter) WriteHeader(int)             {}

// BenchmarkRouteDispatch is the bare path: match, pool a Context, encode JSON.
func BenchmarkRouteDispatch(b *testing.B) {
	r := benchRouter(b)
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkRouteDispatchFullChain adds the middleware stack the server
// actually runs, so the two numbers isolate middleware cost.
func BenchmarkRouteDispatchFullChain(b *testing.B) {
	noop := func(name string) Middleware {
		return func(next Handler) Handler {
			return func(c *Context) error { return next(c) }
		}
	}
	r := benchRouter(b, noop("a"), noop("b"), noop("c"), noop("d"), noop("e"), noop("f"))

	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func BenchmarkContextPool(b *testing.B) {
	r := benchRouter(b)
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := r.acquire(w, req)
		c.release()
	}
}

func BenchmarkJSONResponse(b *testing.B) {
	r := benchRouter(b)
	req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
	w := &discardWriter{}

	payload := benchPayload{ID: 1, Name: "Ada", Email: "ada@example.com"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := r.acquire(w, req)
		_ = c.JSON(http.StatusOK, payload)
		c.release()
	}
}

func BenchmarkBind(b *testing.B) {
	r := benchRouter(b)
	body := `{"id":1,"name":"Ada","email":"ada@example.com"}`
	w := &discardWriter{}

	// Build the request once; re-point its body each iteration so the
	// measurement is Bind, not request construction.
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(body))
	reader := strings.NewReader(body)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(body)
		req.Body = io.NopCloser(reader)
		c := r.acquire(w, req)
		var p benchPayload
		_ = c.Bind(&p)
		c.release()
	}
}

func BenchmarkErrorPath(b *testing.B) {
	r := New(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	r.Get("/fail", func(c *Context) error { return NotFoundf("nope") })
	if err := r.Build(); err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkNotFound measures the synthesized catch-all.
func BenchmarkNotFound(b *testing.B) {
	r := benchRouter(b)
	req := httptest.NewRequest(http.MethodGet, "/no/such/route", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

// BenchmarkManyRoutes checks that lookup does not degrade with route count.
func BenchmarkManyRoutes(b *testing.B) {
	r := New(WithLogger(slog.New(slog.NewTextHandler(io.Discard, nil))))
	h := func(c *Context) error { return c.NoContent(http.StatusOK) }
	for i := 0; i < 200; i++ {
		r.Get("/resource"+string(rune('a'+i%26))+"/"+itoaBench(i)+"/{id}", h)
	}
	if err := r.Build(); err != nil {
		b.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/resourcea/0/42", nil)
	w := &discardWriter{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
	}
}

func itoaBench(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// BenchmarkRouteDispatchParallel checks for lock contention in the hot path:
// the Context pool, the schema cache and the logger are all shared.
func BenchmarkRouteDispatchParallel(b *testing.B) {
	r := benchRouter(b)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/users/42", nil)
		w := &discardWriter{}
		for pb.Next() {
			r.ServeHTTP(w, req)
		}
	})
}
