package middleware

import (
	"database/sql"
	"errors"
	"net/http"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/store"
)

func APIKeyAuth(db *store.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if db == nil {
				writeError(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
				return
			}

			key, ok := apiKeyFromRequest(r)
			if !ok {
				writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
				return
			}

			hash, err := authcrypto.HashAPIKey(key)
			if err != nil {
				writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
				return
			}

			apiKey, err := db.GetAPIKeyByHash(hash)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
					return
				}
				writeError(w, http.StatusInternalServerError, authErrorInternal, "failed to query api key")
				return
			}

			user, err := db.GetUserByID(apiKey.UserID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
					return
				}
				writeError(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
				return
			}

			_ = db.TouchAPIKeyLastUsed(apiKey.ID)

			claims := &authcrypto.Claims{
				UserID: user.ID,
				Email:  user.Email,
				Role:   user.Role,
			}

			next.ServeHTTP(w, r.WithContext(SetUser(r.Context(), claims)))
		})
	}
}

func apiKeyFromRequest(r *http.Request) (string, bool) {
	if r == nil {
		return "", false
	}

	return bearerToken(r.Header.Get("Authorization"))
}
