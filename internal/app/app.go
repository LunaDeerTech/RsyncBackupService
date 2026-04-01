package app

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
	"github.com/LunaDeerTech/RsyncBackupService/internal/repository"
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
		server: newHTTPServer(cfg),
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
		a.server = newHTTPServer(a.Config)
	}

	if err := a.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("run app server: %w", err)
	}

	return nil
}

func newHTTPServer(cfg config.Config) *http.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
}
