# PRD Integration Goal

This is a workflow contract for a Codex `/goal` run. It defines the target state, boundaries, checkpoints, and evidence required to complete a PRD integration workflow.

## Goal

Complete all open ready-for-agent child issues for GitHub PRD issue `#PARENT` in the current integration worktree branch, then open one ready PR to the repository base branch.

Completion requires:

1. frozen child issue queue built from explicit parent-child evidence
2. every frozen child completed, tested, committed, and independently reviewed
3. no frozen child blocked or skipped
4. final integration tests passing
5. final integration review passing, or only explicitly recorded low-risk residual notes
6. final PR created unless user instruction or permission boundary forbids push or PR creation

Low-risk residual notes are limited to spelling, comments, naming nits, or other non-behavioral cleanup. Findings about tests, security, data integrity, compatibility, user-visible behavior, migrations, permissions, or external integrations are not low-risk residual notes.

## Terms

| Term | Meaning |
|---|---|
| Parent PRD issue | GitHub issue `#PARENT`; source of run scope |
| Child issue | GitHub issue explicitly tied to `#PARENT` |
| Ready label | Repository label mapped from canonical role `ready-for-agent` |
| Frozen queue | Ordered child issue set selected during planning |
| Integration branch | Current non-base worktree branch; only branch this goal may push |
| Issue base | `ISSUE_BASE_SHA`, current `HEAD` before one child starts |
| Issue range | `ISSUE_BASE_SHA..HEAD`; required per-child review target |
| Final integration range | Same diff the final PR will present against the resolved base branch |
| Drift | Child issue state changes after planning; report, do not silently expand scope |

## Hard Invariants

Stop and report if any invariant cannot be satisfied.

Repository:

1. Work only inside the current repository worktree.
2. Current branch must be a non-base integration branch.
3. Do not execute this workflow on the repository default/base branch.
4. Do not switch branches, create per-issue branches, merge PRs, force-push, rebase, squash, rewrite completed issue commits, or cherry-pick unrelated commits unless explicitly instructed.
5. Push only the integration branch, and only after all final PR gates pass.

GitHub:

1. Do not manually close GitHub issues.
2. Do not change issue labels, project fields, milestones, assignees, close state, or issue comments unless explicitly instructed.
3. Use `Closes #CHILD_ID` only for completed frozen child issues in the final PR body.
4. Do not include `Closes #...` for blocked, skipped, unrelated, or parent PRD issues.

Worktree:

1. Startup requires a clean working tree.
2. Dirty state is allowed only as in-progress implementation for the current child issue.
3. Do not start independent review from dirty state.
4. Never use uncommitted changes as a review target.
5. If dirty-change ownership is uncertain, stop and report instead of cleaning destructively.

## Startup Gate

Before implementation:

1. Verify repository root, current branch, and clean status.
2. Resolve the repository default/base branch from GitHub or git metadata. Do not infer it from convention.
3. Verify current branch is not the resolved base branch.
4. Verify `gh` can read issues in this repository.
5. Resolve the actual repository label for canonical role `ready-for-agent`.
6. Read repository guidance relevant to the parent PRD and child issue scope, then record which guidance docs were read.

## Planning Gate

Build the frozen queue before implementation.

Child eligibility:

1. issue is open
2. issue has the resolved ready label
3. issue has explicit parent-child evidence tying it to `#PARENT`
4. issue is not clearly unrelated to the parent PRD

Valid parent-child evidence:

1. GitHub sub-issue relationship
2. parent issue task list entry
3. explicit text such as `Parent: #PARENT` or `Part of #PARENT`
4. bidirectional issue references

Queue rules:

1. Casual mention of `#PARENT` is not enough.
2. Respect explicit dependency evidence.
3. Ambiguous dependency evidence blocks dependent work until reported.
4. Newly discovered ready-for-agent children after planning are drift; report before final PR, but do not add them unless explicitly instructed.
5. Closed or label-changed frozen children are drift and count as skipped unless explicitly overridden.
6. Any blocked or skipped frozen child blocks final PR creation unless explicitly overridden.

Planning output must include parent PRD, frozen queue, dependency notes, excluded issues with reasons, and ambiguous relationships if any.

## Per-Child Gate

For each frozen child:

1. Re-check open status and ready label before implementation and before commit.
2. If closed or label-changed, record drift/skipped and do not implement unless explicitly instructed.
3. Record `ISSUE_BASE_SHA`.
4. Implement only the child issue scope.
5. For code or behavior changes, use the `tdd` skill. Let the model choose the appropriate tests according to that skill and repository risk.
6. Commit all child-scope changes. Every child-scope commit must be traceable to the child issue ID.
7. Run independent review on `ISSUE_BASE_SHA..HEAD`.
8. If review finds valid issues, fix within scope, commit fixes, rerun tests, and rerun review on the full issue range.

Per-child evidence must include issue ID, status/label checks, `ISSUE_BASE_SHA`, commit range, tests run, review rounds, review result, and blocker/skipped reason if applicable.

## Review Gate

Independent review means a separate reviewer session evaluates a stable committed range.

Required:

1. per-child review target is the full committed range `ISSUE_BASE_SHA..HEAD`
2. Round 2 after fixes reviews the full issue range
3. final integration review target matches the final PR diff
4. failed, killed, timed-out, or empty review output is not a pass
5. each child issue and final integration each get at most 2 completed independent review rounds
6. valid non-trivial Round 2 findings block that scope

Use `runbook.md` for default `codex review --base` mechanics.

## Blocked Or Skipped Gate

Blocked means the issue cannot be completed under this contract. Skipped means a frozen issue should not be implemented because its external state changed or the user instructed a skip.

When blocked or skipped:

1. record issue ID, reason, tests, review findings, and current commit range
2. revert only that issue's own committed range if those commits are not part of later completed work
3. clean only uncommitted changes known to belong to that issue
4. stop and report if ownership is uncertain
5. continue only with independent child issues
6. do not mutate GitHub issue state

## Final Integration Gate

Run only after all frozen children are completed and none are blocked or skipped.

Before final PR:

1. rediscover children under `#PARENT` and report drift
2. run relevant broader integration tests
3. verify generated artifacts, migrations, rollback notes, SQL/API/frontend type alignment, and cross-issue data relationships where applicable; record `checked` or `N/A` with a reason for each category
4. run final integration review against the same diff the PR will present
5. stop without PR if final review Round 2 still has valid non-trivial findings

## Final PR Gate

If user instruction or permission boundary forbids push or PR creation, stop after local completion and report local status.

Open one ready PR only if:

1. all frozen ready-for-agent children are completed
2. no frozen child is blocked or skipped
3. final integration tests pass
4. final integration review passes or only has recorded low-risk residual notes as defined above
5. working tree is clean
6. PR diff contains only PRD integration scope

PR body must include parent PRD, completed children with `Closes #CHILD_ID`, tests, review evidence, drift, compatibility impact if any, and a note that the parent PRD is not manually closed.

## Final Response

Report completed issues, blocked/skipped issues, PR URL if created, tests, review rounds, final integration review result, drift, compatibility impact, parent PRD readiness for human final review, and clean working tree status.
