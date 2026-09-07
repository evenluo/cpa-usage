# CPA data contract

Status: current
Authority: [Accounting v2 direct cut](accounting-v2-direct-cut.md), ADR 0008 and ADR 0009.

CPA Usage supports the Accounting v2 queue contract pinned to CPA v7.2.152 at `c76dfd4e0edabab9000628b1560ab8ab379eadb8`. It has no old-producer mode, mixed-version decoder, legacy metric fallback or feature switch. [Canonical usage accounting](usage-accounting-contract.md) defines admission, metric units, quality, estimates and raw/rollup ownership.

Account availability, passive quota and model definitions are observations from CPA management responses. Missing quota or capability facts remain unknown even on a supported producer; they are not healthy/zero values. Retry eligibility is not a recovery guarantee. Registered model support does not prove current routing availability. Source timestamps and truthful partial reads remain necessary runtime semantics.

Passive quota uses the existing successful auth-file snapshot, preserves its original observation time and never merges into manual probe evidence. Registered Model Support stays an explicit bounded selected-scope request: 12 accounts, concurrency 4, 15 seconds, at most 36 upstream GETs. It adds no persisted catalog, worker or automatic retry.

## Upgrade and release

The supported upgrade source is published main, not an unpublished intermediate branch. Raw historical usage and local rates/aliases are retained. Schema migration precedes ingestion startup, initializes historical accounting absence in existing rollups and leaves existing coverage checkpoints unchanged. Old token fields are archival only. No history-wide accounting reconstruction is scheduled.

Before deployment, verify a pre-upgrade SQLite copy and confirm the intended upstream producer contract with an authenticated safe sample. Finish all local pending/process_failed inbox messages with the currently deployed consumer before cutover. The accounting migration checks this condition and fails without deleting rows if processing is incomplete. This replaces the historical queue-key identity adapter with one pre-upgrade boundary. Future unsupported producer messages are isolated as `decode_failed`. Do not infer that an image tag or anonymous health establishes payload compatibility. No production queue should be popped merely for validation.

SQLite opens/migrates before scheduled backup startup, so scheduled backups cannot supply a pre-migration copy. Transactional migration and its ledger establish forward application/idempotence, not old-binary rollback compatibility or restoration of omitted upstream facts.

The existing `.github/workflows/release.yml` remains the sole production release owner. This implementation neither deploys nor mutates provider/production database state. Any actual deployment must verify exact artifact identity, terminal deployment result, health, and authenticated behavior separately.
