# 05-02 策略后端

## 前序任务简报

实例后端已完成：`instances` CRUD 接口可用，支持 admin 全量查看和 viewer 权限过滤，实例详情含统计数据，权限配置 API 就绪。

## 当前任务目标

实现备份策略的 CRUD 接口和手动触发功能，校验策略与目标类型的兼容性。

## 实现指导

### 1. 数据模型

```go
type Policy struct {
    ID                int64     `json:"id"`
    InstanceID        int64     `json:"instance_id"`
    Name              string    `json:"name"`
    Type              string    `json:"type"`             // "rolling" / "cold"
    TargetID          int64     `json:"target_id"`
    ScheduleType      string    `json:"schedule_type"`    // "interval" / "cron"
    ScheduleValue     string    `json:"schedule_value"`   // 秒数或 cron 表达式
    Enabled           bool      `json:"enabled"`
    Compression       bool      `json:"compression"`      // 仅冷备份
    Encryption        bool      `json:"encryption"`       // 仅冷备份
    EncryptionKeyHash *string   `json:"-"`                // 不返回
    SplitEnabled      bool      `json:"split_enabled"`    // 仅冷备份
    SplitSizeMB       *int      `json:"split_size_mb,omitempty"`
    RetentionType     string    `json:"retention_type"`   // "time" / "count"
    RetentionValue    int       `json:"retention_value"`  // 天数或数量
    CreatedAt         time.Time `json:"created_at"`
    UpdatedAt         time.Time `json:"updated_at"`
}
```

### 2. Store 层

```go
func (db *DB) CreatePolicy(p *model.Policy) error
func (db *DB) GetPolicyByID(id int64) (*model.Policy, error)
func (db *DB) ListPoliciesByInstance(instanceID int64) ([]model.Policy, error)
func (db *DB) UpdatePolicy(p *model.Policy) error
func (db *DB) DeletePolicy(id int64) error
func (db *DB) ListEnabledPolicies() ([]model.Policy, error) // 调度器用
```

### 3. API 接口

**GET `/api/v1/instances/:id/policies`**（已认证 + 实例权限）：
- 返回该实例的策略列表
- 每个策略附带：上次执行时间、上次执行状态、最近备份 ID

**POST `/api/v1/instances/:id/policies`**（admin）：
- 请求体包含策略全部字段
- 校验规则：
  - `target_id` 对应的目标必须存在
  - 目标的 `backup_type` 必须与策略的 `type` 一致（滚动策略 → 滚动目标，冷策略 → 冷目标）
  - `schedule_type=cron` 时校验 cron 表达式合法性（标准 5 字段）
  - `schedule_type=interval` 时 `schedule_value` 为正整数（秒）
  - `compression`/`encryption`/`split_enabled` 仅在 `type=cold` 时有效
  - `encryption=true` 时请求体中必须提供 `encryption_key`，将其哈希后存入 `encryption_key_hash`，原始密钥不做持久化
  - `split_enabled=true` 时 `split_size_mb` 必填且 > 0

**PUT `/api/v1/instances/:id/policies/:pid`**（admin）：
- 同创建校验规则
- 冷备份加密密钥修改时需提供新密钥

**DELETE `/api/v1/instances/:id/policies/:pid`**（admin）：
- 删除策略及其关联的 backups 记录

**POST `/api/v1/instances/:id/policies/:pid/trigger`**（admin）：
- 手动触发：创建一条 pending 状态的 backup 记录和 task 记录
- 此时备份引擎尚未实现，仅做记录创建，后续阶段六会接入实际执行

### 4. Cron 表达式校验

- 实现简单的 5 字段 cron 解析器或引入轻量库
- 校验格式合法性即可，不需要完整的调度功能（调度器在阶段六实现）

### 5. 加密密钥处理

```go
func HashEncryptionKey(key string) string    // SHA-256 哈希
func ValidateEncryptionKey(key, hash string) bool
```

## 验收目标

1. 可为实例创建滚动/冷备份策略
2. 策略类型与目标类型不兼容时拒绝创建
3. cron 表达式格式错误时返回校验错误
4. 冷备份策略的压缩/加密/分卷选项功能正确
5. 加密密钥哈希存储，原始密钥不可通过 API 获取
6. 手动触发 API 创建 backup 和 task 记录
7. 策略列表附带上次执行信息
8. 为策略校验逻辑编写单元测试
