# PRD Integration Goal

This is the contract for a Codex `/goal` run that completes a GitHub PRD issue through its ready child issues.

It constrains failure modes, not intelligence. GPT-5.5 is expected to choose the implementation path, test strategy, review tactic, and evidence route that best fit the actual repository state. The agent may replace any runbook tactic when it preserves the invariants below and records evidence at least as strong as the default route.

## Completion Contract

For parent PRD issue `#PARENT`, complete every open ready-for-agent child issue that is explicitly tied to the parent, then either open one ready PR to the resolved base branch or finish through the no-diff terminal path.

A successful completion has:

1. a frozen child queue built from explicit parent-child relationships
2. every frozen child completed, tested according to risk, committed when repository state changes, and independently reviewed
3. no blocked or skipped frozen child
4. final integration evidence covering the combined PRD diff or no-diff state
5. no unresolved non-trivial review finding
6. one final PR, except when user instruction, permission boundary, or the no-diff terminal path applies

Low-risk residual notes may remain only for spelling, comments, naming nits, or other non-behavioral cleanup. Findings about tests, security, data integrity, compatibility, user-visible behavior, migrations, permissions, or external integrations are non-trivial.

## Terms

| Term | Meaning |
|---|---|
| Parent PRD issue | GitHub issue `#PARENT`; source of run scope |
| Child issue | GitHub issue explicitly tied to `#PARENT` |
| Ready label | Repository label mapped from canonical role `ready-for-agent` |
| Frozen queue | Ordered child issue set selected during planning |
| Integration branch | Current non-base worktree branch |
| Issue base | `ISSUE_BASE_SHA`, current `HEAD` before one child starts |
| Issue range | `ISSUE_BASE_SHA..HEAD`; default code/doc review range |
| Final integration range | The same diff the final PR will present |
| Reviewable evidence set | Stable evidence an independent reviewer can inspect for a child or final integration |
| HITL gate child | Review or decision child whose acceptance criteria produce a gate decision rather than product code |
| Drift | Child issue state changes after planning |
| No-diff terminal path | Successful completion path when the integration is already in the resolved base branch before PR creation |
| Incomplete terminal | Stopped state that prevents token drain but is not a successful completion |
| Accepted incomplete terminal | Incomplete terminal explicitly accepted by the user as handled |

## Invariants

Stop and report when an invariant cannot be satisfied.

Repository:

1. Work only inside the current repository worktree.
2. The current branch must be a non-base integration branch.
3. Do not switch branches, create per-issue branches, merge PRs, force-push, rebase, squash, rewrite completed issue commits, or cherry-pick unrelated commits unless explicitly instructed.
4. Push only the integration branch, and only after final gates pass.
5. Never manufacture a diff, unrelated commit, history rewrite, or base-branch revert only to satisfy PR creation.

GitHub:

1. Do not manually close issues.
2. Do not change labels, project fields, milestones, assignees, close state, or issue comments unless explicitly instructed.
3. A child acceptance criterion that explicitly requires an issue comment authorizes only that comment. Keep it limited to verified decision, evidence, compatibility decision, and blockers.
4. Do not comment when the decision needs unresolved user or product judgment, or when the comment would expose secrets, credentials, private runtime details, customer data, or sensitive incident information.
5. Use closing keywords only for completed frozen children and the completed parent PRD in the final PR body.

Worktree:

1. Start from a clean working tree.
2. Dirty state is allowed only for the current child issue.
3. Review only stable evidence: committed repository changes, committed evidence artifacts, or explicitly authorized external artifacts.
4. If dirty-change ownership is uncertain, stop instead of cleaning destructively.

## Evidence Model

Independent review evaluates the reviewable evidence set for a stable child or final integration state. It does not have to review only a git diff, but the evidence set must not be empty or implicit.

For a code or docs child, the primary evidence is `ISSUE_BASE_SHA..HEAD`.

For an evidence-only or HITL gate child, the evidence set must include a concrete decision or evidence artifact that the reviewer can inspect. Prefer a meaningful committed note in the repository. If the issue explicitly requires a GitHub comment or other external artifact, that artifact may be part of the evidence set, but the reviewer must still see the exact decision and evidence being approved before the child is marked complete.

Do not create meaningless files only to make a review range non-empty. If no meaningful reviewable evidence can be produced, the child is blocked.

Each child evidence set records:

1. child issue ID and acceptance criteria being satisfied
2. issue base and repository diff, if any
3. decision or evidence artifact, if the child is HITL or evidence-only
4. tests or verification selected for the risk
5. independent review result and any findings
6. fix commits and verification, when findings were valid

Final integration evidence records:

1. the PR diff or verified no-diff state
2. broader integration tests or verification
3. generated artifacts, migrations, rollback notes, SQL/API/frontend type alignment, and cross-issue relationships where applicable
4. final independent review result
5. drift and compatibility impact

## Operating Principles

1. Optimize for the real PRD completion state, not for checking off commands.
2. Let repository evidence choose the path. Broader, narrower, or more direct verification is valid when it matches the risk.
3. Use the smallest reliable evidence route. Do not run redundant commands because a runbook example exists.
4. Treat reviewer output as claims to verify, not instructions to obey blindly.
5. Treat reviewer-run tests as supporting evidence. The main workspace owns canonical repository test evidence.
6. Classify tool limitations separately from product failures.
7. Ask the user only when the next decision changes product behavior, architecture direction, compatibility, ownership, schedule, or external commitments beyond the accepted PRD scope.

## State Machine

Success terminal states:

1. `PRCreated`: all completion requirements pass and one ready PR is opened
2. `NoDiffComplete`: all completion requirements pass except normal PR creation, and the no-diff terminal conditions are verified

Incomplete terminal states:

1. `BlockedReported`: a frozen child, HITL gate, final evidence gate, or review gate cannot be completed under this contract
2. `SkippedReported`: a frozen child is skipped because of drift or explicit user instruction
3. `BoundaryReported`: user instruction, permission boundary, or external access boundary prevents completion

`AcceptedIncomplete` exists only after the user explicitly accepts an incomplete terminal state as handled. Reporting blocked or skipped work stops token drain; it does not by itself make the PRD goal successfully complete.

Progress must be monotonic:

1. the frozen queue is finite and does not grow without explicit user instruction
2. each child advances from not-started to complete, blocked, or skipped
3. each independent review has at most two completed rounds
4. each failed review command or transient tool failure gets at most one retry before it becomes an incomplete gate
5. repeated fix/test cycles require new evidence, a narrower hypothesis, or a clearly different scoped fix; otherwise stop and report the child as blocked
6. each HITL gate may return to an earlier child only for a concrete, owned, still-valid gap
7. if the same HITL gate exposes the same class of gap again after one return-and-fix cycle, stop and report the gate as blocked

Do not continue working only to improve polish, gather redundant evidence, retry unstable tooling, or expand scope after a terminal state is reached.

## Flow

### Startup

Verify repository root, branch, clean status, resolved base branch, GitHub issue access, ready label mapping, and relevant repository guidance.

### Planning

Build a frozen queue before implementation.

A child is eligible when it is open, has the resolved ready label, and is explicitly tied to `#PARENT` by GitHub sub-issue relationship, parent task list entry, `Parent: #PARENT` / `Part of #PARENT` text, or equivalent bidirectional reference.

Casual mentions are not enough. Ambiguous dependencies block dependent work until reported. Newly discovered children after planning are drift; report them before final completion, but do not silently expand scope.

### Per Child

For each frozen child:

1. re-check status and ready label
2. record `ISSUE_BASE_SHA`
3. implement only that child scope
4. produce a non-empty reviewable evidence set
5. run risk-matched tests or verification
6. commit traceably to the child issue when repository state changes
7. run independent review on the child evidence set
8. fix valid findings within scope, then retest
9. run Round 2 review only when Round 1 reports a valid P0/P1 finding; lower-severity findings do not require a second review round

For a HITL gate child, verify the gate against the current branch and the prior wave evidence it depends on. If the gate exposes a valid earlier-child gap, return to that child scope, fix the gap, retest, re-review, then rerun the gate. If the gap has no clear owning child, or repeats after one return-and-fix cycle, stop and report the HITL gate as blocked.

### Review

Independent review means a separate reviewer pass, tool, or session evaluates a stable reviewable evidence set.

Required review properties:

1. per-child review targets the full child evidence set
2. for code or docs children, the evidence set includes `ISSUE_BASE_SHA..HEAD`
3. for HITL or evidence-only children, the evidence set includes the concrete decision or evidence artifact
4. Round 2 is required only after a valid P0/P1 finding in Round 1, and reviews the full evidence set again
5. final review targets the same diff or no-diff state the terminal artifact will present
6. failed, killed, timed-out, or empty review output is not a pass
7. valid non-trivial findings must be fixed, verified, and recorded before success completion
8. fixed Round 2 findings do not require a third pre-PR review round

### Final Integration

After all frozen children are complete:

1. rediscover child drift and report it
2. run broader integration evidence appropriate to the combined diff or no-diff state
3. check generated artifacts, migrations, rollback notes, SQL/API/frontend type alignment, and cross-issue relationships where applicable
4. re-check HITL gate decisions against the final branch state
5. run final independent review against the PR diff or verified no-diff state

Stop without PR when a frozen child is blocked or skipped, a HITL gate now blocks, final evidence fails, or final review has unresolved non-trivial findings.

### Final PR

Open one ready PR only when all success completion requirements pass, the working tree is clean, and the diff contains only PRD integration scope.

The PR body must include the parent and completed children closing keywords, tests/evidence, review summary, drift, compatibility impact if any, final integration checks, residual notes if any, and a note that merging the PR is the parent PRD closure action.

### No-Diff Terminal Path

Use this when PR creation would fail or has failed because the completed integration is already present in the resolved base branch.

It is valid only when all success completion requirements pass except normal PR creation, the resolved base branch and integration branch have no remaining diff, and the integration branch has no commits remaining outside the resolved base branch or an existing PR already merged the same integration head.

Do not create unrelated commits just to open a PR. Report the parent PRD issue closure state separately because there may be no new PR merge event to trigger GitHub closing keywords.

## Final Response

Report terminal state, completed issues, blocked or skipped issues, PR URL or no-diff terminal status, tests/evidence, review result, drift, compatibility impact, parent PRD closure state, and clean working tree status.
