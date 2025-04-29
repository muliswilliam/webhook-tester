package utils

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateSecureToken returns a secure random token of n bytes, hex-encoded.
func GenerateSecureToken(n int) (string, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
