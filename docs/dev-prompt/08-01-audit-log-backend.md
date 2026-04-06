# 08-01 审计日志后端

## 前序任务简报

阶段六（备份引擎）和阶段七（恢复引擎）已完成：完整的备份/恢复流程已打通（rsync 执行 → 滚动/冷备份 → 恢复 → 任务队列调度 → 保留清理）。API 层面支持策略手动触发、恢复二次验证和冷备份下载。

## 当前任务目标

实现审计日志模块的写入功能和查询 API，在所有关键业务操作中插入审计记录。

## 实现指导

### 1. 审计日志模块（`internal/audit`）

```go
// audit/audit.go
type Logger struct {
    db *store.DB
}

func NewLogger(db *store.DB) *Logger

// LogAction 记录审计日志
// instanceID 可为 0（系统级操作）
// userID 可为 0（系统自动操作）
func (l *Logger) LogAction(ctx context.Context, instanceID, userID int64, action string, detail interface{}) error
```

- `detail` 参数序列化为 JSON 存入 `audit_logs.detail` 字段
- 审计日志写入失败不应阻塞业务流程（记录 error log 后继续）

### 2. Action 枚举

```go
const (
    ActionInstanceCreate  = "instance.create"
    ActionInstanceUpdate  = "instance.update"
    ActionInstanceDelete  = "instance.delete"
    ActionPolicyCreate    = "policy.create"
    ActionPolicyUpdate    = "policy.update"
    ActionPolicyDelete    = "policy.delete"
    ActionBackupTrigger   = "backup.trigger"
    ActionBackupComplete  = "backup.complete"
    ActionBackupFail      = "backup.fail"
    ActionRestoreTrigger  = "restore.trigger"
    ActionRestoreComplete = "restore.complete"
    ActionRestoreFail     = "restore.fail"
    ActionUserCreate      = "user.create"
    ActionUserUpdate      = "user.update"
    ActionUserDelete      = "user.delete"
    ActionTargetCreate    = "target.create"
    ActionTargetUpdate    = "target.update"
    ActionTargetDelete    = "target.delete"
    ActionRemoteCreate    = "remote.create"
    ActionRemoteUpdate    = "remote.update"
    ActionRemoteDelete    = "remote.delete"
    ActionSystemConfigUpdate = "system.config.update"
)
```

### 3. 在各 Service 层插入审计

需要在以下模块的关键操作后调用 `audit.LogAction`：

- **实例 Service**：创建/编辑/删除实例
- **策略 Service**：创建/编辑/删除策略
- **Worker Pool**：备份任务完成/失败、恢复任务完成/失败
- **策略 Handler**：手动触发备份
- **恢复 Handler**：触发恢复
- **用户 Service**：创建/编辑/删除用户
- **目标 Service**：创建/编辑/删除目标
- **远程配置 Service**：创建/编辑/删除配置
- **系统配置 Handler**：更新 SMTP、注册开关等

detail 内容示例：
```json
// instance.create
{"name": "my-app", "source_type": "local", "source_path": "/data/app"}
// backup.complete
{"backup_id": 42, "policy_id": 5, "type": "rolling", "duration_seconds": 120}
// user.delete
{"deleted_user_id": 3, "deleted_email": "viewer@example.com"}
```

### 4. 查询 API

**GET `/api/v1/audit-logs`**（admin）：
- 全局审计日志列表
- 支持筛选：`start_date`、`end_date`、`action`（支持多个用逗号分隔）
- 支持分页
- 返回数据含操作人名称（JOIN users 表）

**GET `/api/v1/instances/:id/audit-logs`**（已认证 + 实例权限）：
- 指定实例的审计日志
- 同样支持时间范围和 action 筛选
- 支持分页

### 5. Store 层

```go
type AuditLogQuery struct {
    InstanceID *int64
    StartDate  *time.Time
    EndDate    *time.Time
    Actions    []string
    Pagination Pagination
}

func (db *DB) CreateAuditLog(log *model.AuditLog) error
func (db *DB) ListAuditLogs(query AuditLogQuery) ([]model.AuditLogWithUser, int64, error)
```

```go
type AuditLogWithUser struct {
    model.AuditLog
    UserName  string `json:"user_name"`
    UserEmail string `json:"user_email"`
}
```

## 验收目标

1. 创建/编辑/删除实例时 audit_logs 表中生成对应记录
2. 备份完成/失败时自动记录审计日志
3. 恢复触发/完成/失败时自动记录审计日志
4. 全局审计日志 API 可按时间范围和操作类型筛选
5. 实例级审计日志 API 仅返回该实例的记录
6. 审计日志写入失败不影响业务流程
7. 审计日志包含操作人名称和邮箱
8. 为审计日志查询的筛选逻辑编写单元测试
