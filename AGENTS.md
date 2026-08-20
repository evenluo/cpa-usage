# AI Collaboration Notes

## 默认语言与沟通风格

1. 默认使用中文交流，必要术语可保留英文。
2. 默认给出：结论、关键依据、可验证结果。
3. 如果有选项，标注该选项最适配于什么目的。

## 输出格式约定

1. 先给结果，再给必要细节。
2. 代码说明尽量附文件路径与关键行号。
3. review 场景优先列问题清单（按严重度排序），再给摘要。
4. 除非用户要求，不输出与当前目标无关的长篇扩展。

## 代码决策、兼容性与命名约束

1. 不要基于猜测添加 fallback、兼容分支、保底逻辑或旧路径支持。
2. 当改动涉及外部可观察行为变化时，应单独说明兼容性影响或兼容性决策。
3. fallback 只应在存在明确失败模型、运行时不确定性、迁移阶段约束或外部依赖不稳定时引入。
4. 命名必须表达真实语义，优先体现业务角色、数据来源、生命周期阶段或策略差异。
5. 避免使用 `new`、`old`、`legacy`、`temp`、`misc` 等缺乏语义的信息作为主要命名。

## Agent skills

Before scoped ADR or design docs, read `docs/project/contract.md` for the repository-wide project contract and `docs/project/layout.md` for current ownership boundaries.

### Issue tracker

Issues and PRDs are tracked in GitHub Issues for `evenluo/cpa-usage`. See `docs/agents/issue-tracker.md`.

### Triage labels

The repository uses the default mattpocock/skills triage label vocabulary. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repo with root `CONTEXT.md`, architecture decisions in `docs/adr/`, and project-level SoT docs in `docs/project/`. See `docs/agents/domain.md`.

## Cursor Cloud specific instructions

Services and standard commands live in the `Makefile` and `README.md` (`make dev-backend`, `make dev-frontend`, `make verify-backend`, `make verify-frontend`, `make build-frontend`). The startup update script already runs `npm --prefix web ci` and `go mod download`, so dependencies are ready. Non-obvious caveats:

- The backend refuses to boot without `CPA_BASE_URL` and `CPA_MANAGEMENT_KEY` set (config validation in `internal/config/config.go`), even though it does not connect to CPA at startup. Create a local `.env` (gitignored) from `.env.example` and set placeholder values; keep `AUTH_ENABLED=false` for local work so no login is required.
- One Go process serves both the JSON API and the embedded built SPA from `web/dist` on `APP_PORT` (default `8080`). `web/dist` ships as only a `.gitkeep`, so run `make build-frontend` before the Go server can serve any UI. Rebuild after frontend changes; the Go server embeds `web/dist` at build time and does not hot-reload frontend assets.
- `make dev-frontend` (Vite on `:5173`) has no API proxy, so its `/api/v1/*` calls do not reach the backend. Use the Go server on `:8080` for any end-to-end UI testing; use Vite only for isolated frontend/HMR iteration.
- Without a reachable CPA service the background poller and metadata sync log continuous `connection refused` errors — this is expected and non-fatal; the server keeps serving. All dashboard data originates from CPA usage events, so a fresh local DB shows an empty dashboard.
- Reference Data has data dependencies: a Cost Rate can only be saved for a model that already appears in usage data (otherwise the API returns `model "..." has not been used`), and Key Aliases attach to keys/identities derived from usage events. To exercise these locally without a live CPA, seed `usage_events` into the SQLite DB at `WORK_DIR/app.db` (e.g. decode sample queue messages via `service.DecodeRedisUsageMessage` and insert with `repository.InsertUsageEvents`).
