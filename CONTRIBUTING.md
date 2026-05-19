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

For docs, templates, repository metadata, CI, Docker, or deployment-only changes, use the risk-matched verification policy in `docs/project/contract.md`.

## Project boundaries

Current backend package ownership, frontend module ownership, and documentation SoT boundaries are maintained in `docs/project/layout.md`.

Avoid unrelated behavior changes to CPA queue consumption, SQLite schema semantics, pricing semantics, auth/session behavior, backup behavior, update checks, Docker deployment, and frontend navigation. If a change intentionally affects observable behavior, document the compatibility impact in the pull request.
