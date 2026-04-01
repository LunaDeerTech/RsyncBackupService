# Task 09 Prompt — HTTP API 汇总、系统接口与 WebSocket 实时推送

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 09「HTTP API 汇总、系统接口与 WebSocket 实时推送」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 09
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的任务与进度、系统状态、仪表盘 API 设计部分

前序任务简报
- Task 01 到 Task 08 已完成工程骨架、模型、认证、资源管理、调度、备份/恢复主链路、通知和审计查询基础。
- 当前执行器已经具备产生日志和任务状态的基础，但还没有统一的实时进度广播、运行任务接口和系统聚合接口。

当前任务目标
- 实现 WebSocket Hub 与进度广播总线。
- 实现运行任务列表、取消任务、系统状态和仪表盘聚合 API。
- 将执行器中的进度事件与 WebSocket 推送串联起来。
- 编写运行任务接口与 WebSocket 广播测试，并让它们通过。

当前任务实现指导
- 本任务聚焦 API 汇总和实时事件通路，不要扩展到前端页面或组件实现。
- WebSocket 消息体保持精简，优先承载任务标识、实例标识、百分比、速率、剩余时间和状态。
- 运行任务列表数据应来自 Task 05 的任务注册表，而不是重新扫描数据库。
- 取消任务应复用任务注册表中的 cancel 句柄，不要在 handler 中拼接执行逻辑。
- 仪表盘聚合接口只先满足设计文档定义的统计项，不扩展到复杂报表。

关键方法/接口定义
```go
type ProgressEvent struct {
    TaskID        string  `json:"task_id"`
    InstanceID    uint    `json:"instance_id"`
    Percentage    float64 `json:"percentage"`
    SpeedText     string  `json:"speed_text"`
    RemainingText string  `json:"remaining_text"`
    Status        string  `json:"status"`
}
```

```go
func NewHub() *Hub
func (h *Hub) Register(client *Client)
func (h *Hub) Unregister(client *Client)
func (h *Hub) Broadcast(event ProgressEvent)
```

```go
func (s *ExecutorService) PublishProgress(event ProgressEvent)
func (s *DashboardService) GetSystemStatus(ctx context.Context) (SystemStatus, error)
func (s *DashboardService) GetDashboard(ctx context.Context) (DashboardSummary, error)
```

单元测试要求
- 先补 WebSocket Hub、运行任务接口和系统聚合测试，再实现实时 API 链路。
- 至少覆盖：Hub 广播、client 注册/注销、运行任务列表、取消任务、系统或仪表盘基础聚合。
- 如果需要用户协助做一次真实 WebSocket 或运行中任务验证，可使用 `askQuestion` 请求帮助，并明确步骤、预期消息和你需要确认的字段。

验收目标
- `go test ./internal/api/... ./internal/service -v` 通过。
- 运行任务接口能返回内存中的任务注册表信息。
- WebSocket Hub 能把进度事件广播给已注册客户端。
- 本任务不提前实现前端消费逻辑。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：并发广播安全、client 清理时机、任务数据来源是否正确、API 是否越界承担业务逻辑。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 09 验收回归 prompt”。
- 这段回归 prompt 至少包含：`go test ./internal/api/... ./internal/service -v`，并明确回归 WS 广播、运行任务列表、取消任务和系统状态。
- 如果需要用户协助做一次真实 WS 验证，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先说明实时事件通路的组成：执行器、任务注册表、Hub、HTTP/WS 接口。
- 完成后总结新增接口、广播机制和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到数据来源或接口范围需要确认，请暂停并提问。
