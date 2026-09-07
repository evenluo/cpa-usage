import { cleanup, render, screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { afterEach, describe, expect, it } from "vitest"
import type { AccountingSummary } from "@/types/api"
import { getAccountingCaption, getCanonicalTokenFields } from "@/features/usage-intelligence/view-model"
import { CanonicalTokenComposition } from "./canonical-token-composition"

const accounting: AccountingSummary = {
  total_attempts: 10,
  valid_attempts: 3,
  coverage_pct: 30,
  states: { valid: 3, absent: 7 },
  valid_quality: { complete: 1, inconsistent: 1, unclassified: 1 },
  composition: {
    total_tokens: 155,
    input: { total_tokens: 100, uncached_tokens: 70, cache_read_tokens: 20, cache_write_tokens: 10 },
    output: { total_tokens: 50, non_reasoning_tokens: 40, reasoning_tokens: 10 },
    unclassified_tokens: 5,
  },
}

afterEach(cleanup)

describe("Canonical token composition", () => {
  it("expands within the token surface and qualifies mixed coverage and quality separately from Cost", async () => {
    const user = userEvent.setup()
    render(<CanonicalTokenComposition accounting={accounting} costStatus="available" />)
    const summary = screen.getByText("Canonical token composition").closest("summary")!
    expect(summary.parentElement).not.toHaveAttribute("open")
    await user.click(summary)
    expect(summary.parentElement).toHaveAttribute("open")
    expect(screen.getByText(/3 \/ 10 attempts have valid/)).toBeInTheDocument()
    expect(screen.getByText(/Quality among 3 valid attempts: complete 1, inconsistent 1, unclassified 1/)).toBeInTheDocument()
    expect(screen.getByText(/Canonical facts absent 7/)).toBeInTheDocument()
    expect(screen.getByText(/Local cost estimate completeness: available/)).toBeInTheDocument()
    expect(screen.getByText("Canonical total").nextElementSibling).toHaveTextContent("155")
    expect(screen.getByText("Canonical input total").nextElementSibling).toHaveTextContent("100")
    expect(screen.getByText("Canonical output total").nextElementSibling).toHaveTextContent("50")
    expect(screen.getByText(/Input = uncached/)).toHaveTextContent("Total = input + output + unclassified")
  })

  it("keeps canonical-absent or empty windows unavailable instead of displaying zero canonical totals", () => {
    const unavailable = { ...accounting, valid_attempts: 0, coverage_pct: 0, valid_quality: { complete: 0, inconsistent: 0, unclassified: 0 } }
    render(<CanonicalTokenComposition accounting={unavailable} costStatus="partial" />)
    expect(screen.getByText(/Canonical totals unavailable/)).toBeInTheDocument()
    expect(screen.queryByText("Canonical total")).not.toBeInTheDocument()
    expect(getAccountingCaption(unavailable)).toBe("Canonical tokens unavailable")
    expect(getAccountingCaption({ ...unavailable, total_attempts: 0, coverage_pct: null })).toBe("No attempts")
    expect(getAccountingCaption()).toBe("Canonical tokens unavailable")
  })

  it("does not promote valid inconsistent quality or double-add the reasoning and cache subsets", () => {
    expect(getAccountingCaption({ ...accounting, total_attempts: 3, coverage_pct: 100 })).toBe("Canonical: 100.0% of attempts · qualified quality")
    expect(getAccountingCaption({ ...accounting, valid_quality: { complete: 3, inconsistent: 0, unclassified: 0 } })).toContain("complete quality")
    expect(Object.fromEntries(getCanonicalTokenFields(accounting.composition))).toMatchObject({
      "Canonical total": 155,
      "Canonical input total": 100,
      "Canonical output total": 50,
      "Reasoning output": 10,
      "Cache read input": 20,
      "Unclassified tokens": 5,
    })
  })
})
