package crypto

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

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

func TestEncryptAndDecryptFile(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.txt")
	encryptedPath := filepath.Join(tempDir, "input.txt.enc")
	decryptedPath := filepath.Join(tempDir, "input.txt.dec")
	content := []byte("backup payload\nline 2\n")
	if err := os.WriteFile(inputPath, content, 0o600); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}

	if err := EncryptFile(inputPath, encryptedPath, []byte("Cold#123")); err != nil {
		t.Fatalf("EncryptFile() error = %v", err)
	}
	if err := DecryptFile(encryptedPath, decryptedPath, []byte("Cold#123")); err != nil {
		t.Fatalf("DecryptFile() error = %v", err)
	}

	decrypted, err := os.ReadFile(decryptedPath)
	if err != nil {
		t.Fatalf("ReadFile(decrypted) error = %v", err)
	}
	if string(decrypted) != string(content) {
		t.Fatalf("decrypted content = %q, want %q", decrypted, content)
	}
}

func TestDecryptFileRejectsWrongKey(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.txt")
	encryptedPath := filepath.Join(tempDir, "input.txt.enc")
	decryptedPath := filepath.Join(tempDir, "input.txt.dec")
	if err := os.WriteFile(inputPath, []byte("payload"), 0o600); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}
	if err := EncryptFile(inputPath, encryptedPath, []byte("Cold#123")); err != nil {
		t.Fatalf("EncryptFile() error = %v", err)
	}

	err := DecryptFile(encryptedPath, decryptedPath, []byte("Wrong#123"))
	if err == nil {
		t.Fatal("DecryptFile() error = nil, want authentication failure")
	}
}

func TestEncryptFileWithContextCancel(t *testing.T) {
	tempDir := t.TempDir()
	inputPath := filepath.Join(tempDir, "input.bin")
	encryptedPath := filepath.Join(tempDir, "input.bin.enc")
	if err := os.WriteFile(inputPath, make([]byte, 2*1024*1024), 0o600); err != nil {
		t.Fatalf("WriteFile(input) error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := EncryptFileWithContext(ctx, inputPath, encryptedPath, []byte("Cold#123"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("EncryptFileWithContext() error = %v, want context.Canceled", err)
	}
	if _, statErr := os.Stat(encryptedPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("encrypted file stat error = %v, want not exist", statErr)
	}
}
