# Task 14 Prompt — 单二进制打包、Docker 化与端到端验收

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 14「单二进制打包、Docker 化与端到端验收」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 14
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的构建与部署部分

前序任务简报
- Task 01 到 Task 13 已完成后端、前端、实时任务、资源管理、备份恢复、通知审计和业务页面主流程。
- 当前距离可交付版本还差三块：前端静态资源嵌入 Go 二进制、Docker/Compose 打包、端到端验收脚本与文档。

当前任务目标
- 实现 `embed.FS` 静态资源服务与 SPA fallback。
- 调整 Vite 构建输出到可嵌入目录，并在 Go 构建阶段纳入产物。
- 完善 Docker 多阶段构建、Compose、README 和运行说明。
- 编写 Playwright 关键路径用例，并跑通完整验收矩阵。

当前任务实现指导
- 本任务是打包与验收任务，不再扩展新的业务功能。
- 静态资源嵌入推荐集中到独立的 `internal/webui` 包，避免跨目录 `embed` 的路径问题。
- SPA fallback 只作用于非 API 路由，不能干扰 `/api/*` 和 WebSocket 路径。
- Docker 运行时镜像要安装 `rsync` 和 `openssh-client`，并保持镜像尽量精简。
- Playwright 用例只覆盖关键路径：登录与仪表盘、手动备份主流程、恢复二次确认主流程。
- 验收时只报告真实跑过的命令与结果，不要臆测“应当通过”。

关键方法/接口定义
```go
//go:embed dist
var Dist embed.FS

func RegisterStatic(router *gin.Engine)
```

```go
func RegisterRoutes(router *gin.Engine)
func NewServer(cfg config.Config) *http.Server
```

```ts
export default defineConfig({
  build: {
    outDir: "../internal/webui/dist",
    emptyOutDir: true,
  },
})
```

单元测试要求
- 先补静态资源注册或嵌入相关 smoke test，再完善 Playwright 关键路径与打包逻辑。
- 至少覆盖：静态资源 fallback 基础路径、关键 E2E 登录路径、恢复二次确认路径；如可行，也补充最小打包相关 smoke test。
- 如果当前环境不具备 Docker、Playwright 浏览器或 Linux 运行条件，可使用 `askQuestion` 请求用户协助执行环境相关验证，并明确命令、预期结果和需要回传的信息。

验收目标
- `go test ./...` 通过。
- `npm --prefix web run test` 通过。
- `npm --prefix web run build` 通过。
- `npm --prefix web run test:e2e` 通过。
- `docker compose build` 通过。
- 本任务不再引入新的业务接口或页面需求。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：静态资源嵌入路径是否可靠、SPA fallback 是否不会污染 API、Docker 多阶段构建是否正确、验收结论是否基于真实结果。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 14 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./...`、`npm --prefix web run test`、`npm --prefix web run build`、`npm --prefix web run test:e2e`、`docker compose build`。
- 如果某些命令因环境依赖不足需要用户协助执行，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤、前提和预期结果。

执行与反馈要求
- 开始前先说明“静态资源嵌入”“Docker 打包”“端到端验收”三块工作边界。
- 完成后总结嵌入方式、镜像构建方式、E2E 覆盖面和真实验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到环境依赖缺失、需要用户配合调试或其他阻塞，请暂停并提问。
