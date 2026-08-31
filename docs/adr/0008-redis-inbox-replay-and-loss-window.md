# Keep Redis inbox replay deterministic and the destructive-pop loss window explicit

Event identity note: [ADR 0009](0009-preserve-cpa-usage-attempts.md) supersedes this ADR's request-ID identity and canonical-event-key statements. This ADR remains authoritative for destructive-pop effects, persisted `PoppedAt`, and transport fallback.

CPA Usage consumes the CPA usage queue with `LPOP`, then persists a replay-safe projection of each returned message in SQLite before local decoding and event insertion. Because Redis removal and SQLite insertion cannot be one transaction, a message can be lost after `LPOP` starts but before its inbox row commits; inbox-write failures are returned and logged as observable loss, and the queue Adapter does not retry, requeue, or switch transports after the destructive command may have started.

## Decision

- A persisted inbox row's `PoppedAt` is the only fallback event timestamp when its provider payload has no timestamp. Every retry decodes that row with the same persisted value.
- Processing time may set `ProcessedAt` and appear in operational logs, but it does not participate in event timestamp or identity.
- A zero `PoppedAt` is a persisted-invariant failure. The row is isolated as a decode failure instead of falling back to the current processing time.
- Before inbox persistence, valid messages are projected through the owned usage decoder schema. `fail.body`, `response_headers`, and arbitrary unknown fields are excluded; `fail.status_code` and every field required by event replay remain. Invalid JSON is replaced by a non-sensitive marker containing only its SHA-256 and byte length, then follows the existing `decode_failed` lifecycle.
- Bad-row isolation otherwise remains unchanged. Event identity and local replay deduplication are now owned by ADR 0009.
- There is no automatic recovery for either the ambiguous interval where an `LPOP` response is lost or the interval between a successful response and the SQLite commit. An authoritative CPA claim/ack capability, requeue policy, schema change, or replay service requires a later contract decision.
- Redis reads are bounded by the request context and positive `REQUEST_TIMEOUT`. HTTP fallback is allowed only when failure is proven to precede a destructive `LPOP`; write, response-read, cancellation, or timeout failure after the command may have started ends the pull without switching transports.

## Consequences

- Retrying a persisted row after event insertion or processed-mark failure is deterministic across clock changes.
- Newly popped provider error bodies and response headers do not enter SQLite or its backups. Durable `UsageEvent` and API projections likewise contain only the selected evidence fields.
- Operators receive an explicit failed pull and error log if SQLite cannot persist a popped batch, but must treat that batch as lost rather than recoverable.

## Compatibility

This compatibility statement covers the persisted-timestamp and transport-fallback decision: payloads without a provider timestamp use persisted `PoppedAt` instead of each processing attempt's clock. The later event-identity correction and its compatibility boundary are documented in ADR 0009.
