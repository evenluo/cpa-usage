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
const performance = {
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
  output_tps: { generating_streaming: { population_count: 1199, sample_count: 0, coverage: 0, p50: null, p95: null } },
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
