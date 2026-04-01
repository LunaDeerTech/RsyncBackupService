# Task 03 Prompt — 认证、权限与审计中间件基础

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 03「认证、权限与审计中间件基础」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 03
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的认证、安全与权限模型部分

前序任务简报
- Task 01 已建立工程骨架、配置加载和基础测试构建链路。
- Task 02 已建立 SQLite、核心模型、迁移与首个管理员播种逻辑。
- 当前已经具备落地认证服务、权限检查和审计中间件的基础数据结构。

当前任务目标
- 实现密码哈希、JWT access/refresh token、一次性 verify token 机制。
- 实现 `RequireJWT`、`RequireAdmin`、`RequireInstanceRole`、审计日志等中间件。
- 暴露认证相关 API，以及管理员用户管理/权限管理基础 API。
- 编写登录、verify 和权限拦截测试，并让它们通过。

当前任务实现指导
- access token 与 refresh token 的职责要分离，不要把敏感逻辑散落在 handler 中。
- verify token 只用于敏感操作的二次认证，生命周期短且校验独立。
- 超级管理员应具备全局绕过权限；实例级权限从 `instance_permissions` 表判断。
- 审计中间件先覆盖需要记录的管理操作，不必在本任务实现审计查询接口。
- 本任务聚焦认证/权限/审计基础，不实现实例、策略、存储目标 CRUD。

关键方法/接口定义
```go
type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

func (s *AuthService) Login(ctx context.Context, username, password string) (TokenPair, error)
func (s *AuthService) Refresh(ctx context.Context, refreshToken string) (TokenPair, error)
func (s *AuthService) VerifyPassword(ctx context.Context, userID uint, password string) (string, error)
```

```go
func RequireJWT() gin.HandlerFunc
func RequireAdmin() gin.HandlerFunc
func RequireInstanceRole(minRole string) gin.HandlerFunc
func RequireVerifyToken() gin.HandlerFunc
func AuditLogger(repo repository.AuditLogRepository) gin.HandlerFunc
```

单元测试要求
- 先补登录、refresh、verify token 和中间件拦截测试，再实现认证与权限逻辑。
- 至少覆盖：成功登录、缺失 token 拦截、verify token 生成、管理员或实例权限校验。
- 如果需要用户协助确认某个接口的真实交互行为，可使用 `askQuestion` 请求用户辅助测试，并明确请求的接口、步骤和预期响应。

验收目标
- `go test ./internal/service ./internal/api/... -v` 通过。
- 登录可签发 access/refresh token，verify 接口可生成一次性 verify token。
- 缺少 token 的敏感路由会被正确拦截。
- 本任务不提前实现资源 CRUD、调度或执行器逻辑。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：认证边界是否安全、token 生命周期是否清晰、权限是否可能泄漏、审计中间件是否带来副作用。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 03 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/service ./internal/api/... -v`，并明确检查登录、verify token、权限拦截这三类行为。
- 如果需要用户协助用真实请求做一次最小 API 验证，也要在回归 prompt 中写出通过 `askQuestion` 发起协助的方式。

执行与反馈要求
- 开始前说明认证、权限、审计三部分的边界。
- 完成后汇总新增 API、中间件和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如需要确认 token 结构、权限语义或遇到阻塞，请暂停并提问，不要自行扩展范围。
