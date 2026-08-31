# Preserve each persisted CPA usage attempt

CPA emits one usage record for one upstream provider attempt. A single client request can retry, fail over, or invoke an additional model while retaining the same CPA request ID. Treating that request ID as the unique `usage_events.event_key` therefore drops later attempts and makes event counts, failure rates, provider attribution, tokens, and Cost depend on queue arrival order.

## Decision

- One persisted `redis_usage_inboxes` row owns one `UsageEvent` attempt.
- Redis inbox processing assigns new `usage` queue rows the namespaced event key `redis-inbox:<row-id>`. The inbox row ID is stable across local processing retries, while separate popped rows remain distinct even when their request ID and raw payload bytes are identical.
- `request_id` remains a correlation fact only. It does not own event identity and does not imply a final client-visible outcome.
- Reprocessing the same inbox row remains idempotent through the existing unique event-key constraint. No request aggregate, retry graph, replay service, or additional deduplication layer is introduced.
- Existing historical event keys and rows are not rewritten. Attempts that were already collapsed cannot be reconstructed from local data.

This decision supersedes only the request-ID identity and canonical-event-key portions of [ADR 0008](0008-redis-inbox-replay-and-loss-window.md). ADR 0008 continues to own the destructive-pop loss window, persisted `PoppedAt` fallback timestamp, and pre-effect-only HTTP fallback.

## Consequences

- New event counts and success/failure rates describe CPA upstream attempts. Retry and failover traffic can therefore produce multiple events for one request ID.
- Request Evidence may show request ID as protected correlation detail, but attempt status must not be presented as the final client request outcome.
- Existing API field names remain compatible, while user-facing count and rate labels identify attempt semantics.
- Time ranges that cross this correction can contain historical request-ID-collapsed rows and new attempt-grain rows. No historical repair or runtime cutover inference is attempted; this documented boundary is the authoritative qualification.

## Compatibility

This is an intentional, future-only correction to SQLite event identity and user-visible counting semantics. Existing routes, response field names, stored rows, and the unique `event_key` constraint remain compatible. Historical data is retained unchanged, and deployment does not replay CPA data or synthesize missing attempts.

Processable inbox rows written under the former `queue` key retain their decoded legacy event key. This narrow upgrade adapter prevents a duplicate when an older process committed the event but failed before its separate inbox processed mark. New `usage` rows use attempt identity. Because pending rows have no time-based cleanup boundary, the adapter may be removed only when every supported upgrade path proves that its database cannot contain a processable legacy `queue` row.
