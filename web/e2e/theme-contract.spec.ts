import { expect, test } from "@playwright/test"
import { installMockAPI } from "./mock-api"

for (const theme of ["light", "dark"] as const) {
  test(`${theme} preference preserves card, gradient, and focus styling`, async ({ page, browserName }) => {
    await installMockAPI(page)
    // An explicit preference must win over the operating system theme.
    await page.emulateMedia({ colorScheme: theme === "light" ? "dark" : "light" })
    await page.addInitScript((value) => localStorage.setItem("cpa-theme", value), theme)
    await page.goto("/")
    await expect(page.getByRole("heading", { name: "Intelligence", exact: true })).toBeVisible()
    const card = page.locator(".rounded-xl.bg-card").first()
    await expect(card).toHaveCSS("background-color", theme === "light" ? "rgb(253, 252, 252)" : "rgb(30, 28, 26)")
    await expect(card).toHaveCSS("border-color", theme === "light" ? "rgb(229, 224, 220)" : "rgb(50, 46, 42)")
    await expect(card).toHaveCSS("border-radius", "12px")
    await expect(card).toHaveCSS("box-shadow", /rgba\(0, 0, 0, 0\.05\) 0px 1px 2px 0px/)
    await expect(page.locator("section[aria-labelledby='attention-heading']")).toHaveCSS("background-image", /linear-gradient/)

    await page.goto("/login")
    const password = page.getByPlaceholder("Enter password", { exact: true })
    await password.fill("fixture-password")
    await password.focus()
    await expect(password).toHaveCSS("border-radius", "10px")
    await expect(password).toHaveCSS("box-shadow", /2px/)
    expect(await password.evaluate((node) => getComputedStyle(node, "::placeholder").color)).toBe("rgb(156, 163, 175)")
    if (browserName !== "webkit") {
      await page.emulateMedia({ forcedColors: "active" })
      await expect(password).toHaveCSS("outline-style", "solid")
      await expect(password).toHaveCSS("outline-width", "2px")
    }
    await expect(page.getByRole("button", { name: "Sign in", exact: true })).toHaveCSS("cursor", "pointer")
  })
}

test("system theme changes update the page without a reload", async ({ page }) => {
  await installMockAPI(page)
  await page.emulateMedia({ colorScheme: "light" })
  await page.addInitScript(() => localStorage.setItem("cpa-theme", "system"))
  await page.goto("/")
  const card = page.locator(".rounded-xl.bg-card").first()
  await expect(card).toHaveCSS("background-color", "rgb(253, 252, 252)")
  await page.emulateMedia({ colorScheme: "dark" })
  await expect(card).toHaveCSS("background-color", "rgb(30, 28, 26)")
})
