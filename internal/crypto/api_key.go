package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

const apiKeyPrefix = "rbs_"

func GenerateAPIKey() (string, error) {
	randomBytes := make([]byte, 24)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}

	return apiKeyPrefix + hex.EncodeToString(randomBytes), nil
}

func HashAPIKey(key string) (string, error) {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return "", fmt.Errorf("api key is required")
	}

	hash := sha256.Sum256([]byte(trimmed))
	return hex.EncodeToString(hash[:]), nil
}

func APIKeyDisplayPrefix(key string) string {
	trimmed := strings.TrimSpace(key)
	if len(trimmed) <= 12 {
		return trimmed
	}

	return trimmed[:12]
}
