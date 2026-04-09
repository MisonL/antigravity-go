# Repository Guidelines

## Project Structure & Module Organization
- `cmd/ago/`: CLI entrypoint and workflow commands.
- `internal/`: core backend modules (`agent`, `server`, `llm`, `core`, `tools`, `rpc`, `session`, etc.).
- `frontend/src/`: React + TypeScript UI; build output is copied into `internal/server/dist/` for embedding.
- `docs/`: product, technical, and review records.
- `scripts/`: maintenance scripts (for example, core extraction/update).
- `internal/server/dist/` and `frontend/dist/` are generated artifacts; do not hand-edit.

## Build, Test, and Development Commands
- `make update-core`: extract/update the `antigravity_core` binary metadata.
- `make build`: build frontend first, then compile backend binary `ago`.
- `make run`: run web mode locally (`./ago --web --no-tui`).
- `go test ./...`: run all Go tests.
- `cd frontend && bun install && bun run dev`: start frontend dev server.
- `cd frontend && bun run lint && bun run build`: run ESLint and production build (`tsc` + Vite).

## Coding Style & Naming Conventions
- Go target is `1.24`; always format Go code with `gofmt -w`.
- Go package names are lowercase; exported symbols use `PascalCase`, internal helpers use `camelCase`.
- React component files use `PascalCase` (for example, `TerminalPanel.tsx`); hooks follow `useXxx` naming.
- Keep errors explicit and avoid silent fallbacks in core flows.
- Respect `.gitignore` and do not commit logs, caches, or local binaries.

## Testing Guidelines
- Backend tests are `*_test.go` files colocated with their packages.
- Prefer scoped verification during development (for example, `go test ./internal/server -run TestTasks`), then run `go test ./...` before submission.
- Frontend quality gate is `bun run lint` plus `bun run build`; ensure both pass for UI-related changes.
- Add or update tests whenever behavior changes.

## Commit & Pull Request Guidelines
- Use Conventional Commits, consistent with history: `feat(...)`, `fix(...)`, `refactor(...)`, `docs:`, `chore:`.
- Keep one logical change per commit; include scope when useful (for example, `feat(server): ...`).
- PRs should include: purpose, touched paths, validation commands/results, and UI screenshots for `frontend` changes.
- Link related issue/task IDs and explicitly call out breaking changes.
