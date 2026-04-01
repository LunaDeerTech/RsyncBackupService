package api

import (
	"net/http"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api/handler"
	"github.com/LunaDeerTech/RsyncBackupService/internal/api/middleware"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	AuthService       *service.AuthService
	InstanceService   *service.InstanceService
	SSHKeyService     *service.SSHKeyService
	StorageTargetService *service.StorageTargetService
	StrategyService   *service.StrategyService
	UserService       *service.UserService
	PermissionService *service.PermissionService
	AuditLogRepo      repository.AuditLogRepository
}

func NewRouter(deps Dependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.InjectServices(deps.AuthService, deps.PermissionService))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	authHandler := handler.NewAuthHandler(deps.AuthService)
	instanceHandler := handler.NewInstanceHandler(deps.InstanceService)
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
	instanceGroup.GET("/:id", middleware.RequireInstanceRole(service.RoleViewer), instanceHandler.Get)
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

	return router
}