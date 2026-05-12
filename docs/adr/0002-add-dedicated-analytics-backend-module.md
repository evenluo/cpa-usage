# Add a dedicated analytics backend module

CPA Usage v1 will add key-centric analytics in a dedicated backend module instead of extending the existing broad usage aggregation path. The existing keeper backend is valuable and should be reused, but its current overview, analysis, events, health, and cost aggregation responsibilities are already concentrated; adding Key Alias rankings, trends, drill-downs, and deterministic insights there would make the core analytics path harder to test and evolve.

## Consequences

- Stable keeper capabilities such as CPA queue consumption, SQLite persistence, pricing, events, auth, backup, and deployment remain inherited.
- New Key Alias analytics should be built behind focused service/repository interfaces.
- Existing usage aggregation can be reused where appropriate, but the new module owns the product-facing analytics API shape.
- Key/model/time-bucket statistics should use SQL aggregation first; in-memory processing should only shape bounded result sets for the frontend.
