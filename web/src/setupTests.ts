import '@testing-library/jest-dom/vitest'
import React from 'react'
import { afterEach, beforeEach, vi } from 'vitest'

const originalWarn = console.warn

const ChartContainer = () => React.createElement('div', { 'data-testid': 'chart-container' })
const ChartLeaf = () => null

vi.mock('recharts', () => ({
  Area: ChartLeaf,
  AreaChart: ChartContainer,
  Bar: ChartLeaf,
  BarChart: ChartContainer,
  CartesianGrid: ChartLeaf,
  Cell: ChartLeaf,
  Line: ChartLeaf,
  LineChart: ChartContainer,
  Pie: ChartContainer,
  PieChart: ChartContainer,
  ResponsiveContainer: ChartContainer,
  Tooltip: ChartLeaf,
  XAxis: ChartLeaf,
  YAxis: ChartLeaf,
}))

beforeEach(() => {
  vi.spyOn(console, 'warn').mockImplementation((...args) => {
    if (String(args[0]).includes('The width(-1) and height(-1) of chart should be greater than 0')) {
      return
    }
    originalWarn(...args)
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})
