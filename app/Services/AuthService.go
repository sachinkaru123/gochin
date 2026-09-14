package services

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"sync"
	"time"

	models "github.com/gochin/framework/app/Models"
	requests "github.com/gochin/framework/app/Requests"
	"github.com/gochin/framework/pkg/auth"
	"github.com/gochin/framework/pkg/config"
	"github.com/gochin/framework/pkg/orm"
	"github.com/gochin/framework/pkg/router"
)

// AuthService owns registration, login and token lifecycle.
type AuthService struct{}

func NewAuthService() *AuthService { return &AuthService{} }

var (
	guardOnce sync.Once
	guard     *auth.Guard[models.User]
	guardErr  error
)

// AuthGuard returns the process-wide guard.
//
// It is shared so that middleware, controllers and services all read the same
// authenticated state. Construction is lazy, so no database work happens at
// package initialization.
func AuthGuard() (*auth.Guard[models.User], error) {
	guardOnce.Do(func() {
		svc := NewAuthService()
		cfg := config.Get()

		guard, guardErr = auth.NewGuard(auth.GuardConfig[models.User]{
			FindToken:   svc.FindTokenByDigest,
			LoadUser:    svc.LoadUser,
			TouchToken:  svc.TouchToken,
			IdleTimeout: cfg.Auth.IdleTimeout,
		})
	})
	return guard, guardErr
}

// MustAuthGuard panics if the guard cannot be built, for use at boot.
func MustAuthGuard() *auth.Guard[models.User] {
	g, err := AuthGuard()
	if err != nil {
		panic(err)
	}
	return g
}

// FindTokenByDigest implements the guard's token lookup.
//
// It deliberately converts orm.ErrRecordNotFound into auth.ErrTokenNotFound:
// letting the ORM sentinel escape would hit the global error mapper and
// render an unknown token as 404 instead of 401.
func (s *AuthService) FindTokenByDigest(ctx context.Context, digest string) (*auth.Principal, error) {
	token, err := orm.Query[models.AuthToken]().
		WithContext(ctx).
		Where("token_digest", "=", digest).
		First()
	if err != nil {
		if errors.Is(err, orm.ErrRecordNotFound) {
			return nil, auth.ErrTokenNotFound
		}
		return nil, err // a real store failure: the guard turns this into 503
	}

	principal := &auth.Principal{
		TokenID:   token.ID,
		UserID:    token.UserID,
		Name:      token.Name,
		Abilities: strings.Split(token.Abilities, ","),
	}
	if token.ExpiresAt.Valid {
		principal.ExpiresAt = token.ExpiresAt.Time
	}
	if token.RevokedAt.Valid {
		principal.RevokedAt = token.RevokedAt.Time
	}
	if token.LastUsedAt.Valid {
		principal.LastUsedAt = token.LastUsedAt.Time
	}
	return principal, nil
}

// LoadUser implements the guard's user lookup.
func (s *AuthService) LoadUser(ctx context.Context, userID int64) (*models.User, error) {
	user, err := orm.FindCtx[models.User](ctx, userID)
	if err != nil {
		if errors.Is(err, orm.ErrRecordNotFound) {
			return nil, auth.ErrTokenNotFound
		}
		return nil, err
	}
	return user, nil
}

// TouchToken records that a token was used. Called on a detached context.
func (s *AuthService) TouchToken(ctx context.Context, tokenID int64, at time.Time) error {
	exec, err := orm.DefaultExecutor()
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx,
		`UPDATE auth_tokens SET last_used_at = $1 WHERE id = $2`, at, tokenID)
	return err
}

// Register creates a user and issues their first token.
func (s *AuthService) Register(ctx context.Context, in requests.RegisterRequest) (*models.User, string, error) {
	if err := in.Validate(); err != nil {
		return nil, "", err
	}

	hash, err := auth.Hash(in.Password)
	if err != nil {
		return nil, "", err
	}

	user := &models.User{
		Name:         strings.TrimSpace(in.Name),
		Email:        requests.NormalizeEmail(in.Email),
		PasswordHash: hash,
	}

	// A duplicate email is caught by the unique index and mapped to 409.
	if err := orm.CreateCtx(ctx, user); err != nil {
		return nil, "", err
	}

	token, err := s.IssueToken(ctx, user.ID, "registration")
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// Login verifies credentials and issues a token.
//
// Every failure returns the same error regardless of cause, and the
// no-such-user path performs an equivalent hash, so neither the response nor
// its timing reveals whether an email is registered.
func (s *AuthService) Login(ctx context.Context, in requests.LoginRequest) (*models.User, string, error) {
	if err := in.Validate(); err != nil {
		return nil, "", err
	}

	user, err := orm.Query[models.User]().
		WithContext(ctx).
		Where("email", "=", requests.NormalizeEmail(in.Email)).
		First()
	if err != nil {
		if errors.Is(err, orm.ErrRecordNotFound) {
			auth.VerifyDummy()
			return nil, "", invalidCredentials()
		}
		return nil, "", err
	}

	ok, needsRehash := auth.Verify(user.PasswordHash, in.Password)
	if !ok {
		return nil, "", invalidCredentials()
	}

	if needsRehash {
		if rehashed, err := auth.Hash(in.Password); err == nil {
			user.PasswordHash = rehashed
			_ = orm.UpdateCtx(ctx, user)
		}
	}

	token, err := s.IssueToken(ctx, user.ID, "login")
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

// IssueToken creates a token for a user and returns the plaintext, which is
// the only time it exists outside the client.
func (s *AuthService) IssueToken(ctx context.Context, userID int64, name string) (string, error) {
	plain, digest, err := auth.GenerateToken()
	if err != nil {
		return "", err
	}

	record := &models.AuthToken{
		UserID:      userID,
		Name:        name,
		TokenDigest: digest,
		Abilities:   "*",
	}
	if ttl := config.Get().Auth.TokenTTL; ttl > 0 {
		record.ExpiresAt = sql.NullTime{Time: time.Now().UTC().Add(ttl), Valid: true}
	}

	if err := orm.CreateCtx(ctx, record); err != nil {
		return "", err
	}
	return plain, nil
}

// Logout revokes the token behind the current request.
func (s *AuthService) Logout(ctx context.Context, tokenID int64) error {
	exec, err := orm.DefaultExecutor()
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx,
		`UPDATE auth_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`,
		tokenID)
	return err
}

// RevokeAllForUser revokes every token a user holds, for "log out everywhere"
// and for use after a password change.
func (s *AuthService) RevokeAllForUser(ctx context.Context, userID int64) error {
	exec, err := orm.DefaultExecutor()
	if err != nil {
		return err
	}
	_, err = exec.ExecContext(ctx,
		`UPDATE auth_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID)
	return err
}

// invalidCredentials is the single answer to every failed login.
func invalidCredentials() error {
	return router.Unauthorizedf("invalid credentials").WithCode("invalid_credentials")
}
