package app

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/LunaDeerTech/RsyncBackupService/internal/config"
)

type App struct {
	Config config.Config
	server *http.Server
}

func New(cfg config.Config) *App {
	return &App{
		Config: cfg,
		server: newHTTPServer(cfg),
	}
}

func (a *App) Run() error {
	if err := os.MkdirAll(a.Config.DataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
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