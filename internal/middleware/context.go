package middleware

import (
	"context"

	authcrypto "rsync-backup-service/internal/crypto"
)

type contextKey string

const userContextKey contextKey = "authenticated-user"

func SetUser(ctx context.Context, claims *authcrypto.Claims) context.Context {
	if claims == nil {
		return ctx
	}

	return context.WithValue(ctx, userContextKey, claims)
}

func GetUser(ctx context.Context) *authcrypto.Claims {
	if ctx == nil {
		return nil
	}

	claims, _ := ctx.Value(userContextKey).(*authcrypto.Claims)
	return claims
}

func MustGetUser(ctx context.Context) *authcrypto.Claims {
	claims := GetUser(ctx)
	if claims == nil {
		panic("authenticated user missing from context")
	}

	return claims
}
