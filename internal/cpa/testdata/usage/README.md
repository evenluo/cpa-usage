# Pinned CPA queue fixtures

These are synthetic producer-shaped messages, not production captures. Identity
values and excluded-field markers are invented. Fixtures pin the JSON layout and
arithmetic of these official sources:

- v7.2.62: `3554b63721aac9b4202bf2ef88ba7a82b4e5caf8`,
  [queue producer](https://github.com/router-for-me/CLIProxyAPI/blob/3554b63721aac9b4202bf2ef88ba7a82b4e5caf8/internal/redisqueue/plugin.go).
- v7.2.152: `c76dfd4e0edabab9000628b1560ab8ab379eadb8`,
  [queue producer](https://github.com/router-for-me/CLIProxyAPI/blob/c76dfd4e0edabab9000628b1560ab8ab379eadb8/internal/redisqueue/plugin.go),
  [canonical accounting](https://github.com/router-for-me/CLIProxyAPI/blob/c76dfd4e0edabab9000628b1560ab8ab379eadb8/sdk/cliproxy/usage/accounting.go),
  [producer tests](https://github.com/router-for-me/CLIProxyAPI/blob/c76dfd4e0edabab9000628b1560ab8ab379eadb8/internal/redisqueue/plugin_test.go).

The accounting schema is the v7.2.151 contract retained in v7.2.152. The latter
adds client/session fields which deliberately do not pass our allowlist.

| Fixture | Expected canonical interpretation | Execution |
| --- | --- | --- |
| `v7.2.62-legacy.json` | absent; no reconstruction from legacy tokens | flags/response tier absent |
| `v7.2.152-complete.json` | complete subset: input 100, output 30, total 130 | generating stream; requested auto / response default |
| `v7.2.152-separate-reasoning.json` | complete: output 30 + reasoning 12 = 42, total 142 | generating stream; existing TPS numerator 30 |
| `v7.2.152-independent.json` | complete Anthropic parser: input 100 + read 40 + write 10 = 150; output 30 includes reasoning 12; total 180 | generating stream |
| `v7.2.152-inconsistent.json` | structurally valid; total 160 entirely unclassified, quality inconsistent | TPS unavailable |
| `v7.2.152-unclassified.json` | structurally valid partial subset; 30 unclassified, total 160 | TPS unavailable |

Consumers C/G can decode these with `service.DecodeRedisUsageMessage`, persist
via `repository.InsertUsageEvents`, and read `repository.InterpretUsageAttempt`
or `UsageEventRecord.AttemptFacts`. OpenAI, Anthropic, and Gemini fixtures all have positive reasoning. Existing TPS
uses their unchanged legacy output scalar (30), even though Gemini canonical
output is 42. Explicit false/absent flags are covered by mutation tests.
Use the invalid/missing/version mutations in
`internal/service/redis_usage_accounting_test.go` for qualified negative cases.
No fixture authorizes a probe, a producer upgrade, or a billing interpretation.
