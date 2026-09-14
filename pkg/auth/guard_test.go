package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sachinkaru123/gochin/pkg/router"
)

type testUser struct {
	ID    int64
	Email string
}

type fakeStore struct {
	principal  *Principal
	findErr    error
	loadErr    error
	touched    chan int64
	loadCalled int
}

func (s *fakeStore) find(ctx context.Context, digest string) (*Principal, error) {
	if s.findErr != nil {
		return nil, s.findErr
	}
	return s.principal, nil
}

func (s *fakeStore) load(ctx context.Context, userID int64) (*testUser, error) {
	s.loadCalled++
	if s.loadErr != nil {
		return nil, s.loadErr
	}
	return &testUser{ID: userID, Email: "ada@example.com"}, nil
}

func newGuardFor(t *testing.T, store *fakeStore, tweak func(*GuardConfig[testUser])) *Guard[testUser] {
	t.Helper()
	cfg := GuardConfig[testUser]{
		FindToken: store.find,
		LoadUser:  store.load,
	}
	if tweak != nil {
		tweak(&cfg)
	}
	g, err := NewGuard(cfg)
	if err != nil {
		t.Fatalf("NewGuard: %v", err)
	}
	return g
}

func serveWithGuard(t *testing.T, g *Guard[testUser], h router.Handler, authHeader string) *httptest.ResponseRecorder {
	t.Helper()

	r := router.New()
	r.Get("/protected", h, g.Required())
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func okHandler(c *router.Context) error { return c.NoContent(http.StatusOK) }

func validPrincipal() *Principal {
	return &Principal{
		TokenID:   7,
		UserID:    42,
		Abilities: []string{"*"},
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

func TestGuardAcceptsValidToken(t *testing.T) {
	store := &fakeStore{principal: validPrincipal()}
	g := newGuardFor(t, store, nil)

	rec := serveWithGuard(t, g, okHandler, "Bearer sometoken")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestGuardRejections(t *testing.T) {
	cases := []struct {
		name       string
		header     string
		store      *fakeStore
		wantStatus int
	}{
		{"no header", "", &fakeStore{principal: validPrincipal()}, http.StatusUnauthorized},
		{"wrong scheme", "Basic abc", &fakeStore{principal: validPrincipal()}, http.StatusUnauthorized},
		{"empty bearer", "Bearer ", &fakeStore{principal: validPrincipal()}, http.StatusUnauthorized},
		{"unknown token", "Bearer x", &fakeStore{findErr: ErrTokenNotFound}, http.StatusUnauthorized},
		{"revoked", "Bearer x", &fakeStore{principal: &Principal{TokenID: 1, UserID: 1, RevokedAt: time.Now()}}, http.StatusUnauthorized},
		{"expired", "Bearer x", &fakeStore{principal: &Principal{TokenID: 1, UserID: 1, ExpiresAt: time.Now().Add(-time.Hour)}}, http.StatusUnauthorized},
		{"user rejected", "Bearer x", &fakeStore{principal: validPrincipal(), loadErr: ErrInvalidCredentials}, http.StatusUnauthorized},

		// The critical distinction: a broken store must not look like a bad
		// token, or an outage logs out every client.
		{"store down", "Bearer x", &fakeStore{findErr: errors.New("connection refused")}, http.StatusServiceUnavailable},
		{"user load down", "Bearer x", &fakeStore{principal: validPrincipal(), loadErr: errors.New("connection refused")}, http.StatusServiceUnavailable},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := newGuardFor(t, tc.store, nil)
			rec := serveWithGuard(t, g, okHandler, tc.header)
			if rec.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
		})
	}
}

// A never-expiring token (zero ExpiresAt) must be accepted.
func TestGuardTreatsZeroExpiryAsNever(t *testing.T) {
	store := &fakeStore{principal: &Principal{TokenID: 1, UserID: 1}}
	g := newGuardFor(t, store, nil)

	if rec := serveWithGuard(t, g, okHandler, "Bearer x"); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 for a token with no expiry", rec.Code)
	}
}

func TestGuardIdleTimeout(t *testing.T) {
	store := &fakeStore{principal: &Principal{
		TokenID: 1, UserID: 1, LastUsedAt: time.Now().Add(-2 * time.Hour),
	}}
	g := newGuardFor(t, store, func(cfg *GuardConfig[testUser]) {
		cfg.IdleTimeout = time.Hour
	})

	if rec := serveWithGuard(t, g, okHandler, "Bearer x"); rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 for an idle-expired token", rec.Code)
	}
}

// The user must be reachable both from a controller and from a service, which
// only ever receives a context.Context.
func TestGuardExposesUserToControllerAndService(t *testing.T) {
	store := &fakeStore{principal: validPrincipal()}
	g := newGuardFor(t, store, nil)

	var fromController, fromService *testUser

	handler := func(c *router.Context) error {
		fromController, _ = g.User(c)

		// Simulate a service call: it gets only the context.
		service := func(ctx context.Context) {
			fromService, _ = g.UserFromContext(ctx)
		}
		service(c.Ctx())

		return c.NoContent(http.StatusOK)
	}

	if rec := serveWithGuard(t, g, handler, "Bearer x"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if fromController == nil {
		t.Error("the controller could not read the authenticated user")
	}
	if fromService == nil {
		t.Error("the service could not read the authenticated user from context")
	}
	if fromController != nil && fromService != nil && fromController.ID != fromService.ID {
		t.Error("controller and service saw different users")
	}
}

// A second guard over a different user type must not read this guard's state.
func TestGuardsDoNotShareState(t *testing.T) {
	store := &fakeStore{principal: validPrincipal()}
	g := newGuardFor(t, store, nil)

	type otherUser struct{ ID int64 }
	other, err := NewGuard(GuardConfig[otherUser]{
		FindToken: func(ctx context.Context, digest string) (*Principal, error) {
			return validPrincipal(), nil
		},
		LoadUser: func(ctx context.Context, id int64) (*otherUser, error) {
			return &otherUser{ID: id}, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	var leaked bool
	handler := func(c *router.Context) error {
		_, leaked = other.User(c)
		return c.NoContent(http.StatusOK)
	}

	serveWithGuard(t, g, handler, "Bearer x")
	if leaked {
		t.Error("a guard read another guard's authenticated state")
	}
}

func TestGuardOptionalAllowsAnonymousButRejectsBadToken(t *testing.T) {
	store := &fakeStore{findErr: ErrTokenNotFound}
	g := newGuardFor(t, store, nil)

	r := router.New()
	r.Get("/maybe", okHandler, g.Optional())
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/maybe", nil))
	if rec.Code != http.StatusOK {
		t.Errorf("anonymous status = %d, want 200", rec.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/maybe", nil)
	req.Header.Set("Authorization", "Bearer bad")
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("bad-token status = %d, want 401 (a rejected token must not silently downgrade)", rec.Code)
	}
}

func TestGuardCanChecksAbilities(t *testing.T) {
	store := &fakeStore{principal: &Principal{
		TokenID: 1, UserID: 1, Abilities: []string{"posts:read"},
	}}
	g := newGuardFor(t, store, nil)

	r := router.New()
	r.Get("/read", okHandler, g.Required(), g.Can("posts:read"))
	r.Get("/write", okHandler, g.Required(), g.Can("posts:write"))
	if err := r.Build(); err != nil {
		t.Fatal(err)
	}

	call := func(path string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer x")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	if got := call("/read"); got != http.StatusOK {
		t.Errorf("granted ability status = %d, want 200", got)
	}
	// 403, not 401: the caller is known, retrying will not help.
	if got := call("/write"); got != http.StatusForbidden {
		t.Errorf("missing ability status = %d, want 403", got)
	}
}

func TestGuardRequiresStorageCallbacks(t *testing.T) {
	if _, err := NewGuard(GuardConfig[testUser]{}); err == nil {
		t.Error("NewGuard accepted a config with no FindToken")
	}
	if _, err := NewGuard(GuardConfig[testUser]{
		FindToken: func(context.Context, string) (*Principal, error) { return nil, nil },
	}); err == nil {
		t.Error("NewGuard accepted a config with no LoadUser")
	}
}
