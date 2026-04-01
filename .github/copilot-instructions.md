# Project Guidelines

## Current State

- This repository is still in the documentation-first bootstrap stage. Treat the files under `docs/superpowers/` as the source of truth until implementation catches up.
- Work in the 14-task sequence described in [docs/superpowers/prompts/README.md](../docs/superpowers/prompts/README.md). Do not skip prerequisites unless the user explicitly changes the order.
- Execute one task at a time. Keep changes scoped to the active prompt and its acceptance criteria.
- If the docs, prompts, and current code disagree, surface the conflict and ask instead of guessing.

## Architecture

- Preserve the planned Linux-only single-binary architecture: Go backend plus Vue 3 frontend embedded into the server binary via `embed.FS`.
- Keep backend structure aligned with the design doc: `config -> repository -> service -> executor/scheduler -> api`.
- Keep project structure aligned with the planned layout in [docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md](../docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md).
- Reuse the domain terms defined in the design doc: `backup instance`, `strategy`, `storage target`, `backup record`, `restore record`, `instance permission`, and `audit log`.
- Stay within v1 scope unless the user asks otherwise. Planned v1 storage/notifier implementations are `LocalStorage`, `SSHStorage`, and `SMTPNotifier`.

## Workflow

- Before implementing a task, read the matching prompt in [docs/superpowers/prompts/](../docs/superpowers/prompts/) and follow its boundaries, test requirements, and acceptance criteria.
- Prefer test-first execution when the prompt or plan calls for it.
- If you need the user's help to debug, need the user to confirm a decision, or are uncertain and need the user to choose or suggest a direction, use the `askQuestion` tool to ask instead of guessing.
- After completing a major task or milestone, use the existing `code-reviewer` agent in [.github/agents/code-reviewer.md](agents/code-reviewer.md) to review the result against the relevant prompt or plan step.
- Before claiming work is complete, run the relevant verification commands and report the actual results.
- Match the language and style of the surrounding file when updating documentation. The planning and design docs in this repo are primarily written in Chinese.

## Build And Test

- Only rely on commands that exist in the workspace. The plan documents expected Go, Makefile, Docker, and `web/` commands, but those commands are not available until Task 1 scaffolds them.
- Once Task 1 creates the toolchain, prefer the repo-local commands from the active task prompt, `Makefile`, and package scripts over ad hoc alternatives.
- When changing backend code, run the relevant Go tests for the touched package first, then broader verification as needed.
- When changing frontend code, use the repo-local `web/` scripts once they exist and keep tests/builds scoped to the affected area before broader verification.

## Frontend Conventions

- Follow the `Balanced Flux` design direction and token system defined in [docs/superpowers/specs/2026-04-01-rsync-component-style-design.md](../docs/superpowers/specs/2026-04-01-rsync-component-style-design.md).
- Keep brand and state semantics separate: `Cyan Mint` is the product brand color, while success, warning, error, and info states keep their own independent palettes.
- Do not introduce generic admin-template UI patterns that conflict with the design spec. Prefer the documented token names and component behaviors.

## References

- Task ordering and prompt usage: [docs/superpowers/prompts/README.md](../docs/superpowers/prompts/README.md)
- Implementation plan: [docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md](../docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md)
- Architecture and data model: [docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md](../docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md)
- Frontend visual system: [docs/superpowers/specs/2026-04-01-rsync-component-style-design.md](../docs/superpowers/specs/2026-04-01-rsync-component-style-design.md)
