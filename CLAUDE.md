# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What is RBS

Rsync Backup Service (RBS) is a self-hosted backup management system built on rsync. It provides rolling incremental backups (via `--link-dest`) and cold full backups, with a web dashboard for management, scheduling, restore, risk monitoring, and audit logging. Single Go binary with embedded Vue 3 SPA, SQLite for persistence.

## Build and Run

```bash
make build          # build frontend + backend → bin/rbs
make test           # go test ./...
make docker         # docker build -t rbs:latest .
make clean          # remove bin/ and web/dist/

make dev-backend    # RBS_DEV_MODE=true go run ./cmd/server/main.go
make dev-frontend   # cd web && npm run dev
```

For a single Go test: `go test ./internal/store/ -run TestSomething`

The frontend dev server (port 8080) proxies `/api` to `127.0.0.1:8081`. Set `RBS_PORT=8081` in `.env` or update `web/vite.config.ts` proxy target to match.

## Environment

- Copy `.env.example` to `.env`. `RBS_JWT_SECRET` is required.
- Configuration: `.env` file + process env vars, env vars take precedence.
- Data directory layout: `data/keys`, `data/relay`, `data/temp`, `data/logs`.

## Architecture

Single Go service, dependency flow is one-way:

```
handler → service → store | engine
```

- `cmd/server/main.go` — entrypoint, wires up HTTP server and background scheduler
- `internal/config/` — config loading from `.env` + env vars
- `internal/model/` — data structs
- `internal/store/` — SQLite data access layer (CRUD)
- `internal/service/` — business logic orchestration
- `internal/engine/` — core engine: rsync execution, scheduler, task queue
- `internal/handler/` — HTTP handlers and router (options-based, under `/api/v1`)
- `internal/middleware/` — auth (JWT), logging, CORS
- `internal/notify/` — email/SMTP notifications
- `internal/audit/` — audit logging
- `internal/crypto/` — crypto utilities
- `internal/openlist/` — cloud storage integration via OpenList

All API responses use a JSON envelope: `{code, message, data}` (see `internal/handler/response.go`).

Frontend: `web/src/` — Vue 3 + TypeScript + Tailwind CSS v4 + Pinia + Vue Router.

## Conventions

- Prefer stdlib; keep external deps minimal (currently: JWT, crypto, SQLite).
- Reuse existing patterns in `internal/store`, `internal/service`, `internal/engine`, `internal/middleware`.
- Frontend uses a token-based design system (`web/src/styles/tokens.css`); reuse existing components and tokens.
- Admin-only features: dashboard, remote configs, backup targets, notifications, audit logs, system settings.
- Design docs live in `docs/` — see `docs/system-design.md` for architecture and `docs/development-plan.md` for phase ordering.
- Phase-specific acceptance criteria are in `docs/dev-prompt/`.
