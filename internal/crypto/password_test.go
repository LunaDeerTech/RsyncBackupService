package crypto

import "testing"

func TestHashAndCheckPassword(t *testing.T) {
	hash, err := HashPassword("SecretPass123")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}
	if hash == "SecretPass123" {
		t.Fatal("HashPassword() returned plaintext password")
	}
	if !CheckPassword("SecretPass123", hash) {
		t.Fatal("CheckPassword() = false, want true")
	}
	if CheckPassword("WrongPass123", hash) {
		t.Fatal("CheckPassword() = true for wrong password")
	}
}

func TestHashPasswordRejectsEmpty(t *testing.T) {
	if _, err := HashPassword(""); err == nil {
		t.Fatal("HashPassword() error = nil, want error")
	}
}
