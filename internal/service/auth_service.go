package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	tokenTypeAccess  = "access"
	tokenTypeRefresh = "refresh"

	defaultAccessTokenTTL  = 2 * time.Hour
	defaultRefreshTokenTTL = 7 * 24 * time.Hour
	defaultVerifyTokenTTL  = 5 * time.Minute
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidToken       = errors.New("invalid token")
	ErrTokenExpired       = errors.New("token expired")
	ErrVerifyTokenRequired = errors.New("verify token required")
	ErrVerifyTokenInvalid = errors.New("invalid verify token")
	ErrVerifyTokenExpired = errors.New("verify token expired")
	ErrPasswordMismatch   = errors.New("password mismatch")
	ErrPasswordRequired   = errors.New("password is required")
	ErrUsernameRequired   = errors.New("username is required")
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrInvalidRole        = errors.New("invalid role")
	ErrInstanceNotFound   = errors.New("instance not found")
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type AuthIdentity struct {
	UserID   uint   `json:"id"`
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

type tokenClaims struct {
	TokenType string `json:"token_type"`
	UserVersion int64 `json:"user_version"`
	jwt.RegisteredClaims
}

type verifyTokenRecord struct {
	UserID    uint
	ExpiresAt time.Time
}

type AuthService struct {
	db             *gorm.DB
	jwtSecret      []byte
	clock          func() time.Time
	accessTokenTTL time.Duration
	refreshTTL     time.Duration
	verifyTokenTTL time.Duration

	refreshMu      sync.Mutex
	refreshTokenIDs map[string]time.Time
	verifyMu     sync.Mutex
	verifyTokens map[string]verifyTokenRecord
}

func NewAuthService(db *gorm.DB, jwtSecret string) *AuthService {
	return &AuthService{
		db:             db,
		jwtSecret:      []byte(jwtSecret),
		clock:          time.Now,
		accessTokenTTL: defaultAccessTokenTTL,
		refreshTTL:     defaultRefreshTokenTTL,
		verifyTokenTTL: defaultVerifyTokenTTL,
		refreshTokenIDs: make(map[string]time.Time),
		verifyTokens:   make(map[string]verifyTokenRecord),
	}
}

func (s *AuthService) Login(ctx context.Context, username, password string) (TokenPair, error) {
	user, err := s.findUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return TokenPair{}, ErrInvalidCredentials
		}
		return TokenPair{}, err
	}

	if !passwordMatches(user.PasswordHash, password) {
		return TokenPair{}, ErrInvalidCredentials
	}

	return s.issueTokenPair(user)
}

func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	claims, user, err := s.authenticateToken(ctx, refreshToken, tokenTypeRefresh)
	if err != nil {
		return TokenPair{}, err
	}
	if !s.consumeRefreshTokenID(claims.RegisteredClaims) {
		return TokenPair{}, ErrInvalidToken
	}

	return s.issueTokenPair(user)
}

func (s *AuthService) VerifyPassword(ctx context.Context, userID uint, password string) (string, error) {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return "", err
	}

	if !passwordMatches(user.PasswordHash, password) {
		return "", ErrPasswordMismatch
	}

	token, err := generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generate verify token: %w", err)
	}

	now := s.clock()
	s.verifyMu.Lock()
	defer s.verifyMu.Unlock()

	s.cleanupExpiredVerifyTokensLocked(now)
	s.verifyTokens[token] = verifyTokenRecord{
		UserID:    userID,
		ExpiresAt: now.Add(s.verifyTokenTTL),
	}

	return token, nil
}

func (s *AuthService) ConsumeVerifyToken(ctx context.Context, userID uint, token string) error {
	_ = ctx

	if strings.TrimSpace(token) == "" {
		return ErrVerifyTokenRequired
	}

	now := s.clock()
	s.verifyMu.Lock()
	defer s.verifyMu.Unlock()

	s.cleanupExpiredVerifyTokensLocked(now)

	record, exists := s.verifyTokens[token]
	if !exists {
		return ErrVerifyTokenInvalid
	}
	if record.UserID != userID {
		return ErrVerifyTokenInvalid
	}
	if !record.ExpiresAt.After(now) {
		delete(s.verifyTokens, token)
		return ErrVerifyTokenExpired
	}

	delete(s.verifyTokens, token)
	return nil
}

func (s *AuthService) AuthenticateAccessToken(ctx context.Context, accessToken string) (AuthIdentity, error) {
	_, user, err := s.authenticateToken(ctx, accessToken, tokenTypeAccess)
	if err != nil {
		return AuthIdentity{}, err
	}

	return authIdentityFromUser(user), nil
}

func (s *AuthService) GetUser(ctx context.Context, userID uint) (model.User, error) {
	return s.findUserByID(ctx, userID)
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uint, currentPassword, newPassword string) error {
	user, err := s.findUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if !passwordMatches(user.PasswordHash, currentPassword) {
		return ErrPasswordMismatch
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_hash", passwordHash).Error; err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	s.invalidateVerifyTokensForUser(userID)

	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, userID uint, newPassword string) error {
	if _, err := s.findUserByID(ctx, userID); err != nil {
		return err
	}

	passwordHash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Model(&model.User{}).Where("id = ?", userID).Update("password_hash", passwordHash).Error; err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	s.invalidateVerifyTokensForUser(userID)

	return nil
}

func (s *AuthService) issueTokenPair(user model.User) (TokenPair, error) {
	accessToken, err := s.signToken(user, tokenTypeAccess, s.accessTokenTTL)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := s.signToken(user, tokenTypeRefresh, s.refreshTTL)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

func (s *AuthService) signToken(user model.User, tokenType string, ttl time.Duration) (string, error) {
	now := s.clock()
	tokenID, err := generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generate token id: %w", err)
	}

	claims := tokenClaims{
		TokenType:   tokenType,
		UserVersion: user.UpdatedAt.UTC().UnixNano(),
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        tokenID,
			Subject:   strconv.FormatUint(uint64(user.ID), 10),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(s.jwtSecret)
	if err != nil {
		return "", fmt.Errorf("sign %s token: %w", tokenType, err)
	}

	return signedToken, nil
}

func (s *AuthService) authenticateToken(ctx context.Context, rawToken, expectedType string) (tokenClaims, model.User, error) {
	claims, err := s.parseToken(rawToken)
	if err != nil {
		return tokenClaims{}, model.User{}, err
	}
	if claims.TokenType != expectedType {
		return tokenClaims{}, model.User{}, ErrInvalidToken
	}

	userID, err := strconv.ParseUint(claims.Subject, 10, 64)
	if err != nil {
		return tokenClaims{}, model.User{}, ErrInvalidToken
	}

	user, err := s.findUserByID(ctx, uint(userID))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return tokenClaims{}, model.User{}, ErrInvalidToken
		}
		return tokenClaims{}, model.User{}, err
	}
	if claims.UserVersion != user.UpdatedAt.UTC().UnixNano() {
		return tokenClaims{}, model.User{}, ErrInvalidToken
	}

	return claims, user, nil
}

func (s *AuthService) parseToken(rawToken string) (tokenClaims, error) {
	trimmedToken := strings.TrimSpace(rawToken)
	if trimmedToken == "" {
		return tokenClaims{}, ErrInvalidToken
	}

	parsedToken, err := jwt.ParseWithClaims(trimmedToken, &tokenClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenExpired):
			return tokenClaims{}, ErrTokenExpired
		default:
			return tokenClaims{}, ErrInvalidToken
		}
	}
	if !parsedToken.Valid {
		return tokenClaims{}, ErrInvalidToken
	}

	claims, ok := parsedToken.Claims.(*tokenClaims)
	if !ok {
		return tokenClaims{}, ErrInvalidToken
	}

	return *claims, nil
}

func (s *AuthService) findUserByUsername(ctx context.Context, username string) (model.User, error) {
	trimmedUsername := strings.TrimSpace(username)
	if trimmedUsername == "" {
		return model.User{}, ErrUsernameRequired
	}

	var user model.User
	if err := s.db.WithContext(ctx).Where("username = ?", trimmedUsername).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("find user by username: %w", err)
	}

	return user, nil
}

func (s *AuthService) findUserByID(ctx context.Context, userID uint) (model.User, error) {
	var user model.User
	if err := s.db.WithContext(ctx).First(&user, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("find user by id: %w", err)
	}

	return user, nil
}

func (s *AuthService) cleanupExpiredVerifyTokensLocked(now time.Time) {
	for token, record := range s.verifyTokens {
		if !record.ExpiresAt.After(now) {
			delete(s.verifyTokens, token)
		}
	}
}

func (s *AuthService) invalidateVerifyTokensForUser(userID uint) {
	s.verifyMu.Lock()
	defer s.verifyMu.Unlock()

	for token, record := range s.verifyTokens {
		if record.UserID == userID {
			delete(s.verifyTokens, token)
		}
	}
}

func (s *AuthService) consumeRefreshTokenID(claims jwt.RegisteredClaims) bool {
	if claims.ID == "" || claims.ExpiresAt == nil {
		return false
	}

	now := s.clock()
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	for tokenID, expiresAt := range s.refreshTokenIDs {
		if !expiresAt.After(now) {
			delete(s.refreshTokenIDs, tokenID)
		}
	}
	if _, exists := s.refreshTokenIDs[claims.ID]; exists {
		return false
	}

	s.refreshTokenIDs[claims.ID] = claims.ExpiresAt.Time
	return true
}

func authIdentityFromUser(user model.User) AuthIdentity {
	return AuthIdentity{
		UserID:   user.ID,
		Username: user.Username,
		IsAdmin:  user.IsAdmin,
	}
}

func hashPassword(password string) (string, error) {
	if strings.TrimSpace(password) == "" {
		return "", ErrPasswordRequired
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	return string(passwordHash), nil
}

func passwordMatches(passwordHash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) == nil
}

func generateOpaqueToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(buffer), nil
}