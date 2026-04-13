package handler

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rsync-backup-service/internal/audit"
	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/model"
)

const (
	userErrorNotFound = 40401
	minPasswordLength = 8
)

type createUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type updateUserRequest struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type updatePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type updateProfileRequest struct {
	Name string `json:"name"`
}

type userResponse struct {
	ID        int64     `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	pagination := ParsePagination(r)
	total, err := h.db.CountUsers()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count users")
		return
	}

	users, err := h.db.ListUsersPage(pagination.PageSize, (pagination.Page-1)*pagination.PageSize)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to list users")
		return
	}

	JSON(w, http.StatusOK, PaginatedResponse{
		Items:      toUserResponses(users),
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages(total, pagination.PageSize),
	})
}

func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request createUserRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid email")
		return
	}
	name, err := normalizeUserName(request.Name)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "name is required")
		return
	}
	role, err := normalizeUserRole(request.Role)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid role")
		return
	}

	_, err = h.db.GetUserByEmail(email)
	if err == nil {
		Error(w, http.StatusConflict, authErrorUserExists, "user already exists")
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
		return
	}

	password, passwordHash, err := h.generatePasswordHash()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, err.Error())
		return
	}

	user := &model.User{
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		Role:         role,
	}
	if err := h.db.CreateUser(user); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to create user")
		return
	}

	if err := h.passwordSender.SendPassword(r.Context(), email, password); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to deliver password")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionUserCreate, map[string]any{
		"created_user_id": user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"role":            user.Role,
	})

	JSON(w, http.StatusCreated, toUserResponse(*user))
}

func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	userID, err := userIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid user id")
		return
	}

	var request updateUserRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	name, err := normalizeUserName(request.Name)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "name is required")
		return
	}
	role, err := normalizeUserRole(request.Role)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid role")
		return
	}

	user, err := h.db.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, userErrorNotFound, "user not found")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
		return
	}

	currentUser := middleware.MustGetUser(r.Context())
	if user.ID == currentUser.UserID && user.Role != role {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "cannot modify your own role")
		return
	}

	user.Name = name
	user.Role = role
	if err := h.db.UpdateUser(user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, userErrorNotFound, "user not found")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update user")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionUserUpdate, map[string]any{
		"updated_user_id": user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"role":            user.Role,
	})

	JSON(w, http.StatusOK, toUserResponse(*user))
}

func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	userID, err := userIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid user id")
		return
	}

	currentUser := middleware.MustGetUser(r.Context())
	if userID == currentUser.UserID {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "cannot delete yourself")
		return
	}

	user, err := h.db.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, userErrorNotFound, "user not found")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
		return
	}

	if user.Role == "admin" {
		adminCount, err := h.db.CountUsersByRole("admin")
		if err != nil {
			Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count admins")
			return
		}
		if adminCount <= 1 {
			Error(w, http.StatusBadRequest, authErrorInvalidRequest, "cannot delete the last admin")
			return
		}
	}

	if err := h.db.DeleteUserWithCleanup(userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, userErrorNotFound, "user not found")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to delete user")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionUserDelete, map[string]any{
		"deleted_user_id": user.ID,
		"deleted_email":   user.Email,
		"deleted_name":    user.Name,
		"deleted_role":    user.Role,
	})

	JSON(w, http.StatusOK, map[string]string{"message": "user deleted"})
}

func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	userID, err := userIDFromRequest(r)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid user id")
		return
	}

	user, err := h.db.GetUserByID(userID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, userErrorNotFound, "user not found")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
		return
	}

	password, passwordHash, err := h.generatePasswordHash()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, err.Error())
		return
	}

	user.PasswordHash = passwordHash
	if err := h.db.UpdateUser(user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusNotFound, userErrorNotFound, "user not found")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to reset password")
		return
	}

	if err := h.passwordSender.SendPassword(r.Context(), user.Email, password); err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to deliver password")
		return
	}
	h.writeCurrentUserAudit(r, 0, audit.ActionUserPasswordReset, map[string]any{
		"reset_user_id":    user.ID,
		"reset_user_email": user.Email,
		"reset_user_name":  user.Name,
		"reset_user_role":  user.Role,
	})

	JSON(w, http.StatusOK, map[string]string{"message": "password reset"})
}

func (h *Handler) GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	user, err := h.currentUser(r)
	if err != nil {
		writeCurrentUserError(w, err)
		return
	}

	JSON(w, http.StatusOK, toUserResponse(*user))
}

func (h *Handler) UpdateCurrentUserPassword(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request updatePasswordRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}
	if request.OldPassword == "" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "old password is required")
		return
	}
	if len(request.NewPassword) < minPasswordLength {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "new password must be at least 8 characters")
		return
	}

	user, err := h.currentUser(r)
	if err != nil {
		writeCurrentUserError(w, err)
		return
	}
	if !authcrypto.CheckPassword(request.OldPassword, user.PasswordHash) {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "old password is incorrect")
		return
	}

	passwordHash, err := authcrypto.HashPassword(request.NewPassword)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to hash password")
		return
	}

	user.PasswordHash = passwordHash
	if err := h.db.UpdateUser(user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update password")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"message": "password updated"})
}

func (h *Handler) UpdateCurrentUserProfile(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request updateProfileRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	name, err := normalizeUserName(request.Name)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "name is required")
		return
	}

	user, err := h.currentUser(r)
	if err != nil {
		writeCurrentUserError(w, err)
		return
	}

	user.Name = name
	if err := h.db.UpdateUser(user); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to update profile")
		return
	}

	JSON(w, http.StatusOK, toUserResponse(*user))
}

func (h *Handler) currentUser(r *http.Request) (*model.User, error) {
	claims := middleware.MustGetUser(r.Context())
	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func writeCurrentUserError(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) {
		Error(w, http.StatusUnauthorized, authErrorUnauthorized, "unauthorized")
		return
	}

	Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
}

func userIDFromRequest(r *http.Request) (int64, error) {
	rawID := strings.TrimSpace(r.PathValue("id"))
	if rawID == "" {
		return 0, fmt.Errorf("user id is required")
	}

	userID, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse user id %q: %w", rawID, err)
	}
	if userID <= 0 {
		return 0, fmt.Errorf("user id must be positive")
	}

	return userID, nil
}

func normalizeUserName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", fmt.Errorf("name is required")
	}

	return name, nil
}

func normalizeUserRole(raw string) (string, error) {
	role := strings.ToLower(strings.TrimSpace(raw))
	switch role {
	case "admin", "viewer":
		return role, nil
	default:
		return "", fmt.Errorf("invalid role")
	}
}

func toUserResponse(user model.User) userResponse {
	return userResponse{
		ID:        user.ID,
		Email:     user.Email,
		Name:      user.Name,
		Role:      user.Role,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toUserResponses(users []model.User) []userResponse {
	responses := make([]userResponse, 0, len(users))
	for _, user := range users {
		responses = append(responses, toUserResponse(user))
	}

	return responses
}

func totalPages(total int64, pageSize int) int {
	if total == 0 || pageSize <= 0 {
		return 0
	}

	return int((total + int64(pageSize) - 1) / int64(pageSize))
}

func (h *Handler) generatePasswordHash() (string, string, error) {
	password, err := h.passwordGenerator()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate password")
	}

	passwordHash, err := authcrypto.HashPassword(password)
	if err != nil {
		return "", "", fmt.Errorf("failed to hash password")
	}

	return password, passwordHash, nil
}
