# Repository Guidelines

## Project Structure & Module Organization

Resin is a Go service with an embedded React/Vite dashboard. The service entry point is `cmd/resin`; backend code lives under `internal/`, grouped by capability such as `api`, `proxy`, `routing`, `state`, `metrics`, and `subscription`. SQLite migrations are in `internal/state/migrations/{state,cache}`. Frontend code lives in `webui/src`, with reusable UI in `components`, page features in `features`, and shared helpers in `lib`. Documentation and screenshots are in `doc/` and `doc/images/`; Docker assets are in `Dockerfile`, `.github/Dockerfile.release`, `docker/`, and `docker-compose.yml.example`.

## Build, Test, and Development Commands

- `go test ./...`: run all Go unit and integration-style package tests.
- `go run ./cmd/resin`: start the backend locally; set required `RESIN_*` variables from `.env.example`.
- `cd webui && npm ci`: install frontend dependencies from `package-lock.json`.
- `cd webui && npm run dev`: run the Vite development server.
- `cd webui && npm run build`: type-check and build the dashboard into `webui/dist`.
- `go build -tags "with_quic with_wireguard with_grpc with_utls" -o resin ./cmd/resin`: build a local binary after building the WebUI.
- `docker compose -f docker-compose.yml.example up -d`: start a containerized local deployment after supplying environment values.

## Coding Style & Naming Conventions

Format Go with `gofmt`; keep package names short, lowercase, and capability-oriented. Prefer table-driven tests where behavior branches by input. TypeScript uses ESLint with `typescript-eslint`, React Hooks, and React Refresh rules. Name React components in `PascalCase`, hooks as `useSomething`, feature APIs as `api.ts`, and domain types as `types.ts`. Keep frontend modules colocated with their feature unless broadly reusable.

## Testing Guidelines

Go tests use the standard `testing` package and live beside implementation files as `*_test.go`. Add or update tests for routing, proxy behavior, state persistence, API contracts, and migrations when touching those areas. Run `go test ./...` for backend changes and `npm run lint && npm run build` for WebUI changes.

## Commit & Pull Request Guidelines

Recent history uses concise imperative subjects, sometimes with conventional prefixes such as `feat:` and `docs:`. Examples: `feat: add proxy bypass rules`, `Fix tunnel logs for client-side resets`. Keep commits focused and mention the affected area when useful. Pull requests should describe behavior changes, list validation commands, link related issues, and include screenshots for dashboard-visible UI changes.

## Security & Configuration Tips

Do not commit real tokens, state, cache, logs, or local `.env` files. `RESIN_AUTH_VERSION`, `RESIN_ADMIN_TOKEN`, and `RESIN_PROXY_TOKEN` are required for local runs; set `RESIN_PROXY_TOKEN=""` explicitly only when testing unauthenticated proxy access.
