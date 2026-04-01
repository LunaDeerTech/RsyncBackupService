package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"gorm.io/gorm"
)

type App struct {
	Config config.Config
	DB     *gorm.DB
	server *http.Server
}

func New(cfg config.Config) *App {
	return &App{
		Config: cfg,
	}
}

func (a *App) Run() error {
	if a.DB == nil {
		db, err := repository.OpenSQLite(a.Config.DataDir)
		if err != nil {
			return fmt.Errorf("open sqlite database: %w", err)
		}

		if err := repository.MigrateAndSeed(db, a.Config); err != nil {
			return fmt.Errorf("migrate and seed database: %w", err)
		}

		a.DB = db
	}

	if a.server == nil {
		authService := service.NewAuthService(a.DB, a.Config.JWTSecret)
		instanceService := service.NewInstanceService(a.DB)
		sshKeyService := service.NewSSHKeyService(a.DB)
		storageTargetService := service.NewStorageTargetService(a.DB)
		strategyService := service.NewStrategyService(a.DB)
		userService := service.NewUserService(a.DB, authService)
		permissionService := service.NewPermissionService(a.DB)
		auditRepo := repository.NewAuditLogRepository(a.DB)

		a.server = newHTTPServer(a.Config, api.NewRouter(api.Dependencies{
			AuthService:       authService,
			InstanceService:   instanceService,
			SSHKeyService:     sshKeyService,
			StorageTargetService: storageTargetService,
			StrategyService:   strategyService,
			UserService:       userService,
			PermissionService: permissionService,
			AuditLogRepo:      auditRepo,
		}))
	}

	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("run app server: %w", err)
	}

	return nil
}

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
