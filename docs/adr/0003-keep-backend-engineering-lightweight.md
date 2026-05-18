# Keep backend engineering lightweight

CPA Usage uses the repository-root Makefile as the canonical entrypoint for common development and verification tasks, but keeps targets as thin wrappers around Go and npm commands. Backend engineering keeps the existing `cmd/server` and `internal/...` package layout, records package responsibilities, and avoids package migration unless a future change has a concrete behavioral or maintenance need.

## Consequences

- Common local and CI checks are discoverable from the repository root.
- `verify-backend` includes both backend tests and `go vet`; this makes the backend gate stricter without changing runtime behavior.
- Backend package boundaries are responsibility-based: `internal/app` wires runtime components, `internal/api` owns HTTP contracts, `internal/service` owns use cases, `internal/repository` owns SQLite/GORM persistence, `internal/cpa` owns CPA integration, `internal/quota` owns quota provider behavior, and `internal/poller` owns background execution. Supporting packages keep focused ownership for auth, backup, config, entities, logging, redaction, update checks, and version metadata.
- New backend code should choose an existing package by responsibility before introducing a new package.
- Analytics deepening should prefer same-package file splits and DTO duplication removal over a new `internal/analytics` package unless a concrete multi-adapter seam appears.
- The current product behavior, API contracts, CPA queue consumption, SQLite persistence, pricing, auth/session, backup, update checks, frontend build behavior, and Docker deployment semantics remain compatible.

## Non-goals

- Do not introduce `golangci-lint`, watch runners, process orchestration, bootstrap automation, or a new backend framework as part of this decision.
- Do not migrate packages only to make the tree look more architectural.
- Do not introduce schema changes, API contract changes, CPA integration behavior changes, or frontend product behavior changes as part of this decision.
