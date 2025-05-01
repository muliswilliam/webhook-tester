package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// GenerateAPIKey returns an API key with the given prefix.
// Example: "user_" + 64-char hex key → "user_xxxx..."
func GenerateAPIKey(prefix string, length int) (string, error) {
	if length < 16 {
		return "", fmt.Errorf("API key length must be at least 16 bytes")
	}
	if strings.Contains(prefix, " ") {
		return "", fmt.Errorf("API key prefix must not contain spaces")
	}

	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate secure random bytes: %w", err)
	}

	key := hex.EncodeToString(bytes)
	return fmt.Sprintf("%s%s", prefix, key), nil
}
