import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import App from './App'

describe('App', () => {
  const originalBasePath = window.__APP_BASE_PATH__
  const originalPath = window.location.pathname

  beforeEach(() => {
    window.history.replaceState({}, '', '/')
    vi.restoreAllMocks()
  })

  afterEach(() => {
    window.__APP_BASE_PATH__ = originalBasePath
    window.history.replaceState({}, '', originalPath)
    cleanup()
  })

  it('renders the CPA Usage shell with primary navigation', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: 'CPA Usage' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '数据分析 Analytics' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Key 管理 Keys' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '请求明细 Events' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '计价配置 Pricing' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '系统设置 Settings' })).toBeInTheDocument()
  })

  it('renders analytics KPI and trend data from the analytics API', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/v1/analytics/summary')) {
        return new Response(JSON.stringify({
          range: '7d',
          range_start: '2026-05-06T00:00:00Z',
          range_end: '2026-05-13T00:00:00Z',
          timezone: 'UTC',
          summary: {
            total_cost: 0.49,
            total_tokens: 2100100,
            request_count: 301,
            success_count: 296,
            failure_count: 5,
            success_rate: 98.338,
            cost_available: false,
            cost_status: 'partial',
          },
          trend: [
            { label: '05-11', total_cost: 0.24, total_tokens: 1000000, request_count: 120, success_count: 119, failure_count: 1, cost_available: true, cost_status: 'available' },
            { label: '05-12', total_cost: 0.25, total_tokens: 1100100, request_count: 181, success_count: 177, failure_count: 4, cost_available: false, cost_status: 'partial' },
          ],
          key_alias_breakdown: [
            {
              label: 'Shared Alias',
              alias: 'Shared Alias',
              traceability: 'sk-a*******3456 · OpenAI',
              identity: 'sk-a*******3456',
              auth_type: 2,
              auth_type_name: 'apikey',
              type: 'openai',
              provider: 'OpenAI',
              is_deleted: false,
              total_cost: 2,
              total_tokens: 2000000,
              request_count: 20,
              success_count: 20,
              failure_count: 0,
              success_rate: 100,
              last_used_at: '2026-05-12T23:59:59Z',
              cost_available: true,
              cost_status: 'available',
              trend: [{ label: '05-12', total_cost: 2, total_tokens: 2000000, cost_available: true, cost_status: 'available' }],
            },
            {
              label: 'Shared Alias',
              alias: 'Shared Alias',
              traceability: 'sk-b*******3456 · Anthropic',
              identity: 'sk-b*******3456',
              auth_type: 2,
              auth_type_name: 'apikey',
              type: 'claude',
              provider: 'Anthropic',
              is_deleted: false,
              total_cost: 1,
              total_tokens: 1000000,
              request_count: 10,
              success_count: 9,
              failure_count: 1,
              success_rate: 90,
              last_used_at: '2026-05-12T22:00:00Z',
              cost_available: true,
              cost_status: 'available',
              trend: [{ label: '05-12', total_cost: 1, total_tokens: 1000000, cost_available: true, cost_status: 'available' }],
            },
            {
              label: 'Very Long Key Alias Label That Should Stay Inside The Ranking Row Without Breaking Layout',
              alias: '',
              traceability: 'sk-m*******3456',
              identity: 'sk-m*******3456',
              auth_type: 2,
              auth_type_name: 'apikey',
              type: '',
              provider: '',
              is_deleted: true,
              total_cost: 0,
              total_tokens: 3000000,
              request_count: 3,
              success_count: 3,
              failure_count: 0,
              success_rate: 100,
              last_used_at: null,
              cost_available: false,
              cost_status: 'unavailable',
              trend: [{ label: '05-12', total_cost: 0, total_tokens: 3000000, cost_available: false, cost_status: 'unavailable' }],
            },
          ],
          model_distribution: [
            {
              model: 'gpt-5.5',
              provider: 'OpenAI',
              total_cost: 4.5,
              total_tokens: 2500000,
              request_count: 80,
              success_count: 79,
              failure_count: 1,
              success_rate: 98.75,
              total_latency_ms: 16000,
              latency_sample_count: 80,
              average_latency_ms: 200,
              cost_available: true,
              cost_status: 'available',
            },
            {
              model: 'missing-price-model',
              provider: 'Anthropic',
              total_cost: 0,
              total_tokens: 1800000,
              request_count: 40,
              success_count: 38,
              failure_count: 2,
              success_rate: 95,
              total_latency_ms: 0,
              latency_sample_count: 0,
              average_latency_ms: 0,
              cost_available: false,
              cost_status: 'unavailable',
            },
          ],
          time_breakdown: [
            { label: '05-11', total_cost: 2.25, total_tokens: 1200000, request_count: 120, success_count: 119, failure_count: 1, cost_available: true, cost_status: 'available' },
            { label: '05-12', total_cost: 2.25, total_tokens: 3100000, request_count: 221, success_count: 215, failure_count: 6, cost_available: false, cost_status: 'partial' },
          ],
        }))
      }
      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<App />)

    expect(screen.getByRole('heading', { name: 'Usage and Cost workspace' })).toBeInTheDocument()
    expect(screen.getByText('Cost and Token Trend')).toBeInTheDocument()
    expect(await screen.findByText('$0.49')).toBeInTheDocument()
    expect(screen.getByText('2.1M')).toBeInTheDocument()
    expect(screen.getByText('301')).toBeInTheDocument()
    expect(screen.getByText('98.3%')).toBeInTheDocument()
    expect(screen.getByText('Cost partial')).toBeInTheDocument()
    expect(screen.getByText('Key Alias Ranking')).toBeInTheDocument()
    expect(screen.getByText('Request Health Timeline')).toBeInTheDocument()
    expect(screen.getAllByText('Shared Alias')).toHaveLength(2)
    expect(screen.getByText('sk-a*******3456 · OpenAI')).toBeInTheDocument()
    expect(screen.getByText('sk-b*******3456 · Anthropic')).toBeInTheDocument()
    expect(screen.getByText('Very Long Key Alias Label That Should Stay Inside The Ranking Row Without Breaking Layout')).toBeInTheDocument()
    expect(screen.getByText('Cost unavailable')).toBeInTheDocument()
    expect(screen.getByText('Deleted')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Model' }))
    expect(screen.getByText('Model Distribution')).toBeInTheDocument()
    expect(screen.getByText('gpt-5.5')).toBeInTheDocument()
    expect(screen.getByText(/OpenAI/)).toBeInTheDocument()
    expect(screen.getByText('200ms avg')).toBeInTheDocument()
    expect(screen.getByText('missing-price-model')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Time' }))
    expect(screen.getByText('Time Breakdown')).toBeInTheDocument()
    expect(screen.getByText('221 requests · 6 failures')).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/analytics/summary?range=7d')
  })

  it('renders an empty key alias ranking state', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/v1/analytics/summary')) {
        return new Response(JSON.stringify({
          summary: {
            total_cost: 0,
            total_tokens: 0,
            request_count: 0,
            success_count: 0,
            failure_count: 0,
            success_rate: 0,
            cost_available: true,
            cost_status: 'available',
          },
          trend: [],
          key_alias_breakdown: [],
          model_distribution: [],
          time_breakdown: [],
        }))
      }
      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<App />)

    expect(await screen.findByText('No key alias usage in this range')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Model' }))
    expect(screen.getByText('No model usage in this range')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Time' }))
    expect(screen.getByText('No time bucket usage in this range')).toBeInTheDocument()
  })

  it('renders unavailable analytics cost as unknown instead of zero currency', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/v1/analytics/summary')) {
        return new Response(JSON.stringify({
          range: '7d',
          range_start: '2026-05-06T00:00:00Z',
          range_end: '2026-05-13T00:00:00Z',
          timezone: 'UTC',
          summary: {
            total_cost: 0,
            total_tokens: 1000,
            request_count: 1,
            success_count: 1,
            failure_count: 0,
            success_rate: 100,
            cost_available: false,
            cost_status: 'unavailable',
          },
          trend: [],
        }))
      }
      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<App />)

    expect(await screen.findByText('Cost unavailable')).toBeInTheDocument()
    expect(screen.queryByText('$0.00')).not.toBeInTheDocument()
  })

  it('prefixes navigation links with the configured application base path', () => {
    window.__APP_BASE_PATH__ = '/cpa'

    render(<App />)

    expect(screen.getByRole('link', { name: 'Key 管理 Keys' })).toHaveAttribute('href', '/cpa/keys')
    expect(screen.getByRole('link', { name: '请求明细 Events' })).toHaveAttribute('href', '/cpa/events')
  })

  it('renders the Keys workspace with alias search and inline editing', async () => {
    window.history.replaceState({}, '', '/keys')
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      if (url.includes('/api/v1/usage/identities/page') && url.includes('page=1')) {
        return new Response(JSON.stringify({
          identities: [
            {
              id: 42,
              name: 'OpenAI Team',
              displayName: 'OpenAI Team',
              alias: 'Agent Research',
              auth_type: 2,
              auth_type_name: 'apikey',
              identity: 'sk-cpa...7A91',
              type: 'openai',
              provider: 'OpenAI',
              total_tokens: 4920000,
              total_cost: 18.45,
              cost_available: true,
              last_used_at: '2026-05-13T08:00:00Z',
            },
          ],
          total_count: 2,
          page: 1,
          page_size: 100,
          total_pages: 2,
        }))
      }
      if (url.includes('/api/v1/usage/identities/page') && url.includes('page=2')) {
        return new Response(JSON.stringify({
          identities: [
            {
              id: 43,
              name: 'Claude Desktop',
              displayName: 'Claude Desktop',
              alias: '',
              auth_type: 1,
              auth_type_name: 'oauth',
              identity: 'auth-index-1',
              type: 'claude',
              provider: 'Anthropic',
              total_tokens: 2400,
              total_cost: 0,
              cost_available: false,
              last_used_at: null,
            },
          ],
          total_count: 2,
          page: 2,
          page_size: 100,
          total_pages: 2,
        }))
      }
      if (url.includes('/api/v1/usage/identities/42/alias') && init?.method === 'PUT') {
        return new Response(JSON.stringify({ alias: 'Research Ops' }))
      }
      if (url.includes('/api/v1/usage/identities/42/alias') && init?.method === 'DELETE') {
        return new Response(null, { status: 204 })
      }
      return new Response(null, { status: 404 })
    })
    vi.stubGlobal('fetch', fetchMock)

    render(<App />)

    expect(await screen.findByRole('heading', { name: 'Key Management' })).toBeInTheDocument()
    expect(screen.getByText('Agent Research')).toBeInTheDocument()
    expect(screen.getByText('sk-cpa...7A91')).toBeInTheDocument()
    expect(screen.getByText('OpenAI')).toBeInTheDocument()
    expect(screen.getByText('Claude Desktop')).toBeInTheDocument()
    expect(screen.getByText('4.92M tokens')).toBeInTheDocument()
    expect(screen.getByText('$18.45')).toBeInTheDocument()
    expect(fetchMock).toHaveBeenCalledWith('/api/v1/usage/identities/page?page=2&page_size=100')

    fireEvent.change(screen.getByPlaceholderText('Search alias or key'), { target: { value: 'agent' } })
    expect(screen.getByText('Agent Research')).toBeInTheDocument()
    expect(screen.queryByText('Claude Desktop')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Edit alias for Agent Research' }))
    fireEvent.change(screen.getByLabelText('Alias for Agent Research'), { target: { value: 'Research Ops' } })
    fireEvent.click(screen.getByRole('button', { name: 'Save alias for Agent Research' }))

    fireEvent.change(screen.getByPlaceholderText('Search alias or key'), { target: { value: '' } })
    await waitFor(() => expect(screen.getByText('Research Ops')).toBeInTheDocument())
    fireEvent.click(screen.getByRole('button', { name: 'Clear alias for Research Ops' }))

    await waitFor(() => expect(screen.getByText('OpenAI Team')).toBeInTheDocument())
  })
})
