# 06-03 冷备份流程

## 前序任务简报

rsync 执行器和滚动备份执行器已完成：rsync 命令构建/执行/进度解析可用，滚动备份支持快照目录管理、`--link-dest` 增量备份和双远程中继模式，backup/task 记录正确更新。

## 当前任务目标

实现冷备份（cold backup）的完整执行流程：rsync 全量同步 → 压缩 → 加密 → 分卷 → 移动到目标路径。

## 实现指导

### 1. 冷备份执行器（`engine/cold_backup.go`）

```go
type ColdBackupExecutor struct {
    rsync    *RsyncExecutor
    db       *store.DB
    dataDir  string
}

func NewColdBackupExecutor(rsync *RsyncExecutor, db *store.DB, dataDir string) *ColdBackupExecutor

func (e *ColdBackupExecutor) Execute(ctx context.Context, task *model.Task, policy *model.Policy, instance *model.Instance, target *model.BackupTarget, progressCb func(ProgressInfo)) error
```

### 2. 执行流程

整体分为 5 步（可根据策略配置跳过某些步骤），每步更新 task 的 `current_step`：

**步骤 1：rsync 全量同步**
- 目标路径：`DATA_DIR/temp/<task_id>/data/`
- 不使用 `--link-dest`（冷备份每次全量）
- 进度映射：0-50%

**步骤 2：压缩（可选，`policy.Compression=true` 时）**
- 使用 `tar + gzip` 压缩临时目录
- 输出：`DATA_DIR/temp/<task_id>/<instance_name>-<timestamp>.tar.gz`
- 实现方式：`exec.Command("tar", "-czf", outputPath, "-C", tempDir, "data")`
- 进度映射：50-65%

**步骤 3：加密（可选，`policy.Encryption=true` 时）**
- 使用 AES-256-GCM 对文件进行加密
- 加密密钥：从请求中获取（手动触发时传入）或从策略的 `encryption_key_hash` 验证
- 加密后文件后缀添加 `.enc`
- 进度映射：65-80%

```go
// internal/crypto/aes.go
func EncryptFile(inputPath, outputPath string, key []byte) error
func DecryptFile(inputPath, outputPath string, key []byte) error
// key 由用户提供的密钥通过 SHA-256 派生为 32 字节
```

**步骤 4：分卷（可选，`policy.SplitEnabled=true` 时）**
- 使用 `split` 命令或自行实现文件分割
- 分卷大小：`policy.SplitSizeMB` MB
- 输出文件命名：`<basename>.part001`、`.part002` ...
- 进度映射：80-90%

**步骤 5：移动到目标路径**
- 本地目标：`os.Rename` 移动文件到 `<target.StoragePath>/<instance.Name>/<timestamp>/`
- SSH 远程目标：rsync 将文件推送到远程路径
- 清理 `DATA_DIR/temp/<task_id>/` 临时目录
- 进度映射：90-100%

### 3. 备份文件命名规则

```
<instance_name>-<timestamp>.tar.gz          # 仅压缩
<instance_name>-<timestamp>.tar.gz.enc      # 压缩 + 加密
<instance_name>-<timestamp>.tar.gz.enc.part001  # 压缩 + 加密 + 分卷
```

不压缩时直接使用 tar（不加 gz）或打包为目录：
```
<instance_name>-<timestamp>.tar             # 仅打包
<instance_name>-<timestamp>/                # 不压缩不打包时，保留目录结构
```

### 4. backup 记录更新

- `snapshot_path`：最终文件在目标路径的路径（分卷时记录第一个分卷的路径，后缀 `.part*`）
- `backup_size_bytes`：最终文件/分卷总大小
- `actual_size_bytes`：rsync 报告的原始数据大小

### 5. 临时目录管理

- 每个冷备份任务使用独立临时目录：`DATA_DIR/temp/<task_id>/`
- 无论成功或失败，最终都清理临时目录
- 使用 `defer` 确保清理

### 6. 错误处理

- 任何步骤失败：记录错误 → 清理临时文件 → 标记 backup 为 failed
- Context 取消：中断当前步骤 → 清理 → 标记 cancelled

## 验收目标

1. 不带任何选项的冷备份：rsync 全量同步后文件存放在目标路径
2. 启用压缩：目标路径下生成 `.tar.gz` 文件
3. 启用压缩+加密：目标路径下生成 `.tar.gz.enc` 文件
4. 启用分卷：目标路径下生成 `.partXXX` 系列文件
5. backup 记录的 `snapshot_path`、`backup_size_bytes` 正确
6. 任务失败或取消后临时目录被正确清理
7. Task 进度在各步骤更新
8. 为加密/解密工具编写单元测试
