package crypto

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	argonSaltLen        = 16
)

var ErrInvalidHash = errors.New("invalid encoded password hash")

func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	return encodeHash(salt, key, argonTime, argonMemory, argonThreads), nil
}

func VerifyPassword(plain, encodedHash string) (bool, error) {
	salt, key, iterations, memory, threads, err := decodeHash(encodedHash)
	if err != nil {
		return false, err
	}

	candidate := argon2.IDKey([]byte(plain), salt, iterations, memory, threads, uint32(len(key)))

	return subtle.ConstantTimeCompare(key, candidate) == 1, nil
}

func encodeHash(salt, key []byte, iterations, memory uint32, threads uint8) string {
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		memory,
		iterations,
		threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
}

func decodeHash(encoded string) (salt, key []byte, iterations, memory uint32, threads uint8, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &iterations, &threads); err != nil {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	if salt, err = base64.RawStdEncoding.DecodeString(parts[4]); err != nil || len(salt) == 0 {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	if key, err = base64.RawStdEncoding.DecodeString(parts[5]); err != nil || len(key) == 0 {
		return nil, nil, 0, 0, 0, ErrInvalidHash
	}

	return salt, key, iterations, memory, threads, nil
}
