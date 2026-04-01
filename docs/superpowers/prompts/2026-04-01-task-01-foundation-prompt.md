# Task 01 Prompt — 仓库骨架与开发工具链

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 01「仓库骨架与开发工具链」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 01
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 的总体架构与目录结构部分

前序任务简报
- 无。当前仓库仍处于文档阶段。
- 本任务的目标是建立后续所有后端、前端、测试与构建工作的基础骨架。

当前任务目标
- 搭建 Go 后端入口、配置加载和应用启动骨架。
- 搭建 Vue 3 + Vite 前端壳层、基础路由和主题样式入口。
- 补齐 `.env.example`、`Makefile`、`Dockerfile`、`docker-compose.yml` 的最小可用版本。
- 编写最小后端与前端冒烟测试，并让它们通过。

当前任务实现指导
- 本任务只做工程骨架，不实现数据库模型、认证、业务服务或页面业务逻辑。
- 后端先围绕 `cmd/server`、`internal/app`、`internal/config` 建立最小闭环。
- `Config` 至少包含 `Port`、`DataDir`、`JWTSecret`、`AdminUser`、`AdminPassword`。
- 前端先保证 `App.vue` 能渲染 `RouterView`，并接入 reset、token、light/dark theme 样式文件。
- Vite 和 Vitest 配置保持简洁，优先保证可运行、可测试、可构建。
- 不要提前引入业务依赖或实现 Task 02 之后的内容。

关键方法/接口定义
```go
type Config struct {
    Port          int
    DataDir       string
    JWTSecret     string
    AdminUser     string
    AdminPassword string
}

func Load() (Config, error)
```

```go
func New(cfg config.Config) *App
func (a *App) Run() error
```

```ts
export function createRouter(): Router
```

单元测试要求
- 先补配置加载测试和前端应用壳层冒烟测试，再实现对应骨架代码。
- 至少覆盖配置缺失校验、`App.vue` 根渲染、基础路由装配这三条最小路径。
- 若当前环境缺少 Go/Node 依赖，或需要用户协助做一次最小启动验证，可使用 `askQuestion` 明确请求用户协助测试，并说明执行步骤、预期结果和你需要回收的信息。

验收目标
- `go test ./internal/config -v` 通过。
- `npm --prefix web run test -- src/App.spec.ts` 通过。
- `npm --prefix web run build` 通过。
- 变更范围仅限 Task 01 相关文件，不提前实现 Task 02 之后的能力。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：配置校验是否清晰、脚手架是否越界、构建脚本是否与后续任务兼容、是否引入了不必要依赖。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 01 验收回归 prompt”。
- 这段回归 prompt 只覆盖 Task 01 及其直接影响范围，至少包含：`go test ./internal/config -v`、`npm --prefix web run test -- src/App.spec.ts`、`npm --prefix web run build`。
- 如果你认为还需要用户协助做最小启动验证，也要在这段回归 prompt 中明确写出可通过 `askQuestion` 请求的步骤和预期现象。

执行与反馈要求
- 开始前先用 3 到 5 条要点重述你对本任务边界的理解。
- 完成后给出本任务修改摘要、运行过的验证命令和结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如发现计划与仓库现实不一致、需要确认，或遇到阻塞，请先暂停并提问，不要猜测。
