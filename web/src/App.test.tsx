import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import App from './App'

describe('App', () => {
  it('renders the CPA Usage shell with primary navigation', () => {
    render(<App />)

    expect(screen.getByRole('heading', { name: 'CPA Usage' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '数据分析 Analytics' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Key 管理 Keys' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '请求明细 Events' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '计价配置 Pricing' })).toBeInTheDocument()
    expect(screen.getByRole('link', { name: '系统设置 Settings' })).toBeInTheDocument()
  })
})
