import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { MetricTrendChart, TokenCostCompareChart } from './index'

const subDollarTrend = [
  {
    label: '05-12',
    cost: 0.24,
    tokens: 1000,
    requests: 1,
    failures: 0,
  },
]

describe('analytics charts', () => {
  it('formats sub-dollar cost axis labels with cent precision', () => {
    render(
      <>
        <MetricTrendChart data={subDollarTrend} />
        <TokenCostCompareChart data={subDollarTrend} />
      </>,
    )

    expect(screen.getAllByText('$0.24')).toHaveLength(2)
    expect(screen.queryByText('$0')).not.toBeInTheDocument()
  })
})
