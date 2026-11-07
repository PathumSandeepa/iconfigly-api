package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

func GenerateRandomToken(prefix string, byteLen int) (rawToken string, tokenHash []byte, err error) {
	if byteLen <= 0 {
		return "", nil, errors.New("token byte length must be positive")
	}

	buf := make([]byte, byteLen)
	if _, err := rand.Read(buf); err != nil {
		return "", nil, fmt.Errorf("generate random token: %w", err)
	}

	rawToken = prefix + base64.RawURLEncoding.EncodeToString(buf)

	return rawToken, HashToken(rawToken), nil
}

func HashToken(rawToken string) []byte {
	sum := sha256.Sum256([]byte(rawToken))

	return sum[:]
}
