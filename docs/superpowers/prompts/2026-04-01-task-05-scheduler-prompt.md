# Task 05 Prompt — 调度器、运行任务注册表与冲突控制

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 05「调度器、运行任务注册表与冲突控制」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 05
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的调度与任务冲突处理要求

前序任务简报
- Task 01 到 Task 04 已建立工程骨架、数据库模型、认证授权以及实例/策略/存储/SSH 资源管理能力。
- 当前已经有策略实体和资源绑定关系，但还没有真正的调度注册、任务锁和取消机制。

当前任务目标
- 实现调度器封装，支持 cron 与 interval 两种注册方式。
- 实现运行任务注册表、取消句柄和实例+存储目标级别的冲突锁。
- 实现策略变更后的调度刷新能力。
- 编写调度与冲突跳过测试，并让它们通过。

当前任务实现指导
- 本任务只建立调度与任务生命周期管理框架，不实现实际的 rsync、打包或恢复执行逻辑。
- 冲突控制的基本规则是：同一实例+同一存储目标，同一时间只能运行一个任务。
- 调度触发遇到冲突应跳过并记录；手动触发遇到冲突应能向上层返回冲突结果。
- 调度层要与策略服务解耦，策略服务只负责在增删改后调用刷新接口。
- 保持 scheduler 和 executor/task registry 的职责清晰，不要把执行细节混入调度器。

关键方法/接口定义
```go
type RunningTask struct {
    ID        string
    LockKey   string
    StartedAt time.Time
    Cancel    context.CancelFunc
}

func (m *TaskManager) TryStart(lockKey string, cancel context.CancelFunc) (RunningTask, bool)
func (m *TaskManager) Finish(taskID string)
func (m *TaskManager) Cancel(taskID string) error
```

```go
func (s *Scheduler) RegisterStrategy(strategy model.Strategy, run func(context.Context) error) error
func (s *Scheduler) RemoveStrategy(strategyID uint) error
func (s *SchedulerService) RefreshStrategy(strategy model.Strategy) error
```

单元测试要求
- 先补调度注册表、任务注册表和冲突控制测试，再实现对应逻辑。
- 至少覆盖：重复 lock key 任务不可同时启动、任务可取消、策略刷新后调度注册更新。
- 如果需要用户协助做一次时间相关或长任务的最小人工验证，可使用 `askQuestion` 请求帮助，并明确测试步骤与预期行为。

验收目标
- `go test ./internal/scheduler ./internal/executor -v` 通过。
- 冲突场景下第二个同 lock key 任务不能启动。
- 策略创建或变更后可以正确刷新调度注册，而不提前实现实际备份执行。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：并发安全、锁释放时机、重复注册问题、取消逻辑是否可预测。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 05 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/scheduler ./internal/executor -v`，并明确检查调度刷新、冲突跳过和取消行为。
- 如果需要用户协助做一次手工调度验证，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先说明调度器、任务注册表、冲突锁三者的职责边界。
- 完成后总结调度刷新机制、冲突控制机制和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如需确认调度语义或遇到阻塞，请暂停并提问，不要跳到 Task 06 的执行器实现。
