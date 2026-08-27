# Contributing

Thanks for improving CPA Usage. Keep changes focused and preserve the current product boundaries unless an issue explicitly says otherwise.

Before larger changes, read `docs/project/contract.md` for repository-wide contribution rules and `docs/project/layout.md` for current code ownership boundaries.

## Local setup

From the repository root:

```bash
cp .env.example .env
npm --prefix ./web ci
```

Edit `.env` with a reachable `CPA_BASE_URL` and `CPA_MANAGEMENT_KEY` before running the backend. Do not commit secrets or customer data.

## Development entrypoints

```bash
make dev-app
```

`make dev-app` is the integrated development Interface: it builds `web/dist`, then starts the Go application that serves the UI and `/api/v1` from one origin. It reads `.env` by default; override the path with `ENV_FILE=/path/to/file`.

`make dev-frontend` is isolated Vite HMR only. It has no API proxy and does not prove integrated or end-to-end behavior. `make dev-backend` serves the last built frontend and API without rebuilding the frontend.

Use focused commands while iterating:

```bash
make test-backend
make test-frontend
make lint-frontend
make typecheck-frontend
make build-frontend
```

## Verification before a pull request

The Makefile is the canonical repository-root quality gate. Run the checks for the areas you touched, or run the full gate before larger changes:

```bash
make verify-backend
make verify-frontend
make verify
```

`make verify-backend` runs Go tests and `go vet`. `make verify-frontend` runs `npm ci`, frontend lint, typecheck, Vitest tests through `make test-frontend`, and mobile Playwright checks after the frontend build.

For deployment contracts, `make verify-dokploy-compose` is the canonical fail-closed Compose gate. `make verify-dokploy-compose-static` is only a narrower static check and cannot replace the canonical release gate; `make verify-dokploy-release` adds the local terminal-deployment and health decision fixtures.

For docs, templates, repository metadata, CI, Docker, or deployment-only changes, use the risk-matched verification policy in `docs/project/contract.md`.

## Project boundaries

Current backend package ownership, frontend module ownership, and documentation SoT boundaries are maintained in `docs/project/layout.md`.

Avoid unrelated behavior changes to CPA queue consumption, SQLite schema semantics, pricing semantics, auth/session behavior, backup behavior, update checks, Docker deployment, and frontend navigation. If a change intentionally affects observable behavior, document the compatibility impact in the pull request.
