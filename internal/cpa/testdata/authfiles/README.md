# Auth-file passive quota fixtures

These synthetic, credential-free contract fixtures are derived from CLIProxyAPI commit `c76dfd4e0edabab9000628b1560ab8ab379eadb8`, specifically `sdk/cliproxy/auth/quota_signals.go`, `internal/runtime/executor/helps/{claude_ratelimit,codex_quota}.go`, and the management auth-files projection.

`v7.2.152-passive-quota.json` covers the supported `quota` / `model_quotas` response shape and observed provider units. `v7.2.156-bengalfox-passive-quota.json` covers the later Codex additional-limit header family `X-Codex-Bengalfox-*` with `Limit-Name=GPT-5.3-Codex-Spark`. `without-passive-quota.json` covers a supported management response with no observed passive quota signal. They contain no production identity, credential, request, provider call or database data.
