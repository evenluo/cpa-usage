# Self-Hosted Shared Login Configuration

## Goal

Allow a CPA root service on `https://<your-cpa-host>/` and this `cpa-usage` service on `https://<your-cpa-host>/usage` to accept the same browser login session.

## Runtime Contract

CPA's current management UI persists its remembered login in the same-origin browser `localStorage` entry `cli-proxy-auth`. With **Remember password** enabled, its `state.managementKey` is stored in an `enc::v2::` envelope whose reversible XOR mask uses `cli-proxy-api-webui::secure-storage|<host>` and does not depend on the browser user agent. See the [upstream storage format](https://github.com/router-for-me/Cli-Proxy-API-Management-Center/blob/1650cf2f726685da188237eda9e5345d0bd15354/src/utils/encryption.ts).

`cpa-usage` reads that snapshot and sends `Authorization: Bearer <management key>` to its protected APIs. It does not copy, migrate, or rewrite the upstream storage. Without **Remember password**, CPA keeps the key only in its own page memory, so a separately loaded Usage page cannot reuse it and requires its own login.

`cpa-usage` also supports signed HTTP-only cookies for its own session. These cookies make `cpa-usage` sessions restart-safe and configurable, but the CPA root service does not consume them.

## Required Environment

Configure `cpa-usage` with the CPA management key and its own dashboard login settings:

```dotenv
AUTH_ENABLED=true
CPA_MANAGEMENT_KEY=<same value as CPA management password>
CPA_USAGE_LOGIN_PASSWORD=<usage dashboard password>
AUTH_SESSION_TTL=168h
AUTH_SESSION_SECRET=<random secret with at least 32 characters>
AUTH_SESSION_COOKIE_NAME=cpa_usage_session
AUTH_SESSION_COOKIE_DOMAIN=<your-cpa-host>
AUTH_SESSION_COOKIE_PATH=/
```

`AUTH_SESSION_SECRET` protects `cpa-usage` cookies. `CPA_MANAGEMENT_KEY` is what shares the CPA management login state into `cpa-usage`. Both values must be kept out of git. A suitable cookie secret can be generated with:

```bash
openssl rand -base64 48
```

## CPA Usage Compose

The example compose fragment in `deploy/example/cpa-usage.cutover.compose.yml` expects:

```bash
export CPA_USAGE_LOGIN_PASSWORD='<redacted>'
export AUTH_SESSION_SECRET='<redacted>'
export AUTH_SESSION_COOKIE_NAME=cpa_usage_session
export AUTH_SESSION_COOKIE_DOMAIN=<your-cpa-host>
```

`AUTH_SESSION_COOKIE_PATH` is fixed to `/` in that compose file because `/usage` must share the cookie with the root-path CPA service.

## Verification

After deploying, verify both auth paths. First verify `cpa-usage` password-cookie login:

```bash
cookie_jar="$(mktemp)"

curl -fsS -c "$cookie_jar" \
  -H 'Content-Type: application/json' \
  -d '{"password":"<redacted>"}' \
  https://<your-cpa-host>/usage/api/v1/auth/login \
  -o /dev/null

curl -fsS -b "$cookie_jar" \
  https://<your-cpa-host>/usage/api/v1/auth/session
```

Then verify CPA management bearer login is accepted by both CPA and `cpa-usage`:

```bash
management_key='<redacted>'

curl -fsS -H "Authorization: Bearer $management_key" \
  https://<your-cpa-host>/v8/management/config \
  -o /dev/null

curl -fsS -H "Authorization: Bearer $management_key" \
  https://<your-cpa-host>/usage/api/v1/auth/session
```

Expected result: CPA management returns HTTP 200 and the `cpa-usage` session endpoint reports authenticated. This checks server-side bearer acceptance; it does not verify browser storage decoding.

Also verify the actual browser path in a fresh browser context without a Usage cookie: sign in at `https://<your-cpa-host>/management.html` with **Remember password** enabled, then open `/usage` on the same origin. Usage must open the Intelligence dashboard without another password. Its `/usage/api/v1/auth/session` request must carry the shared bearer and report `authenticated: true`; no Usage password-login request should occur. Reload `/usage` and confirm it remains authenticated.

## Compatibility Decision

This is opt-in. Existing deployments without `AUTH_SESSION_SECRET` keep the previous process-local in-memory sessions and cookie path derived from `APP_BASE_PATH`.

When `AUTH_SESSION_SECRET` is enabled, old in-memory `cpa_usage_session` cookies are not accepted. Users may need to sign in once after the rollout. CPA management login sharing is additive: it does not change CPA root routes or CPA management API behavior.

The v2 storage reader is additive. The existing plaintext `managementKey`, plaintext persisted snapshot and user-agent-bound `enc::v1::` reads retain their behavior for already supported deployments. Version prefixes select their exact decoding masks; the reader does not guess a different format after decoding fails. Browser and bearer verification failures continue through the existing unauthenticated/login flow. No new cookie, server session, storage migration or retry path is introduced.

## Security Notes

- Use HTTPS. The service marks the cookie `Secure` when the request reaches it as TLS or with `X-Forwarded-Proto: https`.
- Keep `SameSite=Lax` for same-site navigation.
- Rotate `AUTH_SESSION_SECRET` only during a planned login reset, because all existing signed sessions become invalid.
- Logout clears the browser cookie. A copied signed token remains cryptographically valid until expiry, so keep `AUTH_SESSION_TTL` aligned with the operator risk model.
- Treat the remembered CPA management key in localStorage as an operator secret. `cpa-usage` reads it only from the same origin and sends it as a bearer token to its own APIs.
- Do not set a parent-domain cookie unless another subdomain explicitly needs the same login cookie.
