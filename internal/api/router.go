package api

import (
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/handler"
	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	wspkg "github.com/LunaDeerTech/RsyncBackupService/internal/api/ws"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	AuthService          *service.AuthService
	AuditService         *service.AuditService
	DashboardService     *service.DashboardService
	ExecutorService      *service.ExecutorService
	InstanceService      *service.InstanceService
	NotificationService  *service.NotificationService
	ProgressHub          *wspkg.Hub
	RestoreService       *service.RestoreService
	SSHKeyService        *service.SSHKeyService
	StorageTargetService *service.StorageTargetService
	StrategyService      *service.StrategyService
	UserService          *service.UserService
	PermissionService    *service.PermissionService
	AuditLogRepo         repository.AuditLogRepository
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.InjectServices(deps.AuthService, deps.PermissionService))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := handler.NewAuthHandler(deps.AuthService)
	auditHandler := handler.NewAuditHandler(deps.AuditService)
	backupHandler := handler.NewBackupHandler(deps.ExecutorService)
	instanceHandler := handler.NewInstanceHandler(deps.InstanceService)
	notificationHandler := handler.NewNotificationHandler(deps.NotificationService)
	systemHandler := handler.NewSystemHandler(deps.DashboardService)
	taskHandler := handler.NewTaskHandler(deps.ExecutorService)
	restoreHandler := handler.NewRestoreHandler(deps.RestoreService)
	sshKeyHandler := handler.NewSSHKeyHandler(deps.SSHKeyService)
	storageTargetHandler := handler.NewStorageTargetHandler(deps.StorageTargetService)
	strategyHandler := handler.NewStrategyHandler(deps.StrategyService)
	userHandler := handler.NewUserHandler(deps.UserService)
	permissionHandler := handler.NewPermissionHandler(deps.PermissionService)

	apiGroup := router.Group("/api")
	apiGroup.Use(middleware.AuditLogger(deps.AuditLogRepo))

	authGroup := apiGroup.Group("/auth")
	authGroup.POST("/login", authHandler.Login)
	authGroup.POST("/refresh", authHandler.Refresh)
	authGroup.POST("/verify", middleware.RequireJWT(), authHandler.Verify)
	authGroup.GET("/me", middleware.RequireJWT(), authHandler.Me)
	authGroup.PUT("/password", middleware.RequireJWT(), authHandler.ChangePassword)

	userGroup := apiGroup.Group("/users", middleware.RequireJWT())
	userGroup.GET("", middleware.RequireAdmin(), userHandler.List)
	userGroup.POST("", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "users.create", ResourceType: "users"}), middleware.RequireAdmin(), userHandler.Create)
	userGroup.PUT("/:id/password", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "users.password.reset", ResourceType: "users"}), middleware.RequireAdmin(), middleware.RequireVerifyToken(), userHandler.ResetPassword)

	instanceGroup := apiGroup.Group("/instances", middleware.RequireJWT())
	instanceGroup.GET("", instanceHandler.List)
	instanceGroup.POST("", instanceHandler.Create)
	instanceGroup.GET("/:id/backups", middleware.RequireInstanceRole(service.RoleViewer), backupHandler.List)
	instanceGroup.GET("/:id/snapshots", middleware.RequireInstanceRole(service.RoleViewer), backupHandler.ListSnapshots)
	instanceGroup.GET("/:id", middleware.RequireInstanceRole(service.RoleViewer), instanceHandler.Get)
	instanceGroup.POST("/:id/restore", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "instances.restore", ResourceType: "restore_records"}), middleware.RequireInstanceRole(service.RoleAdmin), middleware.RequireVerifyToken(), restoreHandler.Create)
	instanceGroup.PUT("/:id", middleware.RequireInstanceRole(service.RoleAdmin), instanceHandler.Update)
	instanceGroup.DELETE("/:id", middleware.RequireInstanceRole(service.RoleAdmin), middleware.RequireVerifyToken(), instanceHandler.Delete)

	instanceStrategyGroup := apiGroup.Group("/instances/:id/strategies", middleware.RequireJWT())
	instanceStrategyGroup.GET("", middleware.RequireInstanceRole(service.RoleViewer), strategyHandler.ListByInstance)
	instanceStrategyGroup.POST("", middleware.RequireInstanceRole(service.RoleAdmin), strategyHandler.Create)

	strategyGroup := apiGroup.Group("/strategies", middleware.RequireJWT())
	strategyGroup.PUT("/:id", strategyHandler.Update)
	strategyGroup.DELETE("/:id", strategyHandler.Delete)

	storageTargetGroup := apiGroup.Group("/storage-targets", middleware.RequireJWT(), middleware.RequireAdmin())
	storageTargetGroup.GET("", storageTargetHandler.List)
	storageTargetGroup.POST("", storageTargetHandler.Create)
	storageTargetGroup.PUT("/:id", storageTargetHandler.Update)
	storageTargetGroup.DELETE("/:id", storageTargetHandler.Delete)
	storageTargetGroup.POST("/:id/test", storageTargetHandler.TestConnection)

	sshKeyGroup := apiGroup.Group("/ssh-keys", middleware.RequireJWT(), middleware.RequireAdmin())
	sshKeyGroup.GET("", sshKeyHandler.List)
	sshKeyGroup.POST("", sshKeyHandler.Create)
	sshKeyGroup.DELETE("/:id", sshKeyHandler.Delete)
	sshKeyGroup.POST("/:id/test", sshKeyHandler.TestConnection)

	permissionGroup := apiGroup.Group("/instances/:id/permissions", middleware.RequireJWT())
	permissionGroup.GET("", middleware.RequireAdmin(), permissionHandler.List)
	permissionGroup.PUT("/:userID", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "instance_permissions.upsert", ResourceType: "instance_permissions"}), middleware.RequireAdmin(), permissionHandler.Upsert)
	instanceGroup.GET("/:id/subscriptions", middleware.RequireInstanceRole(service.RoleViewer), notificationHandler.ListSubscriptions)
	instanceGroup.POST("/:id/subscriptions", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "notification_subscriptions.upsert", ResourceType: "notification_subscriptions"}), middleware.RequireInstanceRole(service.RoleViewer), notificationHandler.UpsertSubscription)

	taskGroup := apiGroup.Group("/tasks", middleware.RequireJWT(), middleware.RequireAdmin())
	taskGroup.GET("/running", taskHandler.ListRunning)
	taskGroup.POST("/:id/cancel", taskHandler.Cancel)

	systemGroup := apiGroup.Group("/system", middleware.RequireJWT(), middleware.RequireAdmin())
	systemGroup.GET("/status", systemHandler.Status)
	systemGroup.GET("/dashboard", systemHandler.Dashboard)

	wsGroup := apiGroup.Group("/ws", middleware.RequireWebSocketJWT(), middleware.RequireAdmin())
	wsGroup.GET("/progress", wspkg.ServeProgress(deps.ProgressHub))

	restoreRecordGroup := apiGroup.Group("/restore-records", middleware.RequireJWT())
	restoreRecordGroup.GET("", restoreHandler.List)

	notificationChannelGroup := apiGroup.Group("/notification-channels", middleware.RequireJWT())
	notificationChannelGroup.GET("", notificationHandler.ListChannels)
	notificationChannelGroup.POST("", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "notification_channels.create", ResourceType: "notification_channels"}), middleware.RequireAdmin(), notificationHandler.CreateChannel)
	notificationChannelGroup.PUT("/:id", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "notification_channels.update", ResourceType: "notification_channels"}), middleware.RequireAdmin(), notificationHandler.UpdateChannel)
	notificationChannelGroup.DELETE("/:id", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "notification_channels.delete", ResourceType: "notification_channels"}), middleware.RequireAdmin(), notificationHandler.DeleteChannel)
	notificationChannelGroup.POST("/:id/test", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "notification_channels.test", ResourceType: "notification_channels"}), middleware.RequireAdmin(), notificationHandler.TestChannel)

	subscriptionGroup := apiGroup.Group("/subscriptions", middleware.RequireJWT())
	subscriptionGroup.DELETE("/:id", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "notification_subscriptions.delete", ResourceType: "notification_subscriptions"}), notificationHandler.DeleteSubscription)

	auditGroup := apiGroup.Group("/audit-logs", middleware.RequireJWT())
	auditGroup.GET("", middleware.RequireAdmin(), auditHandler.List)

	return router
}
