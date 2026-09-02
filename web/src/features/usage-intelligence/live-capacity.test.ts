import { describe, expect, it } from "vitest"
import type { KeyIdentity, QuotaCacheResponse } from "@/types/api"
import {
  buildLiveCapacityRows,
  isSupportedQuotaIdentity,
  mergeLiveCapacityRowOrder,
  orderLiveCapacityRows,
  providerKindFromIdentity,
} from "./live-capacity"

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
    disabled: false,
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
    expect(isSupportedQuotaIdentity(identity({ provider: "Anthropic", type: "auth-file" }))).toBe(false)
    expect(isSupportedQuotaIdentity(identity({ provider: "Google Gemini", type: "auth-file" }))).toBe(false)
    expect(isSupportedQuotaIdentity(identity({ provider: "Moonshot", type: "auth-file" }))).toBe(false)
    expect(isSupportedQuotaIdentity(identity({ provider: "OpenAI", type: "openai" }))).toBe(false)
  })

  it("normalizes official provider kinds and labels for supported capacity accounts", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "codex-auth", provider: "Codex", type: "auth-file" }),
        identity({ identity: "claude-auth", provider: "Claude", type: "auth-file" }),
        identity({ identity: "gemini-auth", provider: "Gemini", type: "gemini-cli" }),
        identity({ identity: "kimi-auth", provider: "Kimi", type: "auth-file" }),
        identity({ identity: "antigravity-auth", provider: "Antigravity", type: "auth-file" }),
      ],
    })

    expect(Object.fromEntries(rows.map((row) => [row.authIndex, [row.providerKind, row.providerLabel]]))).toEqual({
      "antigravity-auth": ["antigravity", "Antigravity"],
      "claude-auth": ["claude", "Claude"],
      "codex-auth": ["codex", "Codex"],
      "gemini-auth": ["gemini-cli", "Gemini CLI"],
      "kimi-auth": ["kimi", "Kimi"],
    })
    expect(providerKindFromIdentity({ provider: "Gemini", type: "gemini-cli" })).toBe("gemini-cli")
  })

  it("keeps brand aliases unsupported when backend registry keys are absent", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "anthropic-auth", provider: "Anthropic", type: "auth-file" }),
        identity({ identity: "google-gemini-auth", provider: "Google Gemini", type: "auth-file" }),
        identity({ identity: "moonshot-auth", provider: "Moonshot", type: "auth-file" }),
      ],
    })

    expect(Object.fromEntries(rows.map((row) => [row.authIndex, {
      providerKind: row.providerKind,
      providerLabel: row.providerLabel,
      status: row.status,
      isPriorityAccount: row.isPriorityAccount,
    }]))).toEqual({
      "anthropic-auth": { providerKind: "unsupported", providerLabel: "Anthropic", status: "unsupported", isPriorityAccount: false },
      "google-gemini-auth": { providerKind: "unsupported", providerLabel: "Google Gemini", status: "unsupported", isPriorityAccount: false },
      "moonshot-auth": { providerKind: "unsupported", providerLabel: "Moonshot", status: "unsupported", isPriorityAccount: false },
    })
  })

  it("maps 5h and Weekly quota windows from cached probe rows", () => {
    const cache: QuotaCacheResponse = {
      items: [{
        id: "codex-auth",
        cachedAt: "2026-08-31T01:00:00Z",
        expiresAt: "2026-08-31T01:05:00Z",
        quota: [
          { key: "rate_limit.primary_window", label: "5h", usedPercent: 25, resetAfterSeconds: 3600, planType: "plus" },
          { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 80, resetAfterSeconds: 7200, planType: "plus" },
        ],
      }],
    }

    const rows = buildLiveCapacityRows({
      identities: [identity({
        identity: "codex-auth",
        active_start: "2026-08-01T00:00:00Z",
        active_until: "2026-09-01T00:00:00Z",
      })],
      cachedQuota: cache,
    })

    expect(rows[0]).toMatchObject({
      authIndex: "codex-auth",
      status: "cached",
      planType: "plus",
      planLabel: "Plus",
      planTone: "ordinary",
      resetLabel: "1h",
      fiveHour: { valueLabel: "25% used", resetLabel: "1h", progress: 25, tone: "green" },
      weekly: { valueLabel: "80% used", resetLabel: "2h", progress: 80, tone: "amber" },
      observedAt: "2026-08-31T01:00:00Z",
      expiresAt: "2026-08-31T01:05:00Z",
      // Past subscription starts are suppressed; only the end date survives.
      activeStart: null,
      activeUntil: "2026-09-01T00:00:00Z",
    })
  })

  it("keeps the subscription start when it is still in the future", () => {
    const futureStart = new Date(Date.now() + 7 * 86_400_000).toISOString()
    const rows = buildLiveCapacityRows({
      identities: [identity({
        identity: "codex-auth",
        active_start: futureStart,
      })],
    })

    expect(rows[0].activeStart).toBe(futureStart)
  })

  it("retains every additional quota row returned by the probe", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      cachedQuota: {
        items: [{
          id: "codex-auth",
          quota: [
            { key: "primary", label: "5h", usedPercent: 25 },
            { key: "secondary", label: "Weekly", usedPercent: 50 },
            { key: "extra-1", label: "Extra 1", usedPercent: 1 },
            { key: "extra-2", label: "Extra 2", usedPercent: 2 },
            { key: "extra-3", label: "Extra 3", usedPercent: 3 },
            { key: "extra-4", label: "Extra 4", usedPercent: 4 },
          ],
        }],
      },
    })

    expect(rows[0].additionalMetrics.map((metric) => metric.label)).toEqual([
      "Extra 1",
      "Extra 2",
      "Extra 3",
      "Extra 4",
    ])
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

  it("builds a disabled row whose status wins over cached quota and task state", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ id: 42, identity: "codex-auth", disabled: true })],
      cachedQuota: {
        items: [{
          id: "codex-auth",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 25 }],
        }],
      },
      taskStates: {
        "codex-auth": { status: "running", taskId: "task-1" },
      },
    })

    expect(rows[0]).toMatchObject({
      id: 42,
      disabled: true,
      status: "disabled",
      statusLabel: "Disabled",
    })
    expect(rows[0].fiveHour).toMatchObject({ valueLabel: "25% used", progress: 25 })
  })

  it("maps the disabled refresh rejection to a Disabled label", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      taskStates: {
        "codex-auth": { status: "failed", error: "disabled" },
      },
    })

    expect(rows[0].status).toBe("failed")
    expect(rows[0].statusLabel).toBe("Disabled")
  })

  it("keeps disabled rows in base business order so the card can sink them at render time", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "beta-codex", displayName: "Beta" }),
        identity({ identity: "alpha-codex", displayName: "Alpha", disabled: true }),
      ],
    })

    expect(rows.map((row) => [row.authIndex, row.status])).toEqual([
      ["alpha-codex", "disabled"],
      ["beta-codex", "no_cache"],
    ])
  })

  it("uses identity plan type for initial priority before quota cache exists", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "plain-codex", displayName: "Codex Team", provider: "Codex", type: "codex", plan_type: "team" }),
        identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex", plan_type: "pro" }),
        identity({ identity: "claude-max", displayName: "Claude Max", provider: "Claude", type: "claude", plan_type: "max20" }),
      ],
      cachedQuota: { items: [] },
    })

    expect(rows.map((row) => row.authIndex)).toEqual(["codex-pro", "claude-max", "plain-codex"])
    expect(rows.map((row) => [row.authIndex, row.planLabel, row.planTone, row.status])).toEqual([
      ["codex-pro", "Pro", "priority", "no_cache"],
      ["claude-max", "Max", "priority", "no_cache"],
      ["plain-codex", "Team", "ordinary", "no_cache"],
    ])
  })

  it("marks priority accounts only from normalized provider kind and plan type", () => {
    const cache: QuotaCacheResponse = {
      items: [
        { id: "codex-pro", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "codex-team", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "claude-max", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "max20" }] },
        { id: "claude-pro", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
      ],
    }

    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "codex-pro", displayName: "Codex Priority", provider: "Codex", type: "codex" }),
        identity({ identity: "codex-team", displayName: "Codex Team", provider: "Codex", type: "codex" }),
        identity({ identity: "claude-max", displayName: "Claude Priority", provider: "Claude", type: "claude" }),
        identity({ identity: "claude-pro", displayName: "Claude Pro", provider: "Claude", type: "claude" }),
      ],
      cachedQuota: cache,
    })

    const priorityByAuthIndex = Object.fromEntries(rows.map((row) => [row.authIndex, {
      isPriorityAccount: row.isPriorityAccount,
      planLabel: row.planLabel,
      planTone: row.planTone,
      priorityLabel: row.priorityLabel,
    }]))
    expect(priorityByAuthIndex).toMatchObject({
      "codex-pro": { isPriorityAccount: true, planLabel: "Pro", planTone: "priority", priorityLabel: "Pro" },
      "codex-team": { isPriorityAccount: false, planLabel: "Team", planTone: "ordinary", priorityLabel: undefined },
      "claude-max": { isPriorityAccount: true, planLabel: "Max", planTone: "priority", priorityLabel: "Max" },
      "claude-pro": { isPriorityAccount: false, planLabel: "Pro", planTone: "ordinary", priorityLabel: undefined },
    })
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

  it("labels a stopped refresh lifecycle as unavailable", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      taskStates: {
        "codex-auth": { status: "failed", error: "refresh_unavailable" },
      },
    })

    expect(rows[0].status).toBe("failed")
    expect(rows[0].statusLabel).toBe("Refresh unavailable")
  })

  it("marks a row refreshing immediately while refresh request is starting", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      cachedQuota: {
        items: [{
          id: "codex-auth",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 25 }],
        }],
      },
      taskStates: {
        "codex-auth": { status: "starting" },
      },
    })

    expect(rows[0].status).toBe("refreshing")
    expect(rows[0].statusLabel).toBe("Starting")
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

  it("keeps failed, refreshing, and constrained states out of ordering priority", () => {
    const cache: QuotaCacheResponse = {
      items: [
        {
          id: "codex-pro",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 99, planType: "pro" }],
        },
        {
          id: "claude-max",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "max" }],
        },
      ],
    }
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "z-codex", displayName: "Zulu Codex", provider: "Codex", type: "codex" }),
        identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
        identity({ identity: "claude-max", displayName: "Claude Max", provider: "Claude", type: "claude" }),
      ],
      cachedQuota: cache,
      taskStates: {
        "z-codex": { status: "failed", error: "API error 500" },
        "claude-max": { status: "running", taskId: "task-claude-max" },
      },
    })

    expect(rows.map((row) => row.authIndex)).toEqual(["codex-pro", "claude-max", "z-codex"])
    expect(rows[0].isConstrained).toBe(true)
    expect(rows[1].status).toBe("refreshing")
    expect(rows[2].status).toBe("failed")
  })

  it("sorts fixed business priority by plan type and supported provider", () => {
    const cache: QuotaCacheResponse = {
      items: [
        {
          id: "plain-codex",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }],
        },
        {
          id: "codex-pro",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }],
        },
        {
          id: "plain-claude",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }],
        },
        {
          id: "claude-max",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "max20" }],
        },
        {
          id: "gemini-auth",
          quota: [{ key: "quota", label: "Code Assist", remaining: 20 }],
        },
      ],
    }

    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "unsupported-auth", displayName: "Unsupported", provider: "OpenAI", type: "openai" }),
        identity({ identity: "gemini-auth", displayName: "Gemini", provider: "Gemini", type: "gemini-cli" }),
        identity({ identity: "plain-codex", displayName: "Codex Team", provider: "Codex", type: "codex" }),
        identity({ identity: "claude-max", displayName: "Claude Max", provider: "Claude", type: "claude" }),
        identity({ identity: "plain-claude", displayName: "Claude Pro Name", provider: "Claude", type: "claude" }),
        identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
      ],
      cachedQuota: cache,
    })

    expect(rows.map((row) => row.authIndex)).toEqual([
      "codex-pro",
      "claude-max",
      "plain-claude",
      "plain-codex",
      "gemini-auth",
      "unsupported-auth",
    ])
  })

  it("keeps the current tile order stable when a single refreshed row changes priority", () => {
    const identities = [
      identity({ identity: "codex-pro", displayName: "Zulu Codex Pro", provider: "Codex", type: "codex" }),
      identity({ identity: "plain-codex", displayName: "Alpha Codex", provider: "Codex", type: "codex" }),
    ]
    const cachedQuota: QuotaCacheResponse = {
      items: [
        {
          id: "codex-pro",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }],
        },
        {
          id: "plain-codex",
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }],
        },
      ],
    }
    const initialRows = buildLiveCapacityRows({ identities, cachedQuota })
    const currentOrder = mergeLiveCapacityRowOrder([], initialRows)

    const refreshedRows = buildLiveCapacityRows({
      identities,
      cachedQuota,
      taskStates: {
        "plain-codex": {
          status: "completed",
          taskId: "task-plain-codex",
          quota: {
            id: "plain-codex",
            quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        },
      },
    })

    expect(initialRows.map((row) => row.authIndex)).toEqual(["codex-pro", "plain-codex"])
    expect(refreshedRows.map((row) => row.authIndex)).toEqual(["plain-codex", "codex-pro"])
    expect(orderLiveCapacityRows(refreshedRows, currentOrder).map((row) => row.authIndex)).toEqual(["codex-pro", "plain-codex"])
  })

  it("appends new live capacity rows after the stable current order", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex" }),
        identity({ identity: "plain-codex", displayName: "Codex Team", provider: "Codex", type: "codex" }),
      ],
      cachedQuota: {
        items: [
          { id: "codex-pro", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
          { id: "plain-codex", quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        ],
      },
    })

    expect(mergeLiveCapacityRowOrder(["plain-codex"], rows)).toEqual(["plain-codex", "codex-pro"])
  })

  it("formats metric reset per quota row and keeps missing reset explicit", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      cachedQuota: {
        items: [{
          id: "codex-auth",
          quota: [
            { key: "rate_limit.primary_window", label: "5h", usedPercent: 25, resetAfterSeconds: 3600 },
            { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 80 },
          ],
        }],
      },
    })

    expect(rows[0].fiveHour?.resetLabel).toBe("1h")
    expect(rows[0].weekly?.resetLabel).toBe("-")
    expect(rows[0].resetLabel).toBe("1h")
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
