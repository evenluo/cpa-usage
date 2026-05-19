# CPA Usage Project Contract

Status: current
Layer: project-contract
Use for: repository-wide contribution rules, compatibility decisions, documentation updates, and risk-matched verification planning
Current SoT: this file

## Positioning

CPA Usage is a human-readable usage dashboard on top of CPA usage data. The product helps operators understand aggregate usage, cost, request health, time patterns, and supporting request evidence without turning the repository into a general CPA administration console.

The repository inherits the CPA usage keeper backend foundation for CPA queue consumption, SQLite persistence, migrations, pricing semantics, auth/session, backup, update checks, and Docker-friendly deployment. Changes must preserve that foundation unless an issue, PRD, or ADR explicitly changes the boundary.

## Domain Doc Reading Order

Read repository guidance in this order when a change needs domain or architecture context:

1. `CONTEXT.md` for glossary, product language, and current domain relationships.
2. `docs/adr/` for accepted architecture decisions that touch the area being changed.
3. `docs/project/` for repository-wide project contract and layout ownership.
4. Scoped design, PRD, deploy, or agent workflow docs that match the work area.

If an optional document does not exist, proceed without treating the missing file as a blocker. Create or update docs only when the change introduces new current facts, new decisions, or material corrections to existing guidance.

## Compatibility Rules

Do not add fallback behavior, compatibility branches, old-path support, or defensive default behavior from guesswork. Compatibility support needs a concrete constraint such as a migration window, runtime uncertainty, external dependency instability, or a documented user impact.

When a change affects externally observable behavior, explicitly state the compatibility decision:

- Compatible: existing API contracts, SQLite data semantics, CPA queue behavior, pricing semantics, auth/session behavior, backup behavior, update checks, Docker deployment, and frontend navigation continue to work as before.
- Incompatible: the affected behavior changes intentionally, and the PR explains the impacted users, operators, or automation.
- Pending confirmation: the impact cannot be proven from the repository alone, and the PR states the exact unknown.

When a change includes fallback behavior, document the trigger condition, covered scope, failure path, observability, and removal condition. If no fallback exists, no fallback section is required.

## Naming Rules

Names must express real semantics: business role, data source, lifecycle stage, responsibility, or policy. Avoid `new`, `old`, `legacy`, `temp`, and `misc` as primary names unless they are external protocol terms, existing database fields, official labels, or documented migration-stage names.

Use the vocabulary in `CONTEXT.md` for product concepts. If a concept is missing there, either choose clearer wording from the current glossary or note the documentation gap.

## Documentation Rules

Update docs when a change affects current behavior, project layout, contribution rules, deployment expectations, public workflows, or accepted domain language.

Use the right documentation layer:

- `CONTEXT.md`: domain glossary, relationships, and product vocabulary.
- `docs/adr/`: durable architecture decisions and their consequences.
- `docs/project/`: repository-wide project contract and layout ownership.
- `docs/design/`: current product or implementation designs that are narrower than an ADR.
- `docs/deploy/`: deployment runbooks and environment-specific operational guidance.
- `docs/prd/`: PRD artifacts and product planning history.
- `docs/agents/`: AI workflow and issue-tracker instructions.

Historical evidence can remain in design, PRD, deploy, or verification reports, but it must not override a current SoT document. If current facts move, update the SoT and leave old reports as dated evidence.

## Verification Policy

Verification should match the risk and blast radius of the change.

- Docs, templates, and repository metadata: inspect changed links and referenced paths, run `git diff --check`, and validate structured config formats that changed.
- GitHub Actions workflows: validate workflow syntax or run `actionlint` if available.
- Backend runtime code: run focused Go tests for the touched package when possible, then use `make verify-backend` for shared behavior, persistence, API, or integration changes.
- Frontend code: run focused frontend tests when possible, then use `make verify-frontend` for shared UI, hooks, build, route, or type changes.
- Docker or deployment behavior: run `make verify-docker` or a deployment-specific validation path when the image, compose, entrypoint, or release workflow changes.
- Cross-stack behavior: run the relevant backend and frontend checks, and document any smoke evidence needed to prove the user-facing path.

Do not run full runtime verification for docs-only changes unless the change also touches Makefile targets, CI workflows, runtime code, frontend code, Docker behavior, or deployment behavior.

## Shared Contribution Invariants

Human contributors and AI agents share these invariants:

- Keep changes scoped to the issue, PRD, or task.
- Preserve CPA native configuration unless the task explicitly changes it.
- Do not commit secrets, tokens, private customer data, or unredacted production samples.
- Do not change API contracts, SQLite schema semantics, frontend navigation, deployment behavior, or background ingestion behavior without documenting compatibility impact.
- Prefer existing package and feature-module boundaries before adding new architecture.
- Keep quick-start docs concise and link to current SoT docs for durable rules.
- Record verification evidence in PRs, including when verification is intentionally limited by risk.

## Follow-Ups

- Code of Conduct adoption requires a real maintainer contact before this repository can add enforcement instructions.

