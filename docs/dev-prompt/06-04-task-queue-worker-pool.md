# 06-04 任务队列与 Worker Pool

## 前序任务简报

rsync 执行器、滚动备份执行器和冷备份执行器均已完成：可执行完整的滚动/冷备份流程，备份结果写入 `backups` 表，进度更新到 `tasks` 表。当前备份执行是直接调用，尚未通过任务队列调度。

## 当前任务目标

实现任务队列和 Worker Pool，支持并发任务管理和同实例互斥控制，实现任务相关 API。

## 实现指导

### 1. Task 模型与 Store

```go
// model/task.go（补充完善）
type Task struct {
    ID            int64      `json:"id"`
    InstanceID    int64      `json:"instance_id"`
    BackupID      *int64     `json:"backup_id,omitempty"`
    Type          string     `json:"type"`         // "rolling" / "cold" / "restore"
    Status        string     `json:"status"`       // "queued" / "running" / "success" / "failed" / "cancelled"
    Progress      int        `json:"progress"`     // 0-100
    CurrentStep   string     `json:"current_step"`
    StartedAt     *time.Time `json:"started_at,omitempty"`
    CompletedAt   *time.Time `json:"completed_at,omitempty"`
    EstimatedEnd  *time.Time `json:"estimated_end,omitempty"`
    ErrorMessage  string     `json:"error_message,omitempty"`
    CreatedAt     time.Time  `json:"created_at"`
}

// store 方法
func (db *DB) CreateTask(t *model.Task) error
func (db *DB) GetTaskByID(id int64) (*model.Task, error)
func (db *DB) UpdateTask(t *model.Task) error
func (db *DB) ListActiveTasks() ([]model.Task, error)         // queued + running
func (db *DB) ListTasksByInstance(instanceID int64) ([]model.Task, error)
func (db *DB) HasRunningTask(instanceID int64) (bool, error)   // 互斥检查
func (db *DB) GetQueuedTasksByInstance(instanceID int64) ([]model.Task, error)
```

### 2. 任务队列（`engine/task_queue.go`）

```go
type TaskQueue struct {
    ch       chan *model.Task
    db       *store.DB
    mu       sync.Mutex
    running  map[int64]context.CancelFunc // instanceID → cancel func
}

func NewTaskQueue(bufferSize int, db *store.DB) *TaskQueue

// Enqueue 将任务加入队列
// 如果该实例已有 running 任务，任务留在 queued 状态等待
func (q *TaskQueue) Enqueue(task *model.Task) error

// Dequeue 消费队列（供 Worker 调用）
func (q *TaskQueue) Dequeue(ctx context.Context) (*model.Task, error)

// Cancel 取消指定任务
func (q *TaskQueue) Cancel(taskID int64) error

// OnTaskComplete 任务完成回调，检查并启动该实例的排队任务
func (q *TaskQueue) OnTaskComplete(instanceID int64)
```

### 3. 并发互斥控制

**核心规则：同一实例同一时刻仅允许一个备份/恢复任务运行**

- Enqueue 时检查：如果实例已有 `running` 任务，新任务保持 `queued` 状态
- Worker 取出任务时再次检查互斥，有冲突则跳过（放回队列尾部）
- 任务完成后检查该实例是否有排队任务，有则取出最早一条投入 Worker

### 4. Worker Pool（`engine/worker_pool.go`）

```go
type WorkerPool struct {
    workers    int
    queue      *TaskQueue
    rolling    *RollingBackupExecutor
    cold       *ColdBackupExecutor
    // restore    *RestoreExecutor  // 阶段七补充
    db         *store.DB
}

func NewWorkerPool(workers int, queue *TaskQueue, ...) *WorkerPool

// Start 启动所有 Worker goroutine
func (wp *WorkerPool) Start(ctx context.Context)

// processTask 单个 Worker 处理任务的逻辑
func (wp *WorkerPool) processTask(ctx context.Context, task *model.Task) error
```

- Worker 数量默认 3，由 `RBS_WORKER_POOL_SIZE` 配置
- 每个 Worker 是独立 goroutine，循环从队列取出任务执行
- 执行前更新 task 状态为 `running` + `started_at` + 实例状态为 `running`
- 根据 task.Type 分发到对应执行器
- 执行结束后更新 task 状态 + 实例状态恢复 `idle` + 调用 `OnTaskComplete`

### 5. 任务 API

**GET `/api/v1/tasks`**（admin）：
- 返回全局活跃任务列表（running + queued）
- 含实例名称

**GET `/api/v1/tasks/:id`**（已认证 + 实例权限）：
- 返回任务详情（含进度、步骤、预计完成时间）

**POST `/api/v1/tasks/:id/cancel`**（admin）：
- queued 状态：直接标记为 cancelled
- running 状态：通过 Context Cancel 终止执行

### 6. 服务启动恢复

- 服务启动时扫描数据库中 `status=running` 的任务，标记为 `failed`（服务重启导致中断）
- 扫描 `status=queued` 的任务，重新加入队列

## 验收目标

1. 手动触发策略后任务进入队列
2. Worker 自动取出任务并执行备份
3. 同一实例的两个任务不会并发执行，第二个排队等待
4. 取消 queued 任务直接标记为 cancelled
5. 取消 running 任务终止 rsync 进程
6. 任务完成后自动触发该实例的下一个排队任务
7. 服务重启后，中断的任务标记为 failed，排队任务重新入队
8. Tasks API 返回正确的任务列表和详情
9. 为任务队列的互斥逻辑编写单元测试
