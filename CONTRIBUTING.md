# Contributing

Thanks for improving CPA Usage. Keep changes focused and preserve the current product boundaries unless an issue explicitly says otherwise.

## Local setup

From the repository root:

```bash
cp .env.example .env
npm --prefix ./web ci
```

Edit `.env` with a reachable `CPA_BASE_URL` and `CPA_MANAGEMENT_KEY` before running the backend. Do not commit secrets or customer data.

## Development entrypoints

```bash
make dev-backend
make dev-frontend
```

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

`make verify-backend` runs Go tests and `go vet`. `make verify-frontend` runs `npm ci`, frontend lint, typecheck, Vitest tests through `make test-frontend`, and frontend build.

## Project boundaries

- `internal/api` owns HTTP contracts and JSON payload shapes.
- `internal/service` owns use cases and orchestration.
- `internal/repository` owns SQLite/GORM persistence, analytics read models, and SQL aggregation.
- `web/src/features/usage-intelligence` owns tested Usage Intelligence view-model derivation.
- `web/src/features/reference-data` owns tested Reference Data Key Alias and Cost Rate interaction logic.
- Route files should keep React state, hooks, mutations, events, toasts, and layout composition.

Avoid unrelated behavior changes to CPA queue consumption, SQLite schema semantics, pricing semantics, auth/session behavior, backup behavior, update checks, Docker deployment, and frontend navigation.
