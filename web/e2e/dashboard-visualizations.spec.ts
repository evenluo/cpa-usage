import { expect, test } from "@playwright/test"
import { installMockAPI, type RecordedAPIRequest } from "./mock-api"
import type { HeatmapData } from "../src/types/api"

// Synthetic observations cover missing facts, partial facts and real zero separately.
const heatmap: HeatmapData = {
  measure: "tokens",
  max_tokens: 100,
  max_requests: 4,
  max_failures: 0,
  max_cost: 0,
  rows: Array.from({ length: 30 }, (_, index) => {
    const date = `2026-05-${String(index + 1).padStart(2, "0")}`
    return {
      date,
      label: `05/${String(index + 1).padStart(2, "0")}`,
      cells: Array.from({ length: 24 }, (_, hour) => ({
        hour,
        in_range: hour !== 23,
        bucket_start: `${date}T${String(hour).padStart(2, "0")}:00:00Z`,
        bucket_end: new Date(Date.UTC(2026, 4, index + 1, hour + 1)).toISOString(),
        total_tokens: hour === 2 || hour === 4 ? 100 : 0,
        request_count: hour === 1 ? 2 : hour === 2 ? 4 : hour === 3 || hour === 4 ? 1 : 0,
        failure_count: 0,
        canonical_valid_attempts: hour === 2 ? 2 : hour === 3 || hour === 4 ? 1 : 0,
        total_cost: 0,
        cost_available: false,
        cost_status: "unavailable",
      })),
    }
  }),
}

for (const theme of ["light", "dark"]) {
  test(`${theme} dashboard charts preserve observation states and local selection`, async ({ page }, testInfo) => {
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    const requests: RecordedAPIRequest[] = []
    await installMockAPI(page, {
      analyticsHeatmap: () => ({ heatmap }),
      onRequest: (request) => requests.push(request),
    })
    await page.goto("/")
    if (theme === "dark") await expect(page.locator("html")).toHaveClass(/dark/)

    const costCard = page.locator(".rounded-xl").filter({ has: page.getByTestId("kpi-value-cost") })
    await expect(costCard).toBeVisible()
    const collapsedKpiBounds = await costCard.boundingBox()
    await page.getByText("Token breakdown", { exact: true }).click()
    await expect(page.getByRole("region", { name: "Input composition" })).toBeVisible()
    await expect(page.getByRole("region", { name: "Output composition" })).toBeVisible()
    const tokenDetails = page.locator("details").filter({ has: page.locator("summary", { hasText: /^Token breakdown$/ }) })
    const expandedKpiBounds = await costCard.boundingBox()
    expect(expandedKpiBounds!.height).toBeCloseTo(collapsedKpiBounds!.height, 0)
    const lastKpiBounds = await page.locator(".rounded-xl").filter({ has: page.getByTestId("kpi-value-cache") }).boundingBox()
    const tokenBounds = await tokenDetails.boundingBox()
    expect(tokenBounds!.y).toBeGreaterThanOrEqual(lastKpiBounds!.y + lastKpiBounds!.height)
    await costCard.scrollIntoViewIfNeeded()
    await page.screenshot({ path: testInfo.outputPath(`kpis-expanded-${theme}.png`), fullPage: false })
    await tokenDetails.evaluate((element) => element.scrollIntoView({ block: "start" }))
    await tokenDetails.screenshot({ path: testInfo.outputPath(`tokens-${theme}.png`) })

    const metric = page.getByRole("group", { name: "Heatmap metric" })
    await expect(metric.getByRole("button", { name: "Tokens", exact: true })).toHaveAttribute("aria-pressed", "true")
    const partial = page.getByRole("button", { name: /^05\/01 02:00/ })
    await expect(partial).toHaveAttribute("data-state", "token-partial")
    await expect(page.getByRole("button", { name: /^05\/01 01:00/ })).toHaveAttribute("data-state", "token-unavailable")
    await expect(page.getByRole("button", { name: /^05\/01 03:00/ })).toHaveAttribute("data-state", "observed")
    if (page.viewportSize()!.width < 1024) {
      const cellBounds = await partial.boundingBox()
      expect(cellBounds!.width).toBeGreaterThanOrEqual(24)
      expect(cellBounds!.height).toBeGreaterThanOrEqual(24)
    }
    await partial.click()
    const tooltip = page.getByRole("tooltip")
    await expect(tooltip).toContainText("Token coverage 2/4")
    const bounds = await tooltip.boundingBox()
    const viewport = page.viewportSize()!
    expect(bounds).not.toBeNull()
    expect(bounds!.x).toBeGreaterThanOrEqual(0)
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(viewport.width)
    expect(bounds!.y).toBeGreaterThanOrEqual(0)
    expect(bounds!.y + bounds!.height).toBeLessThanOrEqual(viewport.height)

    const heatmapRequests = requests.filter((request) => request.path === "/analytics/heatmap").length
    await metric.getByRole("button", { name: "Attempts", exact: true }).click()
    await expect(partial).toHaveAttribute("data-state", "observed")
    await metric.getByRole("button", { name: "Failures", exact: true }).click()
    await expect(page.getByRole("status").filter({ hasText: "No failures in this period" })).toBeVisible()
    await expect(page.getByRole("button", { name: /^05\/01 00:00/ })).toHaveAttribute("data-state", "no-activity")
    await expect(page.getByRole("button", { name: /^05\/01 01:00/ })).toHaveAttribute("data-state", "observed")
    expect(requests.filter((request) => request.path === "/analytics/heatmap")).toHaveLength(heatmapRequests)
    await metric.getByRole("button", { name: "Tokens", exact: true }).click()
    await page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: /^Activity Heatmap/ }) }).screenshot({ path: testInfo.outputPath(`heatmap-${theme}.png`) })

    await expect(page.getByRole("link", { name: "Inspect HTTP 429 failures" })).toContainText("66.7%")
    await page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: "Failure concentration", exact: true }) }).screenshot({ path: testInfo.outputPath(`failures-${theme}.png`) })

    const breakdown = page.locator("details").filter({ has: page.locator("summary", { hasText: /^Breakdown$/ }) })
    await breakdown.locator("summary").click()
    const performanceControls = breakdown.getByLabel("Performance breakdown metric")
    const controlBounds = await performanceControls.boundingBox()
    for (const button of await performanceControls.getByRole("button").all()) {
      const buttonBounds = await button.boundingBox()
      expect(buttonBounds!.x).toBeGreaterThanOrEqual(controlBounds!.x)
      expect(buttonBounds!.x + buttonBounds!.width).toBeLessThanOrEqual(controlBounds!.x + controlBounds!.width)
    }
    await expect(breakdown.getByRole("img")).toHaveCount(3)
    const performanceRequests = requests.filter((request) => request.path === "/usage/performance").length
    await breakdown.getByRole("button", { name: "Failed latency", exact: true }).click()
    await expect(breakdown.getByRole("link", { name: "Inspect sonnet failed attempts at or above p95 latency" })).toBeVisible()
    await breakdown.getByRole("button", { name: "TTFT", exact: true }).click()
    await expect(breakdown.getByRole("link")).toHaveCount(0)
    await breakdown.getByRole("button", { name: "Output TPS", exact: true }).click()
    await expect(breakdown.getByRole("img")).toHaveCount(0)
    await expect(breakdown.getByText("Select a provider to compare actual models.")).toBeVisible()
    expect(requests.filter((request) => request.path === "/usage/performance")).toHaveLength(performanceRequests)
    await breakdown.getByRole("button", { name: "Successful latency", exact: true }).click()
    await breakdown.screenshot({ path: testInfo.outputPath(`performance-${theme}.png`) })
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true)
    await breakdown.getByRole("button", { name: "Failed latency", exact: true }).click()
    await breakdown.getByRole("link", { name: "Inspect sonnet failed attempts at or above p95 latency" }).click()
    await expect(page.getByRole("heading", { name: "Request Evidence", exact: true })).toBeVisible()
    await expect.poll(() => {
      const url = requests.filter((request) => request.path === "/usage/events").at(-1)?.url
      return url ? {
        model: url.searchParams.get("model"),
        result: url.searchParams.get("result"),
        minLatency: url.searchParams.get("min_latency_ms"),
        windowEnd: url.searchParams.get("window_end"),
      } : null
    }).toEqual({ model: "sonnet", result: "failed", minLatency: "12000", windowEnd: "2026-09-07T12:00:00.123456789Z" })
  })
}
