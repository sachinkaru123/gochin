package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// Params configures argon2id hashing.
//
// Defaults follow OWASP's minimum recommended argon2id profile (19 MiB, 2
// iterations, 1 lane) rather than the 64 MiB profile. Memory is charged per
// concurrent hash, so a high setting turns any login endpoint into a memory
// amplifier aimed at this process.
type Params struct {
	Memory      uint32 // KiB
	Time        uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32

	// MaxConcurrent bounds simultaneous hashes. This, not a low memory
	// setting, is what actually caps peak memory under load.
	MaxConcurrent int

	// MaxPasswordBytes rejects oversized input before any work is done.
	MaxPasswordBytes int
}

// DefaultParams returns the recommended configuration.
func DefaultParams() Params {
	return Params{
		Memory:           19456,
		Time:             2,
		Parallelism:      1,
		SaltLength:       16,
		KeyLength:        32,
		MaxConcurrent:    4,
		MaxPasswordBytes: 1024,
	}
}

var (
	paramsMu   sync.RWMutex
	params     = DefaultParams()
	hashTokens = make(chan struct{}, DefaultParams().MaxConcurrent)
)

// Configure replaces the hashing parameters. Call it once at startup.
func Configure(p Params) {
	def := DefaultParams()
	if p.Memory == 0 {
		p.Memory = def.Memory
	}
	if p.Time == 0 {
		p.Time = def.Time
	}
	if p.Parallelism == 0 {
		p.Parallelism = def.Parallelism
	}
	if p.SaltLength == 0 {
		p.SaltLength = def.SaltLength
	}
	if p.KeyLength == 0 {
		p.KeyLength = def.KeyLength
	}
	if p.MaxConcurrent <= 0 {
		p.MaxConcurrent = def.MaxConcurrent
	}
	if p.MaxPasswordBytes <= 0 {
		p.MaxPasswordBytes = def.MaxPasswordBytes
	}

	paramsMu.Lock()
	defer paramsMu.Unlock()
	params = p
	hashTokens = make(chan struct{}, p.MaxConcurrent)
}

func currentParams() (Params, chan struct{}) {
	paramsMu.RLock()
	defer paramsMu.RUnlock()
	return params, hashTokens
}

// derive runs argon2id while holding one slot of the concurrency limiter.
func derive(password, salt []byte, p Params, sem chan struct{}) []byte {
	sem <- struct{}{}
	defer func() { <-sem }()
	return argon2.IDKey(password, salt, p.Time, p.Memory, p.Parallelism, p.KeyLength)
}

// Hash returns a PHC-encoded argon2id hash of plain:
//
//	$argon2id$v=19$m=19456,t=2,p=1$<salt>$<key>
func Hash(plain string) (string, error) {
	p, sem := currentParams()
	if len(plain) > p.MaxPasswordBytes {
		return "", ErrPasswordTooLong
	}

	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("auth: reading salt: %w", err)
	}

	key := derive([]byte(plain), salt, p, sem)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Time, p.Parallelism,
		b64.EncodeToString(salt), b64.EncodeToString(key),
	), nil
}

var b64 = base64.RawStdEncoding

// Verify reports whether plain matches encodedHash, and whether the stored
// hash was produced with weaker parameters than are currently configured (in
// which case the caller should re-hash on successful login).
//
// It never returns an error: every failure mode is an authentication failure,
// and distinguishing them for the caller invites leaking the difference.
func Verify(encodedHash, plain string) (ok bool, needsRehash bool) {
	p, sem := currentParams()

	stored, salt, key, err := decodeHash(encodedHash)
	if err != nil {
		// Spend the same work as a real verification so that an empty or
		// corrupt stored hash is not detectably faster than a wrong password.
		VerifyDummy()
		return false, false
	}
	if len(plain) > p.MaxPasswordBytes {
		VerifyDummy()
		return false, false
	}

	candidate := derive([]byte(plain), salt, stored, sem)

	if subtle.ConstantTimeCompare(candidate, key) != 1 {
		return false, false
	}

	weaker := stored.Memory < p.Memory || stored.Time < p.Time ||
		uint32(len(key)) < p.KeyLength || uint32(len(salt)) < p.SaltLength
	return true, weaker
}

// dummySalt is fixed on purpose: VerifyDummy exists to burn a comparable
// amount of CPU, not to produce a usable hash.
var dummySalt = []byte("gochin-dummy-salt")

// VerifyDummy performs one argon2id derivation and discards it.
//
// Call it on the "no such user" path so that probing for registered emails
// cannot be done by measuring response time.
func VerifyDummy() {
	p, sem := currentParams()
	_ = derive([]byte("gochin-dummy-password"), dummySalt, p, sem)
}

// decodeHash parses a PHC-encoded argon2id string.
func decodeHash(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return Params{}, nil, nil, ErrInvalidHash
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if version != argon2.Version {
		return Params{}, nil, nil, ErrInvalidHash
	}

	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Parallelism); err != nil {
		return Params{}, nil, nil, ErrInvalidHash
	}
	if p.Memory == 0 || p.Time == 0 || p.Parallelism == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	salt, err := b64.DecodeString(parts[4])
	if err != nil || len(salt) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	key, err := b64.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return Params{}, nil, nil, ErrInvalidHash
	}

	p.SaltLength = uint32(len(salt))
	p.KeyLength = uint32(len(key))
	return p, salt, key, nil
}
