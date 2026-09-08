# Quota observation retention

Status: current
Compatibility: incompatible direct cut, authorized for Live Capacity quota observations on 2026-09-08.

## User-visible contract

**Last updated** means the account's latest successful quota observation time, regardless of whether a manual probe or CPA supplied it. Metadata sync time, token refresh time and refresh-attempt time are not quota observations.

Keep the last successful reading and its original time until a later successful observation replaces it. Page reloads, service restarts, failed refreshes, missing passive reports and disabling the account do not erase it. An account with no observation shows **No reading**. There is no automatic provider request, cache-expiry label or age-based invalidation; users decide whether to refresh from the displayed age. Provider-reported quota reset times remain separate facts.

Account and model observations retain their scopes. Different windows may have different ages: the newest observation wins when sources describe the same window, and each reading keeps its own source/time tooltip. The card's **Last updated** is the newest successful observation across its manual, account and model quota sources; it does not imply every visible reading was fetched together.

## Ownership and transitions

- SQLite owns the latest successful manual observation for each auth-file identity. Persist only the normalized quota response and observation time; never store credentials, raw provider payloads or headers.
- A manual check reports success only after its normalized observation has been saved. Persistence failure is an explicit failed operation, leaving the previous observation intact.
- Passive observations remain in their existing identity fields. Metadata updates retain the latest valid account observation and latest valid observation per model. Missing or older inputs do not replace them.
- Retention is one latest snapshot per source/scope, not an audit log. Existing identity lifecycle rules continue to control which accounts are visible; retained observations must not resurrect a deleted account or change refresh eligibility.
- Refresh tasks own queue, running, completed and failed states only. Their internal terminal-record retention may clean up polling receipts; it does not expire successful observations and is not exposed as quota validity.
- The frontend observation query holds the successful snapshot. Starting or failing another task does not overwrite it, and an older task completion cannot replace a newer snapshot. Page loading reads these observations without probing providers.

## HTTP direct cut

`POST /api/v1/quota/observations` replaces `POST /api/v1/quota/cache`. It accepts `auth_indexes` and `limit` and returns `items`, each with `id`, normalized `quota` rows and required `observedAt`.

`POST /api/v1/quota/check` returns the same timestamped successful observation. Refresh-task polling returns that observation in `quota` upon completion. `cachedAt` and `expiresAt` are removed; task responses do not duplicate the observation timestamp at the top level. The old cache route and fields have no aliases or dual-read support. Backend and frontend must be released together.

Existing passive snapshots retain their source timestamps. Manual results already lost from the previous process-memory cache cannot be reconstructed and are not assigned fabricated timestamps; the next successful explicit check creates their retained observation. The change requires a forward SQLite migration and no production data reset.

## Acceptance evidence

- Save a successful probe, expire/clean its task receipt, recreate the service/database connection and read the same quota and timestamp without a provider call.
- A failed subsequent provider call or persistence write leaves the previous successful observation readable; a newer success replaces it and an older observation does not.
- Missing or older passive snapshots preserve the latest account/model readings across metadata sync; account deletion still governs visibility.
- Fresh page loading, refresh success/failure and reads of observations older than 20 minutes retain the appropriate data and **Last updated**. Model-only observations contribute their real timestamp; metadata-only accounts do not.
- No cache-expiry/stale indicator or retired cache API remains in the current application contract. Theme, responsive layout, refresh controls and source/time tooltips continue to work.
