# Auth-file model support contract

Status: current for slice I / issue #149
Authority: Parent #140 r1 and child #149 r1
Upstream evidence: CLIProxyAPI `c76dfd4e0edabab9000628b1560ab8ab379eadb8`

## User-visible claim

CPA Usage can explicitly load the models currently registered to a selected set of known auth-file accounts. This is **Registered Model Support**, not current routing availability, provider health, capacity, or a global provider catalog.

The protected local Interface is `POST /api/v1/usage/identities/model-support` with an `identity_ids` array. Callers never provide an upstream auth-file name or static-catalog channel. The backend validates active auth-file identities from the local read model, resolves each `auth_index` through the existing CPA auth-file lookup, prefers the returned unique auth ID over its filename for the upstream `name` query, and then calls the narrow CPA management GET adapters.

## Pinned upstream facts

- `GET /v0/management/auth-files/models?name=...` returns registered model identity fields (`id` and optional `display_name`, `type`, `owned_by`). A successful empty list does not distinguish every possible upstream reason and does not prove future or immediate routing.
- `GET /v0/management/model-definitions/:channel` returns an embedded static catalog. CPA Usage allowlists only context/input/output limits, thinking support, and input/output modalities. Configuration and unknown fields are discarded.
- Capability metadata joins registered support by exact model ID. There is no family or alias fallback.
- The exact channel map is restricted to the pinned upstream catalog. `gemini-cli` is explicitly mapped to the upstream `gemini` catalog, while `gemini-interactions` keeps its exact pinned channel; unknown local account types retain registered support with an explicit unknown-channel capability state.

## Bounds and failure policy

- Maximum selected scope: 12 accounts. Oversize requests return a client error and require a narrower selection; no account is silently truncated.
- Maximum concurrent upstream requests: 4.
- Total request-scoped timeout: 15 seconds.
- Maximum upstream GETs: 36 (`2 * 12` account name/model reads plus at most one static definition read per selected account channel).
- There is no automatic retry, background worker, persisted result, or permanent cache.

An account model lookup succeeds only after the exact local auth-file identity resolves to a matching CPA `auth_index` and its registered-model response is valid. A failure is returned as a bounded safe category and makes the selected scope partial; it is never counted as unsupported. Static catalog absence or failure qualifies capability metadata but does not erase a successful registered-model reading.

Per-model counts in a partial result are labeled observed counts over loaded accounts. Only a complete selected scope may state that exactly one registered supporting account exists in that scope. That statement remains scoped and is never a global single point or availability claim.

Disabled and transiently unavailable accounts remain selectable. Their existing account state is displayed independently and is not converted into a model-support conclusion.

## Compatibility

This is an additive protected endpoint and additive UI action. Existing routes, auth/session behavior, SQLite schema and data, ingestion, manual account toggle/probe behavior, navigation, backup, release topology, and Cost semantics remain compatible. The feature introduces no CPA write and no provider mutation.
