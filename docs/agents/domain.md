# Domain Docs

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Layout

This is a single-context repo:

- Root glossary and domain language: `CONTEXT.md`
- Architecture decisions: `docs/adr/`
- Project contract and layout ownership: `docs/project/`
- Product design docs: `docs/design/`

## Before exploring, read these

- `CONTEXT.md` at the repo root.
- `docs/project/contract.md` for repository-wide contribution rules, compatibility rules, naming rules, documentation rules, and verification policy.
- `docs/project/layout.md` for current backend, frontend, and documentation ownership boundaries.
- ADRs in `docs/adr/` that touch the area being changed.
- Design docs in `docs/design/` when implementing frontend, analytics, or product behavior.

If any optional file does not exist, proceed silently. Do not suggest creating it upfront unless the current task resolves new terminology or decisions that should be documented.

## Use the glossary's vocabulary

When output names a domain concept, use the terms defined in `CONTEXT.md`. Do not drift to synonyms the glossary explicitly avoids.

If a concept is missing from the glossary, either reconsider the wording or note the gap for a documentation update.

## Flag ADR conflicts

If output contradicts an existing ADR, surface it explicitly rather than silently overriding it.
