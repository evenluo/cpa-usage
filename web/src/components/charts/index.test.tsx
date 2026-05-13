import { render, screen, within } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { AliasRankingChart, MetricTrendChart, TokenCostCompareChart, TrendBucketTooltip } from './index'

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

  it('does not chart unavailable alias costs as zero currency', () => {
    render(
      <AliasRankingChart
        rows={[{
          alias: 'Missing price alias',
          key: 'sk-missing',
          provider: 'OpenAI',
          cost: 0,
          tokens: 1000,
          requests: 1,
          successRate: 100,
          failures: 0,
          trend: [1000],
          costAvailable: false,
          costStatus: 'unavailable',
        }]}
      />,
    )

    expect(screen.getByText('No available alias cost data')).toBeInTheDocument()
    expect(screen.queryByText('$0.00')).not.toBeInTheDocument()
  })

  it('renders trend tooltip bucket values without treating partial cost as zero', () => {
    render(
      <TrendBucketTooltip
        active
        label="05-12 10:00"
        payload={[{
          payload: {
            label: '05-12 10:00',
            cost: 0.25,
            tokens: 1100100,
            requests: 181,
            failures: 4,
            costAvailable: false,
            costStatus: 'partial',
          },
        }]}
      />,
    )

    const tooltip = screen.getByRole('tooltip', { name: 'Trend bucket detail' })
    expect(within(tooltip).getByText('05-12 10:00')).toBeInTheDocument()
    expect(within(tooltip).getByText('$0.25 partial')).toBeInTheDocument()
    expect(within(tooltip).getByText('1.1M tokens')).toBeInTheDocument()
    expect(within(tooltip).getByText('181 requests')).toBeInTheDocument()
    expect(within(tooltip).getByText('4 failures')).toBeInTheDocument()
    expect(within(tooltip).queryByText('$0.00')).not.toBeInTheDocument()
  })
})
