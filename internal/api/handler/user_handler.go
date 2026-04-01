package handler

import (
	"errors"
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type UserHandler struct {
	userService *service.UserService
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

func NewUserHandler(userService *service.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

func (h *UserHandler) List(c *gin.Context) {
	users, err := h.userService.List(c.Request.Context())
	if err != nil {
		writeError(c, http.StatusInternalServerError, "list users failed")
		return
	}

	c.JSON(http.StatusOK, toUserResponses(users))
}

func (h *UserHandler) Create(c *gin.Context) {
	var request createUserRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{
		Action:       "users.create",
		ResourceType: "users",
	})

	user, err := h.userService.Create(c.Request.Context(), request.Username, request.Password, request.IsAdmin)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrUsernameRequired), errors.Is(err, service.ErrPasswordRequired):
			writeError(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrUserExists):
			writeError(c, http.StatusConflict, "user already exists")
		default:
			writeError(c, http.StatusInternalServerError, "create user failed")
		}
		return
	}

	middleware.SetAuditMetadata(c, middleware.AuditMetadata{
		Action:       "users.create",
		ResourceType: "users",
		ResourceID:   user.ID,
	})

	c.JSON(http.StatusCreated, toUserResponse(user))
}

func (h *UserHandler) ResetPassword(c *gin.Context) {
	userID, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	var request resetPasswordRequest
	if !bindJSON(c, &request) {
		return
	}
	middleware.SetAuditMetadata(c, middleware.AuditMetadata{
		Action:       "users.password.reset",
		ResourceType: "users",
		ResourceID:   userID,
	})

	err := h.userService.ResetPassword(c.Request.Context(), userID, request.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrPasswordRequired):
			writeError(c, http.StatusBadRequest, err.Error())
		case errors.Is(err, service.ErrUserNotFound):
			writeError(c, http.StatusNotFound, "user not found")
		default:
			writeError(c, http.StatusInternalServerError, "reset password failed")
		}
		return
	}

	c.Status(http.StatusNoContent)
}