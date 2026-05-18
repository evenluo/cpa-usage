---
name: Feature request
about: Propose a focused CPA Usage improvement
title: "Feature: "
labels: needs-triage
---

## Problem

What user or contributor problem should this solve?

## Proposed outcome

Describe the smallest useful behavior or documentation change.

## Product area

- Usage Intelligence
- Reference Data
- Operations Console
- Backend ingestion or persistence
- Deployment or contributor workflow

## Compatibility considerations

Call out any expected impact on API contracts, SQLite data, CPA queue behavior, pricing semantics, auth/session, backup, update checks, Docker deployment, or frontend navigation.

## Verification expectation

Which checks should prove the change works?

```bash
make verify-backend
make verify-frontend
```

## Out of scope

List related work that should not be included in this request.
