# PRD Integration Goal Template

Status: current
Layer: agent-workflow-template
Use for: starting a Codex goal that completes all ready-for-agent child issues for one GitHub PRD issue on a single integration branch
Do not use for: ordinary chat sessions, per-issue one-off implementation, manual issue triage, or PR review follow-up
Current SoT: this file

## Short Goal Prompt

Use this as the prompt for Codex goal, replacing `#PARENT`:

```text
Read docs/agents/prd-integration-goal-template.md and execute the PRD integration workflow for GitHub PRD issue #PARENT in the current worktree branch.
```

The prompt is intentionally short. The goal runner should read this file as the execution contract after the goal starts.

## Goal Contract

Goal: Complete all open ready-for-agent child issues for GitHub PRD issue `#PARENT` in this worktree branch.

Core model:

1. This worktree branch is the integration branch for the whole PRD.
2. Work in this worktree only.
3. Do not create per-issue branches unless explicitly instructed later.
4. Do not switch back to the base branch while the PRD is incomplete.
5. Do not merge PRs.
6. Do not manually close GitHub issues.
7. Do not change issue labels, project fields, milestones, assignees, close state, or issue comments unless explicitly instructed.
8. Do not force-push.
9. Do not rebase, squash, rewrite completed issue commits, cherry-pick unrelated commits, or switch branches unless explicitly instructed.
10. Push only the integration branch when ready to open the final PR.

Completion policy:

1. The goal is to complete all open ready-for-agent child issues under PRD issue `#PARENT`.
2. If any frozen child issue becomes blocked or skipped, do not open the final PR unless explicitly instructed.
3. "Blocked" is recorded locally in the final summary only; do not mutate GitHub issue state.
4. Do not include `Closes #...` for blocked, skipped, unrelated, or parent PRD issues.

## Startup Gate

Stop before implementation if any requirement fails:

1. Verify this is an active Codex goal session if goal state tools are available. If this is an ordinary chat session, stop and ask the user to start a goal with the short prompt above.
2. Verify the current directory is the target repository worktree:
   - `git rev-parse --show-toplevel`
   - `git branch --show-current`
   - `git status --short --branch`
3. Identify the repository default/base branch using GitHub or git metadata, preferably:
   - `gh repo view --json defaultBranchRef,nameWithOwner`
   - or an equivalent reliable command
   - Do not guess from convention alone. If the base branch cannot be determined, stop and report.
4. Verify the working tree is clean before starting the PRD workflow. If there are uncommitted changes at startup, stop and report them instead of continuing.
5. Verify `setup-matt-pocock-skills` has completed for this repository:
   - `AGENTS.md` or `CLAUDE.md` contains an `## Agent skills` block.
   - `docs/agents/issue-tracker.md` exists.
   - `docs/agents/triage-labels.md` exists.
   - `docs/agents/domain.md` exists.
6. Read `docs/agents/issue-tracker.md` and `docs/agents/triage-labels.md`.
7. Resolve the actual issue tracker and actual label string for canonical role `ready-for-agent` from those docs. Do not infer label vocabulary when setup evidence is missing.
8. Verify `gh` can read GitHub issues for this repository. If not, stop and report the missing access.
9. Read repository guidance files if present:
   - `AGENTS.md`
   - `CLAUDE.md`
   - `CONTEXT.md`
   - `CONTEXT-MAP.md`
   - relevant docs and ADR files

## Planning Phase

1. Read PRD issue `#PARENT`, including body and comments.
2. Discover child issues related to `#PARENT`.
3. Treat an issue as a child only when there is explicit parent-child evidence:
   - GitHub sub-issue relationship
   - parent issue task list entry
   - explicit text such as `Parent: #PARENT` or `Part of #PARENT`
   - bidirectional issue references
4. Do not treat a casual mention of `#PARENT` as sufficient child ownership evidence.
5. Select child issues that:
   - are open
   - have the resolved ready-for-agent label
   - have explicit parent-child evidence tying them to `#PARENT`
6. Exclude closed issues and issues that are clearly unrelated.
7. Build an ordered work queue:
   - respect explicit `Blocked by`, dependency, or sequencing notes
   - if dependency relationships are unclear, stop and report the ambiguity before implementing dependent issues
8. Print the planned frozen queue with issue IDs, titles, blockers, and skipped/excluded issues.

The child issue queue is frozen after planning. This workflow assumes `to-issues` has already completed issue generation before the goal starts. Before final PR, rediscover children only for drift reporting. Newly discovered ready-for-agent child issues are reported but not included in this run unless explicitly instructed.

## Per-Child-Issue Execution Loop

For each child issue in the frozen queue:

1. Re-check that the issue is still open and still has the resolved ready-for-agent label.
2. Record the current `HEAD` as `ISSUE_BASE_SHA`.
3. Read the child issue body, comments, labels, dependencies, and acceptance criteria.
4. Implement only the scope of this child issue.
5. Do not include unrelated cleanup or opportunistic refactors.
6. Run relevant tests for the changed behavior. Use repository-required entry points such as Makefile targets when specified.
7. If tests fail, fix within the issue scope and rerun tests before committing.
8. Before committing, re-check the child issue is still open and still has the resolved ready-for-agent label.
9. Commit the implementation. The commit message must include the child issue ID.
10. Run independent review for only this issue range: `ISSUE_BASE_SHA..HEAD`.
11. Never use uncommitted changes as the per-issue review target.

Working tree rule:

1. A dirty working tree during this loop is allowed only as the in-progress implementation state for the current child issue.
2. Do not start independent review from a dirty tree.
3. If the user asks to commit already-discussed in-progress changes, commit them as normal project history for the current child issue; this is not a workflow violation.

## Independent Review Policy

Independent review means a separate reviewer session evaluates a stable committed range. It does not require cloning the repository or creating a separate worktree. Use a clone or separate worktree only when the current workspace cannot provide an isolated committed range.

Prefer `codex review --base <ref>`. Because `codex review --base` may require a branch-like ref, create a temporary local review ref pointing at `ISSUE_BASE_SHA`; do not switch branches:

```bash
git update-ref refs/codex-review/prd-PARENT-issue-CHILD ISSUE_BASE_SHA
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

Before starting each review, estimate diff size:

```bash
git diff --stat ISSUE_BASE_SHA..HEAD
git diff --shortstat ISSUE_BASE_SHA..HEAD
```

Review timeout classes:

1. small: <=5 files and <=300 changed lines; hard timeout 20 minutes.
2. medium: <=20 files and <=1500 changed lines; hard timeout 40 minutes.
3. large: anything bigger, generated files, lockfiles, snapshots, or broad refactors; hard timeout 90 minutes.

Rules:

1. Treat review as complete only when the review process exits successfully and returns a usable review result.
2. Never treat a failed, killed, timed-out, or empty review as a pass.
3. A hard-timeout review is a failed review command and does not count as a completed review round.
4. Retry a failed review command at most once for the same round.
5. If a review-run verification command fails, classify the failure before acting:
   - product or test failure: fix within scope, rerun the relevant repository-required test target, and review again within budget
   - reviewer command misuse, such as passing unsupported Make arguments: rerun the canonical repository-required command yourself and record the result
   - environment or access failure: retry once if transient; otherwise record it as a review failure

Review budget:

1. Each child issue gets at most 2 completed independent review rounds.
2. Round 1 happens after the initial implementation commit.
3. If Round 1 reports valid findings:
   - fix them within the child issue scope
   - commit the fixes
   - rerun relevant tests
   - run Round 2 on the full range `ISSUE_BASE_SHA..HEAD`
4. If Round 2 still reports valid non-trivial findings:
   - do not loop indefinitely
   - stop work on that child issue
   - record it as blocked or needs human review in the final summary
   - include unresolved findings, tests run, and the issue commit range
5. Cosmetic or low-risk suggestions after Round 2 should be recorded, not chased indefinitely.
6. If the retry also fails, mark the issue as blocked locally with review failure details.

## Blocked Issue Handling

If a child issue is ambiguous, conflicts with the PRD, has unclear dependencies, lacks enough acceptance criteria, cannot pass tests, or cannot pass review within the review budget:

1. Stop work on that child issue.
2. Record the exact blocker, issue ID, current commit range, tests run, and review findings.
3. If the issue produced local commits that are not part of later completed work, revert only that issue's own commit range with `git revert`; do not rewrite history.
4. If the issue has only uncommitted changes, clean only changes known to belong to that issue.
5. If ownership is uncertain, stop and report instead of destructively cleaning.
6. Continue only with child issues that are independent of the blocked issue.
7. Do not mutate GitHub issue state.

## Completed Issue Record

After a child issue passes review:

1. Record it as locally completed.
2. Record:
   - issue ID
   - commit range
   - tests run
   - review rounds performed
   - review result
3. Do not manually close the GitHub issue.
4. Do not modify labels.
5. Continue to the next independent child issue.

## Final Integration Phase

After all frozen child issues are completed and none are blocked:

1. Before final PR creation, rediscover child issues under `#PARENT` and report drift:
   - newly added children
   - removed children
   - closed or label-changed frozen children
   - newly discovered ready-for-agent children, reported as out-of-band unless explicitly included
2. Run the relevant broader test suite for the whole integration branch.
3. Run final integration checks for cross-issue generated artifacts and schema invariants where applicable:
   - generated code is synchronized with source schema or query changes
   - database migrations preserve existing data invariants or explicitly block unsafe states
   - rollback behavior is explicit when rollback cannot restore prior data shape
   - SQL/query models, API DTOs, and frontend types remain aligned
   - cross-issue data relationships introduced by separate child issues are covered by tests or migration review
4. Run a final integration review against the base branch.
5. Final review prompt:

```text
Review the full PRD integration diff for PRD issue #PARENT.
Check:
- coverage of all completed child issues
- consistency with parent PRD #PARENT
- cross-issue interactions
- regressions
- test adequacy
- compatibility impact
- unrelated scope
```

6. Apply the same review timeout policy to the final integration review.
7. Final integration review also has at most 2 completed rounds.
8. Fix valid final integration review findings, commit fixes, rerun tests, and rerun the final review once.
9. If final integration review still has non-trivial unresolved findings after Round 2, do not open the PR. Report unresolved findings and stop.

## Final PR

If the user or current permissions explicitly say not to push or not to open a PR, stop after local completion instead of opening a PR. Report the branch, clean working tree status, completed issue IDs, tests, review result, and that no push or PR was created.

Open one ready PR from this worktree branch to the base branch only if:

1. all frozen ready-for-agent child issues under `#PARENT` are completed
2. no frozen child issue is blocked or skipped
3. final integration tests pass
4. final integration review passes or only has explicitly recorded low-risk residual notes

PR body must include:

1. Parent PRD: `#PARENT`
2. Completed child issues, each with `Closes #CHILD_ID`
3. Tests run, including per-issue and final integration tests
4. Review evidence:
   - per-issue commit ranges
   - per-issue review rounds
   - final integration review result
5. Compatibility impact if externally observable behavior changed
6. A note that the parent PRD issue is not manually closed and should be closed only after human/coordinator final validation if appropriate

## Final Response

Report:

1. completed issue IDs
2. blocked issue IDs and blockers, if any
3. PR URL if created
4. tests run
5. review rounds performed
6. final integration review result
7. drift discovered before final PR
8. whether parent PRD appears ready for human final review
9. if no PR was created by instruction or permission boundary, the local branch completion status and whether the working tree is clean
