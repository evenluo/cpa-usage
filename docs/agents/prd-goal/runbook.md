# PRD Integration Goal Runbook

This file is a cookbook, not the contract. `README.md` is authoritative.

Use these commands when they fit the repository state. Replace them freely with equivalent or stronger commands when they target the same state and produce auditable evidence.

## Startup Examples

```bash
git rev-parse --show-toplevel
git branch --show-current
git status --short --branch
gh repo view --json defaultBranchRef,nameWithOwner
gh issue view PARENT --comments
```

If repository inference fails:

```bash
gh repo view OWNER/REPO --json defaultBranchRef,nameWithOwner
```

## Issue Review Examples

Record the base before starting a child:

```bash
ISSUE_BASE_SHA=$(git rev-parse HEAD)
```

For code or docs changes, inspect the committed issue range:

```bash
git diff --stat "$ISSUE_BASE_SHA..HEAD"
git diff --shortstat "$ISSUE_BASE_SHA..HEAD"
```

Default independent review for a code or docs child:

```bash
git update-ref refs/codex-review/prd-PARENT-issue-CHILD "$ISSUE_BASE_SHA"
codex review --base refs/codex-review/prd-PARENT-issue-CHILD
```

Equivalent review tooling is fine when it evaluates the same reviewable evidence set. Record the base ref or evidence artifact, tool, and result.

Do not make custom prompt delivery part of the contract. If the installed reviewer supports prompts with `--base`, use it; otherwise record the checklist in run notes.

Review checklist:

1. child acceptance criteria
2. parent PRD consistency
3. reviewable evidence completeness
4. unrelated scope
5. test adequacy
6. regressions and edge cases
7. maintainability

## Test Evidence Examples

Prefer repository Make targets when they match the risk:

```bash
make test-api
make test-unit
```

Do not assume Make targets accept pass-through variables. If a target does not wire a variable, use a direct command or another repository-approved route:

```bash
cd server && go test ./internal/api ./internal/worker ./internal/synccontrol
cd server && go test ./internal/api -run 'TestName'
```

Record reviewer-run test failures separately when caused by command misuse or sandbox limitations, such as `Makefile target does not consume TESTARGS` or `httptest port binding denied in reviewer sandbox`.

## Final Review Examples

Check the PR diff base:

```bash
BASE_BRANCH=main
git fetch origin "$BASE_BRANCH"
git merge-base "origin/$BASE_BRANCH" HEAD
git diff --stat "origin/$BASE_BRANCH...HEAD"
git diff --shortstat "origin/$BASE_BRANCH...HEAD"
```

Default final review:

```bash
codex review --base "origin/$BASE_BRANCH"
```

Use another final review command when it reviews the same diff the PR will present, or the same no-diff state the run will report. Record the exact base ref and why it matches the terminal artifact.

Final checklist:

1. all completed children are covered
2. parent PRD consistency
3. cross-issue interactions
4. regressions and test adequacy
5. compatibility impact
6. unrelated scope

## No-Diff Checks

Use when PR creation would fail or has failed because there are no commits between base and head:

```bash
BASE_BRANCH=main
INTEGRATION_BRANCH=$(git branch --show-current)
git fetch origin "$BASE_BRANCH" "$INTEGRATION_BRANCH"
git rev-list --left-right --count "origin/$BASE_BRANCH...origin/$INTEGRATION_BRANCH"
git diff --stat "origin/$BASE_BRANCH...origin/$INTEGRATION_BRANCH"
git diff --shortstat "origin/$BASE_BRANCH...origin/$INTEGRATION_BRANCH"
gh pr list --head "$INTEGRATION_BRANCH" --base "$BASE_BRANCH" --state all --json number,state,title,headRefOid,baseRefName,url
```

Read `git rev-list --left-right --count` as `<base-only> <integration-only>`. If `<integration-only>` is `0` and the diff is empty, finish through the no-diff terminal path instead of manufacturing a commit.

## Failure Classification

| Failure | Handling |
|---|---|
| Product or test failure | Fix within scope and rerun relevant evidence. Open Round 2 only when the failure came from a valid P0/P1 Round 1 review finding. |
| Reviewer command misuse | Rerun the canonical repository command yourself and do not count misuse as a product failure. |
| Reviewer sandbox limitation | Use equivalent main-workspace evidence when the limitation is clearly environmental. |
| Empty, killed, failed, or timed-out review | Not a pass; retry once if transient, otherwise report the incomplete gate. |
| Repeated same test failure | Continue only with new evidence, a narrower hypothesis, or a different scoped fix; otherwise report the child as blocked. |
| Round 1 finding below P1 | Fix when valid, verify, and record; do not open Round 2 only for this finding. |
| Valid Round 2 finding | Fix, verify, record the fix commit and evidence, then continue without requiring Round 3. |
| Repeated HITL gate gap | Stop and report the gate as blocked after one return-and-fix cycle for the same class of gap. |

## HITL Gate Notes

For a HITL gate child:

1. re-read the child acceptance criteria
2. inspect current committed branch state
3. verify prior wave claims that the gate depends on
4. search for named stop conditions
5. prepare the concrete decision or evidence artifact for review
6. run the required per-child independent review on the full HITL evidence set

If a narrow gate is fully verifiable from local committed evidence, record that evidence before independent review. Do not skip the required per-child review; use it to challenge whether the recorded evidence really satisfies the gate.

If the gate has no code or docs diff, use a meaningful decision or evidence artifact as the review target. Do not create a meaningless file just to make a git range non-empty.

If the gate sends work back to an earlier child, record the owning child, gap class, fix commit, and verification. Do not repeat the same return path twice; repeated gap class means the gate is blocked.

If an issue comment is explicitly required, adapt this minimum shape:

```markdown
## Gate decision

Decision: ...

Evidence checked:
- ...

Compatibility decision:
- ...

Remaining blockers:
- ...
```

Do not write labels, assignees, milestones, project fields, or closure state in the comment.

## PR Body Skeleton

```markdown
Parent PRD:
- Closes #PARENT

Completed child issues:
- Closes #CHILD_ID

Tests and evidence:
- ...

Review summary:
- #CHILD_ID: ISSUE_BASE_SHA..HEAD, rounds: N, result: ...
- Final integration review: base..HEAD, rounds: N, result: ...

Drift:
- ...

Compatibility impact:
- ...

Final integration checks:
- Generated artifacts: checked/N/A - ...
- Migrations: checked/N/A - ...
- Rollback notes: checked/N/A - ...
- SQL/API/frontend alignment: checked/N/A - ...
- Cross-issue data relationships: checked/N/A - ...

Residual notes:
- None / low-risk notes

Note: Merging this PR is the parent PRD #PARENT closure action.
```
