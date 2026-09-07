# CPA data contract

Status: current
Authority: [Accounting v2 direct cut](accounting-v2-direct-cut.md), ADR 0008 and ADR 0009.

CPA Usage supports the Accounting v2 queue contract pinned to CPA v7.2.152 at `c76dfd4e0edabab9000628b1560ab8ab379eadb8`. It has no old-producer mode, mixed-version decoder, legacy metric fallback or feature switch. [Canonical usage accounting](usage-accounting-contract.md) defines admission, metric units, quality, estimates and raw/rollup ownership.

Account availability, passive quota and model definitions are observations from CPA management responses. Missing quota or capability facts remain unknown even on a supported producer; they are not healthy/zero values. Retry eligibility is not a recovery guarantee. Registered model support does not prove current routing availability. Source timestamps and truthful partial reads remain necessary runtime semantics.

Passive quota uses the existing successful auth-file snapshot, preserves its original observation time and never merges into manual probe evidence. Registered Model Support stays an explicit bounded selected-scope request: 12 accounts, concurrency 4, 15 seconds, at most 36 upstream GETs. It adds no persisted catalog, worker or automatic retry.

## Upgrade and release

The supported upgrade source is published main, not an unpublished intermediate branch. Raw historical usage and local rates/aliases are retained. Schema migration precedes ingestion startup, initializes historical accounting absence in existing rollups and leaves existing coverage checkpoints unchanged. Old raw event token fields are archival only; inactive derived rollup scalar columns are removed. No history-wide accounting reconstruction is scheduled.

Prepare an explicit verified Accounting v2 image for the independently managed CPA application; confirm its queue contract against that version's offline fixture or a non-production capture. Before restarting the in-memory producer, pause requests and let the published consumer drain the upstream queue and local pending/process_failed inbox. Stop the consumer and take a verified pre-upgrade SQLite copy, upgrade CPA first, then deploy the integrated consumer and verify a controlled attempt before resuming requests. See [Accounting v2 cutover steps](../deploy/self-hosted-cutover-runbook.md#accounting-v2-cutover). The accounting migration checks this condition and fails without deleting rows if processing is incomplete. This replaces the historical queue-key identity adapter with one pre-upgrade boundary. Future unsupported producer messages are isolated as `decode_failed`. Do not infer that an image tag or anonymous health establishes payload compatibility. No production queue should be popped merely for validation.

SQLite opens/migrates before scheduled backup startup, so scheduled backups cannot supply a pre-migration copy. Transactional migration and its ledger establish forward application/idempotence, not old-binary rollback compatibility or restoration of omitted upstream facts.

The existing `.github/workflows/release.yml` remains the sole production release owner. This implementation neither deploys nor mutates provider/production database state. Any actual deployment must verify exact artifact identity, terminal deployment result, health, and authenticated behavior separately.
