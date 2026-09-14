package requests

import (
	"strings"

	"github.com/gochin/framework/pkg/router"
)

// Password length bounds. The minimum follows NIST's guidance to favour
// length over composition rules; the maximum bounds hashing work per request.
const (
	MinPasswordLength = 12
	MaxPasswordLength = 1024
)

// NormalizeEmail folds an address to its canonical stored form. Registration,
// login and the unique index must all agree on this.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// RegisterRequest is the payload for creating an account.
type RegisterRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (r RegisterRequest) Validate() error {
	fields := map[string]string{}

	if strings.TrimSpace(r.Name) == "" {
		fields["name"] = "name is required"
	}
	if msg := validateEmail(r.Email); msg != "" {
		fields["email"] = msg
	}
	if msg := validatePassword(r.Password); msg != "" {
		fields["password"] = msg
	}

	if len(fields) > 0 {
		return router.Unprocessablef("validation failed").WithFields(fields)
	}
	return nil
}

// LoginRequest is the payload for exchanging credentials for a token.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Validate deliberately checks only presence.
//
// Applying the password policy here would reject a too-short password before
// any lookup, telling an attacker their guess was malformed rather than
// simply wrong, and skipping the constant-time path.
func (r LoginRequest) Validate() error {
	fields := map[string]string{}

	if strings.TrimSpace(r.Email) == "" {
		fields["email"] = "email is required"
	}
	if r.Password == "" {
		fields["password"] = "password is required"
	}
	if len(r.Password) > MaxPasswordLength {
		fields["password"] = "password is too long"
	}

	if len(fields) > 0 {
		return router.Unprocessablef("validation failed").WithFields(fields)
	}
	return nil
}

func validatePassword(password string) string {
	switch {
	case password == "":
		return "password is required"
	case len(password) < MinPasswordLength:
		return "password must be at least 12 characters"
	case len(password) > MaxPasswordLength:
		return "password is too long"
	default:
		return ""
	}
}
