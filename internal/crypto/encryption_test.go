package crypto

import "testing"

func TestHashAndValidateEncryptionKey(t *testing.T) {
	hash := HashEncryptionKey("SecretKey#1")
	if hash == "" {
		t.Fatal("HashEncryptionKey() = empty, want digest")
	}
	if hash == "SecretKey#1" {
		t.Fatal("HashEncryptionKey() returned plaintext")
	}
	if !ValidateEncryptionKey("SecretKey#1", hash) {
		t.Fatal("ValidateEncryptionKey() = false, want true")
	}
	if ValidateEncryptionKey("WrongKey#1", hash) {
		t.Fatal("ValidateEncryptionKey() = true for wrong key")
	}
}
