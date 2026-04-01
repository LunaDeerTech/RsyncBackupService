# Task 06 Prompt — 滚动备份执行链路、进度解析与保留策略

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 06「滚动备份执行链路、进度解析与保留策略」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 06
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的滚动备份、进度追踪、保留策略和错误处理部分

前序任务简报
- Task 01 到 Task 05 已建立工程骨架、数据模型、认证授权、资源管理、调度器和任务冲突控制。
- 当前已经具备执行器挂接点，但尚未实现滚动备份的 rsync 命令构建、快照规划、进度解析和保留清理。

当前任务目标
- 实现 rsync runner 抽象、滚动备份命令构建和快照/relay cache 规划。
- 实现 `--info=progress2` 输出解析、滑动窗口速度估算和 ETA 计算。
- 实现超时控制、目标空间预检查、rsync exit code 可读映射和保留策略清理。
- 编写滚动备份、进度解析和保留策略相关测试，并让它们通过。

当前任务实现指导
- 本任务只实现滚动备份路径，不实现冷备份、归档恢复或通知发送。
- 源/目标同为远程时必须采用 relay 模式，并显式规划本地缓存目录。
- 执行器要通过 runner 抽象与外部命令解耦，方便单元测试。
- 目标空间不足时只记录 warning，不要在此处强制中断任务；最终失败应由 rsync/tar 本身反馈。
- `max_execution_seconds` 应转换为 context timeout；常见 rsync 退出码要能映射成人类可读错误。
- 保留策略同时支持数量和天数，两者生效时按并集清理。

关键方法/接口定义
```go
type CommandSpec struct {
    Name string
    Args []string
    Dir  string
}

type Runner interface {
    Run(ctx context.Context, spec CommandSpec, onStdout func(string)) error
}
```

```go
func ParseProgress2(line string) (ProgressSnapshot, bool)
func EstimateRemaining(totalSize, transferred uint64, avgBytesPerSecond float64) time.Duration
func MapRsyncExitCode(code int) error
func WithExecutionTimeout(ctx context.Context, maxSeconds int) (context.Context, context.CancelFunc)
```

```go
type RollingPlan struct {
    RequiresRelay bool
    SnapshotPath  string
    RelayCacheDir string
    LinkDest      string
}

func BuildRollingPlan(instance model.BackupInstance, target model.StorageTarget) RollingPlan
func (s *ExecutorService) CheckTargetSpace(ctx context.Context, backend storage.StorageBackend, path string, estimatedSize uint64) error
```

单元测试要求
- 先补进度解析、滚动计划规划、保留策略和 exit code 映射测试，再实现执行链路。
- 至少覆盖：`progress2` 解析、remote-to-remote relay 规划、保留策略并集删除、超时包装、rsync exit code 可读映射。
- 如果需要真实 `rsync` 或 Linux 环境协助做一次最小执行验证，可使用 `askQuestion` 请求用户帮助，并明确命令、环境前提和预期现象。

验收目标
- `go test ./internal/executor ./internal/service -v` 通过。
- 进度解析、remote-to-remote relay 规划、rsync exit code 映射等关键路径都有测试覆盖。
- 保留策略可按数量/天数清理，且本任务不提前实现冷备份和恢复。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：命令构建安全性、relay 模式正确性、超时与空间检查边界、错误映射是否足够可读。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 06 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/executor ./internal/service -v`，并明确回归进度解析、滚动计划、保留策略、超时和 exit code 映射。
- 如果需要用户协助做一次真实 rsync 验证，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤和预期结果。

执行与反馈要求
- 开始前先说明滚动备份执行链路会由哪些子模块组成。
- 完成后总结 runner 抽象、进度解析、保留策略与错误处理的实现边界及验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到命令构建语义不清、需要用户确认，或被实际环境阻塞，请先提问。
