## Summary

- 

## Verification

Include the commands, link checks, config validation, screenshots, or smoke notes that match the touched areas:

```bash
make verify-backend
make verify-frontend
```

For full repository verification, run:

```bash
make verify
```

Docs/template/config-only PRs should still include verification evidence such as changed-link inspection, `git diff --check`, and structured config validation when applicable.

## Compatibility impact

Note any impact on Usage Intelligence, Reference Data, Operations Console, API contracts, SQLite persistence, CPA queue behavior, pricing semantics, auth/session, backup, update checks, Docker deployment, or frontend navigation.

If there is no externally observable behavior change, say that explicitly.

## Docs SoT impact

Note whether this changes current behavior, project layout, contribution rules, deployment expectations, public workflows, or accepted domain language. Link the updated SoT doc, or say no docs SoT update is needed.

## Screenshots or evidence

Add screenshots, command output, or notes for user-visible changes.

## Checklist

- [ ] Tests or verification evidence are included above.
- [ ] Compatibility impact is described above.
- [ ] Docs SoT impact is described above.
- [ ] Secrets, tokens, and private customer data are not included.
