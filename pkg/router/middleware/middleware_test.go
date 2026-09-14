package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sachinkaru123/gochin/pkg/router"
	mw "github.com/sachinkaru123/gochin/pkg/router/middleware"
)

func serve(t *testing.T, build func(*router.Router)) *router.Router {
	t.Helper()
	r := router.New()
	build(r)
	if err := r.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func get(r *router.Router, target string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
	return rec
}

func TestRecoverTurnsPanicIntoJSON500(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.Recover(false))
		r.Get("/boom", func(c *router.Context) error {
			panic("kaboom")
		})
	})

	rec := get(r, "/boom")

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content-type = %q, want JSON", ct)
	}
	// The panic value must not reach the client outside debug mode.
	if strings.Contains(rec.Body.String(), "kaboom") {
		t.Errorf("panic detail leaked to client: %s", rec.Body.String())
	}

	// The server must keep serving after a panic.
	if rec := get(r, "/boom"); rec.Code != http.StatusInternalServerError {
		t.Errorf("second request status = %d, want 500", rec.Code)
	}
}

// http.ErrAbortHandler is the stdlib's silent-abort signal and must not be
// swallowed by Recover.
func TestRecoverRepanicsAbortHandler(t *testing.T) {
	defer func() {
		if p := recover(); p != http.ErrAbortHandler {
			t.Errorf("recovered %v, want http.ErrAbortHandler to propagate", p)
		}
	}()

	r := serve(t, func(r *router.Router) {
		r.Use(mw.Recover(false))
		r.Get("/abort", func(c *router.Context) error {
			panic(http.ErrAbortHandler)
		})
	})

	get(r, "/abort")
}

func TestRequestIDIsGeneratedAndEchoed(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.RequestID(false))
		r.Get("/x", func(c *router.Context) error {
			if c.RequestID() == "" {
				t.Error("handler saw an empty request id")
			}
			return c.NoContent(http.StatusOK)
		})
	})

	rec := get(r, "/x")
	if id := rec.Header().Get(mw.HeaderRequestID); id == "" {
		t.Error("response is missing the request id header")
	}
}

// An untrusted inbound id must be replaced, or clients can poison log
// correlation by picking their own.
func TestRequestIDIgnoresUntrustedInbound(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.RequestID(false))
		r.Get("/x", func(c *router.Context) error { return c.NoContent(http.StatusOK) })
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set(mw.HeaderRequestID, "attacker-chosen")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get(mw.HeaderRequestID); got == "attacker-chosen" {
		t.Error("untrusted inbound request id was accepted")
	}
}

func TestRateLimitReturns429WithRetryAfter(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.RateLimit(mw.RateLimitConfig{RequestsPerMinute: 60, Burst: 3}))
		r.Get("/x", func(c *router.Context) error { return c.NoContent(http.StatusOK) })
	})

	var limited *httptest.ResponseRecorder
	for i := 0; i < 10; i++ {
		rec := get(r, "/x")
		if rec.Code == http.StatusTooManyRequests {
			limited = rec
			break
		}
	}

	if limited == nil {
		t.Fatal("burst of 3 was never rate limited")
	}
	if limited.Header().Get("Retry-After") == "" {
		t.Error("429 response is missing Retry-After")
	}
	if limited.Header().Get("X-RateLimit-Limit") == "" {
		t.Error("429 response is missing X-RateLimit-Limit")
	}
}

func TestRateLimitDisabledWhenZero(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.RateLimit(mw.RateLimitConfig{RequestsPerMinute: 0}))
		r.Get("/x", func(c *router.Context) error { return c.NoContent(http.StatusOK) })
	})

	for i := 0; i < 50; i++ {
		if rec := get(r, "/x"); rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200 (limiting should be off)", i, rec.Code)
		}
	}
}

func TestCORSPreflightAndOriginEcho(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.CORS(mw.CORSConfig{AllowedOrigins: []string{"https://example.com"}}))
		r.Get("/x", func(c *router.Context) error { return c.NoContent(http.StatusOK) })
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://example.com")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://example.com" {
		t.Errorf("allow-origin = %q, want the allowed origin echoed", got)
	}
	if !strings.Contains(rec.Header().Get("Vary"), "Origin") {
		t.Error("response is missing Vary: Origin")
	}

	// A disallowed origin must not receive the header.
	req = httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Origin", "https://evil.test")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("allow-origin = %q, want empty for a disallowed origin", got)
	}
}

func TestTimeoutCancelsRequestContext(t *testing.T) {
	r := serve(t, func(r *router.Router) {
		r.Use(mw.Timeout(50 * time.Millisecond))
		r.Get("/slow", func(c *router.Context) error {
			select {
			case <-c.Ctx().Done():
				return c.Ctx().Err()
			case <-time.After(3 * time.Second):
				return c.NoContent(http.StatusOK)
			}
		})
	})

	start := time.Now()
	rec := get(r, "/slow")
	elapsed := time.Since(start)

	if elapsed > time.Second {
		t.Errorf("request took %v; the timeout did not cancel the handler", elapsed)
	}
	if rec.Code != http.StatusGatewayTimeout {
		t.Errorf("status = %d, want 504", rec.Code)
	}
}
