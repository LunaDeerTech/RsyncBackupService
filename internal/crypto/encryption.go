package crypto

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func HashEncryptionKey(key string) string {
	checksum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(checksum[:])
}

func ValidateEncryptionKey(key, hash string) bool {
	if key == "" || hash == "" {
		return false
	}

	return HashEncryptionKey(key) == hash
}

func RequireEncryptionKeyHash(key string) (*string, error) {
	if key == "" {
		return nil, fmt.Errorf("encryption_key is required when encryption is enabled")
	}

	hash := HashEncryptionKey(key)
	return &hash, nil
}
