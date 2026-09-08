import { cleanup, render, screen, within } from "@testing-library/react"
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
  it("expands natively and qualifies mixed coverage and quality separately from Cost", async () => {
    const user = userEvent.setup()
    render(<CanonicalTokenComposition accounting={accounting} costStatus="available" />)
    const summary = screen.getByText("Token breakdown").closest("summary")!
    expect(summary.parentElement).not.toHaveAttribute("open")
    await user.click(summary)
    expect(summary.parentElement).toHaveAttribute("open")
    expect(screen.getByText(/3 \/ 10 valid/)).toBeInTheDocument()
    expect(screen.getByText(/complete 1 · inconsistent 1 · unclassified 1/)).toBeInTheDocument()
    expect(screen.getByText(/Canonical facts absent 7/)).toBeInTheDocument()
    expect(screen.getByText(/Local estimate \(available\)/)).toBeInTheDocument()
    expect(screen.getByText("Total tokens").nextElementSibling).toHaveTextContent("155")
    const input = within(screen.getByRole("region", { name: "Input composition" }))
    const output = within(screen.getByRole("region", { name: "Output composition" }))
    expect(input.getByText("100 tokens")).toBeInTheDocument()
    expect(output.getByText("50 tokens")).toBeInTheDocument()
    expect(input.getByText("Cache read").nextElementSibling).toHaveTextContent("20 · 20.0%")
    expect(output.getByText("Reasoning").nextElementSibling).toHaveTextContent("10 · 20.0%")
    expect(screen.getByText("Unclassified tokens").nextElementSibling).toHaveTextContent("5")
    expect(screen.getByText(/Input = uncached \+ cache read \+ cache write; output = non-reasoning \+ reasoning\./)).toHaveTextContent("Total = input + output + unclassified")
  })

  it("keeps canonical-absent or empty windows unavailable instead of displaying zero canonical totals", () => {
    const unavailable = { ...accounting, valid_attempts: 0, coverage_pct: 0, valid_quality: { complete: 0, inconsistent: 0, unclassified: 0 } }
    render(<CanonicalTokenComposition accounting={unavailable} costStatus="partial" />)
    expect(screen.getByText(/Token totals unavailable/)).toBeInTheDocument()
    expect(screen.queryByText("Total tokens")).not.toBeInTheDocument()
    expect(getAccountingCaption(unavailable)).toBe("Canonical tokens unavailable")
    expect(getAccountingCaption({ ...unavailable, total_attempts: 0, coverage_pct: null })).toBe("No attempts")
  })

  it("keeps an observed zero input unavailable for percentages while output remains independently scaled", async () => {
    render(<CanonicalTokenComposition accounting={{
      ...accounting,
      composition: {
        ...accounting.composition,
        total_tokens: 55,
        input: { total_tokens: 0, uncached_tokens: 0, cache_read_tokens: 0, cache_write_tokens: 0 },
      },
    }} costStatus="partial" />)
    await userEvent.click(screen.getByText("Token breakdown"))
    const input = screen.getByRole("region", { name: "Input composition" })
    expect(input).toHaveTextContent("0 tokens")
    expect(input).not.toHaveTextContent("%")
    expect(screen.getByRole("region", { name: "Output composition" })).toHaveTextContent("20.0%")
    expect(screen.getByText("Total tokens").nextElementSibling).toHaveTextContent("55")
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
