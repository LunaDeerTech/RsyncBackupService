package middleware

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

const (
	authServiceKey       = "auth-service"
	permissionServiceKey = "permission-service"
	currentUserKey       = "current-user"
)

func InjectServices(authService *service.AuthService, permissionService *service.PermissionService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService != nil {
			c.Set(authServiceKey, authService)
		}
		if permissionService != nil {
			c.Set(permissionServiceKey, permissionService)
		}
		c.Next()
	}
}

func RequireJWT() gin.HandlerFunc {
	return func(c *gin.Context) {
		authService, ok := authServiceFromContext(c)
		if !ok {
			abortJSON(c, http.StatusInternalServerError, "auth service unavailable")
			return
		}

		token, ok := extractBearerToken(c.GetHeader("Authorization"))
		if !ok {
			abortJSON(c, http.StatusUnauthorized, "missing bearer token")
			return
		}

		identity, err := authService.AuthenticateAccessToken(c.Request.Context(), token)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrTokenExpired):
				abortJSON(c, http.StatusUnauthorized, "token expired")
			default:
				abortJSON(c, http.StatusUnauthorized, "invalid token")
			}
			return
		}

		c.Set(currentUserKey, identity)
		c.Next()
	}
}

func RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			abortJSON(c, http.StatusUnauthorized, "authentication required")
			return
		}
		if !user.IsAdmin {
			abortJSON(c, http.StatusForbidden, "admin access required")
			return
		}

		c.Next()
	}
}

func RequireInstanceRole(minRole string) gin.HandlerFunc {
	return func(c *gin.Context) {
		user, ok := CurrentUser(c)
		if !ok {
			abortJSON(c, http.StatusUnauthorized, "authentication required")
			return
		}

		permissionService, ok := permissionServiceFromContext(c)
		if !ok {
			abortJSON(c, http.StatusInternalServerError, "permission service unavailable")
			return
		}

		instanceID, err := parseUintParam(c.Param("id"))
		if err != nil {
			abortJSON(c, http.StatusBadRequest, "invalid instance id")
			return
		}

		allowed, err := permissionService.HasInstanceRole(c.Request.Context(), user, instanceID, minRole)
		if err != nil {
			switch {
			case errors.Is(err, service.ErrInvalidRole):
				abortJSON(c, http.StatusInternalServerError, "invalid role configuration")
			default:
				abortJSON(c, http.StatusInternalServerError, "permission check failed")
			}
			return
		}
		if !allowed {
			abortJSON(c, http.StatusForbidden, "insufficient instance permission")
			return
		}

		c.Next()
	}
}

func CurrentUser(c *gin.Context) (service.AuthIdentity, bool) {
	value, exists := c.Get(currentUserKey)
	if !exists {
		return service.AuthIdentity{}, false
	}

	identity, ok := value.(service.AuthIdentity)
	if !ok {
		return service.AuthIdentity{}, false
	}

	return identity, true
}

func authServiceFromContext(c *gin.Context) (*service.AuthService, bool) {
	value, exists := c.Get(authServiceKey)
	if !exists {
		return nil, false
	}

	authService, ok := value.(*service.AuthService)
	return authService, ok
}

func permissionServiceFromContext(c *gin.Context) (*service.PermissionService, bool) {
	value, exists := c.Get(permissionServiceKey)
	if !exists {
		return nil, false
	}

	permissionService, ok := value.(*service.PermissionService)
	return permissionService, ok
}

func extractBearerToken(headerValue string) (string, bool) {
	parts := strings.Fields(strings.TrimSpace(headerValue))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}

	return parts[1], true
}

func parseUintParam(value string) (uint, error) {
	parsedValue, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}

	return uint(parsedValue), nil
}

func abortJSON(c *gin.Context, statusCode int, message string) {
	c.AbortWithStatusJSON(statusCode, gin.H{"error": message})
}