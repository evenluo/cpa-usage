---
name: Bug report
about: Report behavior that is broken or surprising
title: "Bug: "
labels: needs-triage
---

## What happened

Describe the broken behavior and what you expected instead.

## Where you saw it

- Area: Usage Intelligence / Reference Data / Operations Console / backend / deployment
- CPA Usage version or commit:
- Browser and OS, if frontend-related:

## Steps to reproduce

1.
2.
3.

## Evidence

Include screenshots, logs with secrets removed, failing command output, or sample request details.

## Verification already tried

Include command output, link checks, config validation, screenshots, or smoke notes that show what has already been checked.

```bash
make verify-backend
make verify-frontend
```

## Compatibility impact

Call out any suspected impact on Usage Intelligence, Reference Data, Operations Console, API contracts, SQLite persistence, CPA queue behavior, pricing semantics, auth/session, backup, update checks, Docker deployment, or frontend navigation.

## Docs SoT impact

If the bug suggests current behavior, project layout, contribution rules, deployment expectations, public workflows, or accepted domain language are wrong or unclear, name the affected SoT doc. Otherwise write "No docs SoT impact known."

## Additional context

Add any relevant configuration notes, redacted data shape, or timing details.
