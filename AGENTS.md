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

### Issue tracker

Issues and PRDs are tracked in GitHub Issues for `evenluo/cpa-usage`. See `docs/agents/issue-tracker.md`.

### Triage labels

The repository uses the default mattpocock/skills triage label vocabulary. See `docs/agents/triage-labels.md`.

### Domain docs

This is a single-context repo with root `CONTEXT.md` and architecture decisions in `docs/adr/`. See `docs/agents/domain.md`.
