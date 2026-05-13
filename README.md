# CPA Usage

CPA Usage is a human-readable usage dashboard on top of CPA usage data.

This repository starts from the stable CPA usage keeper backend foundation and keeps CPA queue consumption, SQLite persistence, migrations, pricing semantics, auth/session, backup, update check, and Docker-friendly deployment behavior intact. The frontend baseline is a minimal React, TypeScript, and Vite shell that the next design-system slice will replace with the full analytics workspace.

## Verification

Run the local checks from the repository root:

```bash
make verify-backend
make verify-frontend
```

`make verify` runs both checks. `make verify-docker` builds the deployment image.

## Development

```bash
npm --prefix ./web ci
npm --prefix ./web run dev
go run ./cmd/server/main.go --env .env
```

The Go server serves the built frontend assets from `web/dist` when `npm --prefix ./web run build` has been run.

## Compatibility

- CPA native configuration is not mutated.
- Existing usage events, pricing semantics, SQLite persistence, auth/session, backup, update check, and Docker deployment behavior are inherited from the keeper backend.
- The frontend structure is intentionally not compatible with the old keeper SCSS-module UI; the old UI is reference material only.
