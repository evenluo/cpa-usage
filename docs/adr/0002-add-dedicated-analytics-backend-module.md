# Add a dedicated analytics backend module

Status: Superseded by ADR 0003 and ADR 0004.

CPA Usage originally planned to add key-centric analytics in a dedicated backend module instead of extending the existing broad usage aggregation path. ADR 0003 and ADR 0004 replaced that direction: analytics read models now stay in `internal/repository`, analytics implementation is split by same-package files, and `internal/api` remains the HTTP contract owner.

## Consequences

- Stable keeper capabilities such as CPA queue consumption, SQLite persistence, pricing, events, auth, backup, and deployment remain inherited.
- Superseded: new Key Alias analytics are not owned by a separate backend module in the current implementation.
- Superseded: product-facing analytics HTTP payload shape remains owned by `internal/api`; analytics read models are owned by `internal/repository`.
- Key/model/time-bucket statistics should use SQL aggregation first; in-memory processing should only shape bounded result sets for the frontend.
