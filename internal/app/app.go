package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/api"
	wspkg "github.com/LunaDeerTech/RsyncBackupService/internal/api/ws"
	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	executorpkg "github.com/LunaDeerTech/RsyncBackupService/internal/executor"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
	schedulerpkg "github.com/LunaDeerTech/RsyncBackupService/internal/scheduler"
	"github.com/LunaDeerTech/RsyncBackupService/internal/service"
	"gorm.io/gorm"
)

type App struct {
	Config config.Config
	DB     *gorm.DB
	server *http.Server
	stopProgressBridge func()
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
		notificationService := service.NewNotificationService(a.DB)
		sshKeyService := service.NewSSHKeyService(a.DB)
		strategyScheduler := schedulerpkg.NewScheduler()
		taskManager := executorpkg.NewTaskManager()
		executorService := service.NewExecutorService(a.DB, a.Config, nil, taskManager, notificationService)
		dashboardService := service.NewDashboardService(a.DB, a.Config, executorService)
		progressHub := wspkg.NewHub()
		a.stopProgressBridge = wspkg.BridgeProgress(executorService, progressHub)
		restoreService := service.NewRestoreService(a.DB, a.Config, nil, authService, notificationService)
		schedulerService := service.NewSchedulerService(strategyScheduler, executorService.RunStrategy)
		storageTargetService := service.NewStorageTargetService(a.DB, schedulerService)
		strategyService := service.NewStrategyService(a.DB, schedulerService)
		userService := service.NewUserService(a.DB, authService)
		permissionService := service.NewPermissionService(a.DB)
		auditService := service.NewAuditService(a.DB)
		auditRepo := repository.NewAuditLogRepository(a.DB)
		if err := service.BootstrapSchedules(context.Background(), a.DB, schedulerService); err != nil {
			return fmt.Errorf("bootstrap persisted strategy schedules: %w", err)
		}

		a.server = newHTTPServer(a.Config, api.NewRouter(api.Dependencies{
			AuthService:          authService,
			AuditService:         auditService,
			DashboardService:     dashboardService,
			ExecutorService:      executorService,
			InstanceService:      instanceService,
			NotificationService:  notificationService,
			ProgressHub:          progressHub,
			RestoreService:       restoreService,
			SSHKeyService:        sshKeyService,
			StorageTargetService: storageTargetService,
			StrategyService:      strategyService,
			UserService:          userService,
			PermissionService:    permissionService,
			AuditLogRepo:         auditRepo,
		}))
	}
	defer func() {
		if a.stopProgressBridge != nil {
			a.stopProgressBridge()
			a.stopProgressBridge = nil
		}
	}()

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
