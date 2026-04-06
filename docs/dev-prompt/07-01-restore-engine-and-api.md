# 07-01 恢复引擎与 API

## 前序任务简报

阶段六备份引擎已完成：rsync 执行器（命令构建/进度解析）、滚动备份（快照目录 + link-dest + 中继模式）、冷备份（rsync + 压缩 + 加密 + 分卷）、任务队列 + Worker Pool（并发控制 + 实例互斥）、调度器（interval + cron 自动触发）、保留策略清理（按时间/数量 + 定时全量扫描）全部就绪。

## 当前任务目标

实现备份恢复引擎（滚动/冷备份恢复）和恢复 API（含安全验证和冷备份下载）。

## 实现指导

### 1. 恢复执行器（`engine/restore.go`）

```go
type RestoreExecutor struct {
    rsync    *RsyncExecutor
    db       *store.DB
    dataDir  string
}

func NewRestoreExecutor(rsync *RsyncExecutor, db *store.DB, dataDir string) *RestoreExecutor

func (e *RestoreExecutor) Execute(ctx context.Context, task *model.Task, backup *model.Backup, restoreReq *RestoreRequest, progressCb func(ProgressInfo)) error
```

### 2. RestoreRequest

```go
type RestoreRequest struct {
    RestoreType   string // "source" (恢复到原位) / "custom" (恢复到指定路径)
    TargetPath    string // custom 时必填
    EncryptionKey string // 加密冷备份解密用
}
```

### 3. 滚动备份恢复

1. 从 `backup.SnapshotPath` 获取快照目录路径
2. 确定恢复目标路径：
   - `source`：使用原实例的 `source_path`
   - `custom`：使用请求中的 `target_path`
3. 执行 rsync：
   - 恢复到源位置：`rsync -avz --delete <snapshot>/ <target>/`（`--delete` 确保完全一致）
   - 恢复到自定义位置：`rsync -avz <snapshot>/ <target>/`（不用 `--delete`）
4. 处理远程路径：快照或恢复目标可能在 SSH 远程，需要正确拼装 rsync 参数

### 4. 冷备份恢复

1. 从目标路径获取备份文件
2. **合并分卷**（如有）：`cat <file>.part* > <file>`
3. **解密**（如有）：调用 `crypto.DecryptFile`，使用用户提供的 `EncryptionKey`
4. **解压**：`tar -xzf <file> -C <temp_dir>`
5. **rsync 同步**：rsync 从解压目录 → 恢复路径
6. 清理临时文件

冷备份恢复使用临时目录：`DATA_DIR/temp/restore-<task_id>/`

### 5. Worker Pool 集成

在 `WorkerPool.processTask` 中添加 `type=restore` 的分支，调用 `RestoreExecutor.Execute`

### 6. API 接口

**POST `/api/v1/instances/:id/backups/:bid/restore`**（admin）：

请求体：
```json
{
  "restore_type": "source",
  "target_path": "/path/to/restore",
  "instance_name": "my-instance",
  "password": "current-user-password",
  "encryption_key": "my-secret-key"
}
```

验证逻辑：
1. 验证 `instance_name` 与实例名称一致（防止误操作确认）
2. 验证 `password` 为当前登录用户的密码（二次确认）
3. 验证 backup 存在且状态为 `success`
4. `restore_type=custom` 时 `target_path` 必填
5. 加密的冷备份恢复时 `encryption_key` 必填，且哈希与存储的 hash 匹配
6. 创建 restore 类型的 task → 投入任务队列

**GET `/api/v1/instances/:id/backups/:bid/download`**（已认证 + 下载权限）：
- 仅冷备份可下载
- 生成一次性临时下载令牌（JWT 或 UUID，有效期 5 分钟）
- 返回下载 URL：`/api/v1/download/<token>`
- 实际下载接口验证 token 有效性后流式返回文件

```go
// 临时令牌管理
type DownloadTokenManager struct {
    mu     sync.Mutex
    tokens map[string]*DownloadToken // token → info
}

type DownloadToken struct {
    BackupID  int64
    FilePath  string
    ExpiresAt time.Time
}

func (m *DownloadTokenManager) Generate(backupID int64, filePath string) string
func (m *DownloadTokenManager) Validate(token string) (*DownloadToken, error)
func (m *DownloadTokenManager) Revoke(token string)
```

## 验收目标

1. 滚动备份可恢复到原始位置（rsync --delete）
2. 滚动备份可恢复到自定义路径
3. 冷备份可解密 → 解压 → 恢复
4. 恢复 API 要求输入实例名称和密码二次验证
5. 密码错误时恢复被拒绝
6. 冷备份下载接口生成一次性临时 token
7. 恢复任务通过任务队列调度，与备份任务共享互斥控制
8. 恢复完成后 task 状态正确更新
9. 为恢复流程的分步逻辑编写单元测试
