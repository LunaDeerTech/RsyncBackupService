package crypto

import (
	"strings"
	"testing"
)

func TestAESEncryptAndDecrypt(t *testing.T) {
	key := DeriveAESKey("jwt-secret")
	ciphertext, err := AESEncrypt("smtp-password", key)
	if err != nil {
		t.Fatalf("AESEncrypt() error = %v", err)
	}
	if ciphertext == "" || ciphertext == "smtp-password" {
		t.Fatalf("AESEncrypt() = %q, want encoded ciphertext", ciphertext)
	}

	plaintext, err := AESDecrypt(ciphertext, key)
	if err != nil {
		t.Fatalf("AESDecrypt() error = %v", err)
	}
	if plaintext != "smtp-password" {
		t.Fatalf("AESDecrypt() = %q, want %q", plaintext, "smtp-password")
	}
}

func TestAESDecryptRejectsWrongKey(t *testing.T) {
	ciphertext, err := AESEncrypt("smtp-password", DeriveAESKey("jwt-secret"))
	if err != nil {
		t.Fatalf("AESEncrypt() error = %v", err)
	}

	_, err = AESDecrypt(ciphertext, DeriveAESKey("other-secret"))
	if err == nil {
		t.Fatal("AESDecrypt() error = nil, want authentication failure")
	}
	if !strings.Contains(err.Error(), "decrypt ciphertext") {
		t.Fatalf("AESDecrypt() error = %q, want decrypt failure", err)
	}
}