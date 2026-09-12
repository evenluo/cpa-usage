import { describe, expect, it } from "vitest"
import type { KeyIdentity, QuotaObservationsResponse } from "@/types/api"
import {
  buildLiveCapacityRows,
  capacityLayout,
  isSupportedQuotaIdentity,
  mergeCapacityEntries,
  mergeLiveCapacityRowOrder,
  orderLiveCapacityRows,
  providerKindFromIdentity,
  resetCountdown,
} from "./live-capacity"

const OBSERVED_AT = "2026-09-07T09:00:00Z"

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
    canonical_valid_attempts: 0,
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

  it("maps 5h and Weekly quota windows from manual observations", () => {
    const observationSnapshot: QuotaObservationsResponse = {
      items: [{
        id: "codex-auth",
        observedAt: "2026-08-31T01:00:00Z",

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
      observations: observationSnapshot,
    })

    expect(rows[0]).toMatchObject({
      authIndex: "codex-auth",
      status: "observed",
      planType: "plus",
      planLabel: "Plus",
      planTone: "ordinary",
      fiveHour: { valueLabel: "25% used", resetAfterSeconds: 3600, progress: 25, tone: "green" },
      weekly: { valueLabel: "80% used", resetAfterSeconds: 7200, progress: 80, tone: "amber" },
      observedAt: "2026-08-31T01:00:00Z",

      // Stale subscription metadata is suppressed: past starts and past ends
      // are both display noise.
      activeStart: null,
      activeUntil: null,
    })
  })

  it("prefers window.seconds over label-only rows regardless of array position", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "legacy_5h", label: "5h", usedPercent: 10 },
            { key: "rate_limit.primary_window", label: "Codex 5h", usedPercent: 25, window: { seconds: 18_000 } },
            { key: "legacy_weekly", label: "Weekly", usedPercent: 20 },
            { key: "rate_limit.secondary_window", label: "Spark Weekly", usedPercent: 80, window: { seconds: 604_800 } },
          ],
        }],
      },
    })

    expect(rows[0].fiveHour).toMatchObject({ label: "Codex 5h", valueLabel: "25% used", windowSeconds: 18_000 })
    expect(rows[0].weekly).toMatchObject({ label: "Spark Weekly", valueLabel: "80% used", windowSeconds: 604_800 })
  })

  it("does not put Spark in the primary 5h meter when rate_limit is Weekly-only", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "rate_limit.primary_window", label: "Weekly", scope: "window", usedPercent: 57, window: { seconds: 604_800 } },
            { key: "additional_rate_limits.GPT-5.3-Codex-Spark.primary_window", label: "GPT-5.3-Codex-Spark 5h", scope: "additional", metric: "codex_bengalfox", usedPercent: 0, window: { seconds: 18_000 } },
            { key: "additional_rate_limits.GPT-5.3-Codex-Spark.secondary_window", label: "GPT-5.3-Codex-Spark Weekly", scope: "additional", metric: "codex_bengalfox", usedPercent: 0, window: { seconds: 604_800 } },
            { key: "additional_rate_limits.gpt-reserve.primary_window", label: "gpt-reserve Weekly", scope: "additional", metric: "base_model_inference", usedPercent: 0, window: { seconds: 604_800 } },
          ],
        }],
      },
    })

    expect(rows[0].fiveHour).toBeUndefined()
    expect(rows[0].weekly).toMatchObject({ label: "Weekly", valueLabel: "57% used", windowSeconds: 604_800 })
    expect(rows[0].additionalMetrics.map((metric) => metric.label)).toEqual([
      "GPT-5.3-Codex-Spark 5h",
      "GPT-5.3-Codex-Spark Weekly",
      "gpt-reserve Weekly",
    ])

    const layout = capacityLayout(mergeCapacityEntries(rows[0]), "codex")
    expect(layout.main.map((entry) => entry.metric.label)).toEqual(["Weekly"])
    expect(layout.extras.map((entry) => entry.metric.label)).toContain("GPT-5.3-Codex-Spark 5h")
    expect(layout.extras.map((entry) => entry.metric.label)).toContain("GPT-5.3-Codex-Spark Weekly")
    expect(layout.main.some((entry) => entry.metric.label.includes("Spark"))).toBe(false)
  })

  it("derives metric window seconds from Kimi duration+unit windows", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "kimi-auth", provider: "Kimi", type: "kimi" })],
      observations: {
        items: [{
          id: "kimi-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "limits.hourly", label: "Hourly quota", usedPercent: 10, window: { duration: 5, unit: "hour" } },
            { key: "limits.weekly", label: "Weekly quota", usedPercent: 20, window: { duration: 7, unit: "day" } },
            { key: "limits.custom", label: "Custom quota", usedPercent: 30, window: { duration: 2, unit: "fortnight" } },
          ],
        }],
      },
    })

    expect(rows[0].weekly).toMatchObject({ label: "Weekly quota", windowSeconds: 604_800 })
    expect(rows[0].additionalMetrics.map((metric) => metric.windowSeconds)).toEqual([18_000, undefined])
  })

  it("slots normalized Kimi summary and limits rows into Weekly and 5h", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "kimi-auth", provider: "Kimi", type: "kimi" })],
      observations: {
        items: [{
          id: "kimi-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "usage", label: "Weekly", scope: "summary", usedPercent: 10.4, window: { seconds: 604_800 } },
            { key: "limits.0", label: "5h", usedPercent: 69.5, window: { duration: 300, unit: "minute", seconds: 18_000 } },
          ],
        }],
      },
    })

    expect(rows[0].fiveHour).toMatchObject({ label: "5h", valueLabel: "70% used", windowSeconds: 18_000 })
    expect(rows[0].weekly).toMatchObject({ label: "Weekly", valueLabel: "10% used", windowSeconds: 604_800 })
    expect(rows[0].additionalMetrics).toEqual([])
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
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
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

  it("keeps missing observations explicit without starting a probe", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "claude-auth", provider: "Claude", type: "claude" })],
      observations: { items: [] },
    })

    expect(rows[0].status).toBe("no_observation")
    expect(rows[0].errorLabel).toBeUndefined()
    expect(rows[0].fiveHour).toBeUndefined()
  })

  it("keeps account availability evidence separate from quota-probe state", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({
        status: "error",
        unavailable: true,
        metadata_observed_at: "2026-09-07T08:00:00Z",
        last_refresh: "2026-09-07T07:45:00Z",
        next_retry_after: "2026-09-07T08:30:00Z",
      })],
      observations: { items: [] },
    })

    expect(rows[0]).toMatchObject({
      status: "no_observation",
      unavailable: true,
      accountState: { kind: "error", label: "Error", tone: "red" },
      metadataObservedAt: "2026-09-07T08:00:00Z",
      lastRefresh: "2026-09-07T07:45:00Z",
      nextRetryAfter: "2026-09-07T08:30:00Z",
    })
  })

  it("keeps passive CPA observations separate from manual probe state and plan", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({
        identity: "codex-auth",
        plan_type: "team",
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          active_limit: "codex_bengalfox",
          quota: [
            { key: "codex.rate_limit.primary", label: "5h", usedPercent: 99, planType: "pro", window: { seconds: 18_000 } },
          ],
        },
        passive_model_quotas: [
          {
            source: "cpa_passive",
            scope: "model",
            model: "gpt-5.3-codex",
            observed_at: "2026-09-07T07:30:00Z",
            quota: [
              { key: "codex.model.secondary", label: "Weekly", usedPercent: 80, window: { seconds: 604_800 } },
            ],
          },
        ],
      })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: "2026-09-07T09:00:00Z",

          quota: [{ key: "manual", label: "5h", usedPercent: 10, planType: "team" }],
        }],
      },
    })

    expect(rows[0]).toMatchObject({
      planType: "team",
      planLabel: "Team",
      isPriorityAccount: false,
      isConstrained: false,
      observedAt: "2026-09-07T09:00:00Z",

      fiveHour: { valueLabel: "10% used" },
      passiveQuota: {
        source: "cpa_passive",
        observedAt: "2026-09-07T08:00:00Z",
        activeLimit: "codex_bengalfox",
        metrics: [{ label: "5h", valueLabel: "99% used" }],
      },
      passiveModelQuotas: [{
        model: "gpt-5.3-codex",
        observedAt: "2026-09-07T07:30:00Z",
        metrics: [{ label: "Weekly", valueLabel: "80% used" }],
      }],
    })
  })

  it("drops malformed or unsupported passive projections without inventing zero state", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({
          identity: "unsupported",
          provider: "Gemini",
          type: "gemini-cli",
          passive_quota: { source: "cpa_passive", scope: "account", observed_at: "2026-09-07T08:00:00Z", quota: [{ key: "bad", label: "Bad", usedPercent: 0 }] },
        }),
        identity({
          identity: "malformed",
          passive_quota: { source: "cpa_passive", scope: "account", observed_at: "bad-time", quota: [{ key: "bad", label: "Bad", usedPercent: 0 }] },
          passive_model_quotas: [],
        }),
      ],
    })

    expect(rows.map((row) => [row.authIndex, row.passiveQuota, row.passiveModelQuotas])).toEqual([
      ["malformed", undefined, []],
      ["unsupported", undefined, []],
    ])
  })

  it("preserves standalone limit state, zero-second reset and active-limit observations", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          active_limit: "codex_bengalfox",
          quota: [
            { key: "state", label: "Limit state", limitReached: true },
            { key: "reset", label: "Retry hint", resetAfterSeconds: 0 },
          ],
        },
        passive_model_quotas: [{
          source: "cpa_passive",
          scope: "model",
          model: "gpt-5.3-codex",
          observed_at: "2026-09-07T07:30:00Z",
          active_limit: "model_limit",
          quota: [],
        }],
      })],
    })

    expect(rows[0].passiveQuota).toMatchObject({
      activeLimit: "codex_bengalfox",
      metrics: [
        { valueLabel: "Limit reached", tone: "red" },
        { resetAfterSeconds: 0 },
      ],
    })
    expect(rows[0].passiveModelQuotas).toEqual([{
      source: "cpa_passive",
      model: "gpt-5.3-codex",
      observedAt: "2026-09-07T07:30:00Z",
      activeLimit: "model_limit",
      metrics: [],
    }])
  })

  it("keeps numeric limit state visible and does not mark unlimited credits exhausted", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          quota: [
            { key: "window", label: "Weekly", usedPercent: 53, allowed: false, limitReached: true },
            { key: "credits", label: "Credits", remaining: 0, unit: "credits", hasCredits: false, unlimited: true },
          ],
        },
      })],
    })

    expect(rows[0].passiveQuota?.metrics).toEqual([
      expect.objectContaining({ valueLabel: "53% used · Blocked", tone: "red" }),
      expect.objectContaining({ valueLabel: "No credits", tone: "muted" }),
    ])
  })

  it("keeps missing account state and availability explicit", () => {
    const rows = buildLiveCapacityRows({ identities: [identity({ status: undefined, unavailable: undefined })] })

    expect(rows[0].unavailable).toBeNull()
    expect(rows[0].accountState).toMatchObject({ kind: "not_reported", label: "State not reported", tone: "muted" })
    expect(rows[0].accountState.kind).not.toBe("active")
  })

  it.each([
    ["active", "Active", "green"],
    ["pending", "Pending", "amber"],
    ["refreshing", "Refreshing", "amber"],
    ["error", "Error", "red"],
    ["disabled", "Disabled state", "amber"],
    ["unknown", "Unknown", "muted"],
    ["other", "Other state", "muted"],
  ] as const)("maps the bounded %s account status without provider text", (status, label, tone) => {
    const rows = buildLiveCapacityRows({ identities: [identity({ status })] })
    expect(rows[0].accountState).toMatchObject({ kind: status, label, tone })
  })

  it("shows quota refresh progress independently of the disabled account state", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ id: 42, identity: "codex-auth", disabled: true, unavailable: true, status: "error" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
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
      unavailable: true,
      accountState: { kind: "error" },
      status: "refreshing",
    })
    expect(rows[0].fiveHour).toMatchObject({ valueLabel: "25% used", progress: 25 })
  })

  it("shows a disabled account's refresh failure while retaining its reading", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth", disabled: true })],
      observations: {
        items: [{ id: "codex-auth", observedAt: OBSERVED_AT, quota: [{ key: "primary", label: "5h", usedPercent: 25 }] }],
      },
      taskStates: {
        "codex-auth": { status: "failed", error: "HTTP 401" },
      },
    })

    expect(rows[0].status).toBe("failed")
    expect(rows[0].disabled).toBe(true)
    expect(rows[0].errorLabel).toBe("HTTP 401")
    expect(rows[0].fiveHour).toMatchObject({ valueLabel: "25% used", progress: 25 })
    expect(rows[0].observedAt).toBe(OBSERVED_AT)
  })

  it("keeps disabled rows in base business order so the card can sink them at render time", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "beta-codex", displayName: "Beta" }),
        identity({ identity: "alpha-codex", displayName: "Alpha", disabled: true }),
      ],
    })

    expect(rows.map((row) => [row.authIndex, row.status])).toEqual([
      ["alpha-codex", "no_observation"],
      ["beta-codex", "no_observation"],
    ])
  })

  it("uses identity plan type for initial priority before a quota observation exists", () => {
    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "plain-codex", displayName: "Codex Team", provider: "Codex", type: "codex", plan_type: "team" }),
        identity({ identity: "codex-pro", displayName: "Codex Pro", provider: "Codex", type: "codex", plan_type: "pro" }),
        identity({ identity: "claude-max", displayName: "Claude Max", provider: "Claude", type: "claude", plan_type: "max20" }),
      ],
      observations: { items: [] },
    })

    expect(rows.map((row) => row.authIndex)).toEqual(["codex-pro", "claude-max", "plain-codex"])
    expect(rows.map((row) => [row.authIndex, row.planLabel, row.planTone, row.status])).toEqual([
      ["codex-pro", "Pro", "priority", "no_observation"],
      ["claude-max", "Max", "priority", "no_observation"],
      ["plain-codex", "Team", "ordinary", "no_observation"],
    ])
  })

  it("marks priority accounts only from normalized provider kind and plan type", () => {
    const observationSnapshot: QuotaObservationsResponse = {
      items: [
        { id: "codex-pro", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
        { id: "codex-team", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        { id: "claude-max", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "max20" }] },
        { id: "claude-pro", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
      ],
    }

    const rows = buildLiveCapacityRows({
      identities: [
        identity({ identity: "codex-pro", displayName: "Codex Priority", provider: "Codex", type: "codex" }),
        identity({ identity: "codex-team", displayName: "Codex Team", provider: "Codex", type: "codex" }),
        identity({ identity: "claude-max", displayName: "Claude Priority", provider: "Claude", type: "claude" }),
        identity({ identity: "claude-pro", displayName: "Claude Pro", provider: "Claude", type: "claude" }),
      ],
      observations: observationSnapshot,
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

  it("keeps the last successful metrics visible when a later refresh failed", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
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
    expect(rows[0].errorLabel).toBe("Refresh unavailable")
  })

  it("marks a row refreshing immediately while refresh request is starting", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 25 }],
        }],
      },
      taskStates: {
        "codex-auth": { status: "starting" },
      },
    })

    expect(rows[0].status).toBe("refreshing")
    expect(rows[0].errorLabel).toBeUndefined()
    expect(rows[0].fiveHour).toMatchObject({ valueLabel: "25% used", progress: 25 })
  })

  it("renders supported non-window quota rows as capacity metrics", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "gemini-auth", provider: "Gemini", type: "gemini-cli" })],
      observations: {
        items: [{
          id: "gemini-auth",
          observedAt: OBSERVED_AT,
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

    expect(rows[0].status).toBe("observed")
    expect(rows[0].additionalMetrics[0]).toMatchObject({
      label: "gemini-2.5-pro_vertex",
      valueLabel: "98% used",
      progress: 98,
      tone: "red",
    })
    expect(rows[0].additionalMetrics[1]).toMatchObject({
      label: "Code Assist Credit",
      valueLabel: "10 left",
    })
    expect(rows[0].isConstrained).toBe(true)
    expect(rows[0].additionalMetrics[0].resetAt).toBe("2026-05-09T12:00:00Z")
  })

  it("treats remaining-only zero quota as constrained", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "gemini-auth", provider: "Gemini", type: "gemini-cli" })],
      observations: {
        items: [{
          id: "gemini-auth",
          observedAt: OBSERVED_AT,
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
    const observationSnapshot: QuotaObservationsResponse = {
      items: [
        {
          id: "codex-pro",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 99, planType: "pro" }],
        },
        {
          id: "claude-max",
          observedAt: OBSERVED_AT,
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
      observations: observationSnapshot,
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
    const observationSnapshot: QuotaObservationsResponse = {
      items: [
        {
          id: "plain-codex",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }],
        },
        {
          id: "codex-pro",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }],
        },
        {
          id: "plain-claude",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }],
        },
        {
          id: "claude-max",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "max20" }],
        },
        {
          id: "gemini-auth",
          observedAt: OBSERVED_AT,
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
      observations: observationSnapshot,
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
    const observations: QuotaObservationsResponse = {
      items: [
        {
          id: "codex-pro",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "pro" }],
        },
        {
          id: "plain-codex",
          observedAt: OBSERVED_AT,
          quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 10, planType: "team" }],
        },
      ],
    }
    const initialRows = buildLiveCapacityRows({ identities, observations })
    const currentOrder = mergeLiveCapacityRowOrder([], initialRows)

    const refreshedRows = buildLiveCapacityRows({
      identities,
      observations: {
        items: [
          ...observations.items.filter((observation) => observation.id !== "plain-codex"),
          {
            id: "plain-codex",
            observedAt: "2026-09-07T10:00:00Z",
            quota: [{ key: "rate_limit.primary_window", label: "5h", usedPercent: 20, planType: "pro" }],
          },
        ],
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
      observations: {
        items: [
          { id: "codex-pro", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "pro" }] },
          { id: "plain-codex", observedAt: OBSERVED_AT, quota: [{ key: "quota", label: "5h", usedPercent: 10, planType: "team" }] },
        ],
      },
    })

    expect(mergeLiveCapacityRowOrder(["plain-codex"], rows)).toEqual(["plain-codex", "codex-pro"])
  })

  it("passes metric reset hints through per quota row and keeps missing reset explicit", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "rate_limit.primary_window", label: "5h", usedPercent: 25, resetAfterSeconds: 3600 },
            { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 80 },
          ],
        }],
      },
    })

    expect(rows[0].fiveHour?.resetAfterSeconds).toBe(3600)
    expect(rows[0].weekly?.resetAfterSeconds).toBeUndefined()
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

describe("resetCountdown", () => {
  const now = new Date("2026-09-07T12:00:00Z").getTime()

  it("derives an absolute reset instant from resetAt", () => {
    expect(resetCountdown({ resetAt: "2026-09-07T14:13:00Z" }, undefined, now)).toEqual({
      relativeLabel: "2h 13m",
      isDue: false,
      resetAt: "2026-09-07T14:13:00.000Z",
    })
  })

  it("anchors resetAfterSeconds to the observation time, not to now", () => {
    expect(resetCountdown({ resetAfterSeconds: 120 }, "2026-09-07T11:59:00Z", now)).toEqual({
      relativeLabel: "1m",
      isDue: false,
      resetAt: "2026-09-07T12:01:00.000Z",
    })
  })

  it("reports a past reset as due", () => {
    expect(resetCountdown({ resetAt: "2026-09-07T11:00:00Z" }, undefined, now)).toMatchObject({ relativeLabel: "due", isDue: true })
  })

  it("stays unavailable when no usable reset hint exists", () => {
    expect(resetCountdown({}, "2026-09-07T11:59:00Z", now)).toBeUndefined()
    expect(resetCountdown({ resetAfterSeconds: 120 }, undefined, now)).toBeUndefined()
  })
})

describe("metric reset pass-through", () => {
  it("keeps resetAt and resetAfterSeconds on metrics for countdown rendering", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "rate_limit.primary_window", label: "5h", usedPercent: 25, resetAt: "2026-09-07T14:13:00Z", window: { seconds: 18_000 } },
            { key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 80, resetAfterSeconds: 7200, window: { seconds: 604_800 } },
          ],
        }],
      },
    })

    expect(rows[0].fiveHour?.resetAt).toBe("2026-09-07T14:13:00Z")
    expect(rows[0].weekly?.resetAfterSeconds).toBe(7200)
  })
})

describe("mergeCapacityEntries", () => {
  function rowWithProbeAndPassive(input: {
    probeQuota: QuotaObservationsResponse["items"][number]["quota"]
    probeObservedAt: string
    passiveQuota?: KeyIdentity["passive_quota"]
    passiveModelQuotas?: KeyIdentity["passive_model_quotas"]
  }) {
    return buildLiveCapacityRows({
      identities: [identity({
        passive_quota: input.passiveQuota ?? null,
        passive_model_quotas: input.passiveModelQuotas ?? null,
      })],
      observations: { items: [{ id: "codex-auth", observedAt: input.probeObservedAt, quota: input.probeQuota }] },
    })[0]
  }

  it("keeps the newer probe reading when both sources report the same base window", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-07T09:00:00Z",
      probeQuota: [{ key: "manual", label: "5h", usedPercent: 10 }],
      passiveQuota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T08:00:00Z",
        quota: [{ key: "passive", label: "5h", usedPercent: 99, window: { seconds: 18_000 } }],
      },
    })

    const entries = mergeCapacityEntries(row)
    expect(entries).toHaveLength(1)
    expect(entries[0]).toMatchObject({ source: "probe", observedAt: "2026-09-07T09:00:00Z", isBaseWindow: true, windowRole: "short" })
    expect(entries[0].metric.valueLabel).toBe("10% used")
  })

  it("lets a newer reported reading replace an older probe reading of the same window", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-07T07:00:00Z",
      probeQuota: [{ key: "manual", label: "Weekly", usedPercent: 10, window: { seconds: 604_800 } }],
      passiveQuota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T08:00:00Z",
        quota: [{ key: "passive", label: "Weekly", usedPercent: 80 }],
      },
    })

    const entries = mergeCapacityEntries(row)
    expect(entries).toHaveLength(1)
    expect(entries[0]).toMatchObject({ source: "reported", observedAt: "2026-09-07T08:00:00Z", isBaseWindow: true, windowRole: "long" })
    expect(entries[0].metric.valueLabel).toBe("80% used")
  })

  it("prefers the probe reading when both sources share an observation time", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-07T08:00:00Z",
      probeQuota: [{ key: "manual", label: "5h", usedPercent: 10, window: { seconds: 18_000 } }],
      passiveQuota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T08:00:00Z",
        quota: [{ key: "passive", label: "5h", usedPercent: 99, window: { seconds: 18_000 } }],
      },
    })

    const entries = mergeCapacityEntries(row)
    expect(entries).toHaveLength(1)
    expect(entries[0].source).toBe("probe")
  })

  it("never merges an unknown-window row into a short or long window row", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-07T09:00:00Z",
      probeQuota: [{ key: "manual", label: "5h", usedPercent: 10 }],
      passiveQuota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T08:00:00Z",
        quota: [{ key: "passive", label: "Window", usedPercent: 40 }],
      },
    })

    expect(mergeCapacityEntries(row).map((entry) => [entry.metric.label, entry.source, entry.windowRole])).toEqual([
      ["5h", "probe", "short"],
      ["Window", "reported", "unknown"],
    ])
  })

  it("passes rows without window semantics through unmerged", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-07T09:00:00Z",
      probeQuota: [{ key: "credits", label: "Credits", remaining: 5, unit: "credits" }],
      passiveQuota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T08:00:00Z",
        quota: [{ key: "credits", label: "Credits", remaining: 0, hasCredits: false }],
      },
    })

    const entries = mergeCapacityEntries(row)
    expect(entries.map((entry) => [entry.source, entry.metric.valueLabel, entry.windowRole])).toEqual([
      ["probe", "5 credits left", null],
      ["reported", "No credits", null],
    ])
  })

  it("classifies named additional limits as non-base and merges matching label stems", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-07T09:00:00Z",
      probeQuota: [
        { key: "primary", label: "5h", usedPercent: 10, window: { seconds: 18_000 } },
        { key: "spark-primary", label: "GPT-5.3-Codex-Spark 5h", usedPercent: 20 },
      ],
      passiveQuota: {
        source: "cpa_passive",
        scope: "account",
        observed_at: "2026-09-07T09:30:00Z",
        quota: [{ key: "spark", label: "GPT-5.3-Codex-Spark 5h", usedPercent: 25, window: { seconds: 18_000 } }],
      },
    })

    const entries = mergeCapacityEntries(row)
    expect(entries).toHaveLength(2)
    expect(entries[0]).toMatchObject({ source: "probe", isBaseWindow: true })
    expect(entries[1]).toMatchObject({ source: "reported", isBaseWindow: false, windowRole: "short" })
    expect(entries[1].metric.valueLabel).toBe("25% used")
  })

  it("surfaces only exact account-level gpt-reserve readings as Luna Reserve", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-11T06:00:00Z",
      probeQuota: [
        { key: "rate_limit.primary_window", label: "Weekly", usedPercent: 100, window: { seconds: 604_800 } },
        { key: "additional_rate_limits.gpt-reserve.secondary_window", label: "gpt-reserve Weekly", metric: "base_model_inference", usedPercent: 10, window: { seconds: 604_800 } },
        { key: "additional_rate_limits.other.secondary_window", label: "other Weekly", metric: "base_model_inference", usedPercent: 30, window: { seconds: 604_800 } },
      ],
      passiveQuota: {
        source: "cpa_passive", scope: "account", observed_at: "2026-09-11T07:00:00Z",
        quota: [{ key: "codex.additional-gpt-reserve.secondary", label: "gpt-reserve Weekly", metric: "gpt-reserve", usedPercent: 12, window: { seconds: 604_800 } }],
      },
    })

    const entries = mergeCapacityEntries(row)
    expect(entries).toHaveLength(3)
    const layout = capacityLayout(entries, "codex")
    expect(layout.hasWindowSkeleton).toBe(false)
    expect(layout.main.map((entry) => entry.metric.valueLabel)).toEqual(["100% used"])
    expect(layout.reserve).toHaveLength(1)
    expect(layout.reserve?.[0]).toMatchObject({
      source: "reported", observedAt: "2026-09-11T07:00:00Z",
      metric: { displayLabel: "Luna Reserve Weekly", valueLabel: "12% used" },
    })
    expect(layout.extras.map((entry) => entry.metric.label)).toEqual(["other Weekly"])
    expect(layout.extras[0].metric.displayLabel).toBeUndefined()
    expect(layout.extras[0].metric.description).toBeUndefined()
  })

  it("identifies Luna Reserve from gpt-reserve key or metric rather than the label", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({ identity: "codex-auth" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: OBSERVED_AT,
          quota: [
            { key: "additional_rate_limits.gpt-reserve.primary_window", label: "Gpt Reserve Weekly", scope: "additional", metric: "base_model_inference", usedPercent: 0, window: { seconds: 604_800 } },
          ],
        }],
      },
    })
    const probeLayout = capacityLayout(mergeCapacityEntries(rows[0]), "codex")
    expect(probeLayout.reserve).toHaveLength(1)
    expect(probeLayout.reserve[0].metric).toMatchObject({
      isLunaReserve: true,
      displayLabel: "Luna Reserve Weekly",
      valueLabel: "0% used",
    })

    const passiveRows = buildLiveCapacityRows({
      identities: [identity({
        identity: "codex-auth",
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: OBSERVED_AT,
          quota: [{ key: "codex.additional-gpt-reserve.primary", label: "Gpt Reserve Weekly", usedPercent: 12, window: { seconds: 604_800 } }],
        },
      })],
    })
    const passiveLayout = capacityLayout(mergeCapacityEntries(passiveRows[0]), "codex")
    expect(passiveLayout.reserve).toHaveLength(1)
    expect(passiveLayout.reserve[0].metric).toMatchObject({
      isLunaReserve: true,
      displayLabel: "Luna Reserve Weekly",
      valueLabel: "12% used",
    })
  })

  it("does not promote a Codex model snapshot into an account Reserve reading", () => {
    const row = rowWithProbeAndPassive({
      probeObservedAt: "2026-09-11T06:00:00Z",
      probeQuota: [{ key: "rate_limit.secondary_window", label: "Weekly", usedPercent: 86, window: { seconds: 604_800 } }],
      passiveModelQuotas: [{
        source: "cpa_passive",
        scope: "model",
        model: "gpt-5.6-terra",
        observed_at: "2026-09-11T08:00:00Z",
        quota: [{ key: "additional_rate_limits.gpt-reserve.secondary_window", label: "gpt-reserve Weekly", usedPercent: 12, window: { seconds: 604_800 } }],
      }],
    })

    expect(row.passiveModelQuotas[0].metrics[0]).toMatchObject({ label: "gpt-reserve Weekly", displayLabel: "Luna Reserve Weekly" })
    expect(capacityLayout(mergeCapacityEntries(row), "codex").reserve).toEqual([])
  })

  it("omits the state suffix for healthy allowed and not-limit-reached rows", () => {
    const rows = buildLiveCapacityRows({
      identities: [identity({
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          quota: [
            { key: "a", label: "5h", usedPercent: 10, allowed: true, window: { seconds: 18_000 } },
            { key: "b", label: "Weekly", usedPercent: 20, limitReached: false, window: { seconds: 604_800 } },
          ],
        },
      })],
    })

    expect(rows[0].passiveQuota?.metrics.map((metric) => metric.valueLabel)).toEqual(["10% used", "20% used"])
  })
})

describe("capacityLayout", () => {
  it("surfaces only observed Codex base windows and folds its other quota entries", () => {
    const row = buildLiveCapacityRows({
      identities: [identity({
        passive_quota: {
          source: "cpa_passive",
          scope: "account",
          observed_at: "2026-09-07T08:00:00Z",
          quota: [
            { key: "window", label: "Window", usedPercent: 40 },
            { key: "credits", label: "Credits", remaining: 0, hasCredits: false },
          ],
        },
      })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: "2026-09-07T09:00:00Z",
          quota: [
            { key: "primary", label: "5h", usedPercent: 10, window: { seconds: 18_000 } },
            { key: "secondary", label: "Weekly", usedPercent: 20, window: { seconds: 604_800 } },
            { key: "spark", label: "GPT-5.3-Codex-Spark 5h", usedPercent: 30 },
          ],
        }],
      },
    })[0]

    const layout = capacityLayout(mergeCapacityEntries(row), "codex")
    expect(layout.hasWindowSkeleton).toBe(false)
    expect(layout.baseShort).toBeUndefined()
    expect(layout.baseLong).toBeUndefined()
    expect(layout.main.map((entry) => entry.metric.label)).toEqual(["5h", "Weekly"])
    expect(layout.extras.map((entry) => entry.metric.label)).toEqual(["GPT-5.3-Codex-Spark 5h", "Window", "Credits"])
  })

  it("keeps the flat surface split for providers without a window skeleton", () => {
    const row = buildLiveCapacityRows({
      identities: [identity({ provider: "Kimi", type: "kimi" })],
      observations: {
        items: [{
          id: "codex-auth",
          observedAt: "2026-09-07T09:00:00Z",
          quota: [
            { key: "primary", label: "5h", usedPercent: 10, window: { seconds: 18_000 } },
            { key: "credits", label: "Credits", remaining: 5, unit: "credits" },
            { key: "spark", label: "Spark 5h", usedPercent: 30 },
          ],
        }],
      },
    })[0]

    const layout = capacityLayout(mergeCapacityEntries(row), "kimi")
    expect(layout.hasWindowSkeleton).toBe(false)
    expect(layout.baseShort).toBeUndefined()
    expect(layout.baseLong).toBeUndefined()
    expect(layout.main.map((entry) => entry.metric.label)).toEqual(["5h", "Credits"])
    expect(layout.extras.map((entry) => entry.metric.label)).toEqual(["Spark 5h"])
  })
})
