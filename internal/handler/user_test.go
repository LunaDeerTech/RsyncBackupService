package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/model"
	"rsync-backup-service/internal/store"
)

func TestListUsersRequiresAdminAndSupportsPagination(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")

	router := NewRouter(db, WithJWTSecret("secret"))

	adminResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/users?page=1&page_size=1", nil, mustAccessTokenForUser(t, admin, "secret"))
	if adminResponse.Code != http.StatusOK {
		t.Fatalf("admin GET /api/v1/users status = %d, want %d", adminResponse.Code, http.StatusOK)
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(adminResponse.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode users response: %v", err)
	}
	var page struct {
		Items      []map[string]any `json:"items"`
		Total      int64            `json:"total"`
		Page       int              `json:"page"`
		PageSize   int              `json:"page_size"`
		TotalPages int              `json:"total_pages"`
	}
	if err := json.Unmarshal(envelope.Data, &page); err != nil {
		t.Fatalf("decode users page: %v", err)
	}
	if page.Total != 2 {
		t.Fatalf("total = %d, want %d", page.Total, 2)
	}
	if page.Page != 1 || page.PageSize != 1 || page.TotalPages != 2 {
		t.Fatalf("pagination = %+v, want page=1 page_size=1 total_pages=2", page)
	}
	if len(page.Items) != 1 {
		t.Fatalf("items len = %d, want %d", len(page.Items), 1)
	}
	if _, exists := page.Items[0]["password_hash"]; exists {
		t.Fatal("list item unexpectedly contains password_hash")
	}

	viewerResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/users", nil, mustAccessTokenForUser(t, viewer, "secret"))
	assertAPIError(t, viewerResponse, http.StatusForbidden, 40301, "forbidden")
}

func TestAdminCreateAndUpdateUser(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	target := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	sender := newRecordingPasswordSender()

	router := NewRouter(
		db,
		WithJWTSecret("secret"),
		withPasswordSender(sender),
		withPasswordGenerator(sequencePasswordGenerator("TempPass456")),
	)

	createResponse := performAuthorizedJSONRequest(t, router, http.MethodPost, "/api/v1/users", map[string]string{
		"email": "new-admin@example.com",
		"name":  "New Admin",
		"role":  "admin",
	}, mustAccessTokenForUser(t, admin, "secret"))
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("POST /api/v1/users status = %d, want %d", createResponse.Code, http.StatusCreated)
	}
	if sender.PasswordFor("new-admin@example.com") != "TempPass456" {
		t.Fatalf("generated password = %q, want %q", sender.PasswordFor("new-admin@example.com"), "TempPass456")
	}
	created, err := db.GetUserByEmail("new-admin@example.com")
	if err != nil {
		t.Fatalf("GetUserByEmail(new-admin) error = %v", err)
	}
	if created.Role != "admin" {
		t.Fatalf("created role = %q, want %q", created.Role, "admin")
	}
	if !authcrypto.CheckPassword("TempPass456", created.PasswordHash) {
		t.Fatal("created password hash does not match generated password")
	}

	var createEnvelope apiEnvelope
	if err := json.Unmarshal(createResponse.Body.Bytes(), &createEnvelope); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	var createdPayload map[string]any
	if err := json.Unmarshal(createEnvelope.Data, &createdPayload); err != nil {
		t.Fatalf("decode create payload: %v", err)
	}
	if _, exists := createdPayload["password_hash"]; exists {
		t.Fatal("create response unexpectedly contains password_hash")
	}

	updateResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/"+itoa(target.ID), map[string]string{
		"name": "Promoted Viewer",
		"role": "admin",
	}, mustAccessTokenForUser(t, admin, "secret"))
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/users/{id} status = %d, want %d", updateResponse.Code, http.StatusOK)
	}
	updated, err := db.GetUserByID(target.ID)
	if err != nil {
		t.Fatalf("GetUserByID(target) error = %v", err)
	}
	if updated.Name != "Promoted Viewer" || updated.Role != "admin" {
		t.Fatalf("updated user = %+v, want name=%q role=%q", updated, "Promoted Viewer", "admin")
	}

	selfUpdate := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/"+itoa(admin.ID), map[string]string{
		"name": "Admin",
		"role": "viewer",
	}, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, selfUpdate, http.StatusBadRequest, authErrorInvalidRequest, "cannot modify your own role")
}

func TestAdminDeleteUserPreventsSelfAndCleansRelations(t *testing.T) {
	db := newAuthTestDB(t)
	admin := createHandlerTestUser(t, db, "admin@example.com", "Admin", "admin", "AdminPass123")
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "ViewerPass123")
	instanceID := createHandlerTestInstance(t, db, "instance-one")

	if _, err := db.Exec(
		`INSERT INTO instance_permissions (user_id, instance_id, permission, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		viewer.ID,
		instanceID,
		"readonly",
	); err != nil {
		t.Fatalf("insert instance permission error = %v", err)
	}
	if _, err := db.Exec(
		`INSERT INTO notification_subscriptions (user_id, instance_id, enabled, created_at) VALUES (?, ?, ?, CURRENT_TIMESTAMP)`,
		viewer.ID,
		instanceID,
		true,
	); err != nil {
		t.Fatalf("insert notification subscription error = %v", err)
	}

	router := NewRouter(db, WithJWTSecret("secret"))

	selfDelete := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/users/"+itoa(admin.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	assertAPIError(t, selfDelete, http.StatusBadRequest, authErrorInvalidRequest, "cannot delete yourself")

	deleteResponse := performAuthorizedJSONRequest(t, router, http.MethodDelete, "/api/v1/users/"+itoa(viewer.ID), nil, mustAccessTokenForUser(t, admin, "secret"))
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("DELETE /api/v1/users/{id} status = %d, want %d", deleteResponse.Code, http.StatusOK)
	}
	if _, err := db.GetUserByID(viewer.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("GetUserByID(deleted viewer) error = %v, want sql.ErrNoRows", err)
	}
	assertRowCount(t, db, `SELECT COUNT(*) FROM instance_permissions WHERE user_id = ?`, viewer.ID, 0)
	assertRowCount(t, db, `SELECT COUNT(*) FROM notification_subscriptions WHERE user_id = ?`, viewer.ID, 0)
}

func TestCurrentUserProfileAndPasswordEndpoints(t *testing.T) {
	db := newAuthTestDB(t)
	viewer := createHandlerTestUser(t, db, "viewer@example.com", "Viewer", "viewer", "OldPass123")
	router := NewRouter(db, WithJWTSecret("secret"))
	token := mustAccessTokenForUser(t, viewer, "secret")

	meResponse := performAuthorizedJSONRequest(t, router, http.MethodGet, "/api/v1/users/me", nil, token)
	if meResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/users/me status = %d, want %d", meResponse.Code, http.StatusOK)
	}
	var meEnvelope apiEnvelope
	if err := json.Unmarshal(meResponse.Body.Bytes(), &meEnvelope); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	var me map[string]any
	if err := json.Unmarshal(meEnvelope.Data, &me); err != nil {
		t.Fatalf("decode me payload: %v", err)
	}
	if me["email"] != "viewer@example.com" || me["name"] != "Viewer" || me["role"] != "viewer" {
		t.Fatalf("me payload = %+v, want viewer identity", me)
	}

	profileResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/me/profile", map[string]string{"name": "Renamed Viewer"}, token)
	if profileResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/users/me/profile status = %d, want %d", profileResponse.Code, http.StatusOK)
	}
	updatedProfile, err := db.GetUserByID(viewer.ID)
	if err != nil {
		t.Fatalf("GetUserByID(profile updated) error = %v", err)
	}
	if updatedProfile.Name != "Renamed Viewer" {
		t.Fatalf("updated profile name = %q, want %q", updatedProfile.Name, "Renamed Viewer")
	}

	wrongPassword := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/me/password", map[string]string{
		"old_password": "WrongPass123",
		"new_password": "NewPass123",
	}, token)
	assertAPIError(t, wrongPassword, http.StatusBadRequest, authErrorInvalidRequest, "old password is incorrect")

	passwordResponse := performAuthorizedJSONRequest(t, router, http.MethodPut, "/api/v1/users/me/password", map[string]string{
		"old_password": "OldPass123",
		"new_password": "NewPass123",
	}, token)
	if passwordResponse.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/users/me/password status = %d, want %d", passwordResponse.Code, http.StatusOK)
	}
	updatedPasswordUser, err := db.GetUserByID(viewer.ID)
	if err != nil {
		t.Fatalf("GetUserByID(password updated) error = %v", err)
	}
	if !authcrypto.CheckPassword("NewPass123", updatedPasswordUser.PasswordHash) {
		t.Fatal("updated password hash does not match new password")
	}
}

func createHandlerTestUser(t *testing.T, db *store.DB, email, name, role, password string) *model.User {
	t.Helper()

	passwordHash, err := authcrypto.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword(%q) error = %v", password, err)
	}

	user := &model.User{
		Email:        email,
		Name:         name,
		PasswordHash: passwordHash,
		Role:         role,
	}
	if err := db.CreateUser(user); err != nil {
		t.Fatalf("CreateUser(%q) error = %v", email, err)
	}

	return user
}

func createHandlerTestInstance(t *testing.T, db *store.DB, name string) int64 {
	t.Helper()

	result, err := db.Exec(
		`INSERT INTO instances (name, source_type, source_path, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		name,
		"local",
		"/data/"+name,
		"idle",
	)
	if err != nil {
		t.Fatalf("insert instance %q error = %v", name, err)
	}

	instanceID, err := result.LastInsertId()
	if err != nil {
		t.Fatalf("LastInsertId() error = %v", err)
	}

	return instanceID
}

func mustAccessTokenForUser(t *testing.T, user *model.User, secret string) string {
	t.Helper()

	token, err := authcrypto.GenerateAccessToken(authcrypto.Claims{
		UserID: user.ID,
		Email:  user.Email,
		Role:   user.Role,
	}, secret)
	if err != nil {
		t.Fatalf("GenerateAccessToken() error = %v", err)
	}

	return token
}

func performAuthorizedJSONRequest(t *testing.T, handler http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()

	var requestBody *bytes.Reader
	if body == nil {
		requestBody = bytes.NewReader(nil)
	} else {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		requestBody = bytes.NewReader(payload)
	}

	request := httptest.NewRequest(method, path, requestBody)
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)
	return recorder
}

func assertAPIError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code int, message string) {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("status = %d, want %d", recorder.Code, status)
	}

	var payload Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if payload.Code != code {
		t.Fatalf("code = %d, want %d", payload.Code, code)
	}
	if payload.Message != message {
		t.Fatalf("message = %q, want %q", payload.Message, message)
	}
}

func assertRowCount(t *testing.T, db *store.DB, query string, arg any, want int) {
	t.Helper()

	var got int
	if err := db.QueryRow(query, arg).Scan(&got); err != nil {
		t.Fatalf("QueryRow(%q) error = %v", query, err)
	}
	if got != want {
		t.Fatalf("row count for %q = %d, want %d", query, got, want)
	}
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}