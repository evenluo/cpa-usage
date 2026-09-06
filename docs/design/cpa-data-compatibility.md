# CPA data compatibility

Status: current
Layer: scoped design contract
Use for: interpreting CPA producer data, upgrading SQLite, and judging release evidence
Current SoT: this file together with ADR 0008 and ADR 0009

## Supported producer shapes

CPA Usage uses field-based decoding. It does not choose a decoder from the CPA
application version string. The pinned producer references define the supported
shapes and fixtures:

- CPA v7.2.62 at `3554b63721aac9b4202bf2ef88ba7a82b4e5caf8`
  emits the legacy usage fields, including token scalars and request/attempt
  evidence. Its queue payload does not contain `accounting_version`,
  `token_breakdown`, `generate`, `stream`, or `response_service_tier`.
- CPA v7.2.152 at `c76dfd4e0edabab9000628b1560ab8ab379eadb8`
  retains the v7.2.151 accounting-v2 and passive quota contracts. The relevant
  accounting and quota contract files are byte-identical between v7.2.151 and
  v7.2.152.

The v7.2.152 queue also exposes `session_id` and `parent_session_id`. They are
outside the persisted compatibility contract described here; their presence
does not change attempt identity or accounting interpretation.

A v7.2.62-shaped attempt remains a valid legacy attempt with accounting state
`absent`. Absence is neither zero usage nor malformed accounting. Unsupported,
incomplete, or malformed canonical fields are explicitly qualified and do not
cause an otherwise valid legacy attempt to be discarded. Canonical values are
never reconstructed from legacy totals, provider names, aliases, or tiers.

Account availability fields are optional observations. Missing `status`,
`unavailable`, `last_refresh`, or `next_retry_after` does not establish an
active, available, fresh, or recovered account. `next_retry_after` is only the
earliest retry eligibility.

The v7.2.62 management response does not expose the passive `quota` and
`model_quotas` observations supported from v7.2.152. Their absence remains
unavailable, rather than zero or healthy. Registered model support and static
model definitions are capability metadata; they do not prove current routing
availability.

## Collection and replay

Accounting-v2, execution/tier, availability, and passive quota facts are
future-only. CPA Usage preserves them only when a supporting queue payload or
management response reaches the corresponding deployed consumer. It does not
synthesize historical facts, quota history, missing attempts, or earlier
availability state.

The existing Redis inbox is the replay owner. Its allowlisted projection,
`PoppedAt`, lifecycle, and inbox-row attempt identity continue to follow ADR
0008 and ADR 0009. Replay is deterministic and idempotent for the inbox row;
`request_id` remains correlation only. A row can yield only fields retained when
it was persisted. Replay cannot recover fields discarded by an older allowlist
or reconstruct attempts lost before durable inbox persistence.

The narrow legacy `queue` event-key adapter remains governed by ADR 0009. This
contract adds no general old-payload adapter, queue rereader, second consumer,
requeue path, or raw-fact repair path.

## Passive quota and model support

Passive quota reads only the existing successful `/v0/management/auth-files`
metadata snapshot for supported Claude and Codex entries. Identity persistence
uses nullable JSON-serialized TEXT columns for normalized account and model
observations. The migration adds the columns without historical quota backfill;
existing rows remain absent until a later supported snapshot supplies a fact.

The API exposes normalized `source=cpa_passive`, scope, original `observed_at`,
and allowlisted bounded values. It does not persist raw signal maps, infer
expiry or history, replace retry state, or merge passive facts with the manual
probe cache. Page reads do not call providers.

Registered Model Support adds no database migration, persisted model catalog,
worker, page-load request, or automatic retry. A protected endpoint performs an
explicit bounded Load for selected known auth-file identity IDs. The request is
limited to 12 accounts, concurrency 4, a 15-second timeout, and at most 36
upstream GETs.

Registered model IDs join static capability definitions by exact ID. Known
channel mapping is exact: `claude` to `claude`; `gemini` and `gemini-cli` to
`gemini`; `gemini-interactions` to itself; `vertex`, `aistudio`, `codex`, `kimi`,
and `antigravity` to themselves; and `xai`, `x-ai`, and `grok` to `xai`.
Unmapped account types retain registered support while capability status is
`unknown_channel`. Failed accounts make the selected scope partial and are not
reported as unsupported.

## SQLite upgrade behavior

The accounting migration is additive. It adds nullable canonical and execution
columns, explicit presence/malformed markers, and
`accounting_state = 'absent'` for historical rows. It does not rewrite legacy
tokens, requested tier, event identity, inbox payloads, or raw events. The
availability and passive quota migrations are also additive and leave
previously unobserved facts absent. Registered Model Support adds no schema.

The hourly rollup migration adds accounting state/quality counters and canonical
sums as `INTEGER NOT NULL DEFAULT 0`. It schedules reconstruction through the
existing bounded rollup backfill owner. When raw events exist, migration reads
the earliest and latest UTC hours through indexed extrema, preserves a later
existing target, moves an existing coverage checkpoint to no later than one
hour before the earliest raw bucket, and resets the backfill lifecycle to
pending. If no coverage checkpoint exists, it remains absent and the existing
full-backfill semantics apply.

The backfill deletes and reconstructs affected hourly rollup rows from persisted
`usage_events` in bounded batches. It never updates raw events or infers missing
accounting facts. Pre-upgrade rows therefore remain `accounting_state =
'absent'`; rebuilt rollups count them as absent and contain no fabricated
canonical composition. `backfill_incomplete` remains authoritative until the
existing runner reaches the retained target. Migration success alone does not
establish complete accounting rollup coverage.

Migrations are transactional and recorded in `schema_migrations`. This proves
forward application and idempotence only. It does not prove schema downgrade,
old-binary compatibility with the upgraded database, restoration of omitted
producer fields, or a production rollback procedure.

The application opens and migrates SQLite before it creates the background
backup runner. Scheduled application backups therefore cannot serve as a
pre-migration copy. Any deployment that requires a recoverable pre-upgrade
database must create and verify that copy before starting the upgraded binary.
The current backup writer is not a restore command or schema rollback owner.

## Release and evidence boundary

`.github/workflows/release.yml` is the sole production mutation owner for the
CPA Usage Dokploy Compose application. This compatibility contract adds no
second deployment command, migration job, compatibility switch, or rollback
workflow. A production change must continue through that release owner and its
fresh release evidence.

Container image identity and Docker health describe the running artifact and
container state only. They do not prove authenticated CPA or CPA Usage behavior,
management payload shape, queue contents, migration completion, or protected UI
paths. Anonymous health, an expected unauthenticated response, and an image tag
must not be reported as authenticated or payload smoke evidence.

Compatibility verification should cover fresh and upgraded databases with
v7.2.62-shaped, v7.2.152-shaped, and mixed fixtures; unchanged historical raw
rows; additive defaults and migration ordering; coverage reset and bounded
rollup reconstruction; processable inbox replay; and raw/rollup agreement.
These fixtures establish the repository contract, not production payload or
deployment behavior.
