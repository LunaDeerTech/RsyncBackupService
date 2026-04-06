# 06-02 滚动备份流程

## 前序任务简报

rsync 命令执行器已完成：命令构建器可拼装各种源/目标组合的 rsync 参数，执行器可运行命令并实时解析 `--info=progress2` 进度和 `--stats` 统计信息，支持 Context 取消。

## 当前任务目标

实现滚动备份（rolling backup）的完整执行流程，包括快照目录管理、`--link-dest` 增量备份和双远程中继模式。

## 实现指导

### 1. 滚动备份执行器（`engine/rolling_backup.go`）

```go
type RollingBackupExecutor struct {
    rsync    *RsyncExecutor
    db       *store.DB
}

func NewRollingBackupExecutor(rsync *RsyncExecutor, db *store.DB) *RollingBackupExecutor

// Execute 执行一次滚动备份
func (e *RollingBackupExecutor) Execute(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error
```

### 2. 执行流程

1. **解析远程配置**：根据 instance.RemoteConfigID 和 target.RemoteConfigID 加载 RemoteConfig
2. **确定快照路径**：`<target.StoragePath>/<instance.Name>/<timestamp>/`，timestamp 格式 `20060102-150405`
3. **查找上次成功快照**：查询 `backups` 表获取该实例+策略的最近一次 `status=success` 的记录，取其 `snapshot_path` 作为 `--link-dest`
4. **创建 backup 记录**：状态 `running`，记录 `started_at`
5. **执行 rsync**：
   - 构建 RsyncConfig（源、目标、link-dest）
   - 调用执行器，传递 progressCb 更新 task 进度
6. **更新 latest 符号链接**：在 `<target.StoragePath>/<instance.Name>/` 下创建/更新 `latest` → 当前快照目录
7. **更新记录**：
   - 成功：`status=success`、`completed_at`、`backup_size_bytes`、`actual_size_bytes`、`duration_seconds`、`rsync_stats`（JSON）
   - 失败：`status=failed`、`error_message`

### 3. 快照目录管理

- 本地目标：直接用 `os.MkdirAll` 创建目录
- SSH 远程目标：通过 SSH 执行 `mkdir -p <path>` 创建目录

### 4. 符号链接管理

- 本地目标：`os.Symlink` 创建/更新（先 Remove 旧链接）
- SSH 远程目标：`ssh <remote> "ln -sfn <snapshot_dir> <target>/<instance>/latest"`

### 5. 双远程中继模式

当源和目标均为 SSH 远程时：

```go
func (e *RollingBackupExecutor) ExecuteRelay(ctx context.Context, ...) error
```

1. **阶段一（Pull）**：rsync 从远程源 → 本地中继目录（`DATA_DIR/relay/<instance_id>/`）
   - link-dest 使用中继目录中上次的快照
2. **阶段二（Push）**：rsync 从本地中继目录 → 远程目标的快照路径
   - link-dest 使用远程目标上上次的快照
3. 中继目录仅保留当前快照（下次备份用），不累积历史

### 6. 进度回调处理

```go
// progressCb 内更新 task 记录
func (e *RollingBackupExecutor) updateTaskProgress(task *model.Task, progress ProgressInfo) {
    // 更新 task.Progress, task.CurrentStep, task.EstimatedEnd
    // 写入数据库
}
```

- 中继模式下：阶段一进度映射为 0-50%，阶段二映射为 50-100%

## 验收目标

1. 本地→本地滚动备份：fast照目录正确创建，`latest` 链接指向新快照
2. 首次备份（无 link-dest）正常完成
3. 第二次备份使用 `--link-dest` 指向上次快照，未变更文件硬链接节省空间
4. backup 记录中 `snapshot_path`、`backup_size_bytes`、`rsync_stats` 正确记录
5. 备份失败时 backup 记录为 `failed` + 错误信息
6. Task 进度在执行过程中被更新
7. 为快照路径构建和目录管理编写单元测试
