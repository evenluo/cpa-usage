# Frontend Read State and Pagination Contract

Status: current  
Scope: authentication session reads, Usage Intelligence, Reference Data, and paginated usage reads  
Authority: GitHub Issue #122

## Read State

Only a successful session response containing a boolean `authenticated` field establishes authenticated or unauthenticated state. Transport, HTTP, and malformed-response failures remain explicit errors, and protected UI stays unavailable until its session read succeeds.

Usage Intelligence and Reference Data reads keep independent loading, error, empty, and ready state. A failed refresh may retain a previously completed result, but an initial failure must not be presented as zero or empty data. Retry targets only the failed query.

## Pagination

Successful paginated responses publish positive integer `page`, `page_size`, and `total_pages` metadata plus a non-negative integer `total_count`. A successful empty response is one non-navigable page: `page: 1`, `total_pages: 1`, and `total_count: 0`.

Frontend paginated collectors validate every page and expose data only after the complete advertised result has been collected. Missing, invalid, inconsistent, or later-page-failed responses remain explicit errors; clients do not guess page counts or publish a successful prefix.

## Compatibility

Incompatible (intentional): empty event, account, and API-key page responses previously used `total_pages: 0` and could echo a requested page greater than one. They now use the normalized one-page empty representation above. API consumers that asserted zero pages must adopt the positive-page contract.

Compatible: backend session payloads and authentication modes do not change. The frontend now distinguishes a valid `authenticated: false` response from session read failures instead of redirecting both cases to sign-in. Existing successful authenticated and unauthenticated flows retain their routes and payloads.
