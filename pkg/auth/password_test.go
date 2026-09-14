package auth

import (
	"strings"
	"testing"
)

func TestHashVerifyRoundTrip(t *testing.T) {
	hash, err := Hash("correct horse battery staple")
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$") {
		t.Errorf("hash = %q, want a PHC argon2id string", hash)
	}
	if strings.Contains(hash, "correct horse") {
		t.Fatal("the plaintext password appears in the hash")
	}

	ok, needsRehash := Verify(hash, "correct horse battery staple")
	if !ok {
		t.Error("Verify rejected the correct password")
	}
	if needsRehash {
		t.Error("a freshly created hash should not need a rehash")
	}

	if ok, _ := Verify(hash, "wrong password"); ok {
		t.Error("Verify accepted the wrong password")
	}
}

// Equal passwords must not produce equal hashes, or the salt is not working
// and the store becomes rainbow-table-able.
func TestHashIsSaltedPerCall(t *testing.T) {
	a, err := Hash("same password")
	if err != nil {
		t.Fatal(err)
	}
	b, err := Hash("same password")
	if err != nil {
		t.Fatal(err)
	}

	if a == b {
		t.Error("two hashes of the same password are identical; salt is not applied")
	}
	if ok, _ := Verify(a, "same password"); !ok {
		t.Error("first hash failed to verify")
	}
	if ok, _ := Verify(b, "same password"); !ok {
		t.Error("second hash failed to verify")
	}
}

// Every malformed stored hash must fail closed rather than panic or, worse,
// accept.
func TestVerifyRejectsMalformedHashes(t *testing.T) {
	valid, err := Hash("pw")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(valid, "$")

	cases := map[string]string{
		"empty":             "",
		"not a hash":        "hello",
		"empty fields":      "$$$$$",
		"wrong algorithm":   "$argon2i$v=19$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"wrong version":     "$argon2id$v=16$m=19456,t=2,p=1$" + parts[4] + "$" + parts[5],
		"zero memory":       "$argon2id$v=19$m=0,t=2,p=1$" + parts[4] + "$" + parts[5],
		"bad base64 salt":   "$argon2id$v=19$m=19456,t=2,p=1$!!!!$" + parts[5],
		"bad base64 key":    "$argon2id$v=19$m=19456,t=2,p=1$" + parts[4] + "$!!!!",
		"missing key":       "$argon2id$v=19$m=19456,t=2,p=1$" + parts[4],
		"truncated params":  "$argon2id$v=19$m=19456$" + parts[4] + "$" + parts[5],
		"bcrypt-looking":    "$2a$12$abcdefghijklmnopqrstuv",
		"legacy empty user": "",
	}

	for name, hash := range cases {
		t.Run(name, func(t *testing.T) {
			if ok, _ := Verify(hash, "pw"); ok {
				t.Errorf("Verify accepted a malformed hash: %q", hash)
			}
			if ok, _ := Verify(hash, ""); ok {
				t.Errorf("Verify accepted a malformed hash with an empty password: %q", hash)
			}
		})
	}
}

// A tampered key must not verify, even though the rest of the string parses.
func TestVerifyRejectsTamperedKey(t *testing.T) {
	hash, err := Hash("pw")
	if err != nil {
		t.Fatal(err)
	}

	parts := strings.Split(hash, "$")
	key := []byte(parts[5])
	key[0] ^= 'A' // flip the first character
	parts[5] = string(key)

	if ok, _ := Verify(strings.Join(parts, "$"), "pw"); ok {
		t.Error("Verify accepted a hash whose key was modified")
	}
}

// A hash stored under weaker parameters must still verify, and must be
// flagged for rehashing.
func TestVerifyDetectsWeakerStoredParams(t *testing.T) {
	weak := DefaultParams()
	weak.Memory = 8192
	weak.Time = 1
	Configure(weak)

	hash, err := Hash("pw")
	if err != nil {
		t.Fatal(err)
	}

	Configure(DefaultParams())
	t.Cleanup(func() { Configure(DefaultParams()) })

	ok, needsRehash := Verify(hash, "pw")
	if !ok {
		t.Fatal("a hash made with weaker params must still verify")
	}
	if !needsRehash {
		t.Error("a hash made with weaker params should be flagged for rehash")
	}
}

func TestHashRejectsOversizedPassword(t *testing.T) {
	p := DefaultParams()
	p.MaxPasswordBytes = 64
	Configure(p)
	t.Cleanup(func() { Configure(DefaultParams()) })

	if _, err := Hash(strings.Repeat("x", 65)); err == nil {
		t.Error("Hash accepted a password over the configured limit")
	}

	hash, err := Hash(strings.Repeat("x", 64))
	if err != nil {
		t.Fatalf("Hash rejected a password at the limit: %v", err)
	}
	// A longer password must not authenticate by being truncated to a
	// matching prefix.
	if ok, _ := Verify(hash, strings.Repeat("x", 65)); ok {
		t.Error("an over-long password authenticated against a shorter one's hash")
	}
}

func TestVerifyDummyDoesNotPanic(t *testing.T) {
	VerifyDummy()
}
