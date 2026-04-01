package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService *service.AuthService
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type verifyRequest struct {
	Password string `json:"password"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type userResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	IsAdmin   bool   `json:"is_admin"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var request loginRequest
	if !bindJSON(c, &request) {
		return
	}

	tokens, err := h.authService.Login(c.Request.Context(), request.Username, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			writeError(c, http.StatusUnauthorized, "invalid credentials")
		case errors.Is(err, service.ErrUsernameRequired), errors.Is(err, service.ErrPasswordRequired):
			writeError(c, http.StatusBadRequest, err.Error())
		default:
			writeError(c, http.StatusInternalServerError, "login failed")
		}
		return
	}

	c.JSON(http.StatusOK, tokens)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var request refreshRequest
	if !bindJSON(c, &request) {
		return
	}

	tokens, err := h.authService.Refresh(c.Request.Context(), request.RefreshToken)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrTokenExpired):
			writeError(c, http.StatusUnauthorized, "refresh token expired")
		case errors.Is(err, service.ErrInvalidToken):
			writeError(c, http.StatusUnauthorized, "invalid refresh token")
		default:
			writeError(c, http.StatusInternalServerError, "refresh failed")
		}
		return
	}

	c.JSON(http.StatusOK, tokens)
}

func (h *AuthHandler) Verify(c *gin.Context) {
	user, ok := middleware.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{
		Action:       "auth.verify",
		ResourceType: "users",
		ResourceID:   user.UserID,
	})

	var request verifyRequest
	if !bindJSON(c, &request) {
		return
	}

	verifyToken, err := h.authService.VerifyPassword(c.Request.Context(), user.UserID, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPasswordMismatch):
			writeError(c, http.StatusUnauthorized, "password verification failed")
		case errors.Is(err, service.ErrUserNotFound):
			writeError(c, http.StatusNotFound, "user not found")
		default:
			writeError(c, http.StatusInternalServerError, "verify failed")
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"verify_token": verifyToken})
}

func (h *AuthHandler) Me(c *gin.Context) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}

	user, err := h.authService.GetUser(c.Request.Context(), identity.UserID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUserNotFound):
			writeError(c, http.StatusNotFound, "user not found")
		default:
			writeError(c, http.StatusInternalServerError, "load current user failed")
		}
		return
	}

	c.JSON(http.StatusOK, toUserResponse(user))
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	identity, ok := middleware.CurrentUser(c)
	if !ok {
		writeError(c, http.StatusUnauthorized, "authentication required")
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{
		Action:       "auth.password.change",
		ResourceType: "users",
		ResourceID:   identity.UserID,
	})

	var request changePasswordRequest
	if !bindJSON(c, &request) {
		return
	}

	err := h.authService.ChangePassword(c.Request.Context(), identity.UserID, request.CurrentPassword, request.NewPassword)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPasswordMismatch):
			writeError(c, http.StatusUnauthorized, "current password is incorrect")
		case errors.Is(err, service.ErrPasswordRequired):
			writeError(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			writeError(c, http.StatusNotFound, "user not found")
		default:
			writeError(c, http.StatusInternalServerError, "change password failed")
		}
		return
	}

	c.Status(http.StatusNoContent)
}

func bindJSON(c *gin.Context, destination any) bool {
	if err := c.ShouldBindJSON(destination); err != nil {
		writeError(c, http.StatusBadRequest, "invalid request body")
		return false
	}

	return true
}

func writeError(c *gin.Context, statusCode int, message string) {
	c.AbortWithStatusJSON(statusCode, gin.H{"error": message})
}

func toUserResponse(user model.User) userResponse {
	return userResponse{
		ID:        user.ID,
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		CreatedAt: user.CreatedAt.UTC().Format(http.TimeFormat),
		UpdatedAt: user.UpdatedAt.UTC().Format(http.TimeFormat),
	}
}

func toUserResponses(users []model.User) []userResponse {
	responses := make([]userResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, toUserResponse(user))
	}

	return responses
}

func parseIDParam(c *gin.Context, name string) (uint, bool) {
	parsedValue, err := strconv.ParseUint(strings.TrimSpace(c.Param(name)), 10, 64)
	if err != nil {
		writeError(c, http.StatusBadRequest, "invalid id")
		return 0, false
	}

	return uint(parsedValue), true
}