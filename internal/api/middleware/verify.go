package middleware

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

func RequireVerifyToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			abortJSON(c, http.StatusUnauthorized, "authentication required")
			return
		}

		authService, ok := authServiceFromContext(c)
		if !ok {
			abortJSON(c, http.StatusInternalServerError, "auth service unavailable")
			return
		}

		err := authService.ConsumeVerifyToken(c.Request.Context(), user.UserID, c.GetHeader("X-Verify-Token"))
		if err != nil {
			switch {
			case errors.Is(err, service.ErrVerifyTokenExpired):
				abortJSON(c, http.StatusUnauthorized, "verify token expired")
			default:
				abortJSON(c, http.StatusUnauthorized, "invalid verify token")
			}
			return
		}

		c.Request = c.Request.WithContext(service.MarkVerifyTokenValidated(c.Request.Context(), user.UserID, c.GetHeader("X-Verify-Token")))

		c.Next()
	}
}