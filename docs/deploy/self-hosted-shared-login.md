# Self-Hosted Shared Login Configuration

## Goal

Allow a CPA root service on `https://<your-cpa-host>/` and this `cpa-usage` service on `https://<your-cpa-host>/usage` to accept the same browser login session.

## Runtime Contract

CPA's management UI uses the same-origin browser `localStorage` value `managementKey` and sends it to management APIs as `Authorization: Bearer <management key>`. `cpa-usage` accepts that same bearer token for protected APIs, while still keeping its own password-cookie login path.

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
  https://<your-cpa-host>/v0/management/config \
  -o /dev/null

curl -fsS -H "Authorization: Bearer $management_key" \
  https://<your-cpa-host>/usage/api/v1/auth/session
```

Expected result: CPA management returns HTTP 200 and the `cpa-usage` session endpoint reports authenticated. In a browser, signing in to `https://<your-cpa-host>/management.html` should let `/usage` load without a second login because both paths share the same origin localStorage.

## Compatibility Decision

This is opt-in. Existing deployments without `AUTH_SESSION_SECRET` keep the previous process-local in-memory sessions and cookie path derived from `APP_BASE_PATH`.

When `AUTH_SESSION_SECRET` is enabled, old in-memory `cpa_usage_session` cookies are not accepted. Users may need to sign in once after the rollout. CPA management login sharing is additive: it does not change CPA root routes or CPA management API behavior.

## Security Notes

- Use HTTPS. The service marks the cookie `Secure` when the request reaches it as TLS or with `X-Forwarded-Proto: https`.
- Keep `SameSite=Lax` for same-site navigation.
- Rotate `AUTH_SESSION_SECRET` only during a planned login reset, because all existing signed sessions become invalid.
- Logout clears the browser cookie. A copied signed token remains cryptographically valid until expiry, so keep `AUTH_SESSION_TTL` aligned with the operator risk model.
- Treat CPA `managementKey` localStorage as an operator secret. `cpa-usage` reads it only from the same origin and sends it as a bearer token to its own APIs.
- Do not set a parent-domain cookie unless another subdomain explicitly needs the same login cookie.
