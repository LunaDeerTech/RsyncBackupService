# 04-02 备份目标与健康检查后端

## 前序任务简报

远程配置（`remote_configs`）后端已完成：CRUD 接口可用，私钥通过 multipart 上传存储到 `DATA_DIR/keys/`（权限 0600），SSH 连接测试接口可用，API 响应不泄露私钥路径。

## 当前任务目标

实现备份目标的 CRUD 接口和健康检查引擎（本地 + SSH 远程），含后台定时健康检查。

## 实现指导

### 1. 数据模型

```go
type BackupTarget struct {
    ID                 int64      `json:"id"`
    Name               string     `json:"name"`
    BackupType         string     `json:"backup_type"`   // "rolling" / "cold"
    StorageType        string     `json:"storage_type"`  // "local" / "ssh" / "cloud"
    StoragePath        string     `json:"storage_path"`
    RemoteConfigID     *int64     `json:"remote_config_id,omitempty"`
    TotalCapacityBytes *int64     `json:"total_capacity_bytes,omitempty"`
    UsedCapacityBytes  *int64     `json:"used_capacity_bytes,omitempty"`
    LastHealthCheck    *time.Time `json:"last_health_check,omitempty"`
    HealthStatus       string     `json:"health_status"`  // "healthy" / "degraded" / "unreachable"
    HealthMessage      string     `json:"health_message"`
    CreatedAt          time.Time  `json:"created_at"`
    UpdatedAt          time.Time  `json:"updated_at"`
}
```

### 2. Store 层

```go
func (db *DB) CreateBackupTarget(t *model.BackupTarget) error
func (db *DB) GetBackupTargetByID(id int64) (*model.BackupTarget, error)
func (db *DB) ListBackupTargets() ([]model.BackupTarget, error)
func (db *DB) UpdateBackupTarget(t *model.BackupTarget) error
func (db *DB) DeleteBackupTarget(id int64) error
func (db *DB) IsBackupTargetInUse(id int64) (bool, error) // 检查是否被 policy 引用
func (db *DB) UpdateHealthStatus(id int64, status, message string, total, used *int64) error
```

### 3. API 接口

**GET `/api/v1/targets`**（admin）：目标列表，支持分页
**POST `/api/v1/targets`**（admin）：创建目标
**PUT `/api/v1/targets/:id`**（admin）：编辑目标
**DELETE `/api/v1/targets/:id`**（admin）：删除（有策略引用时拒绝）
**POST `/api/v1/targets/:id/health-check`**（admin）：手动触发健康检查

创建/编辑校验：
- 名称唯一
- 类型合法组合：滚动备份仅支持 `local`/`ssh`，冷备份支持 `local`/`ssh`/`cloud`
- `storage_type` 为 `ssh` 时 `remote_config_id` 必填且引用有效
- 路径非空

### 4. 健康检查引擎（`engine/health_checker.go`）

```go
type HealthChecker struct {
    db *store.DB
}

func NewHealthChecker(db *store.DB) *HealthChecker
func (hc *HealthChecker) CheckTarget(target *model.BackupTarget) (status, message string, total, used *int64, err error)
func (hc *HealthChecker) CheckAll()                // 检查所有目标
func (hc *HealthChecker) StartSchedule(ctx context.Context) // 启动 30 分钟定时检查
```

**本地目标检查**：
1. 路径是否存在（`os.Stat`）
2. 路径是否可写（创建临时文件测试）
3. 获取磁盘容量（`syscall.Statfs` / `golang.org/x/sys/unix`）

**SSH 远程目标检查**：
1. 通过关联的 `remote_config` 建立 SSH 连接
2. 执行 `test -d <path> && test -w <path> && echo ok` 验证路径
3. 执行 `df -B1 <path>` 获取容量信息（解析总容量和已用容量）

**检查结果处理**：
- 全部通过：`healthy` + 成功信息
- 部分通过（如容量获取失败但路径可访问）：`degraded` + 详情
- 连接/路径失败：`unreachable` + 错误信息

### 5. 后台定时任务

- 在服务启动时调用 `StartSchedule`，启动独立 goroutine
- 每 30 分钟执行 `CheckAll()`
- 使用 `context.Context` 支持优雅停止

## 验收目标

1. 可创建本地和 SSH 类型备份目标
2. 类型非法组合（如滚动备份 + cloud）被拒绝
3. 手动健康检查：本地目标能正确检测路径存在性和容量
4. 手动健康检查：SSH 目标能正确检测连通性和路径（需有效的 SSH 远程配置）
5. 健康状态更新到数据库并在列表接口中返回
6. 后台定时检查每 30 分钟自动执行
7. 删除被策略引用的目标返回错误
8. 为健康检查的本地路径检测编写单元测试
