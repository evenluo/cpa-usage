import { describe, expect, it } from "vitest"
import type { KeyIdentity, QuotaCacheResponse } from "@/types/api"
import { buildLiveCapacityRows, isSupportedQuotaIdentity } from "./live-capacity"

function identity(overrides: Partial<KeyIdentity>): KeyIdentity {
  return {
    id: 1,
    name: "codex-auth",
    displayName: "Codex Auth",
    alias: "",
    auth_type: 1,
    auth_type_name: "oauth",
    identity: "codex-auth",
    type: "codex",
    provider: "Codex",
    total_tokens: 0,
    total_cost: 0,
    cost_available: false,
    last_used_at: null,
    ...overrides,
  }
}

describe("Live Capacity view model", () => {
  it("recognizes supported provider and type names", () => {
    expect(isSupportedQuotaIdentity(identity({ provider: "Claude", type: "auth-file" }))).toBe(true)
    expect(isSupportedQuotaIdentity(identity({ provider: "Gemini", type: "gemini-cli" }))).toBe(true)
    expect(isSupportedQuotaIdentity(identity({ provider: "OpenAI", type: "openai" }))).toBe(false)
  })

  it("maps 5h and Weekly quota windows from cached probe rows", () => {
    const cache: QuotaCacheResponse = {
      items: [{
        id: "codex-auth",
        quota: [
          { key: "rate_limit.primary_window", label: "5h", usedPercent: 25, resetAfterSeconds: 3600, planType: "plus" },
          { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 80, resetAfterSeconds: 7200, planType: "plus" },
        ],
      }],
    }

    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      cachedQuota: cache,
    })

    expect(rows[0]).toMatchObject({
      authIndex: "codex-auth",
      status: "cached",
      planType: "plus",
      resetLabel: "1h",
      fiveHour: { valueLabel: "25% used", progress: 25, tone: "green" },
      weekly: { valueLabel: "80% used", progress: 80, tone: "amber" },
    })
  })

  it("keeps empty cache explicit without starting a probe", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "claude-auth", provider: "Claude", type: "claude" })],
      cachedQuota: { items: [] },
    })

    expect(rows[0].status).toBe("no_cache")
    expect(rows[0].statusLabel).toBe("No cached probe")
    expect(rows[0].fiveHour).toBeUndefined()
  })

  it("keeps cached metrics visible when a later refresh failed", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      cachedQuota: {
        items: [{
          id: "codex-auth",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 25 }],
        }],
      },
      taskStates: {
        "codex-auth": { status: "failed", error: "API error 500" },
      },
    })

    expect(rows[0].status).toBe("failed")
    expect(rows[0].fiveHour).toMatchObject({ valueLabel: "25% used", progress: 25 })
  })

  it("renders supported non-window quota rows as capacity metrics", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "gemini-auth", provider: "Gemini", type: "gemini-cli" })],
      cachedQuota: {
        items: [{
          id: "gemini-auth",
          quota: [
            {
              key: "bucket.gemini-2.5-pro_vertex.PROMPT",
              label: "gemini-2.5-pro_vertex",
              scope: "model",
              metric: "PROMPT",
              remainingFraction: 0.02,
              resetAt: "2026-05-09T12:00:00Z",
            },
            {
              key: "code_assist.current_tier.GOOGLE_ONE_AI",
              label: "Code Assist Credit",
              scope: "credits",
              metric: "GOOGLE_ONE_AI",
              remaining: 10,
            },
          ],
        }],
      },
    })

    expect(rows[0].status).toBe("cached")
    expect(rows[0].additionalMetrics[0]).toMatchObject({
      label: "gemini-2.5-pro_vertex",
      valueLabel: "2% left",
      progress: 98,
      tone: "red",
    })
    expect(rows[0].additionalMetrics[1]).toMatchObject({
      label: "Code Assist Credit",
      valueLabel: "10 left",
    })
    expect(rows[0].isConstrained).toBe(true)
    expect(rows[0].resetLabel).toContain("May 9")
  })

  it("treats remaining-only zero quota as constrained", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "gemini-auth", provider: "Gemini", type: "gemini-cli" })],
      cachedQuota: {
        items: [{
          id: "gemini-auth",
          quota: [{
            key: "code_assist.current_tier.GOOGLE_ONE_AI",
            label: "Code Assist Credit",
            scope: "credits",
            metric: "GOOGLE_ONE_AI",
            remaining: 0,
          }],
        }],
      },
    })

    expect(rows[0].additionalMetrics[0]).toMatchObject({
      valueLabel: "0 left",
      progress: null,
      tone: "red",
    })
    expect(rows[0].isConstrained).toBe(true)
  })

  it("promotes failed and constrained rows before stable provider/name sorting", () => {
    const cache: QuotaCacheResponse = {
      items: [{
        id: "codex-auth",
        quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 99 }],
      }],
    }
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "z-auth", displayName: "Zulu", provider: "Codex", type: "codex" }),
        identity({ identity: "codex-auth", displayName: "Codex", provider: "Codex", type: "codex" }),
        identity({ identity: "bad-auth", displayName: "Bad", provider: "Claude", type: "claude" }),
      ],
      cachedQuota: cache,
      taskStates: {
        "bad-auth": { status: "failed", error: "unsupported" },
      },
    })

    expect(rows.map((row) => row.authIndex)).toEqual(["bad-auth", "codex-auth", "z-auth"])
    expect(rows[0].statusLabel).toBe("Unsupported")
    expect(rows[1].isConstrained).toBe(true)
  })

  it("lets provider filtering happen before row derivation", () => {
    const all = [
      identity({ identity: "codex-auth", provider: "Codex" }),
      identity({ identity: "claude-auth", provider: "Claude", type: "claude" }),
    ]
    const filtered = all.filter((item) => item.provider === "Claude")

    expect(buildLiveCapacityRows({ identities: filtered }).map((row) => row.provider)).toEqual(["Claude"])
  })
})
