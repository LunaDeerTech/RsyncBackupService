package middleware

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/store"
)

const (
	authErrorUnauthorized = 40101
	authErrorForbidden    = 40301
	authErrorInvalid      = 40002
	authErrorInternal     = 50001
)

func Auth(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
				return
			}

			claims, err := authcrypto.ParseToken(token, jwtSecret)
			if err != nil {
				writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
				return
			}
			if claims.UserID == 0 || claims.Email == "" || claims.Role == "" {
				writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
				return
			}

			next.ServeHTTP(w, r.WithContext(SetUser(r.Context(), claims)))
		})
	}
}

func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetUser(r.Context()) == nil {
			writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := GetUser(r.Context())
		if claims == nil {
			writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
			return
		}
		if claims.Role != "admin" {
			writeError(w, http.StatusForbidden, authErrorForbidden, "forbidden")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func RequireInstanceAccess(db *store.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetUser(r.Context())
			if claims == nil {
				writeError(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
				return
			}
			if claims.Role == "admin" {
				next.ServeHTTP(w, r)
				return
			}
			if db == nil {
				writeError(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
				return
			}

			instanceID, err := instanceIDFromRequest(r)
			if err != nil {
				writeError(w, http.StatusBadRequest, authErrorInvalid, "invalid instance id")
				return
			}

			_, err = db.GetInstancePermission(claims.UserID, instanceID)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					writeError(w, http.StatusForbidden, authErrorForbidden, "forbidden")
					return
				}

				writeError(w, http.StatusInternalServerError, authErrorInternal, "failed to query instance permission")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}

	return parts[1], true
}

func instanceIDFromRequest(r *http.Request) (int64, error) {
	value := strings.TrimSpace(r.PathValue("id"))
	if value == "" {
		return 0, fmt.Errorf("instance id path parameter is required")
	}

	instanceID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse instance id %q: %w", value, err)
	}
	if instanceID <= 0 {
		return 0, fmt.Errorf("instance id must be positive")
	}

	return instanceID, nil
}
