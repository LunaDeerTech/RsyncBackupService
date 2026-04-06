package main

import (
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"rsync-backup-service/internal/config"
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

	routerOptions := make([]handler.RouterOption, 0, 1)
	routerOptions = append(routerOptions, handler.WithJWTSecret(cfg.JWTSecret))
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
