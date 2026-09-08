import { expect, test } from "@playwright/test"
import { installMockAPI, usageAttemptPerformance } from "./mock-api"
import failureFixture from "../src/test/contracts/usage_failure_distribution.json" with { type: "json" }

// Synthetic low-failure, multi-model observations exercise the everyday reading path.
const modelRows = [
  { name: "gpt-5.6-sol", attempts: 995, p50: 9290, p95: 54440 },
  { name: "codex-auto-review", attempts: 180, p50: 4210, p95: 12400 },
  { name: "gpt-6-astra", attempts: 12, p50: 4900, p95: 14800 },
  { name: "gpt-5.6-terra", attempts: 8, p50: 3700, p95: 8900 },
  { name: "gpt-5.6-luna", attempts: 5, p50: 1900, p95: 6200 },
]
const performance = withHistograms({
  ...usageAttemptPerformance,
  total_attempts: 1200,
  successful_attempts: 1199,
  failed_attempts: 1,
  successful_execution: { generating_streaming: 1199, non_generating: 0, non_streaming: 0, unknown: 0 },
  latency_ms: {
    successful: { population_count: 1199, sample_count: 1199, coverage: 1, p50: 9290, p95: 54440 },
    failed: { population_count: 1, sample_count: 1, coverage: 1, p50: 7800, p95: 7800 },
  },
  ttft_ms: {
    generating_streaming: { population_count: 1199, sample_count: 1199, coverage: 1, p50: 4930, p95: 8990 },
    unknown_execution: { population_count: 0, sample_count: 0, coverage: null, p50: null, p95: null },
  },
  output_tps: { generating_streaming: { population_count: 1199, sample_count: 1199, coverage: 1, p50: 43.6, p95: 145.2 } },
  providers: { items: [], other_count: 1200 },
  accounts: { items: [], other_count: 1200 },
  models: {
    other_count: 0,
    items: modelRows.map((row, index) => ({
      ...usageAttemptPerformance.models.items[0],
      value: row.name,
      label: row.name,
      attempt_count: row.attempts,
      successful_attempts: row.attempts - (index === 0 ? 1 : 0),
      failed_attempts: index === 0 ? 1 : 0,
      successful_execution: { generating_streaming: row.attempts - (index === 0 ? 1 : 0), non_generating: 0, non_streaming: 0, unknown: 0 },
      ttft_ms: {
        generating_streaming: { population_count: row.attempts - (index === 0 ? 1 : 0), sample_count: row.attempts - (index === 0 ? 1 : 0), coverage: 1, p50: row.p50 / 2, p95: row.p95 / 2 },
        unknown_execution: { population_count: 0, sample_count: 0, coverage: null, p50: null, p95: null },
      },
      output_tps: { generating_streaming: { population_count: row.attempts - (index === 0 ? 1 : 0), sample_count: 0, coverage: 0, p50: null, p95: null } },
      latency_ms: {
        successful: { population_count: row.attempts - (index === 0 ? 1 : 0), sample_count: row.attempts - (index === 0 ? 1 : 0), coverage: 1, p50: row.p50, p95: row.p95 },
        failed: { population_count: index === 0 ? 1 : 0, sample_count: index === 0 ? 1 : 0, coverage: index === 0 ? 1 : null, p50: index === 0 ? 7800 : null, p95: index === 0 ? 7800 : null },
      },
    })),
  },
})


// Deterministic samples preserve the fixture's exact nearest-rank percentiles
// while exercising shared bins, the full tail and low-sample states.
function withHistograms<T>(value: T, upper = 60000): T {
  if (!value || typeof value !== "object") return value
  if (Array.isArray(value)) return value.map((item) => withHistograms(item, upper)) as T
  const record = value as Record<string, unknown>
  if (typeof record.sample_count === "number") {
    const count = record.sample_count
    const counts = Array<number>(24).fill(0)
    const p50 = Number(record.p50)
    const p95 = Number(record.p95)
    const rank50 = Math.ceil(count * 0.5)
    const rank95 = Math.ceil(count * 0.95)
    for (let rank = 1; rank <= count; rank++) {
      const sample = rank <= rank50 ? p50 * rank / rank50
        : rank <= rank95 ? p50 + (p95 - p50) * (rank - rank50) / (rank95 - rank50)
          : p95 + (upper - p95) * (rank - rank95) / (count - rank95)
      counts[Math.min(23, Math.floor(sample / upper * 24))]++
    }
    return { ...record, histogram: count > 0 ? { upper_bound: upper, counts } : null } as T
  }
  return Object.fromEntries(Object.entries(record).map(([key, item]) => [key, withHistograms(item, key === "ttft_ms" ? 30000 : key === "output_tps" ? 200 : upper)])) as T
}

for (const theme of ["light", "dark"]) {
  test(`${theme} diagnostics prioritize visible comparisons and compact investigation`, async ({ page }, testInfo) => {
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    await installMockAPI(page)
    await page.route(/\/api\/v1\/usage\/performance(?:\?|$)/, (route) => route.fulfill({ json: performance }))
    await page.route(/\/api\/v1\/usage\/failures(?:\?|$)/, (route) => route.fulfill({ json: {
      ...failureFixture,
      total_failures: 1,
      ...Object.fromEntries(["categories", "statuses", "providers", "accounts", "models", "endpoints"].map((key) => [key, {
        items: [{ value: key === "statuses" ? "408" : "codex", label: key === "statuses" ? "HTTP 408" : "codex", count: 1 }],
        other_count: 0,
      }])),
    } }))
    await page.goto("/")
    const performanceCard = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: "Attempt performance", exact: true }) })
    await expect(performanceCard.getByRole("img")).toHaveCount(5)
    await expect(performanceCard.getByRole("img").first()).toBeVisible()
    const density = performanceCard.getByRole("button", { name: "View sample distribution for gpt-5.6-sol", exact: true })
    await expect(density.locator("[data-heatmap-bin]")).toHaveCount(24)
    const sum = await density.locator("[data-heatmap-bin]").evaluateAll((bins) => bins.reduce((n, bin) => n + Number(bin.getAttribute("data-count")), 0))
    expect(sum).toBe(994)
    await density.focus()
    await page.keyboard.press("Enter")
    await expect(page.getByRole("dialog")).toContainText("994 valid samples")
    await expect(page.getByRole("dialog").getByRole("row")).toHaveCount(25)
    await page.keyboard.press("Escape")
    await expect(density).toBeFocused()
    await performanceCard.getByRole("button", { name: "View sample distribution for gpt-5.6-luna", exact: true }).click()
    await expect(page.getByRole("dialog")).toContainText("5 valid samples")
    await expect(page.getByRole("dialog")).toContainText("Few samples")
    await page.keyboard.press("Escape")
    await expect(performanceCard).toContainText("54.44s")
    await expect(performanceCard.getByText("100% coverage", { exact: true })).toHaveCount(0)
    const sampleToggle = performanceCard.getByLabel("Sample details for gpt-5.6-sol", { exact: true }).first()
    await sampleToggle.click()
    await expect(performanceCard.getByText("994 / 994 samples · 100% coverage")).toBeVisible()
    await sampleToggle.click()
    const tracks = await performanceCard.getByRole("img").all()
    const firstTrack = await tracks[0].boundingBox()
    for (const track of tracks.slice(1)) {
      const bounds = await track.boundingBox()
      expect(bounds!.x).toBeCloseTo(firstTrack!.x, 1)
      expect(bounds!.width).toBeCloseTo(firstTrack!.width, 1)
    }
    const metricControls = performanceCard.getByLabel("Performance metric", { exact: true })
    await metricControls.scrollIntoViewIfNeeded()
    const controlsTop = await metricControls.evaluate((element) => element.getBoundingClientRect().top + window.scrollY)
    await metricControls.getByRole("button", { name: "TTFT", exact: true }).click()
    expect(await metricControls.evaluate((element) => element.getBoundingClientRect().top + window.scrollY)).toBeCloseTo(controlsTop, 1)
    const info = performanceCard.getByRole("button", { name: "About TTFT execution unknown" })
    await info.focus()
    await page.keyboard.press("Enter")
    const detail = page.getByRole("dialog", { name: "Unknown execution TTFT" })
    await expect(detail).toBeVisible()
    await expect(detail).toContainText("0 / 0 samples · Coverage unavailable")
    expect(await metricControls.evaluate((element) => element.getBoundingClientRect().top + window.scrollY)).toBeCloseTo(controlsTop, 1)
    await page.keyboard.press("Escape")
    await expect(detail).not.toBeVisible()
    await expect(info).toBeFocused()
    await metricControls.getByRole("button", { name: "Output TPS", exact: true }).click()
    expect(await metricControls.evaluate((element) => element.getBoundingClientRect().top + window.scrollY)).toBeCloseTo(controlsTop, 1)
    await metricControls.getByRole("button", { name: "Successful latency", exact: true }).click()
    const failures = page.getByRole("region", { name: "Failure analysis" })
    await expect(failures).toContainText("1 failed attempt")
    await expect(failures.locator("details")).not.toHaveAttribute("open", "")
    await expect(failures.getByRole("link", { name: "Inspect HTTP 408 failures" })).not.toBeVisible()
    const mappingDetails = page.locator("details").filter({ has: page.locator("summary").filter({ hasText: "Observed model mappings" }) })
    await expect(mappingDetails).not.toHaveAttribute("open", "")
    await performanceCard.screenshot({ animations: "disabled", path: testInfo.outputPath(`comparison-${theme}.png`) })
    const healthCard = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: "Attempt Health", exact: true }) })
    await healthCard.screenshot({ animations: "disabled", path: testInfo.outputPath(`health-compact-${theme}.png`) })
    await mappingDetails.locator("summary").first().click()
    await expect(mappingDetails.getByRole("link", { name: "Inspect route-a to actual-a attempts" })).toBeVisible()
    await expect(mappingDetails.getByText(/Names are observed values/)).not.toBeVisible()
    await mappingDetails.getByText("About these mappings", { exact: true }).click()
    await expect(mappingDetails.getByText(/Names are observed values/)).toBeVisible()
    await mappingDetails.getByText("About these mappings", { exact: true }).click()
    await mappingDetails.screenshot({ animations: "disabled", path: testInfo.outputPath(`mappings-expanded-${theme}.png`) })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
  })
}

test("performance summary stays stable at intermediate widths with numeric TPS", async ({ page }) => {
  await installMockAPI(page)
  await page.route(/\/api\/v1\/usage\/performance(?:\?|$)/, (route) => route.fulfill({ json: performance }))
  await page.goto("/")
  const card = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: "Attempt performance", exact: true }) })
  await page.evaluate(() => document.fonts.ready)
  const controls = card.getByLabel("Performance metric", { exact: true })
  for (const width of [360, 430, 540, 640, 820, 1024, 1440]) {
    await page.setViewportSize({ width, height: 1000 })
    await controls.getByRole("button", { name: "Successful latency", exact: true }).click()
    const top = await controls.evaluate((element) => element.getBoundingClientRect().top - element.closest(".rounded-xl")!.getBoundingClientRect().top)
    for (const name of ["TTFT", "Output TPS"]) {
      await controls.getByRole("button", { name, exact: true }).click()
      expect(await controls.evaluate((element) => element.getBoundingClientRect().top - element.closest(".rounded-xl")!.getBoundingClientRect().top), `width ${width}, ${name}`).toBeCloseTo(top, 1)
    }
  }
})
