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

	permissionGroup := apiGroup.Group("/instances/:id/permissions", middleware.RequireJWT())
	permissionGroup.GET("", middleware.RequireAdmin(), permissionHandler.List)
	permissionGroup.PUT("/:userID", middleware.WithAuditMetadata(middleware.AuditMetadata{Action: "instance_permissions.upsert", ResourceType: "instance_permissions"}), middleware.RequireAdmin(), permissionHandler.Upsert)

	return router
}