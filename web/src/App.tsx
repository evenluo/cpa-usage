import './index.css'

const navItems = [
  { label: '数据分析 Analytics', href: '/' },
  { label: 'Key 管理 Keys', href: '/keys' },
  { label: '请求明细 Events', href: '/events' },
  { label: '计价配置 Pricing', href: '/pricing' },
  { label: '系统设置 Settings', href: '/settings' },
]

function App() {
  return (
    <main className="shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <div>
          <p className="eyebrow">CPA Usage</p>
          <h1>CPA Usage</h1>
        </div>
        <nav>
          {navItems.map((item) => (
            <a key={item.href} href={item.href}>
              {item.label}
            </a>
          ))}
        </nav>
      </aside>

      <section className="workspace" aria-labelledby="workspace-title">
        <header className="workspace-header">
          <div>
            <p className="eyebrow">Application baseline</p>
            <h2 id="workspace-title">Deployable analytics shell</h2>
          </div>
          <span className="status">Go + SQLite + React + Vite</span>
        </header>

        <div className="summary-grid">
          <article>
            <span>Total Cost</span>
            <strong>Ready for pricing data</strong>
          </article>
          <article>
            <span>Total Tokens</span>
            <strong>Ready for CPA events</strong>
          </article>
          <article>
            <span>Request Events</span>
            <strong>Backend routes inherited</strong>
          </article>
        </div>

        <section className="panel">
          <p>
            This minimal shell proves the frontend can be built by Vite and served by the inherited Go backend.
            The detailed analytics visual system is intentionally deferred to the HITL design slice.
          </p>
        </section>
      </section>
    </main>
  )
}

export default App
