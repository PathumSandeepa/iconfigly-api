package crypto

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
)

func TestHashPasswordUsesArgon2idPHCFormat(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$v=19$m=65536,t=1,p=4$") {
		t.Fatalf("unexpected hash format: %s", hash)
	}
}

func TestVerifyPasswordAcceptsCorrectPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ok, err := VerifyPassword("correct horse battery staple", hash)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !ok {
		t.Fatal("expected the correct password to verify")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	ok, err := VerifyPassword("wrong password", hash)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ok {
		t.Fatal("expected a wrong password to be rejected")
	}
}

func TestHashPasswordProducesUniqueSalts(t *testing.T) {
	first, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	second, err := HashPassword("same password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if first == second {
		t.Fatal("expected different hashes for the same password")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	tests := []struct {
		name string
		hash string
	}{
		{"empty", ""},
		{"not a hash", "plaintext"},
		{"wrong algorithm", "$argon2i$v=19$m=65536,t=1,p=4$c2FsdHNhbHQ$aGFzaA"},
		{"missing parts", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdA"},
		{"bad version", "$argon2id$v=x$m=65536,t=1,p=4$c2FsdHNhbHQ$aGFzaA"},
		{"unsupported version", "$argon2id$v=16$m=65536,t=1,p=4$c2FsdHNhbHQ$aGFzaA"},
		{"bad parameters", "$argon2id$v=19$m=abc$c2FsdHNhbHQ$aGFzaA"},
		{"bad salt encoding", "$argon2id$v=19$m=65536,t=1,p=4$***$aGFzaA"},
		{"bad hash encoding", "$argon2id$v=19$m=65536,t=1,p=4$c2FsdHNhbHQ$***"},
		{"empty salt", "$argon2id$v=19$m=65536,t=1,p=4$$aGFzaA"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := VerifyPassword("password", tt.hash)

			if !errors.Is(err, ErrInvalidHash) {
				t.Fatalf("expected ErrInvalidHash, got %v", err)
			}
			if ok {
				t.Fatal("expected a malformed hash never to verify")
			}
		})
	}
}

func TestGenerateRandomTokenAppliesPrefixAndHash(t *testing.T) {
	raw, hash, err := GenerateRandomToken("icfg_pat_", 32)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if !strings.HasPrefix(raw, "icfg_pat_") {
		t.Fatalf("expected prefix on token, got %s", raw)
	}
	if len(hash) != sha256.Size {
		t.Fatalf("expected %d byte hash, got %d", sha256.Size, len(hash))
	}
	if !bytes.Equal(hash, HashToken(raw)) {
		t.Fatal("expected returned hash to match HashToken of the raw token")
	}
}

func TestGenerateRandomTokenEncodedLength(t *testing.T) {
	raw, _, err := GenerateRandomToken("", 32)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(raw) != 43 {
		t.Fatalf("expected 43 base64url characters for 32 bytes, got %d", len(raw))
	}
	if strings.ContainsAny(raw, "+/=") {
		t.Fatalf("expected URL-safe unpadded encoding, got %s", raw)
	}
}

func TestGenerateRandomTokenIsUnique(t *testing.T) {
	first, _, err := GenerateRandomToken("", 32)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	second, _, err := GenerateRandomToken("", 32)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if first == second {
		t.Fatal("expected two generated tokens to differ")
	}
}

func TestGenerateRandomTokenRejectsNonPositiveLength(t *testing.T) {
	for _, length := range []int{0, -1} {
		if _, _, err := GenerateRandomToken("icfg_", length); err == nil {
			t.Fatalf("expected an error for byte length %d", length)
		}
	}
}

func TestHashTokenIsDeterministic(t *testing.T) {
	first := HashToken("icfg_pat_example")
	second := HashToken("icfg_pat_example")

	if !bytes.Equal(first, second) {
		t.Fatal("expected identical input to produce identical hashes")
	}
}

func TestHashTokenDiffersForDifferentInput(t *testing.T) {
	if bytes.Equal(HashToken("token-a"), HashToken("token-b")) {
		t.Fatal("expected different inputs to produce different hashes")
	}
}

func TestHashTokenMatchesKnownSHA256Vector(t *testing.T) {
	want := sha256.Sum256([]byte("abc"))

	if !bytes.Equal(HashToken("abc"), want[:]) {
		t.Fatal("expected HashToken to equal the SHA-256 digest")
	}
}
