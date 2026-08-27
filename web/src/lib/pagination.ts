export interface PaginationMetadata {
  page: number
  pageSize: number
  totalPages: number
  totalCount: number
}

type PaginationPayload = {
  page?: unknown
  page_size?: unknown
  total_pages?: unknown
  total_count?: unknown
}

function positiveSafeInteger(value: unknown, field: string, resource: string): number {
  if (!Number.isSafeInteger(value) || (value as number) <= 0) {
    throw new Error(`${resource} returned invalid ${field}`)
  }
  return value as number
}

function nonNegativeSafeInteger(value: unknown, field: string, resource: string): number {
  if (!Number.isSafeInteger(value) || (value as number) < 0) {
    throw new Error(`${resource} returned invalid ${field}`)
  }
  return value as number
}

/**
 * Validates the transport contract shared by all paginated reads. Empty data
 * is one non-navigable page; missing or guessed pagination fields are errors.
 */
export function validatePaginationMetadata(
  payload: unknown,
  expectedPage: number,
  resource: string,
  expectedPageSize?: number,
): PaginationMetadata {
  if (typeof payload !== "object" || payload === null) {
    throw new Error(`${resource} returned invalid pagination metadata`)
  }
  const fields = payload as PaginationPayload
  const page = positiveSafeInteger(fields.page, "page", resource)
  const pageSize = positiveSafeInteger(fields.page_size, "page_size", resource)
  const totalPages = positiveSafeInteger(fields.total_pages, "total_pages", resource)
  const totalCount = nonNegativeSafeInteger(fields.total_count, "total_count", resource)
  const calculatedTotalPages = Math.max(1, Math.ceil(totalCount / pageSize))

  if (totalPages !== calculatedTotalPages || page > totalPages) {
    throw new Error(`${resource} returned inconsistent pagination metadata`)
  }
  if (expectedPageSize !== undefined && pageSize !== expectedPageSize) {
    throw new Error(`${resource} returned page_size ${pageSize} while ${expectedPageSize} was requested`)
  }
  const normalizedEmptyPage = totalCount === 0 && page === 1
  if (page !== expectedPage && !normalizedEmptyPage) {
    throw new Error(`${resource} returned page ${page} while page ${expectedPage} was requested`)
  }

  return { page, pageSize, totalPages, totalCount }
}

export function validatePaginatedPage<TPage, TItem>(input: {
  payload: TPage
  expectedPage: number
  resource: string
  expectedPageSize?: number
  getItems: (payload: TPage) => unknown
}): { metadata: PaginationMetadata; items: TItem[] } {
  const metadata = validatePaginationMetadata(
    input.payload,
    input.expectedPage,
    input.resource,
    input.expectedPageSize,
  )
  const items = input.getItems(input.payload)
  if (!Array.isArray(items)) {
    throw new Error(`${input.resource} returned invalid items`)
  }
  if (items.length > metadata.pageSize) {
    throw new Error(`${input.resource} returned more items than page_size`)
  }
  if (metadata.totalCount === 0 && items.length !== 0) {
    throw new Error(`${input.resource} returned items for an empty page`)
  }
  if (metadata.totalCount > 0 && items.length === 0) {
    throw new Error(`${input.resource} returned an empty populated page`)
  }
  return { metadata, items: items as TItem[] }
}

/**
 * Collects every advertised page or fails the whole read. Later-page metadata
 * must match the first page, and the collected item count must be complete.
 */
export async function collectPaginatedItems<TPage, TItem>(input: {
  fetchPage: (page: number) => Promise<TPage>
  getItems: (payload: TPage) => unknown
  resource: string
  expectedPageSize?: number
}): Promise<TItem[]> {
  const firstPayload = await input.fetchPage(1)
  const first = validatePaginatedPage<TPage, TItem>({
    payload: firstPayload,
    expectedPage: 1,
    resource: input.resource,
    expectedPageSize: input.expectedPageSize,
    getItems: input.getItems,
  })
  if (first.metadata.totalPages === 1) {
    if (first.items.length !== first.metadata.totalCount) {
      throw new Error(`${input.resource} returned an incomplete item count`)
    }
    return first.items
  }

  const remainingPayloads = await Promise.all(
    Array.from(
      { length: first.metadata.totalPages - 1 },
      (_, index) => input.fetchPage(index + 2),
    ),
  )
  const pages = [
    first,
    ...remainingPayloads.map((payload, index) =>
      validatePaginatedPage<TPage, TItem>({
        payload,
        expectedPage: index + 2,
        resource: input.resource,
        expectedPageSize: input.expectedPageSize,
        getItems: input.getItems,
      }),
    ),
  ]

  for (const current of pages.slice(1)) {
    if (
      current.metadata.pageSize !== first.metadata.pageSize ||
      current.metadata.totalPages !== first.metadata.totalPages ||
      current.metadata.totalCount !== first.metadata.totalCount
    ) {
      throw new Error(`${input.resource} changed pagination metadata between pages`)
    }
  }

  const items = pages.flatMap((page) => page.items)
  if (items.length !== first.metadata.totalCount) {
    throw new Error(`${input.resource} returned an incomplete item count`)
  }
  return items
}
