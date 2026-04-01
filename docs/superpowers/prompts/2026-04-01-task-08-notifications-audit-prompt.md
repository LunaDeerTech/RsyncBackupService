# Task 08 Prompt — 通知渠道、用户订阅与审计查询

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 08「通知渠道、用户订阅与审计查询」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 08
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的通知系统与审计日志部分

前序任务简报
- Task 01 到 Task 07 已完成工程骨架、模型、认证授权、资源管理、调度、滚动/冷备份和恢复主链路。
- 当前已经有通知相关模型和审计日志记录基础，但尚未实现 SMTP 发送、用户订阅和审计查询 API。

当前任务目标
- 定义通知接口并实现首个 `SMTPNotifier`。
- 实现通知渠道 CRUD、实例级订阅管理和任务完成后的通知分发服务。
- 实现审计日志筛选查询服务与 API。
- 编写 SMTP 校验/重试和通知服务相关测试，并让它们通过。

当前任务实现指导
- 本任务只实现 SMTP 邮件通知，不实现 Webhook、Telegram 等扩展渠道。
- SMTP 发送要支持配置校验、30 秒超时和最多 3 次指数退避重试。
- 用户订阅是实例级配置，发送前应再次校验用户对该实例是否仍有权限。
- 审计查询先实现分页与基础过滤，不扩展成复杂报表。
- 不要把 WebSocket、系统仪表盘或前端页面一起塞进本任务。

关键方法/接口定义
```go
type Notifier interface {
    Type() string
    Send(ctx context.Context, event NotifyEvent) error
    Validate(config json.RawMessage) error
}

type NotifyEvent struct {
    Type       string
    Instance   string
    Strategy   string
    Message    string
    Detail     any
    OccurredAt time.Time
}
```

```go
func (n *SMTPNotifier) Send(ctx context.Context, event NotifyEvent) error
func (n *SMTPNotifier) Validate(config json.RawMessage) error
func (s *NotificationService) Notify(ctx context.Context, event NotifyEvent) error
func (s *AuditService) List(ctx context.Context, req ListAuditLogsRequest) ([]model.AuditLog, int64, error)
```

单元测试要求
- 先补 SMTP 配置校验、重试逻辑、订阅过滤和审计查询测试，再实现通知与审计能力。
- 至少覆盖：SMTP 配置非法、发送重试成功或失败、实例订阅过滤、审计分页与筛选。
- 如果需要真实 SMTP 服务或用户协助执行一次测试通知验证，可使用 `askQuestion` 请求帮助，并明确所需配置、步骤和预期邮件/结果。

验收目标
- `go test ./internal/notify ./internal/service ./internal/api/... -v` 通过。
- SMTP 渠道具备配置校验和重试能力。
- 通知渠道 CRUD、实例订阅管理和审计查询 API 可用。
- 本任务不提前实现 WebSocket 推送、仪表盘聚合或前端页面。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：通知配置是否泄露敏感信息、重试是否可控、订阅权限是否正确、审计查询是否高内聚。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 08 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/notify ./internal/service ./internal/api/... -v`，并明确回归通知校验、重试逻辑、订阅管理和审计查询。
- 如果需要用户协助发送真实测试通知，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前说明“通知渠道”“用户订阅”“审计查询”三块能力的边界。
- 完成后总结 notifier 设计、订阅分发逻辑、审计查询能力和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到渠道配置语义不清、需要确认，或被外部依赖阻塞，请先提问。
