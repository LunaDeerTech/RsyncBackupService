# 06-06 备份保留与自动清理

## 前序任务简报

调度器已完成：支持 interval 和 cron 两种调度方式，策略变更时自动重载调度计划，服务启动时恢复所有已启用策略的调度。现在备份引擎的核心执行流程已经完整：手动/自动触发 → 任务队列 → Worker 执行 → 滚动/冷备份完成 → 记录写入。

## 当前任务目标

实现备份保留策略的自动清理机制，按时间或数量删除过期备份。

## 实现指导

### 1. 保留策略清理器（`engine/retention.go`）

```go
type RetentionCleaner struct {
    db      *store.DB
    dataDir string
}

func NewRetentionCleaner(db *store.DB, dataDir string) *RetentionCleaner

// CleanByPolicy 根据策略的保留规则清理过期备份
func (rc *RetentionCleaner) CleanByPolicy(ctx context.Context, policy *model.Policy) error

// CleanAll 全量扫描所有策略执行清理
func (rc *RetentionCleaner) CleanAll(ctx context.Context) error

// StartSchedule 启动定时全量清理（每 6 小时）
func (rc *RetentionCleaner) StartSchedule(ctx context.Context)
```

### 2. 按时间保留

```go
func (rc *RetentionCleaner) cleanByTime(ctx context.Context, policy *model.Policy) error
```

- 查询该策略下 `status=success` 且 `completed_at < now - retention_value 天` 的备份记录
- 对每条记录执行删除

### 3. 按数量保留

```go
func (rc *RetentionCleaner) cleanByCount(ctx context.Context, policy *model.Policy) error
```

- 查询该策略下 `status=success` 的备份，按 `completed_at DESC` 排序
- 保留前 `retention_value` 条，多余的执行删除

### 4. 删除流程

**单条备份的删除流程**：

1. **删除磁盘文件**：
   - 滚动备份：删除快照目录（`rm -rf snapshot_path`）
   - 冷备份：删除文件（可能是单文件或多个分卷 `.partXXX`）
   - SSH 远程存储：通过 SSH 执行 `rm -rf`
2. **更新符号链接**：如果删除的快照是 `latest` 所指向的，需要更新 `latest` 指向下一个最新的快照
3. **更新数据库**：删除 `backups` 表中的记录
4. **删除关联 task**：清理关联的 `tasks` 记录

**失败处理**：
- 磁盘文件删除失败：写入 audit_log（`backup.cleanup_failed`），跳过该条继续处理下一条，不阻塞整体清理
- 数据库操作失败：记录日志

### 5. 触发时机

- **备份完成后**：在 Worker Pool 的任务完成回调中调用 `CleanByPolicy(policy)`
- **定时全量扫描**：每 6 小时执行一次 `CleanAll()`，处理可能遗漏的清理

### 6. Store 方法

```go
func (db *DB) ListExpiredBackups(policyID int64, before time.Time) ([]model.Backup, error)
func (db *DB) ListExcessBackups(policyID int64, keepCount int) ([]model.Backup, error)
func (db *DB) DeleteBackup(id int64) error
func (db *DB) DeleteTaskByBackupID(backupID int64) error
```

### 7. 安全保护

- 不删除 `status=running` 的备份
- 不删除策略没有任何成功备份时的最后一条（至少保留 1 条成功备份）
- 清理前记录日志，便于审计追踪

## 验收目标

1. 按时间保留：超过保留天数的备份被正确删除（磁盘 + 数据库）
2. 按数量保留：超过保留数量的老备份被正确删除
3. 备份完成后自动触发该策略的清理检查
4. 后台每 6 小时执行一次全量磁盘清理
5. 磁盘删除失败时不阻塞清理流程，错误被记录
6. `latest` 符号链接在快照删除后正确更新
7. 为按时间/按数量清理逻辑编写单元测试（mock 数据库和文件系统）
