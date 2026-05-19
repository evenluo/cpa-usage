# CPA Usage Project Layout

Status: current
Layer: project-layout
Use for: locating code ownership boundaries before adding or moving files
Current SoT: this file

## Backend

The backend keeps a responsibility-based Go package layout. Choose an existing package by ownership before introducing a new package.

- `cmd/server`: executable entrypoint.
- `internal/app`: application wiring for config, database, CPA clients, services, HTTP routing, and background runners.
- `internal/api`: HTTP contracts, handlers, request parsing, response payloads, and route-level API behavior.
- `internal/service`: business use cases, orchestration, and service DTO assembly that should not live in handlers or persistence code.
- `internal/repository`: SQLite/GORM persistence, migrations, analytics read models, and SQL aggregation.
- `internal/cpa`: CPA external API client boundaries and CPA DTOs.
- `internal/quota`: quota provider capability and quota-specific test helpers.
- `internal/poller`: background queue consumption and polling execution.

Supporting backend packages keep focused ownership:

- `internal/auth`: session management and authentication primitives.
- `internal/backup`: SQLite backup and retention behavior.
- `internal/config`: environment loading and runtime configuration parsing.
- `internal/entities`: GORM persistence models.
- `internal/logging`: process logging configuration.
- `internal/redact`: display-safe masking and stable redaction helpers.
- `internal/updatecheck`: release update checks.
- `internal/version`: build version metadata.

## Frontend

The current frontend lives in `web/` and uses React, TypeScript, Vite, Tailwind, and shadcn-style UI primitives.

- `web/src/routes`: route files and route-level composition. Route files may own page-local React state, data fetching hooks, mutations, events, toasts, and layout composition.
- `web/src/features/usage-intelligence`: tested Usage Intelligence view-model derivation.
- `web/src/features/reference-data`: tested Reference Data interaction and model logic for Key Aliases and Cost Rates.
- `web/src/hooks`: reusable API-facing hooks and query wrappers.
- `web/src/lib`: shared client utilities such as API access, formatting, and class-name helpers.
- `web/src/components/ui`: low-level reusable UI primitives.
- `web/src/components/charts`: chart components and chart-specific presentation helpers.
- `web/src/components/layout`: shared navigation and shell components.
- `web/src/components/intelligence`: Usage Intelligence presentation components that are narrower than a route but broader than a primitive.
- `web/src/components/providers`: app-level React providers.
- `web/src/types`: shared frontend API and domain types.
- `web/src/test`: frontend test setup and contract fixtures.

Frontend feature behavior should live in `web/src/features/` when it is reusable, testable, or domain-specific. Route files should stay focused on page composition and interaction wiring.

## Documentation

Current SoT areas:

- `CONTEXT.md`: domain glossary, current product language, and product relationships.
- `docs/adr/`: accepted architecture decisions.
- `docs/project/`: repository-wide project contract and current layout ownership.
- `docs/deploy/`: deployment runbooks and current operational procedures.
- `docs/agents/`: AI workflow, issue tracker, and agent documentation consumption rules.
- `docs/assets/`: public documentation image assets, including README and GitHub homepage screenshots.

Historical or scoped evidence areas:

- `docs/design/`: product and implementation design documents. A design doc may be current for its scope, but dated verification and exploration notes are evidence rather than global project rules.
- `docs/prd/`: PRD artifacts and planning history.
- `docs/design/assets/`: supporting design images and artifacts.

When current behavior conflicts with older evidence, update the current SoT first and preserve historical documents as dated context unless the task explicitly asks to rewrite them.
