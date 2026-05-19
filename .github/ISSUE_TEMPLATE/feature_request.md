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

## Compatibility impact

Call out any expected impact on API contracts, SQLite data, CPA queue behavior, pricing semantics, auth/session, backup, update checks, Docker deployment, or frontend navigation.

## Docs SoT impact

Name any current behavior, project layout, contribution rule, deployment expectation, public workflow, or accepted domain-language doc that should change. If none, write "No docs SoT impact expected."

## Verification expectation

Which checks, link inspections, config validations, screenshots, or smoke notes should prove the change works?

```bash
make verify-backend
make verify-frontend
```

## Out of scope

List related work that should not be included in this request.
