import { describe, expect, it, vi } from "vitest"
import {
  collectPaginatedItems,
  validatePaginatedPage,
  validatePaginationMetadata,
} from "./pagination"

function page(
  current: number,
  totalPages: number,
  totalCount: number,
  items: number[],
  pageSize = 2,
) {
  return {
    page: current,
    page_size: pageSize,
    total_pages: totalPages,
    total_count: totalCount,
    items,
  }
}

describe("validatePaginationMetadata", () => {
  it("accepts populated and normalized empty pages", () => {
    expect(validatePaginationMetadata(page(2, 2, 3, [3]), 2, "records")).toEqual({
      page: 2,
      pageSize: 2,
      totalPages: 2,
      totalCount: 3,
    })
    expect(validatePaginationMetadata(page(1, 1, 0, []), 7, "records")).toMatchObject({
      page: 1,
      totalPages: 1,
      totalCount: 0,
    })
  })

  it.each([
    ["missing", undefined],
    ["zero", 0],
    ["fractional", 1.5],
    ["negative", -1],
    ["non-finite", Number.POSITIVE_INFINITY],
    ["unsafe", Number.MAX_SAFE_INTEGER + 1],
  ])("rejects %s positive-integer metadata", (_name, invalid) => {
    for (const field of ["page", "page_size", "total_pages"] as const) {
      expect(() =>
        validatePaginationMetadata({ ...page(1, 1, 0, []), [field]: invalid }, 1, "records"),
      ).toThrow(`invalid ${field}`)
    }
  })

  it.each([undefined, -1, 1.5, Number.NaN, Number.MAX_SAFE_INTEGER + 1])(
    "rejects invalid total_count %s",
    (invalid) => {
      expect(() =>
        validatePaginationMetadata({ ...page(1, 1, 0, []), total_count: invalid }, 1, "records"),
      ).toThrow("invalid total_count")
    },
  )

  it("rejects inconsistent counts, page totals, and response page numbers", () => {
    expect(() => validatePaginationMetadata(page(1, 3, 3, [1, 2]), 1, "records")).toThrow("inconsistent")
    expect(() => validatePaginationMetadata(page(2, 1, 1, [1]), 2, "records")).toThrow("inconsistent")
    expect(() => validatePaginationMetadata(page(1, 2, 3, [1, 2]), 2, "records")).toThrow("page 1")
  })

  it.each([null, undefined, "page one"])("rejects a malformed page payload %s", (payload) => {
    expect(() => validatePaginationMetadata(payload, 1, "records")).toThrow("invalid pagination metadata")
  })
})

describe("validatePaginatedPage", () => {
  it("rejects missing, oversized, or count-inconsistent item arrays", () => {
    expect(() => validatePaginatedPage({ payload: { ...page(1, 1, 0, []), items: undefined }, expectedPage: 1, resource: "records", getItems: (payload) => payload.items })).toThrow("invalid items")
    expect(() => validatePaginatedPage({ payload: page(1, 2, 3, [1, 2, 3]), expectedPage: 1, resource: "records", getItems: (payload) => payload.items })).toThrow("more items")
    expect(() => validatePaginatedPage({ payload: page(1, 1, 0, [1]), expectedPage: 1, resource: "records", getItems: (payload) => payload.items })).toThrow("items for an empty page")
    expect(() => validatePaginatedPage({ payload: page(1, 1, 1, []), expectedPage: 1, resource: "records", getItems: (payload) => payload.items })).toThrow("empty populated page")
  })
})

describe("collectPaginatedItems", () => {
  it("collects every consistent page", async () => {
    const fetchPage = vi.fn(async (current: number) => page(current, 2, 3, current === 1 ? [1, 2] : [3]))

    await expect(collectPaginatedItems({ fetchPage, getItems: (payload) => payload.items, resource: "records" })).resolves.toEqual([1, 2, 3])
    expect(fetchPage).toHaveBeenCalledTimes(2)
  })

  it("fails the whole collection when a later page fails", async () => {
    const fetchPage = vi.fn(async (current: number) => {
      if (current === 2) throw new Error("page unavailable")
      return page(current, 2, 3, [1, 2])
    })

    await expect(collectPaginatedItems({ fetchPage, getItems: (payload) => payload.items, resource: "records" })).rejects.toThrow("page unavailable")
  })

  it("rejects later-page metadata drift and incomplete collections", async () => {
    await expect(collectPaginatedItems({
      fetchPage: async (current) => current === 1 ? page(1, 2, 3, [1, 2]) : page(2, 3, 5, [3, 4]),
      getItems: (payload) => payload.items,
      resource: "records",
    })).rejects.toThrow("changed pagination metadata")

    await expect(collectPaginatedItems({
      fetchPage: async (current) => page(current, 2, 4, current === 1 ? [1, 2] : [3]),
      getItems: (payload) => payload.items,
      resource: "records",
    })).rejects.toThrow("incomplete item count")
  })
})
