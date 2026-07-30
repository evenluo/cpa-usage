import type { ServiceHealthBlock } from "@/types/api"

export function cellColor(block: ServiceHealthBlock): string {
  if (block.rate < 0) return "bg-muted/20"
  const total = block.success + block.failure
  if (total === 0) return "bg-muted/20"
  if (block.failure === 0) return "bg-emerald-500"
  if (block.rate >= 0.99) return "bg-emerald-400"
  if (block.rate >= 0.95) return "bg-amber-400"
  return "bg-red-400"
}

function divisors(value: number): number[] {
  const result: number[] = []
  for (let candidate = 1; candidate <= value; candidate++) {
    if (value % candidate === 0) result.push(candidate)
  }
  return result
}

export function computeLayout(containerWidth: number, blockCount: number): { columns: number } {
  const labelWidth = 56
  const targetRows = 6
  const targetCellSize = 12
  const minCellSize = 6
  const rowGap = 1
  const availableWidth = Math.max(containerWidth - labelWidth, minCellSize)

  if (blockCount <= 0) {
    return { columns: 1 }
  }

  const maxColumns = Math.max(
    1,
    Math.min(blockCount, Math.floor((availableWidth + rowGap) / (minCellSize + rowGap)))
  )

  const candidates = divisors(blockCount).filter((columns) => columns <= maxColumns)
  if (candidates.length === 0) {
    return { columns: maxColumns }
  }

  const best = candidates.reduce((currentBest, columns) => {
    const rows = Math.ceil(blockCount / columns)
    const cellSize = (availableWidth - (columns - 1) * rowGap) / columns
    const score = Math.abs(rows - targetRows) * 2 + Math.abs(cellSize - targetCellSize)

    const bestRows = Math.ceil(blockCount / currentBest)
    const bestCellSize = (availableWidth - (currentBest - 1) * rowGap) / currentBest
    const bestScore =
      Math.abs(bestRows - targetRows) * 2 + Math.abs(bestCellSize - targetCellSize)

    return score < bestScore ? columns : currentBest
  }, candidates[0])

  return { columns: best }
}
