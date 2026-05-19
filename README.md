# CPA Usage

CPA Usage is a human-readable usage dashboard on top of CPA usage data.

This repository starts from the stable CPA usage keeper backend foundation and keeps CPA queue consumption, SQLite persistence, migrations, pricing semantics, auth/session, backup, update check, and Docker-friendly deployment behavior intact. The frontend is the `web/` React, TypeScript, Vite, Tailwind, and shadcn-style analytics workspace.

Project-level contribution rules live in `docs/project/contract.md`. Current backend, frontend, and documentation ownership boundaries live in `docs/project/layout.md`.

## Verification

Run the local checks from the repository root:

```bash
make verify-backend
make verify-frontend
```

`make verify` runs both checks. `make verify-backend` runs backend tests and `go vet`; `make verify-frontend` installs frontend dependencies with `npm ci`, then runs lint, typecheck, Vitest feature tests through `make test-frontend`, and build. `make verify-docker` builds the deployment image. GitHub Actions runs the same backend and frontend verification targets for pull requests and pushes to `main`.

## Development

Prepare local configuration and frontend dependencies once before running the dev servers:

```bash
cp .env.example .env
npm --prefix ./web ci
```

Edit `.env` with a reachable `CPA_BASE_URL` and `CPA_MANAGEMENT_KEY`. `make dev-backend` loads `.env` explicitly, so a missing file or missing required CPA settings fails fast.
For self-hosted shared login between the CPA root service and this `/usage` service, see `docs/deploy/self-hosted-shared-login.md`.

```bash
make dev-backend
make dev-frontend
```

The Go server serves the built frontend assets from `web/dist` when `npm --prefix ./web run build` has been run.

## Project docs

- `docs/project/contract.md`: repository positioning, compatibility rules, naming rules, documentation rules, shared contribution invariants, and risk-matched verification policy.
- `docs/project/layout.md`: current backend package ownership, frontend ownership, and documentation SoT boundaries.
- `CONTEXT.md`: domain glossary and product vocabulary.
- `docs/adr/`: accepted architecture decisions.

## Deployment

Production images are published as immutable GHCR version tags. Do not deploy `latest`.

For Dokploy, this repository now owns only the independent `cpa-usage` Compose app. The release workflow renders `deploy/dokploy/cpa-usage.compose.yml`, updates the Dokploy app referenced by `DOKPLOY_CPA_USAGE_COMPOSE_ID`, and deploys that app only. Adjacent CPA infrastructure such as the root CPA service, its Postgres database, and other proxy services is outside the release blast radius.

Deployment docs:

- `docs/deploy/dokploy-release.md`: Dokploy release chain, required GitHub variables, required Dokploy environment, and one-time split migration.
- `docs/deploy/self-hosted-cutover-runbook.md`: generic self-hosted cutover flow.
- `docs/deploy/self-hosted-shared-login.md`: optional same-origin login sharing.

For Dokploy, set environment-specific values in Dokploy, not in git:

```dotenv
PUBLIC_HOST=<your-cpa-host>
MANAGEMENT_PASSWORD=<existing CPA management password>
CPA_USAGE_LOGIN_PASSWORD=<usage dashboard login password>
AUTH_SESSION_SECRET=<random secret with at least 32 characters>
AUTH_SESSION_COOKIE_DOMAIN=<your-cpa-host>
```

Common backend targets:

```bash
make test-backend
make fmt-backend
make vet-backend
make build-backend
```

Common frontend targets:

```bash
make test-frontend
make lint-frontend
make typecheck-frontend
make build-frontend
```

`make test-frontend` runs Vitest feature tests and frontend type checking. The Makefile is the canonical repository-root entrypoint for common development and verification tasks. Targets intentionally stay as thin wrappers around Go and npm commands; use the underlying tools directly for focused package or file-level work.
