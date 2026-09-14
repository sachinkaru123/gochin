package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/sachinkaru123/gochin/pkg/router"
)

// Principal is the transport-independent record behind a presented token.
// It is what this package needs to enforce policy without knowing the
// application's user type.
type Principal struct {
	TokenID    int64
	UserID     int64
	Name       string
	Abilities  []string // "*" grants everything
	ExpiresAt  time.Time
	RevokedAt  time.Time
	LastUsedAt time.Time
}

// HasAbility reports whether the principal carries an ability.
func (p *Principal) HasAbility(name string) bool {
	for _, a := range p.Abilities {
		if a == "*" || a == name {
			return true
		}
	}
	return false
}

// GuardConfig supplies the storage a Guard needs. The application owns
// persistence; the Guard owns policy.
type GuardConfig[U any] struct {
	// FindToken looks a token up by digest. It MUST return ErrTokenNotFound
	// when no row matches and MUST NOT let orm.ErrRecordNotFound escape: the
	// global error mapper renders that as 404, not 401. Any other error is
	// treated as a store failure and surfaces as 503, never as 401 — a
	// database outage must not look like mass logout to every client.
	FindToken func(ctx context.Context, digest string) (*Principal, error)

	// LoadUser fetches the user behind an already-validated token. Return
	// ErrInvalidCredentials for a user who exists but may not authenticate.
	LoadUser func(ctx context.Context, userID int64) (*U, error)

	// TouchToken records use. Optional; called at most once per
	// TouchInterval per token, on a detached context, with its error logged
	// and discarded.
	TouchToken func(ctx context.Context, tokenID int64, at time.Time) error

	TouchInterval time.Duration // 0 -> 5m, negative -> never
	IdleTimeout   time.Duration // 0 -> disabled
	Now           func() time.Time
	Logger        *slog.Logger
}

// authState is what the Guard stores per request.
//
// It is unexported and generic in U, so a Guard[Admin]'s value fails the type
// assertion inside Guard[Customer].User — a mis-keyed read degrades to
// "unauthenticated" rather than to reading the wrong user.
type authState[U any] struct {
	user      *U
	principal *Principal
}

var guardSeq atomic.Uint64

// Guard authenticates requests carrying a bearer token.
type Guard[U any] struct {
	cfg GuardConfig[U]
	key string
	now func() time.Time
	log *slog.Logger
}

// NewGuard validates cfg and fails at boot rather than at the first request.
func NewGuard[U any](cfg GuardConfig[U]) (*Guard[U], error) {
	if cfg.FindToken == nil {
		return nil, errors.New("auth: GuardConfig.FindToken is required")
	}
	if cfg.LoadUser == nil {
		return nil, errors.New("auth: GuardConfig.LoadUser is required")
	}
	if cfg.TouchInterval == 0 {
		cfg.TouchInterval = 5 * time.Minute
	}

	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	return &Guard[U]{
		cfg: cfg,
		// Package-qualified and per-guard, so neither another package's
		// c.Set nor a second guard can collide with this slot.
		key: "github.com/sachinkaru123/gochin/pkg/auth#" + strconv.FormatUint(guardSeq.Add(1), 10),
		now: now,
		log: log,
	}, nil
}

// Required rejects any request that does not carry a valid token.
func (g *Guard[U]) Required() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			if err := g.authenticate(c); err != nil {
				return err
			}
			return next(c)
		}
	}
}

// Optional allows anonymous requests, but still rejects a token that was
// presented and failed.
//
// Silently downgrading a rejected token to anonymous would hide revocation
// from the client and turn a logged-out session into a half-working one.
func (g *Guard[U]) Optional() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			if BearerToken(c.Request.Header.Get("Authorization")) == "" {
				return next(c)
			}
			if err := g.authenticate(c); err != nil {
				return err
			}
			return next(c)
		}
	}
}

// Can requires that the authenticated token carries every listed ability.
//
// It answers 403, not 401: the caller is known, so retrying with the same
// token will never help.
func (g *Guard[U]) Can(abilities ...string) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(c *router.Context) error {
			p, ok := g.Principal(c)
			if !ok {
				return router.Unauthorizedf("authentication required").WithCode("unauthenticated")
			}
			for _, a := range abilities {
				if !p.HasAbility(a) {
					return router.Forbiddenf("this token is not permitted to %s", a).
						WithCode("insufficient_ability")
				}
			}
			return next(c)
		}
	}
}

// authenticate validates the request's token and stores the resulting state.
func (g *Guard[U]) authenticate(c *router.Context) error {
	raw := BearerToken(c.Request.Header.Get("Authorization"))
	if raw == "" {
		return unauthenticated()
	}

	principal, err := g.cfg.FindToken(c.Ctx(), Digest(raw))
	if err != nil {
		if errors.Is(err, ErrTokenNotFound) {
			return invalidToken()
		}
		// Distinguishing this from a bad token matters: answering 401 during
		// an outage makes every client discard a perfectly good token.
		g.log.Error("auth store unavailable",
			"error", err, "request_id", c.RequestID())
		return router.NewError(http.StatusServiceUnavailable, "authentication is temporarily unavailable").
			WithCode("auth_unavailable")
	}

	now := g.now()

	if !principal.RevokedAt.IsZero() {
		// Presenting a revoked token is a compromise signal worth alerting on.
		g.log.Warn("revoked token presented",
			"token_id", principal.TokenID,
			"user_id", principal.UserID,
			"ip", c.ClientIP(),
			"request_id", c.RequestID())
		return invalidToken()
	}

	if !principal.ExpiresAt.IsZero() && now.After(principal.ExpiresAt) {
		return tokenExpired()
	}

	if g.cfg.IdleTimeout > 0 && !principal.LastUsedAt.IsZero() &&
		now.Sub(principal.LastUsedAt) > g.cfg.IdleTimeout {
		return tokenExpired()
	}

	user, err := g.cfg.LoadUser(c.Ctx(), principal.UserID)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			return invalidToken()
		}
		if errors.Is(err, ErrTokenNotFound) {
			// A live token whose user is gone means the cascade failed.
			g.log.Error("token references a missing user",
				"token_id", principal.TokenID, "user_id", principal.UserID)
			return invalidToken()
		}
		g.log.Error("loading authenticated user", "error", err)
		return router.NewError(http.StatusServiceUnavailable, "authentication is temporarily unavailable").
			WithCode("auth_unavailable")
	}

	g.store(c, authState[U]{user: user, principal: principal})
	g.touch(principal, now)
	return nil
}

// store puts the state on both the pooled router Context (for controllers)
// and the request's context.Context (so services, which only ever receive a
// context.Context, can read it without a signature change).
func (g *Guard[U]) store(c *router.Context, state authState[U]) {
	c.Set(g.key, state)
	c.SetRequest(c.Request.WithContext(
		context.WithValue(c.Ctx(), g.ctxKey(), state),
	))
}

// ctxKey uses the guard pointer itself, which nothing outside this package
// can forge a duplicate of.
func (g *Guard[U]) ctxKey() any { return g }

// touch records token use at most once per TouchInterval.
func (g *Guard[U]) touch(p *Principal, now time.Time) {
	if g.cfg.TouchToken == nil || g.cfg.TouchInterval < 0 {
		return
	}
	if !p.LastUsedAt.IsZero() && now.Sub(p.LastUsedAt) < g.cfg.TouchInterval {
		return
	}

	// Capture plain values: the Context is pooled and reset the moment this
	// request ends, and its context is cancelled by the Timeout middleware.
	tokenID := p.TokenID
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := g.cfg.TouchToken(ctx, tokenID, now); err != nil {
			g.log.Warn("recording token use", "error", err, "token_id", tokenID)
		}
	}()
}

// User returns the authenticated user from a controller.
func (g *Guard[U]) User(c *router.Context) (*U, bool) {
	v, ok := c.Get(g.key)
	if !ok {
		return nil, false
	}
	state, ok := v.(authState[U])
	if !ok {
		return nil, false
	}
	return state.user, true
}

// MustUser returns the authenticated user or a 401 error.
func (g *Guard[U]) MustUser(c *router.Context) (*U, error) {
	user, ok := g.User(c)
	if !ok {
		return nil, unauthenticated()
	}
	return user, nil
}

// UserFromContext returns the authenticated user from a service, which only
// receives a context.Context.
func (g *Guard[U]) UserFromContext(ctx context.Context) (*U, bool) {
	state, ok := ctx.Value(g.ctxKey()).(authState[U])
	if !ok {
		return nil, false
	}
	return state.user, true
}

// MustUserFromContext returns the authenticated user or ErrInvalidCredentials.
func (g *Guard[U]) MustUserFromContext(ctx context.Context) (*U, error) {
	user, ok := g.UserFromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("%w: no authenticated user in context", ErrInvalidCredentials)
	}
	return user, nil
}

// Principal returns the token record behind the current request.
func (g *Guard[U]) Principal(c *router.Context) (*Principal, bool) {
	v, ok := c.Get(g.key)
	if !ok {
		return nil, false
	}
	state, ok := v.(authState[U])
	if !ok {
		return nil, false
	}
	return state.principal, true
}

// PrincipalFromContext returns the token record from a service.
func (g *Guard[U]) PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	state, ok := ctx.Value(g.ctxKey()).(authState[U])
	if !ok {
		return nil, false
	}
	return state.principal, true
}

// Every rejection below renders an identical body apart from its code, so
// that no response distinguishes one user from another.

func unauthenticated() error {
	return router.Unauthorizedf("authentication required").WithCode("unauthenticated")
}

func invalidToken() error {
	return router.Unauthorizedf("invalid or expired token").WithCode("invalid_token")
}

func tokenExpired() error {
	return router.Unauthorizedf("invalid or expired token").WithCode("token_expired")
}
