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

  it('renders the HITL analytics prototype sections', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: 'Usage and Cost workspace' })).toBeInTheDocument()
    expect(screen.getByText('Cost and Token Trend')).toBeInTheDocument()
    expect(screen.getByText('Key Alias Ranking')).toBeInTheDocument()
    expect(screen.getByText('Model Distribution')).toBeInTheDocument()
    expect(screen.getByText('Request Health Timeline')).toBeInTheDocument()
    expect(screen.getAllByText('Agent Research')).toHaveLength(2)
    expect(screen.getByText('sk-cpa...7A91 · codex')).toBeInTheDocument()
    expect(screen.getByText('$1,006')).toBeInTheDocument()
    expect(screen.getByText('12.27M')).toBeInTheDocument()
    expect(screen.getByText('4.92M tokens')).toBeInTheDocument()
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
