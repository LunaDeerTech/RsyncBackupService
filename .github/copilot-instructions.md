# Project Guidelines

## Architecture

- RBS is a single Go service with an embedded Vue 3 SPA. Preserve that shape unless a task explicitly calls for architectural change. See `docs/system-design.md` for system boundaries and `docs/development-plan.md` for phase order and acceptance scope.
- Keep backend dependency flow one-way: `handler -> service -> store|engine`. Avoid pushing HTTP concerns into lower layers or introducing reverse dependencies. `cmd/server/main.go` is the reference for application wiring.
- Keep APIs under `/api/v1` and use the existing JSON envelope from `internal/handler/response.go` (`code`, `message`, `data`). Register routes through the options-based router in `internal/handler/router.go`.
- Configuration is loaded from `.env` plus process environment variables, with environment values taking precedence. `RBS_JWT_SECRET` is required, and the data directory layout under `data/keys`, `data/relay`, `data/temp`, and `data/logs` is part of the runtime contract.

## Build and Test

- Use the root `Makefile` first: `make build`, `make test`, `make docker`, `make clean`.
- For split development, use `make dev-backend` and `make dev-frontend`. Frontend-only validation is `cd web && npm run build`.
- Watch the local dev port mismatch: `web/vite.config.ts` proxies `/api` to `127.0.0.1:8081`, while `.env.example` defaults `RBS_PORT` to `8080`. Set `RBS_PORT=8081` locally or update the proxy before relying on split frontend/backend development.
- For embed, container, or deployment-facing changes, also verify `docker compose config` and, when practical, `docker compose up -d --build`.

## Conventions

- Prefer stdlib-first Go changes and keep new dependencies rare. The project already relies on a small set of external packages for JWT, crypto, and SQLite.
- Reuse existing backend patterns before inventing new ones: `internal/store` for persistence, `internal/service` for orchestration, `internal/engine` for backup/scheduler/task execution, `internal/middleware` for cross-cutting request logic.
- Reuse the frontend token system and component patterns instead of introducing one-off styles. See `docs/component-style-design.md`, `web/src/styles/tokens.css`, `web/src/components`, and `web/src/layouts`.
- Respect role boundaries already enforced by the backend. Admin-only areas include dashboard surfaces, remote configs, backup targets, notifications, audit logs, and most system settings.
- Use the matching prompt under `docs/dev-prompt/` when implementing or extending a planned phase. Those files are the scope and acceptance source for feature work.
- Before modifying a completed feature area, search `/memories/repo` for matching task notes. The repo already has recorded pitfalls around snapshot uniqueness, retention scope, stale detail-page requests, task resync after reconnect, and exact notification event names.