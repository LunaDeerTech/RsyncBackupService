package service

import (
	"context"
	"strings"
)

type validatedVerifyTokenContextKey struct{}

type validatedVerifyToken struct {
	userID uint
	token  string
}

func MarkVerifyTokenValidated(ctx context.Context, userID uint, token string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	return context.WithValue(ctx, validatedVerifyTokenContextKey{}, validatedVerifyToken{
		userID: userID,
		token:  strings.TrimSpace(token),
	})
}

func HasValidatedVerifyToken(ctx context.Context, userID uint, token string) bool {
	trimmedToken := strings.TrimSpace(token)
	if ctx == nil || trimmedToken == "" {
		return false
	}

	validatedToken, ok := ctx.Value(validatedVerifyTokenContextKey{}).(validatedVerifyToken)
	if !ok {
		return false
	}

	return validatedToken.userID == userID && validatedToken.token == trimmedToken
}