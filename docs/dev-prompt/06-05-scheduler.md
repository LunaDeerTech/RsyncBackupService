# 06-05 调度器

## 前序任务简报

任务队列和 Worker Pool 已完成：任务可通过队列调度到 Worker 执行，同实例互斥控制生效，取消/恢复机制就绪，Tasks API 可查询和取消任务。手动触发策略 → 创建 task → Worker 执行备份的端到端流程已打通。

## 当前任务目标

实现调度器，根据策略的调度配置（interval / cron）自动触发备份任务。

## 实现指导

### 1. 调度器（`engine/scheduler.go`）

```go
type Scheduler struct {
    db       *store.DB
    queue    *TaskQueue
    mu       sync.Mutex
    timers   map[int64]*time.Timer  // policyID → 定时器
    stopCh   chan struct{}
}

func NewScheduler(db *store.DB, queue *TaskQueue) *Scheduler

// Start 启动调度器，加载所有已启用策略的调度计划
func (s *Scheduler) Start(ctx context.Context) error

// Stop 停止调度器
func (s *Scheduler) Stop()

// ReloadPolicy 重新加载单个策略的调度（策略创建/编辑/删除/启停时调用）
func (s *Scheduler) ReloadPolicy(policyID int64)

// RemovePolicy 移除策略的调度
func (s *Scheduler) RemovePolicy(policyID int64)
```

### 2. 调度规则

**Interval 类型**：
- 计算下次触发时间 = 上次该策略备份完成时间 + interval 秒数
- 如果从未执行过，下次触发时间 = now + interval
- 如果计算出的触发时间已过期（服务长时间停机），立即触发一次

**Cron 类型**：
- 解析标准 5 字段 cron 表达式（分 时 日 月 周）
- 计算从当前时间起的下一个匹配时间点
- 需要实现或引入 cron 解析器

### 3. Cron 解析器

```go
// engine/cron.go
type CronExpr struct {
    Minutes  []int
    Hours    []int
    Days     []int
    Months   []int
    Weekdays []int
}

func ParseCron(expr string) (*CronExpr, error)
func (c *CronExpr) Next(from time.Time) time.Time
```

- 支持：`*`、`,`（列表）、`-`（范围）、`/`（步长）
- 示例：`0 2 * * *`（每天凌晨 2 点）、`*/30 * * * *`（每 30 分钟）
- 可选择引入成熟库如 `github.com/robfig/cron/v3` 的表达式解析部分

### 4. 调度循环

```
for 每个已启用策略:
    计算 nextTriggerTime
    设置 time.AfterFunc(nextTriggerTime - now, func() {
        创建 backup 记录（pending）
        创建 task 记录（queued）
        投入任务队列
        重新计算下次触发时间
        设置新 timer
    })
```

### 5. 策略变更钩子

在策略 service 层的以下操作后调用调度器方法：
- 创建策略（enabled=true 时）：`ReloadPolicy`
- 编辑策略：`ReloadPolicy`
- 删除策略：`RemovePolicy`
- 启用/禁用策略切换：`ReloadPolicy`

### 6. 手动触发不影响自动调度

- 手动触发创建的 task 不更新策略的「上次执行时间」（或标记为手动触发）
- 自动调度的 timer 按照上次自动执行的完成时间计算，不受手动触发影响

### 7. 服务启动恢复

在 `Start()` 中：
1. 查询所有 `enabled=true` 的策略
2. 对每个策略计算下次触发时间
3. 设置定时器

## 验收目标

1. 创建 interval 策略（如每 60 秒）后，调度器自动触发备份任务
2. 创建 cron 策略后，在指定时间点触发备份任务
3. 禁用策略后自动停止调度
4. 重新启用后恢复调度
5. 手动触发不影响下次自动触发时间
6. 服务重启后调度计划自动恢复
7. 为 cron 解析器的 `Next()` 方法编写单元测试（多种表达式场景）
8. 为 interval 下次触发时间计算编写单元测试
