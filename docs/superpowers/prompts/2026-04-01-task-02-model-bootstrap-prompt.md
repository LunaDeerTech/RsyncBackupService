# Task 02 Prompt — 数据模型、SQLite 初始化与管理员引导

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 02「数据模型、SQLite 初始化与管理员引导」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 02
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的数据模型与首次启动要求

前序任务简报
- Task 01 已建立 Go/Vue 工程骨架、配置加载、基础构建脚本和最小测试链路。
- 当前后端已有 `cmd/server`、`internal/app`、`internal/config` 的最小启动闭环。

当前任务目标
- 定义设计文档中的核心 GORM 模型与关联关系。
- 建立 SQLite 打开、自动迁移和管理员播种逻辑。
- 在应用启动中接入数据库初始化流程。
- 编写迁移与管理员播种测试，并让它们通过。

当前任务实现指导
- 严格按设计文档定义核心表：`users`、`ssh_keys`、`backup_instances`、`storage_targets`、`strategies`、`backup_records`、`restore_records`、`notification_channels`、`notification_subscriptions`、`instance_permissions`、`audit_logs`。
- 先专注模型、数据库连接和初始化逻辑，不实现认证 API、资源 API 或调度逻辑。
- 管理员播种规则：若无用户，则按 `.env` 中的管理员账号创建首个用户，且其 `is_admin = true`。
- 保持 repository 层边界清晰，数据库启动与迁移逻辑集中在 `internal/repository`。
- 不要在本任务加入额外的业务校验或 handler。

关键方法/接口定义
```go
func OpenSQLite(dataDir string) (*gorm.DB, error)
func MigrateAndSeed(db *gorm.DB, cfg config.Config) error
func EnsureAdminUser(db *gorm.DB, username, password string) error
```

```go
type User struct {
    ID           uint
    Username     string
    PasswordHash string
    IsAdmin      bool
}
```

```go
type Strategy struct {
    ID              uint
    InstanceID      uint
    Name            string
    BackupType      string
    CronExpr        *string
    IntervalSeconds int
    RetentionDays   int
    RetentionCount  int
}
```

单元测试要求
- 先补迁移和管理员播种测试，再实现模型和数据库初始化逻辑。
- 至少覆盖：自动迁移成功、首次启动创建管理员、重复执行不重复播种。
- 如果需要用户协助验证真实初始化流程或数据目录行为，可使用 `askQuestion` 请求用户执行一次最小初始化验证，并明确步骤、预期结果和返回信息。

验收目标
- `go test ./internal/repository -v` 通过。
- `go test ./...` 在当前已实现范围内通过。
- 首次启动场景下可正确创建管理员，重复启动不会重复播种。
- 不提前实现认证、权限、资源 CRUD 或调度逻辑。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：模型字段是否与设计文档一致、表关系是否清晰、管理员播种是否幂等、数据库启动职责是否集中。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 02 验收回归 prompt”。
- 这段回归 prompt 只覆盖 Task 01 到 Task 02 的稳定性，至少包含：`go test ./internal/repository -v` 和 `go test ./...`。
- 如果需要用户协助做一次真实初始化验证，也要在回归 prompt 中写出可以通过 `askQuestion` 请求的具体步骤。

执行与反馈要求
- 开始前概述本任务会新增的模型和初始化职责。
- 完成后总结新增模型、数据库初始化流程以及验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如发现设计文档与实际目录结构不一致，先说明再处理；如需确认或遇到阻塞，暂停并提问。
