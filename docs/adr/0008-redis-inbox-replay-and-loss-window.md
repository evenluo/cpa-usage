# Keep Redis inbox replay deterministic and the destructive-pop loss window explicit

CPA Usage consumes the CPA usage queue with `LPOP`, then persists each returned raw message in SQLite before local decoding and event insertion. Because Redis removal and SQLite insertion cannot be one transaction, a message can be lost after `LPOP` starts but before its inbox row commits; inbox-write failures are returned and logged as observable loss, and the queue Adapter does not retry, requeue, or switch transports after the destructive command may have started.

## Decision

- A persisted inbox row's `PoppedAt` is the only fallback event timestamp when its provider payload has no timestamp. Every retry decodes that row with the same persisted value, so a missing request ID produces the same canonical event key.
- Processing time may set `ProcessedAt` and appear in operational logs, but it does not participate in event timestamp or identity.
- A zero `PoppedAt` is a persisted-invariant failure. The row is isolated as a decode failure instead of falling back to the current processing time.
- Request-ID identity, canonical-key assignment, deduplication, and bad-row isolation otherwise remain unchanged.
- There is no automatic recovery for either the ambiguous interval where an `LPOP` response is lost or the interval between a successful response and the SQLite commit. An authoritative CPA claim/ack capability, requeue policy, schema change, or replay service requires a later contract decision.
- Redis reads are bounded by the request context and positive `REQUEST_TIMEOUT`. HTTP fallback is allowed only when failure is proven to precede a destructive `LPOP`; write, response-read, cancellation, or timeout failure after the command may have started ends the pull without switching transports.

## Consequences

- Retrying a persisted row after event insertion or processed-mark failure is deterministic across clock changes.
- Operators receive an explicit failed pull and error log if SQLite cannot persist a popped batch, but must treat that batch as lost rather than recoverable.

## Compatibility

Compatible: this is an intentional correction within the existing fallback contract. For queue payloads without a provider timestamp, the fallback event timestamp now comes from the persisted `PoppedAt` instead of each processing attempt's clock; when the request ID is also absent, the canonical event key follows that stable timestamp. The CPA queue protocol, SQLite schema, and payloads with provider timestamps or request IDs retain their existing semantics.
