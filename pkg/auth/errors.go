// Package auth provides password hashing, opaque API tokens, and a guard
// middleware that authenticates requests.
//
// It is deliberately model-agnostic: the application supplies storage
// callbacks, and this package owns the security policy. That keeps pkg/auth
// free of any dependency on app/.
package auth

import "errors"

var (
	// ErrInvalidCredentials is the single answer to every failed
	// authentication attempt. Callers must not distinguish "no such user"
	// from "wrong password": doing so tells an attacker which emails are
	// registered.
	ErrInvalidCredentials = errors.New("auth: invalid credentials")

	// ErrTokenNotFound must be returned by a GuardConfig.FindToken when no
	// row matches. It exists so that orm.ErrRecordNotFound never escapes the
	// auth path, where the global error mapper would render it as a 404
	// instead of a 401.
	ErrTokenNotFound = errors.New("auth: token not found")

	// ErrTokenExpired marks a token past its expiry or idle window.
	ErrTokenExpired = errors.New("auth: token expired")

	// ErrTokenRevoked marks a token that was explicitly revoked.
	ErrTokenRevoked = errors.New("auth: token revoked")

	// ErrPasswordTooLong guards against unbounded hashing work.
	ErrPasswordTooLong = errors.New("auth: password exceeds the maximum length")

	// ErrInvalidHash marks a stored hash that cannot be parsed.
	ErrInvalidHash = errors.New("auth: invalid password hash")
)
