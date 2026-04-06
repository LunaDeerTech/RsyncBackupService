package crypto

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	accessTokenTTL  = 24 * time.Hour
	refreshTokenTTL = 7 * 24 * time.Hour
)

type Claims struct {
	UserID    int64  `json:"sub"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	ExpiresAt int64  `json:"exp"`
	IssuedAt  int64  `json:"iat"`
}

func GenerateAccessToken(claims Claims, secret string) (string, error) {
	return generateToken(claims, secret, accessTokenTTL)
}

func GenerateRefreshToken(claims Claims, secret string) (string, error) {
	return generateToken(claims, secret, refreshTokenTTL)
}

func ParseToken(tokenStr string, secret string) (*Claims, error) {
	if tokenStr == "" {
		return nil, fmt.Errorf("token is required")
	}
	if secret == "" {
		return nil, fmt.Errorf("secret is required")
	}

	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method == nil || token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		return nil, fmt.Errorf("parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

func generateToken(claims Claims, secret string, ttl time.Duration) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("secret is required")
	}

	now := time.Now().UTC()
	payload := Claims{
		UserID:    claims.UserID,
		Email:     claims.Email,
		Role:      claims.Role,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}

	return signed, nil
}

func (c Claims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}

func (c Claims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

func (Claims) GetNotBefore() (*jwt.NumericDate, error) {
	return nil, nil
}

func (Claims) GetIssuer() (string, error) {
	return "", nil
}

func (Claims) GetSubject() (string, error) {
	return "", nil
}

func (Claims) GetAudience() (jwt.ClaimStrings, error) {
	return nil, nil
}
