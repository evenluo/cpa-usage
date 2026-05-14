# PRD Integration Goal Runbook

This runbook supports `README.md`.

`README.md` defines the workflow contract. This file only provides command mechanics, review prompts, timeout classes, failure classification, and PR body scaffolding.

## Default Startup Commands

```bash
git rev-parse --show-toplevel
git branch --show-current
git status --short --branch
gh repo view --json defaultBranchRef,nameWithOwner
gh issue view PARENT --comments
```

If `gh repo view --json defaultBranchRef,nameWithOwner` fails because the local `gh` version or repository inference differs, use an equivalent explicit repository command such as:

```bash
gh repo view OWNER/REPO --json defaultBranchRef,nameWithOwner
```

## Per-Child Review Mechanics

Record the issue base before implementation:

```bash
ISSUE_BASE_SHA=$(git rev-parse HEAD)
```

After implementation and commit, estimate diff size:

```bash
git diff --stat "$ISSUE_BASE_SHA..HEAD"
git diff --shortstat "$ISSUE_BASE_SHA..HEAD"
```

Default review tactic:

```bash
git update-ref refs/codex-review/prd-PARENT-issue-CHILD "$ISSUE_BASE_SHA"
codex review --base refs/codex-review/prd-PARENT-issue-CHILD "Review the committed changes for child issue #CHILD under PRD #PARENT.
Review only ISSUE_BASE_SHA..HEAD.
Check:
- child issue acceptance criteria
- consistency with parent PRD #PARENT
- unrelated scope
- test adequacy
- regressions
- edge cases
- maintainability"
```

The reviewer must be a separate reviewer session. Do not count same-pass self-review as independent review.

## Final Integration Review Mechanics

The final review target should match the final PR diff. Prefer the resolved remote base branch or merge-base used by the PR tooling.

Useful checks:

```bash
BASE_BRANCH=main
INTEGRATION_BRANCH=$(git branch --show-current)
git fetch origin "$BASE_BRANCH"
git merge-base "origin/$BASE_BRANCH" HEAD
git diff --stat "origin/$BASE_BRANCH...HEAD"
git diff --shortstat "origin/$BASE_BRANCH...HEAD"
```

Default final review tactic:

```bash
codex review --base "origin/$BASE_BRANCH" "Review the full PRD integration diff for PRD issue #PARENT.
Check:
- coverage of all completed child issues
- consistency with parent PRD #PARENT
- cross-issue interactions
- regressions
- test adequacy
- compatibility impact
- unrelated scope"
```

If the repository uses a different PR diff base, record the exact base ref and why it matches the final PR.

## No-Diff Terminal Checks

Use these checks only when final PR creation fails because the integration branch and resolved base branch have no commits between them.

```bash
BASE_BRANCH=main
INTEGRATION_BRANCH=$(git branch --show-current)
git fetch origin "$BASE_BRANCH" "$INTEGRATION_BRANCH"
git rev-list --left-right --count "origin/$BASE_BRANCH...origin/$INTEGRATION_BRANCH"
git diff --stat "origin/$BASE_BRANCH...origin/$INTEGRATION_BRANCH"
git diff --shortstat "origin/$BASE_BRANCH...origin/$INTEGRATION_BRANCH"
gh pr list --head "$INTEGRATION_BRANCH" --base "$BASE_BRANCH" --state all --json number,state,title,headRefOid,baseRefName,url
```

If `git rev-list --left-right --count` returns `0 0` and PR creation reports no commits between base and head, do not create unrelated commits just to open a PR. Finish through the no-diff terminal path.

## Review Timeout Classes

| Class | Diff size | Hard timeout |
|---|---:|---:|
| small | <=5 files and <=300 changed lines | 20 minutes |
| medium | <=20 files and <=1500 changed lines | 40 minutes |
| large | anything bigger, generated files, lockfiles, snapshots, or broad refactors | 90 minutes |

A hard-timeout review is a failed review command and does not count as a completed review round.

## Review Failure Classification

| Failure type | Handling |
|---|---|
| Product or test failure | Fix within scope, rerun the repository-required test target, then review again within budget. |
| Reviewer command misuse | Run the canonical repository-required command yourself, record the result, and do not treat reviewer misuse as a product failure. |
| Environment or access failure | Retry once if transient; otherwise record it as review failure. |
| Empty, killed, or timed-out review | Not a pass; retry at most once for the same round. |
| Valid non-trivial Round 2 finding | Fix within scope, rerun relevant tests and targeted verification, record the fix summary, then continue to the Final PR Gate. |

## Round 2 Fix Recovery

The workflow allows at most 2 completed independent review rounds for each child issue and final integration. A valid non-trivial Round 2 finding does not require Round 3 or extra approval after it is fixed and verified. The PR's own review process handles any later review feedback.

If a valid non-trivial Round 2 finding is fixed after Round 2:

1. commit only the scoped fix
2. rerun the relevant repository test target and any targeted verification for the finding
3. record the finding, fix commit, commands, and results
4. continue to the Final PR Gate

If the finding is not fixed or verification fails, stop without PR and report the unresolved finding.

## Test Evidence

For each child issue, record:

1. commands run
2. pass/fail result

For final integration, record:

1. broader suite commands run
2. generated artifact, migration, rollback, SQL/API/frontend alignment, and cross-issue relationship checks as `checked` or `N/A` with reason
3. any skipped tests and the reason

## PR Body Skeleton

```markdown
Parent PRD:
- Closes #PARENT

Completed child issues:
- Closes #CHILD_ID

Tests:
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

Low-risk residual notes:
- None / spelling-comment-naming-only notes

Note: Merging this PR is the parent PRD #PARENT closure action.
```
