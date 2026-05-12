# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is RBS

Rsync Backup Service (RBS) is a self-hosted backup management system built on rsync. It provides rolling incremental backups (via `--link-dest`) and cold full backups, with a web dashboard for management, scheduling, restore, risk monitoring, and audit logging. It ships as a single Go binary with an embedded Vue 3 SPA and uses SQLite for persistence.

## Build, Test, and Dev

```bash
make build          # install frontend deps, build web/dist, build bin/rbs
make test           # go test ./...
make docker         # docker build -t rbs:latest .
make clean          # remove bin/ and web/dist/

make dev-backend    # RBS_DEV_MODE=true go run ./cmd/server/main.go
make dev-frontend   # cd web && npm run dev
cd web && npm run build   # frontend type-check + production build
```

- Single Go test: `go test ./internal/store -run TestSomething`
- There is no dedicated lint target in the repo today; use `make test` for backend validation and `cd web && npm run build` for frontend validation.
- `make build` requires the frontend build first because production assets are embedded into the Go binary.
- For deploy-facing changes, also verify `docker compose config` and, when practical, `docker compose up -d --build`.

## Environment

- Copy `.env.example` to `.env`. `RBS_JWT_SECRET` is required.
- Config loads from `.env` plus process env vars; env vars take precedence.
- `.env.example` defaults `RBS_PORT=8080`, but the Vite dev server proxies `/api` to `http://127.0.0.1:8081`. For split frontend/backend dev, set `RBS_PORT=8081` locally or update `web/vite.config.ts`.
- Runtime data directory contract: `data/keys`, `data/relay`, `data/temp`, `data/logs`.
- Docker Compose mounts `/data` and sets `RBS_DATA_DIR=/data`.
- Local development/runtime prerequisites from the README: Go 1.22+, Node.js 20+, npm 10+, rsync 3.1+, and `openssh-client`.

## Architecture

RBS is a single-process Go application with an embedded SPA. Keep the backend dependency flow one-way:

```text
handler -> service -> store | engine
```

Key runtime wiring lives in `cmd/server/main.go`:

- load config and ensure data directories
- open SQLite and run migrations
- derive the AES key from `RBS_JWT_SECRET`
- start background subsystems: email sender, disaster recovery cache, risk detector, health checker, retention cleaner, task queue, scheduler, and worker pool
- construct the router, optionally mount embedded frontend assets, and start the HTTP server

That startup flow is the quickest way to understand which subsystems are long-lived and how they connect.

### Backend shape

- `internal/store/` — SQLite persistence and migrations
- `internal/service/` — business orchestration and higher-level domain logic
- `internal/engine/` — rsync executors, scheduler, task queue/worker pool, retention cleanup, health checks, and risk detection
- `internal/handler/` — HTTP handlers and router composition
- `internal/middleware/` — auth, CORS, CSRF, logging, and permission gates
- `internal/audit/`, `internal/notify/`, `internal/crypto/`, `internal/openlist/` — cross-cutting support modules

### API surface

`internal/handler/router.go` is the best map of the product surface.

- `/api/v1` is the main SPA/backend API surface, mostly JWT-authenticated.
- `/api/v2` exposes API-key-authenticated instance-facing endpoints and the OpenAPI document.
- Routes are assembled with `RouterOption`s so `main.go` can inject shared infrastructure like the scheduler, task queue, data dir, and system-config services.
- API responses use the JSON envelope `{code, message, data}` from `internal/handler/response.go`.
- `withAPIErrors` normalizes API-path 404/405 responses, so preserve that behavior when adding routes.

### Task execution model

Most non-trivial backup behavior fans into `internal/engine/`:

- `TaskQueue` persists and recovers queued/running work across restarts.
- `Scheduler` turns policy schedules into queued tasks.
- `WorkerPool` executes rolling backup, cold backup, and restore work.
- `RetentionCleaner` and `HealthChecker` run as background maintenance jobs.
- `RiskDetector` and disaster-recovery services turn execution/health state into risk signals and notifications.

When changing backup lifecycle behavior, follow the flow across `store` state, `engine` execution, and audit/risk side effects rather than editing a single layer in isolation.

### Frontend shape

- `web/src/` is a Vue 3 + TypeScript SPA using Pinia, Vue Router, and Tailwind CSS v4.
- Production frontend assets are embedded into the Go binary; when `RBS_DEV_MODE=true`, run the backend and Vite dev server separately instead.
- Reuse the token-based design system in `web/src/styles/tokens.css` and the patterns described in `docs/component-style-design.md`.

## Conventions

- Prefer stdlib-first Go changes; keep new dependencies rare.
- Reuse existing backend patterns before inventing new abstractions: persistence in `store`, orchestration in `service`, execution/scheduling in `engine`, cross-cutting request logic in `middleware`.
- Keep APIs under the existing `/api/v1` or `/api/v2` surfaces and preserve the current auth model and response envelope.
- Respect role boundaries already present in the router: dashboard, remote configs, backup targets, notifications, audit logs, and most system settings are admin-only; instance access is permission-scoped.
- Use `docs/system-design.md` for system boundaries, `docs/development-plan.md` for phase order, and the matching file under `docs/dev-prompt/` as the scope/acceptance source when implementing planned features.
