package crypto

import (
	"testing"
	"time"
)

func TestGenerateAndParseAccessToken(t *testing.T) {
	token, err := GenerateAccessToken(Claims{
		UserID: 7,
		Email:  "admin@example.com",
		Role:   "admin",
	}, "secret")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	claims, err := ParseToken(token, "secret")
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}

	if claims.UserID != 7 {
		t.Fatalf("claims.UserID = %d, want %d", claims.UserID, 7)
	}
	if claims.Email != "admin@example.com" {
		t.Fatalf("claims.Email = %q, want %q", claims.Email, "admin@example.com")
	}
	if claims.Role != "admin" {
		t.Fatalf("claims.Role = %q, want %q", claims.Role, "admin")
	}
	if duration := time.Unix(claims.ExpiresAt, 0).Sub(time.Unix(claims.IssuedAt, 0)); duration != 24*time.Hour {
		t.Fatalf("access token ttl = %s, want %s", duration, 24*time.Hour)
	}
}

func TestGenerateAndParseRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken(Claims{
		UserID: 9,
		Email:  "viewer@example.com",
		Role:   "viewer",
	}, "secret")
	if err != nil {
		t.Fatalf("GenerateRefreshToken() error = %v", err)
	}

	claims, err := ParseToken(token, "secret")
	if err != nil {
		t.Fatalf("ParseToken() error = %v", err)
	}

	if duration := time.Unix(claims.ExpiresAt, 0).Sub(time.Unix(claims.IssuedAt, 0)); duration != 7*24*time.Hour {
		t.Fatalf("refresh token ttl = %s, want %s", duration, 7*24*time.Hour)
	}
}

func TestParseTokenRejectsWrongSecret(t *testing.T) {
	token, err := GenerateAccessToken(Claims{UserID: 1, Email: "user@example.com", Role: "viewer"}, "secret")
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	if _, err := ParseToken(token, "other-secret"); err == nil {
		t.Fatal("ParseToken() error = nil, want signature validation error")
	}
}
