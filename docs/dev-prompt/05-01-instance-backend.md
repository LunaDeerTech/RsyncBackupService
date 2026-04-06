# 05-01 实例后端

## 前序任务简报

阶段四已完成：远程配置 CRUD（含私钥上传和连接测试）和备份目标 CRUD（含健康检查引擎，支持本地/SSH，后台 30 分钟定时检查）的后端接口与前端页面均已就位。数据库中 `remote_configs` 和 `backup_targets` 表可正常读写。

## 当前任务目标

实现备份实例的后端 CRUD 接口，含实例统计数据和访问权限配置。

## 实现指导

### 1. 数据模型

```go
type Instance struct {
    ID             int64     `json:"id"`
    Name           string    `json:"name"`
    SourceType     string    `json:"source_type"`      // "local" / "ssh"
    SourcePath     string    `json:"source_path"`
    RemoteConfigID *int64    `json:"remote_config_id,omitempty"`
    Status         string    `json:"status"`            // "idle" / "running"
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

### 2. Store 层

```go
func (db *DB) CreateInstance(inst *model.Instance) error
func (db *DB) GetInstanceByID(id int64) (*model.Instance, error)
func (db *DB) ListInstances() ([]model.Instance, error)
func (db *DB) ListInstancesByUserPermission(userID int64) ([]model.Instance, error) // viewer 用
func (db *DB) UpdateInstance(inst *model.Instance) error
func (db *DB) DeleteInstance(id int64) error
func (db *DB) UpdateInstanceStatus(id int64, status string) error
```

### 3. API 接口

**GET `/api/v1/instances`**（已认证）：
- admin：返回所有实例
- viewer：仅返回有权限的实例（查 `instance_permissions`）
- 每个实例附带基础统计：上次备份时间、上次备份状态、备份总数
- 支持分页

**POST `/api/v1/instances`**（admin）：
- 请求体：`{ "name", "source_type", "source_path", "remote_config_id" }`
- 校验：名称唯一、source_type 为 ssh 时 remote_config_id 必填且有效

**GET `/api/v1/instances/:id`**（已认证 + 实例权限）：
- 返回实例详情 + 概览统计数据
- 统计包含：备份总数、成功备份数、总备份大小、上次备份信息、策略数量

**PUT `/api/v1/instances/:id`**（admin）：
- 只有 `idle` 状态可编辑

**DELETE `/api/v1/instances/:id`**（admin）：
- 只有 `idle` 状态可删除
- 级联清理：删除关联的 policies、backups、tasks、instance_permissions、notification_subscriptions、audit_logs

**GET `/api/v1/instances/:id/stats`**（已认证 + 实例权限）：
- 返回实例统计：备份总数、成功数、失败数、总大小、最近 7 天备份趋势

**PUT `/api/v1/instances/:id/permissions`**（admin）：
- 请求体：`{ "permissions": [{ "user_id": 1, "permission": "readonly" }, ...] }`
- 全量覆盖该实例的权限配置

### 4. 实例统计查询

在 store 层实现统计查询方法：

```go
func (db *DB) GetInstanceStats(instanceID int64) (*model.InstanceStats, error)
func (db *DB) GetLastBackup(instanceID int64) (*model.Backup, error)
```

## 验收目标

1. admin 可创建、编辑、删除实例
2. viewer 仅可见被授权的实例
3. 实例列表附带上次备份时间和状态
4. 实例详情返回统计数据
5. running 状态的实例不可编辑/删除
6. 删除实例时级联清理关联数据
7. 权限配置 API 可正常设置 viewer 对实例的访问权限
8. 为 store 层和权限逻辑编写单元测试
