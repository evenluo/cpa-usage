import { expect, type Page, test } from "@playwright/test"
import { installMockAPI } from "./mock-api"

test.beforeEach(async ({ page }) => {
  await installMockAPI(page)
})

test("mobile uses bottom navigation without the fixed desktop sidebar", async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem("cpa-theme", "light")
  })
  await page.goto("/")
  await expect(page.getByRole("heading", { name: "Intelligence" })).toBeVisible()

  const usesMobileNav = await page.evaluate(() => window.matchMedia("(max-width: 767px)").matches)
  if (usesMobileNav) {
    await expect(page.getByLabel("Desktop navigation")).toBeHidden()
    await expect(page.getByLabel("Mobile navigation")).toBeVisible()
    await expectMobileNavigationPinnedToViewportBottom(page)
  } else {
    await expect(page.getByLabel("Desktop navigation")).toBeVisible()
    await expect(page.getByLabel("Mobile navigation")).toBeHidden()
  }

  await expectNoDocumentOverflow(page)
  await page.getByRole("link", { name: /Reference/ }).click()
  await expect(page.getByRole("heading", { name: "Reference" })).toBeVisible()
  await expectNoDocumentOverflow(page)
  await page.getByRole("link", { name: /Operations/ }).click()
  await expect(page.getByRole("heading", { name: "Operations" })).toBeVisible()
  await expectNoDocumentOverflow(page)
})

test("dashboard controls and evidence stay inside each responsive viewport", async ({ page }) => {
  let identityReads = 0
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.endsWith("/api/v1/usage/identities/page")) identityReads++
  })
  await page.addInitScript(() => {
    localStorage.setItem("cpa-theme", "dark")
  })
  await page.goto("/")

  await page.getByRole("button", { name: "30 days" }).click()
  await page.getByRole("button", { name: "Day", exact: true }).click()
  await page.getByRole("button", { name: "Trend view: Tokens" }).click()

  await expect(page.getByText("Trends")).toBeVisible()
  const chartLegend = page.locator(".recharts-legend-wrapper")
  await expect(chartLegend.getByText("Tokens", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Input", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Output", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Reasoning", { exact: true })).toBeVisible()
  await expect(chartLegend.getByText("Cache read", { exact: true })).toBeVisible()
  await expect(page.getByText("Key Leaderboard")).toBeVisible()
  await expect(page.getByText("Model Mix")).toBeVisible()
  await expect(page.getByText("priced-model")).toBeVisible()
  await expect(page.getByText("Needs attention", { exact: true })).toBeVisible()
  await expect(page.getByText("Pricing Missing")).toBeVisible()
  const capacity = page.locator(".rounded-xl").filter({ has: page.getByRole("heading", { name: /^Live Capacity/ }) })
  await expect(capacity.getByRole("heading", { name: /^Live Capacity/ })).toBeAttached()
  expect(identityReads).toBe(0)
  await capacity.scrollIntoViewIfNeeded()
  await expect.poll(() => identityReads).toBe(1)
  await expect(page.getByText("Agent Codex")).toBeVisible()
  await expect(page.getByText("Plus", { exact: true })).toBeVisible()
  await expect(page.getByText(/^Last updated /)).toBeVisible()
  await page.getByText(/··· \d+ more/).first().click()
  const accountTiming = page.getByRole("group", { name: "Account and observation timing" })
  await expect(accountTiming.getByText("Observed", { exact: true })).toBeVisible()
  await expect(accountTiming.locator("time[datetime='2026-08-31T09:05:00Z']")).toBeVisible()
  await expect(accountTiming.getByText("Ends", { exact: true })).toBeVisible()
  await expect(accountTiming.locator("time[datetime='2026-09-25T07:15:00Z']")).toBeVisible()
  await expect(page.getByText("Unsupported OpenAI")).toBeVisible()
  await expect(page.getByLabel("OpenAI", { exact: true }).getByText("OA", { exact: true })).toBeVisible()
  const codexLogoWell = page.locator('[aria-label="Codex"]')
  await expect(codexLogoWell).toHaveCount(1)
  await expect
    .poll(async () => codexLogoWell.evaluate((element) => {
      // CSS Color 4 may serialize the same white as oklab() instead of rgba().
      const canvas = document.createElement("canvas")
      canvas.width = canvas.height = 1
      const context = canvas.getContext("2d")!
      context.fillStyle = getComputedStyle(element).backgroundColor
      context.fillRect(0, 0, 1, 1)
      return Array.from(context.getImageData(0, 0, 1, 1).data).slice(0, 3)
    }))
    .toEqual([255, 255, 255])
  await expect(page.getByText("Request evidence")).toBeVisible()
  await expect(page.getByText("Agent API Key").first()).toBeVisible()
  const evidenceCard = page.getByText("Request evidence").locator("xpath=ancestor::*[contains(@class,'rounded-xl')][1]")
  await expect(evidenceCard.getByRole("status")).toHaveText("Live")
  await expect(evidenceCard.getByText("Latest request", { exact: true })).toHaveCount(0)
  await expect(evidenceCard.getByText("Output TPS", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("48.3 tok/s", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("Latency", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("21.25s", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("Tokens", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByText("105.09K", { exact: true })).toBeVisible()
  await expect(evidenceCard.getByRole("button", { name: /Show request evidence/ })).toHaveCount(0)
  await expectOverviewCardLayout(page)
  await evidenceCard.getByRole("link", { name: "View all attempts" }).click()
  await expect(page).toHaveURL(/\/requests\?/)
  const requestsURL = new URL(page.url())
  expect(requestsURL.pathname).toBe("/requests")
  expect(requestsURL.searchParams.get("provider")).toBe("")
  expect(requestsURL.searchParams.get("model")).toBe("")
  expect(requestsURL.searchParams.get("result")).toBe("")
  await expect(page.getByRole("heading", { name: "Request evidence" })).toBeVisible()
  await expect(page.getByText("Page 1 of 2", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "Select attempt 2" }).click()
  const selectedAttempt = page.getByRole("region", { name: "Selected attempt" })
  await expect(selectedAttempt.getByText("Output TPS", { exact: true }).locator("..").getByText("-", { exact: true })).toBeVisible()
  await page.getByRole("button", { name: "Next page" }).click()
  await expect(page.getByText("Page 2 of 2", { exact: true })).toBeVisible()
  await expectNoDocumentOverflow(page)
})

test("reference data controls are usable without viewport overflow", async ({ page }) => {
  await page.goto("/reference")

  await page.getByPlaceholder("Search").fill("Agent")
  await page.getByRole("button", { name: "Key alias scope: Accounts" }).click()
  await page.getByRole("button", { name: "Key alias scope: API Keys" }).click()
  await page.getByRole("button", { name: /Edit alias for Agent API Key/ }).click()
  await page.locator('input[name^="key-alias-"]').last().fill("Agent API Key Mobile")
  await page.getByRole("button", { name: /Save alias for Agent API Key/ }).click()

  await expect(page.getByText("Alias saved")).toBeVisible()
  await expectNoDocumentOverflow(page)
})

test("login and operations remain usable on small screens", async ({ page }) => {
  await page.goto("/login")
  await expect(page.getByRole("heading", { name: "Sign in" })).toBeVisible()
  await expectNoDocumentOverflow(page)

  await page.goto("/operations")
  await expect(page.getByText("Operational Status")).toBeVisible()
  await expect(page.getByText("Ingestion observations")).toBeVisible()
  await expect(page.getByText(/Local process only/)).toBeVisible()
  await expect(page.getByText("12.5 events/min")).toBeVisible()
  await expect(page.getByText("Runner idle")).toBeVisible()
  await page.getByRole("button", { name: "Sync now" }).click()
  await expect(page.getByText("Sync triggered")).toBeVisible()
  await expectNoDocumentOverflow(page)
})

async function expectNoDocumentOverflow(page: Page) {
  await expect
    .poll(async () =>
      page.evaluate(() => ({
        clientWidth: document.documentElement.clientWidth,
        scrollWidth: document.documentElement.scrollWidth,
      }))
    )
    .toEqual(expect.objectContaining({ scrollWidth: expect.any(Number) }))

  const overflow = await page.evaluate(() => {
    const root = document.documentElement
    return root.scrollWidth - root.clientWidth
  })
  expect(overflow).toBeLessThanOrEqual(1)
}

async function expectMobileNavigationPinnedToViewportBottom(page: Page) {
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight))

  const navPosition = await page.getByLabel("Mobile navigation").evaluate((node) => {
    const rect = node.getBoundingClientRect()
    const style = window.getComputedStyle(node)
    return {
      bottomGap: Math.abs(window.innerHeight - rect.bottom),
      position: style.position,
      transform: style.transform,
      willChange: style.willChange,
    }
  })

  expect(navPosition.position).toBe("fixed")
  expect(navPosition.bottomGap).toBeLessThanOrEqual(1)
  expect(navPosition.transform).toBe("none")
  expect(navPosition.willChange).toBe("auto")
}

async function expectOverviewCardLayout(page: Page) {
  const heights = await page.evaluate(() => {
    const healthHeading = Array.from(document.querySelectorAll("h3")).find((node) => node.textContent?.includes("Attempt Health"))
    const evidenceHeading = Array.from(document.querySelectorAll("h3")).find((node) => node.textContent?.includes("Request evidence"))
    const healthCard = healthHeading?.closest(".rounded-xl")
    const evidenceCard = evidenceHeading?.closest(".rounded-xl")
    return {
      isWide: window.matchMedia("(min-width: 1280px)").matches,
      healthTop: healthCard?.getBoundingClientRect().top ?? 0,
      evidenceTop: evidenceCard?.getBoundingClientRect().top ?? 0,
      evidence: evidenceCard?.getBoundingClientRect().height ?? 0,
    }
  })

  if (!heights.isWide) return
  expect(Math.abs(heights.healthTop - heights.evidenceTop)).toBeLessThanOrEqual(1)
  expect(heights.evidence).toBeLessThanOrEqual(330)
}
