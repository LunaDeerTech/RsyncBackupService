# Project Guidelines

## Current State

- Treat `docs/superpowers/` as the source of truth whenever implementation, prompts, and specs diverge.
- Follow the 14-task sequence in [docs/superpowers/prompts/README.md](../docs/superpowers/prompts/README.md) and execute one task at a time unless the user explicitly changes the order.
- Keep changes scoped to the active task prompt and its acceptance criteria; surface scope or prerequisite conflicts instead of guessing.

## Architecture

- Preserve the planned Linux-only single-binary architecture: Go backend plus Vue 3 frontend embedded into the server binary via `embed.FS`.
- Keep backend responsibilities aligned with the design doc layer order: `config -> repository -> service -> executor/scheduler -> api`.
- Reuse the domain terms defined in the design doc: `backup instance`, `strategy`, `storage target`, `backup record`, `restore record`, `instance permission`, and `audit log`.
- Stay within v1 scope unless the user asks otherwise. Planned v1 storage/notifier implementations are `LocalStorage`, `SSHStorage`, and `SMTPNotifier`.

## Workflow

- Before implementing a task, read the matching prompt in [docs/superpowers/prompts/](../docs/superpowers/prompts/) and follow its boundaries, test requirements, and acceptance criteria.
- If the user asks you to work from the implementation plan directly, use the `subagent-driven-development` skill or `executing-plans` skill as required by the plan header.
- Prefer test-first execution when the prompt or plan calls for it.
- If you hit blockers, missing context, need confirmation, see docs/code conflicts, or a choice would change scope, stop and ask instead of guessing.
- After completing a major task or milestone, use the existing `code-reviewer` agent in [.github/agents/code-reviewer.md](agents/code-reviewer.md) to review the result against the relevant prompt or plan step.
- Before claiming work is complete, run the relevant verification commands and report the actual results.
- Match the language and style of the surrounding file when updating documentation. The planning and design docs in this repo are primarily written in Chinese.

## Build And Test

- `go.mod` requires Go `1.22`.
- Prefer repo-local commands: `make build`, `make test`, `make run`, `make build-backend`, `make build-frontend`, `make test-backend`, and `make test-frontend`.
- For focused backend changes, run the narrowest relevant package tests first, then broaden to `go test ./...` before claiming completion.
- For focused frontend changes, use `npm --prefix web run test -- <spec>` or `npm --prefix web run test`, and build with `npm --prefix web run build` when UI assets are affected.
- Use `docker build -t <tag> .` or `docker compose up` only for packaging and integration checks. The current Dockerfile builds the Go service but does not yet build or embed frontend assets, so packaging work should follow Task 14.

## Frontend Conventions

- Follow the `Balanced Flux` design direction and token system defined in [docs/superpowers/specs/2026-04-01-rsync-component-style-design.md](../docs/superpowers/specs/2026-04-01-rsync-component-style-design.md).
- Keep brand and state semantics separate: `Cyan Mint` is the product brand color, while success, warning, error, and info states keep their own independent palettes.
- Do not introduce generic admin-template UI patterns that conflict with the design spec. Prefer the documented token names and component behaviors.

## References

- Task ordering and prompt usage: [docs/superpowers/prompts/README.md](../docs/superpowers/prompts/README.md)
- Implementation plan: [docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md](../docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md)
- Architecture and data model: [docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md](../docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md)
- Frontend visual system: [docs/superpowers/specs/2026-04-01-rsync-component-style-design.md](../docs/superpowers/specs/2026-04-01-rsync-component-style-design.md)
