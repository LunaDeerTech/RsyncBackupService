package handler

import (
	crand "crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"sync"
	"time"

	"rsync-backup-service/internal/audit"
	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
)

const (
	authErrorInvalidRequest  = 40002
	authErrorUserExists      = 40901
	authErrorUnauthorized    = 40101
	authErrorTooManyAttempts = 42901
	authErrorInternal        = 50001
	loginFailureLimit        = 5
	loginLockDuration        = 15 * time.Minute
	randomPasswordLength     = 12
	passwordAlphabet         = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

type registerRequest struct {
	Email string `json:"email"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type loginAttempt struct {
	Count     int
	LockUntil time.Time
}

type loginRateLimiter struct {
	mu       sync.Mutex
	attempts map[string]loginAttempt
	now      func() time.Time
}

func newLoginRateLimiter(now func() time.Time) *loginRateLimiter {
	if now == nil {
		now = time.Now
	}

	return &loginRateLimiter{
		attempts: make(map[string]loginAttempt),
		now:      now,
	}
}

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request registerRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	email, err := normalizeEmail(request.Email)
	if err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid email")
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

	count, err := h.db.CountUsers()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to count users")
		return
	}

	password, err := h.passwordGenerator()
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to generate password")
		return
	}

	passwordHash, err := authcrypto.HashPassword(password)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to hash password")
		return
	}

	role := "viewer"
	if count == 0 {
		role = "admin"
	}

	user := &model.User{
		Email:        email,
		Name:         defaultUserName(email),
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
	h.writeAuditLog(r.Context(), 0, user.ID, audit.ActionUserCreate, map[string]any{
		"created_user_id": user.ID,
		"email":           user.Email,
		"name":            user.Name,
		"role":            user.Role,
		"source":          "register",
	})

	JSON(w, http.StatusOK, map[string]string{"message": "请查收邮件获取密码"})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request loginRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}

	email, err := normalizeEmail(request.Email)
	if err != nil || request.Password == "" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid email or password")
		return
	}

	clientIP := clientIP(r)
	if _, locked := h.loginLimiter.locked(clientIP, email); locked {
		Error(w, http.StatusTooManyRequests, authErrorTooManyAttempts, "too many login attempts, try again later")
		return
	}

	user, err := h.db.GetUserByEmail(email)
	if err != nil {
		h.loginLimiter.recordFailure(clientIP, email)
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusUnauthorized, authErrorUnauthorized, "invalid email or password")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
		return
	}

	if !authcrypto.CheckPassword(request.Password, user.PasswordHash) {
		h.loginLimiter.recordFailure(clientIP, email)
		Error(w, http.StatusUnauthorized, authErrorUnauthorized, "invalid email or password")
		return
	}

	h.loginLimiter.reset(clientIP, email)

	accessToken, refreshToken, err := h.issueTokens(*user)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to issue token")
		return
	}

	JSON(w, http.StatusOK, map[string]interface{}{
		"access_token":  accessToken,
		"refresh_token": refreshToken,
		"user":          user,
	})
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	if h.db == nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "database unavailable")
		return
	}

	var request refreshRequest
	if !decodeRequestBody(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.RefreshToken) == "" {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "refresh token is required")
		return
	}

	claims, err := authcrypto.ParseToken(request.RefreshToken, h.jwtSecret)
	if err != nil {
		Error(w, http.StatusUnauthorized, authErrorUnauthorized, "invalid refresh token")
		return
	}

	user, err := h.db.GetUserByID(claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			Error(w, http.StatusUnauthorized, authErrorUnauthorized, "invalid refresh token")
			return
		}

		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to query user")
		return
	}

	accessToken, err := authcrypto.GenerateAccessToken(authcrypto.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}, h.jwtSecret)
	if err != nil {
		Error(w, http.StatusInternalServerError, authErrorInternal, "failed to issue token")
		return
	}

	JSON(w, http.StatusOK, map[string]string{"access_token": accessToken})
}

func (h *Handler) issueTokens(user model.User) (string, string, error) {
	claims := authcrypto.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}

	accessToken, err := authcrypto.GenerateAccessToken(claims, h.jwtSecret)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := authcrypto.GenerateRefreshToken(claims, h.jwtSecret)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func (l *loginRateLimiter) locked(ip, email string) (time.Time, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.key(ip, email)
	attempt, ok := l.attempts[key]
	if !ok {
		return time.Time{}, false
	}

	now := l.now()
	if !attempt.LockUntil.IsZero() && now.Before(attempt.LockUntil) {
		return attempt.LockUntil, true
	}

	if !attempt.LockUntil.IsZero() && !now.Before(attempt.LockUntil) {
		delete(l.attempts, key)
	}

	return time.Time{}, false
}

func (l *loginRateLimiter) recordFailure(ip, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	key := l.key(ip, email)
	now := l.now()
	attempt := l.attempts[key]
	if !attempt.LockUntil.IsZero() && !now.Before(attempt.LockUntil) {
		attempt = loginAttempt{}
	}

	attempt.Count++
	if attempt.Count >= loginFailureLimit {
		attempt.Count = loginFailureLimit
		attempt.LockUntil = now.Add(loginLockDuration)
	}

	l.attempts[key] = attempt
}

func (l *loginRateLimiter) reset(ip, email string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	delete(l.attempts, l.key(ip, email))
}

func (l *loginRateLimiter) key(ip, email string) string {
	return ip + ":" + strings.ToLower(strings.TrimSpace(email))
}

func decodeRequestBody(w http.ResponseWriter, r *http.Request, target interface{}) bool {
	defer r.Body.Close()

	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid request body")
		return false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, authErrorInvalidRequest, "invalid request body")
		return false
	}

	return true
}

func normalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" {
		return "", fmt.Errorf("email is required")
	}

	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email {
		return "", fmt.Errorf("invalid email")
	}

	return email, nil
}

func defaultUserName(email string) string {
	name, _, found := strings.Cut(email, "@")
	if !found || name == "" {
		return email
	}

	return name
}

func clientIP(r *http.Request) string {
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); forwarded != "" {
		return forwarded
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}

	return host
}

func generateRandomPassword() (string, error) {
	password := make([]byte, randomPasswordLength)
	limit := big.NewInt(int64(len(passwordAlphabet)))
	for index := range password {
		value, err := crand.Int(crand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate random password: %w", err)
		}
		password[index] = passwordAlphabet[value.Int64()]
	}

	return string(password), nil
}
