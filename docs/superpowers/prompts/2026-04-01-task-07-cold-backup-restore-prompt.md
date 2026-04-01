# Task 07 Prompt — 冷备份、恢复与危险操作保护

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 07「冷备份、恢复与危险操作保护」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 07
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的冷备份、恢复流程和二次认证要求

前序任务简报
- Task 01 到 Task 06 已完成工程骨架、数据库模型、认证授权、资源管理、调度框架和滚动备份执行链路。
- 当前已经有执行器基础设施、runner 抽象和任务记录模型，但尚未支持 tar 压缩归档、分卷上传或恢复流程。

当前任务目标
- 实现冷备份的打包、可选分卷和上传流程。
- 实现快照/归档恢复服务，并将敏感恢复操作绑定到 verify token 校验。
- 暴露备份历史、可恢复快照/归档列表和恢复发起接口。
- 编写冷备份与恢复相关测试，并让它们通过。

当前任务实现指导
- 本任务只聚焦冷备份和恢复主链路，不实现通知发送或 WebSocket 推送细节。
- 归档格式按设计文档支持 `tar.gz` 和 `split` 分卷模式。
- 恢复必须区分滚动快照恢复与冷备份归档恢复，但二者都需要写入 `RestoreRecord`。
- 敏感恢复接口必须依赖 Task 03 中的一次性 verify token，而不是仅靠普通 JWT。
- 备份历史与恢复列表 API 先满足当前页面/后续任务需求，不要扩展成复杂搜索系统。

关键方法/接口定义
```go
func BuildArchiveCommand(sourceDir, outputBase string, volumeSize *string) CommandSpec
func (e *ColdExecutor) Run(ctx context.Context, req ColdBackupRequest) error
```

```go
type RestoreRequest struct {
    InstanceID         uint
    BackupRecordID     uint
    RestoreTargetPath  string
    Overwrite          bool
    VerifyToken        string
}

func (s *RestoreService) Start(ctx context.Context, req RestoreRequest) (*model.RestoreRecord, error)
func (s *RestoreService) List(ctx context.Context, req ListRestoreRecordsRequest) ([]model.RestoreRecord, error)
```

单元测试要求
- 先补归档命令构建、恢复校验和相关 handler 测试，再实现冷备份与恢复逻辑。
- 至少覆盖：无分卷归档、分卷归档、恢复必须要求 verify token、恢复记录持久化。
- 如果需要真实 `tar`、`split`、文件路径或用户协助进行恢复演练，可使用 `askQuestion` 请求帮助，并明确步骤、风险提示和预期结果。

验收目标
- `go test ./internal/executor ./internal/service ./internal/api/... -v` 通过。
- 冷备份支持不分卷和分卷两种归档构建方式。
- 恢复必须要求 verify token，且能正确写入 `RestoreRecord`。
- 本任务不提前实现通知、审计查询或实时推送。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：危险操作保护是否足够、临时文件清理是否完整、恢复目标路径处理是否安全、记录写入是否一致。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 07 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/executor ./internal/service ./internal/api/... -v`，并明确回归归档构建、verify token 保护、恢复记录落库这三类路径。
- 如果需要用户协助做一次最小恢复演练，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先区分本任务中“冷备份”和“恢复”两条子链路的职责。
- 完成后总结归档构建、恢复校验、API 暴露和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到恢复语义不清、需要确认，或被环境问题阻塞，请暂停并提问。
