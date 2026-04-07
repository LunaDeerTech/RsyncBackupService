package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"rsync-backup-service/internal/audit"
	"rsync-backup-service/internal/config"
	"rsync-backup-service/internal/engine"
	"rsync-backup-service/internal/handler"
	"rsync-backup-service/internal/middleware"
	"rsync-backup-service/internal/store"
	frontend "rsync-backup-service/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	if err := config.EnsureDataDirs(cfg.DataDir); err != nil {
		log.Fatalf("initialize data directory: %v", err)
	}

	db, err := store.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("close database: %v", err)
		}
	}()

	if err := db.Migrate(); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}))
	slog.SetDefault(logger)

	serverCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	healthChecker := engine.NewHealthChecker(db)
	healthChecker.StartSchedule(serverCtx)
	retentionCleaner := engine.NewRetentionCleaner(db, cfg.DataDir)

	taskQueue := engine.NewTaskQueue(cfg.WorkerPoolSize*4, db)
	scheduler := engine.NewScheduler(db, taskQueue)
	taskQueue.SetScheduler(scheduler)
	workerPool := engine.NewWorkerPool(
		cfg.WorkerPoolSize,
		taskQueue,
		engine.NewRollingBackupExecutor(nil, db),
		engine.NewColdBackupExecutor(nil, db, cfg.DataDir),
		db,
		retentionCleaner,
	)
	workerPool.SetAuditLogger(audit.NewLogger(db))
	if err := taskQueue.Recover(); err != nil {
		log.Fatalf("recover task queue: %v", err)
	}
	workerPool.Start(serverCtx)
	if err := scheduler.Start(serverCtx); err != nil {
		log.Fatalf("start scheduler: %v", err)
	}
	retentionCleaner.StartSchedule(serverCtx)

	routerOptions := make([]handler.RouterOption, 0, 1)
	routerOptions = append(routerOptions, handler.WithJWTSecret(cfg.JWTSecret))
	routerOptions = append(routerOptions, handler.WithDataDir(cfg.DataDir))
	routerOptions = append(routerOptions, handler.WithTaskQueue(taskQueue))
	routerOptions = append(routerOptions, handler.WithScheduler(scheduler))
	switch {
	case cfg.DevMode:
		logger.Info("embedded frontend disabled in development mode")
	case true:
		frontendFS, ok := frontend.DistFS()
		if !ok {
			logger.Warn("embedded frontend assets unavailable; static frontend disabled",
				"hint", "run make build to generate web/dist or set RBS_DEV_MODE=true for split frontend/backend development",
			)
			break
		}

		routerOptions = append(routerOptions, handler.WithFrontend(handler.NewFrontendHandler(frontendFS)))
		logger.Info("embedded frontend enabled")
	}

	router := middleware.Logger(middleware.CORS(handler.NewRouter(db, routerOptions...)))
	server := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		<-serverCtx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server shutdown failed", "error", err)
		}
	}()

	logger.Info("RBS backend startup completed",
		"addr", server.Addr,
		"data_dir", cfg.DataDir,
	)

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server exited", "error", err)
		os.Exit(1)
	}
}

func parseLogLevel(raw string) slog.Level {
	switch raw {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
