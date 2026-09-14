package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

// TokenBytes is the entropy of a generated API token.
const TokenBytes = 32

// GenerateToken returns a new token and the digest to store for it.
//
// The plaintext is shown to the client exactly once and never persisted; the
// database only ever holds the digest, so a leaked database dump cannot be
// replayed against the API.
func GenerateToken() (plain string, digest string, err error) {
	buf := make([]byte, TokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", "", fmt.Errorf("auth: reading token entropy: %w", err)
	}

	plain = base64.RawURLEncoding.EncodeToString(buf)
	return plain, Digest(plain), nil
}

// Digest returns the value stored for a token.
//
// SHA-256 rather than a password hash is correct here: a token is already 256
// bits of uniform entropy, so there is no dictionary to attack, and a fast
// digest keeps authentication to a single indexed lookup.
func Digest(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

// SameDigest compares two digests in constant time.
func SameDigest(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// BearerToken extracts a token from an Authorization header value, returning
// "" when the header is absent or not a bearer credential.
func BearerToken(header string) string {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
