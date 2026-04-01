# Task 04 Prompt — 备份实例、策略、存储目标与 SSH 密钥管理

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 04「备份实例、策略、存储目标与 SSH 密钥管理」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 04
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的实例、策略、存储目标、SSH 密钥与 API 设计部分

前序任务简报
- Task 01 已搭建后端/前端骨架和构建测试链路。
- Task 02 已完成模型、SQLite 初始化和管理员播种。
- Task 03 已实现认证、权限和审计基础能力，可保护管理类 API。

当前任务目标
- 实现备份实例、策略、存储目标、SSH 密钥的 repository、service 和 handler。
- 定义 `StorageBackend` 接口，并先提供本地与 SSH 两种实现。
- 落实关键业务校验，例如 `cron_expr` 与 `interval_seconds` 互斥、保留策略字段有效性、SSH 密钥文件权限为 `0600`。
- 暴露资源 CRUD 与连通性测试 API，并通过相关测试。

当前任务实现指导
- 当前任务只聚焦资源管理与静态校验，不实现调度、任务执行、滚动/冷备份逻辑。
- `StorageBackend` 的实现以连通性检查和基础文件操作能力为主，真正的备份执行留到后续任务。
- 策略与存储目标的多对多绑定关系要清晰，但不要在本任务引入执行编排。
- SSH 密钥数据库只保存名称、指纹、路径等元数据，不暴露完整敏感路径到普通 API 响应。
- 资源 API 要正确接入认证和权限校验。

关键方法/接口定义
```go
type StorageBackend interface {
    Type() string
    Upload(ctx context.Context, localPath, remotePath string) error
    Download(ctx context.Context, remotePath, localPath string) error
    List(ctx context.Context, prefix string) ([]StorageObject, error)
    Delete(ctx context.Context, path string) error
    SpaceAvailable(ctx context.Context, path string) (uint64, error)
    TestConnection(ctx context.Context) error
}
```

```go
func (s *StrategyService) ValidateCreate(req CreateStrategyRequest) error
func (s *SSHKeyService) Register(ctx context.Context, name, privateKeyPath string) error
func (s *StorageTargetService) TestConnection(ctx context.Context, id uint) error
```

单元测试要求
- 先补资源校验测试，再实现实例、策略、存储目标和 SSH 密钥相关逻辑。
- 至少覆盖：`cron_expr` 与 `interval_seconds` 互斥、保留策略字段合法性、SSH 密钥文件权限校验、资源 API 的基本授权路径。
- 如果某些连通性校验需要真实 SSH 主机、真实路径或用户协助，可使用 `askQuestion` 请求用户提供测试条件或执行人工验证，并明确步骤与预期结果。

验收目标
- `go test ./internal/service ./internal/storage ./internal/api/... -v` 通过。
- 关键校验项能被测试覆盖并正确触发。
- 资源 CRUD 与连通性测试 API 可用，且不提前进入调度/执行器逻辑。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：资源边界是否清晰、敏感路径是否泄露、校验规则是否完整、存储后端接口是否过度设计。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 04 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/service ./internal/storage ./internal/api/... -v`，并明确回归实例、策略、存储目标、SSH 密钥四类资源路径。
- 如果需要用户协助执行真实连通性测试，也要在回归 prompt 中写出可通过 `askQuestion` 请求的验证步骤。

执行与反馈要求
- 开始前列出本任务会落地的四类资源及其边界。
- 完成后汇总新增 service/repository/handler、关键校验和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如发现设计文档有歧义、需要用户确认，或遇到阻塞，请先提问。
