package utils

import (
	"crypto/rand"
	"encoding/hex"
	"unicode/utf8"
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

// Abbreviate returns s if it fits in limit bytes, and otherwise as much of s
// as fits followed by "...", never cutting inside a UTF-8 character.
func Abbreviate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	const ellipsis = "..."
	cut := limit - len(ellipsis)
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:max(cut, 0)] + ellipsis
}
