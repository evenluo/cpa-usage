import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'

import App from './App'

describe('App', () => {
  const originalBasePath = window.__APP_BASE_PATH__

  afterEach(() => {
    window.__APP_BASE_PATH__ = originalBasePath
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
  })

  it('prefixes navigation links with the configured application base path', () => {
    window.__APP_BASE_PATH__ = '/cpa'

    render(<App />)

    expect(screen.getByRole('link', { name: 'Key 管理 Keys' })).toHaveAttribute('href', '/cpa/keys')
    expect(screen.getByRole('link', { name: '请求明细 Events' })).toHaveAttribute('href', '/cpa/events')
  })
})
