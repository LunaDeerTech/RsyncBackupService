package handler

import (
	"bytes"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"rsync-backup-service/internal/audit"
	authcrypto "rsync-backup-service/internal/crypto"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/notify"
	"rsync-backup-service/internal/service"
	"rsync-backup-service/internal/store"
)

type Handler struct {
	db                *store.DB
	jwtSecret         string
	passwordSender    notify.PasswordSender
	passwordGenerator func() (string, error)
	loginLimiter      *loginRateLimiter
	remoteConfigs     *service.RemoteConfigService
	systemConfigs     *service.SystemConfigService
	taskQueue         *engine.TaskQueue
	scheduler         *engine.Scheduler
	disasterRecovery  *service.DisasterRecoveryService
	downloadTokens    *DownloadTokenManager
	audit             *audit.Logger
}

type RouterOption func(*routerOptions)

type routerOptions struct {
	frontend          http.Handler
	jwtSecret         string
	passwordSender    notify.PasswordSender
	passwordGenerator func() (string, error)
	loginLimiter      *loginRateLimiter
	dataDir           string
	remoteConfigs     *service.RemoteConfigService
	systemConfigs     *service.SystemConfigService
	taskQueue         *engine.TaskQueue
	scheduler         *engine.Scheduler
	disasterRecovery  *service.DisasterRecoveryService
	downloadTokens    *DownloadTokenManager
}

func WithFrontend(frontend http.Handler) RouterOption {
	return func(options *routerOptions) {
		options.frontend = frontend
	}
}

func WithJWTSecret(secret string) RouterOption {
	return func(options *routerOptions) {
		options.jwtSecret = secret
	}
}

func WithDataDir(dataDir string) RouterOption {
	return func(options *routerOptions) {
		options.dataDir = dataDir
	}
}

func WithTaskQueue(taskQueue *engine.TaskQueue) RouterOption {
	return func(options *routerOptions) {
		options.taskQueue = taskQueue
	}
}

func WithScheduler(scheduler *engine.Scheduler) RouterOption {
	return func(options *routerOptions) {
		options.scheduler = scheduler
	}
}

func WithDisasterRecoveryService(disasterRecovery *service.DisasterRecoveryService) RouterOption {
	return func(options *routerOptions) {
		options.disasterRecovery = disasterRecovery
	}
}

func WithSystemConfigService(systemConfigs *service.SystemConfigService) RouterOption {
	return func(options *routerOptions) {
		options.systemConfigs = systemConfigs
	}
}

func withPasswordSender(sender notify.PasswordSender) RouterOption {
	return func(options *routerOptions) {
		options.passwordSender = sender
	}
}

func withPasswordGenerator(generator func() (string, error)) RouterOption {
	return func(options *routerOptions) {
		options.passwordGenerator = generator
	}
}

func withLoginLimiter(limiter *loginRateLimiter) RouterOption {
	return func(options *routerOptions) {
		options.loginLimiter = limiter
	}
}

func withRemoteConfigService(remoteConfigs *service.RemoteConfigService) RouterOption {
	return func(options *routerOptions) {
		options.remoteConfigs = remoteConfigs
	}
}

func NewRouter(db *store.DB, options ...RouterOption) http.Handler {
	resolved := routerOptions{}
	for _, option := range options {
		option(&resolved)
	}
	if resolved.passwordGenerator == nil {
		resolved.passwordGenerator = generateRandomPassword
	}
	if resolved.loginLimiter == nil {
		resolved.loginLimiter = newLoginRateLimiter(time.Now)
	}
	if strings.TrimSpace(resolved.dataDir) == "" {
		resolved.dataDir = filepath.Join(".", "data")
	}
	aesKey := authcrypto.DeriveAESKey(resolved.jwtSecret)
	if resolved.remoteConfigs == nil {
		resolved.remoteConfigs = service.NewRemoteConfigService(db, resolved.dataDir, nil)
	}
	if resolved.systemConfigs == nil {
		resolved.systemConfigs = service.NewSystemConfigService(db, aesKey)
	}
	if resolved.passwordSender == nil {
		resolved.passwordSender = notify.NewPasswordSender(db, aesKey)
	}
	if resolved.downloadTokens == nil {
		resolved.downloadTokens = NewDownloadTokenManager()
	}
	if resolved.disasterRecovery == nil {
		resolved.disasterRecovery = service.NewDisasterRecoveryService(db)
	}
	auditLogger := audit.NewLogger(db)
	resolved.remoteConfigs.SetAuditLogger(auditLogger)

	handler := &Handler{
		db:                db,
		jwtSecret:         resolved.jwtSecret,
		passwordSender:    resolved.passwordSender,
		passwordGenerator: resolved.passwordGenerator,
		loginLimiter:      resolved.loginLimiter,
		remoteConfigs:     resolved.remoteConfigs,
		systemConfigs:     resolved.systemConfigs,
		taskQueue:         resolved.taskQueue,
		scheduler:         resolved.scheduler,
		disasterRecovery:  resolved.disasterRecovery,
		downloadTokens:    resolved.downloadTokens,
		audit:             auditLogger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", handler.Health)
	mux.HandleFunc("GET /api/v2/openapi.json", handler.OpenAPIDocument)
	mux.HandleFunc("GET /api/v1/system/registration", handler.GetRegistrationStatus)
	mux.HandleFunc("POST /api/v1/auth/register", handler.Register)
	mux.HandleFunc("POST /api/v1/auth/login", handler.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", handler.Refresh)
	authenticated := middleware.Auth(resolved.jwtSecret)
	apiKeyAuthenticated := middleware.APIKeyAuth(db)
	mux.Handle("GET /api/v1/system/smtp", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.GetSMTPConfig))))
	mux.Handle("PUT /api/v1/system/smtp", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateSMTPConfig))))
	mux.Handle("POST /api/v1/system/smtp/test", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.TestSMTP))))
	mux.Handle("PUT /api/v1/system/registration", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateRegistrationStatus))))
	mux.Handle("GET /api/v1/audit-logs", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListAuditLogs))))
	mux.Handle("GET /api/v1/users", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListUsers))))
	mux.Handle("POST /api/v1/users", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CreateUser))))
	mux.Handle("PUT /api/v1/users/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateUser))))
	mux.Handle("DELETE /api/v1/users/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.DeleteUser))))
	mux.Handle("POST /api/v1/users/{id}/reset-password", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ResetUserPassword))))
	mux.Handle("GET /api/v1/remotes", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListRemoteConfigs))))
	mux.Handle("POST /api/v1/remotes", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CreateRemoteConfig))))
	mux.Handle("PUT /api/v1/remotes/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateRemoteConfig))))
	mux.Handle("DELETE /api/v1/remotes/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.DeleteRemoteConfig))))
	mux.Handle("POST /api/v1/remotes/{id}/test", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.TestRemoteConfig))))
	mux.Handle("GET /api/v1/targets", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListBackupTargets))))
	mux.Handle("POST /api/v1/targets", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CreateBackupTarget))))
	mux.Handle("PUT /api/v1/targets/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateBackupTarget))))
	mux.Handle("DELETE /api/v1/targets/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.DeleteBackupTarget))))
	mux.Handle("POST /api/v1/targets/{id}/health-check", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CheckBackupTargetHealth))))
	mux.Handle("GET /api/v1/instances", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.ListInstances))))
	mux.Handle("POST /api/v1/instances", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CreateInstance))))
	mux.Handle("GET /api/v1/instances/{id}", authenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetInstance)))))
	mux.Handle("GET /api/v1/instances/{id}/audit-logs", authenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.ListInstanceAuditLogs)))))
	mux.Handle("GET /api/v1/instances/{id}/disaster-recovery", authenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetDisasterRecoveryScore)))))
	mux.Handle("PUT /api/v1/instances/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateInstance))))
	mux.Handle("DELETE /api/v1/instances/{id}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.DeleteInstance))))
	mux.Handle("GET /api/v1/instances/{id}/stats", authenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetInstanceStats)))))
	mux.Handle("GET /api/v1/instances/{id}/policies", authenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.ListPolicies)))))
	mux.Handle("POST /api/v1/instances/{id}/policies", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CreatePolicy))))
	mux.Handle("PUT /api/v1/instances/{id}/policies/{pid}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdatePolicy))))
	mux.Handle("DELETE /api/v1/instances/{id}/policies/{pid}", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.DeletePolicy))))
	mux.Handle("POST /api/v1/instances/{id}/policies/{pid}/trigger", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.TriggerPolicy))))
	mux.Handle("GET /api/v1/instances/{id}/backups", authenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.ListBackups)))))
	mux.Handle("POST /api/v1/instances/{id}/backups/{bid}/restore", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.RestoreBackup))))
	mux.Handle("GET /api/v1/instances/{id}/backups/{bid}/download", authenticated(middleware.RequireAuth(middleware.RequireInstanceDownload(db)(http.HandlerFunc(handler.GenerateBackupDownloadURL)))))
	mux.HandleFunc("GET /api/v1/download/{token}", handler.DownloadBackupByToken)
	mux.Handle("GET /api/v1/tasks", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListTasks))))
	mux.Handle("GET /api/v1/tasks/{id}", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.GetTask))))
	mux.Handle("POST /api/v1/tasks/{id}/cancel", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.CancelTask))))
	mux.Handle("GET /api/v1/dashboard/overview", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.GetDashboardOverview))))
	mux.Handle("GET /api/v1/dashboard/risks", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListDashboardRisks))))
	mux.Handle("GET /api/v1/dashboard/trends", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.GetDashboardTrends))))
	mux.Handle("GET /api/v1/dashboard/focus-instances", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListDashboardFocusInstances))))
	mux.Handle("GET /api/v1/dashboard/upcoming-tasks", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListDashboardUpcomingTasks))))
	mux.Handle("PUT /api/v1/instances/{id}/permissions", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.UpdateInstancePermissions))))
	mux.Handle("GET /api/v1/instances/{id}/permissions", authenticated(middleware.RequireAdmin(http.HandlerFunc(handler.ListInstancePermissions))))
	mux.Handle("GET /api/v1/users/me", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.GetCurrentUser))))
	mux.Handle("GET /api/v1/users/me/api-keys", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.ListCurrentUserAPIKeys))))
	mux.Handle("POST /api/v1/users/me/api-keys", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.CreateCurrentUserAPIKey))))
	mux.Handle("DELETE /api/v1/users/me/api-keys/{id}", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.DeleteCurrentUserAPIKey))))
	mux.Handle("GET /api/v1/users/me/subscriptions", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.GetCurrentUserSubscriptions))))
	mux.Handle("PUT /api/v1/users/me/password", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.UpdateCurrentUserPassword))))
	mux.Handle("PUT /api/v1/users/me/profile", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.UpdateCurrentUserProfile))))
	mux.Handle("PUT /api/v1/users/me/subscriptions", authenticated(middleware.RequireAuth(http.HandlerFunc(handler.UpdateCurrentUserSubscriptions))))
	mux.Handle("GET /api/v2/instances", apiKeyAuthenticated(middleware.RequireAuth(http.HandlerFunc(handler.ListV2Instances))))
	mux.Handle("GET /api/v2/instances/{id}/overview", apiKeyAuthenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetV2InstanceOverview)))))
	mux.Handle("GET /api/v2/instances/{id}/current-task", apiKeyAuthenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetV2InstanceCurrentTask)))))
	mux.Handle("GET /api/v2/instances/{id}/policies", apiKeyAuthenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.ListV2InstancePolicies)))))
	mux.Handle("GET /api/v2/instances/{id}/plan", apiKeyAuthenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetV2InstancePlan)))))
	mux.Handle("GET /api/v2/instances/{id}/disaster-recovery", apiKeyAuthenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.GetV2DisasterRecoveryScore)))))
	mux.Handle("GET /api/v2/instances/{id}/backups", apiKeyAuthenticated(middleware.RequireAuth(middleware.RequireInstanceAccess(db)(http.HandlerFunc(handler.ListV2InstanceBackups)))))
	if resolved.frontend != nil {
		mux.Handle("/", resolved.frontend)
	}

	return withAPIErrors(middleware.CORS(mux))
}

func withAPIErrors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAPIPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		recorder := newBufferedResponseWriter()
		next.ServeHTTP(recorder, r)

		switch recorder.statusCode() {
		case http.StatusNotFound:
			Error(w, http.StatusNotFound, 40401, "resource not found")
			return
		case http.StatusMethodNotAllowed:
			copyHeaders(w.Header(), recorder.Header())
			Error(w, http.StatusMethodNotAllowed, 40001, "method not allowed")
			return
		}

		copyHeaders(w.Header(), recorder.Header())
		w.WriteHeader(recorder.statusCode())
		_, _ = w.Write(recorder.body.Bytes())
	})
}

func isAPIPath(path string) bool {
	if strings.HasPrefix(path, "/api/v1/download/") {
		return false
	}
	return path == "/api" || strings.HasPrefix(path, "/api/")
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}

type bufferedResponseWriter struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedResponseWriter() *bufferedResponseWriter {
	return &bufferedResponseWriter{header: make(http.Header)}
}

func (w *bufferedResponseWriter) Header() http.Header {
	return w.header
}

func (w *bufferedResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}

	return w.body.Write(data)
}

func (w *bufferedResponseWriter) WriteHeader(status int) {
	w.status = status
}

func (w *bufferedResponseWriter) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}

	return w.status
}
