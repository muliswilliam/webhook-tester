package utils

import (
	"crypto/rand"
	"crypto/sha1"
	"encoding/hex"
	"log"
)

func GenerateApiKey() string {
	bytes := make([]byte, 100)
	if _, err := rand.Read(bytes); err != nil {
		log.Printf("Failed to generate API Key: %v", err)
	}

	sum := sha1.Sum(bytes[:])
	hash := hex.EncodeToString(sum[:])
	return hash
}
