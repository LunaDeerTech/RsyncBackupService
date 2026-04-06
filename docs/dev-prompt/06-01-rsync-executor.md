# 06-01 rsync 命令执行器

## 前序任务简报

阶段一至五已完成：后端工程骨架、认证系统、远程配置/备份目标/实例/策略的 CRUD 全部就绪。前端 AppLayout、UI 组件库、远程配置/目标/实例列表/实例详情页面均已实现。策略手动触发可创建 backup + task 记录。现在进入核心备份引擎开发。

## 当前任务目标

实现 rsync 命令构建器和执行器，支持本地/SSH 源和目标的各种组合，实现 `--info=progress2` 实时进度解析。

## 实现指导

### 1. rsync 命令构建器（`engine/rsync.go`）

```go
type RsyncConfig struct {
    SourcePath     string
    SourceType     string     // "local" / "ssh"
    SourceRemote   *model.RemoteConfig
    DestPath       string
    DestType       string     // "local" / "ssh"
    DestRemote     *model.RemoteConfig
    LinkDestPath   string     // --link-dest 路径（可选）
    ExtraArgs      []string   // 额外 rsync 参数
}

type RsyncResult struct {
    ExitCode       int
    Stats          RsyncStats
    Stdout         string
    Stderr         string
}

type RsyncStats struct {
    TotalSize      int64   // 数据原始大小
    TransferSize   int64   // 实际传输大小
    TotalFiles     int
    TransferFiles  int
    Speed          string
    Duration       time.Duration
}

func BuildRsyncArgs(cfg RsyncConfig) []string
```

### 2. 命令参数组装规则

基础参数：`-avz --delete --stats --info=progress2`

**源/目标路径格式**：
- 本地源 → 本地目标：`rsync ... <source>/ <dest>/`
- 本地源 → SSH 目标：`rsync ... <source>/ <user>@<host>:<dest>/`
- SSH 源 → 本地目标：`rsync ... <user>@<host>:<source>/ <dest>/`
- SSH 源 → SSH 目标：不直接支持，由上层使用中继模式处理

**SSH 参数**：当源或目标为 SSH 时，添加：
```
--rsh="ssh -i <private_key_path> -p <port> -o StrictHostKeyChecking=accept-new -o BatchMode=yes"
```

**link-dest 参数**：当 `LinkDestPath` 非空时：
```
--link-dest=<link_dest_path>
```

### 3. 命令执行器

```go
type RsyncExecutor struct{}

func NewRsyncExecutor() *RsyncExecutor

// Execute 执行 rsync 命令并实时解析进度
// progressCb 在每次进度更新时回调（可为 nil）
func (e *RsyncExecutor) Execute(ctx context.Context, cfg RsyncConfig, progressCb func(ProgressInfo)) (*RsyncResult, error)
```

- 使用 `exec.CommandContext` 执行 rsync 命令
- 通过 `cmd.StdoutPipe()` 实时读取 stdout
- 通过 `cmd.StderrPipe()` 捕获 stderr
- 支持 `context.Context` 实现取消操作（通过 `cmd.Process.Kill()`）

### 4. 进度解析

```go
type ProgressInfo struct {
    BytesTransferred int64
    Percentage       int     // 0-100
    Speed            string  // "12.34MB/s"
    Remaining        string  // "0:01:23"
}

func ParseProgress(line string) (*ProgressInfo, bool)
```

rsync `--info=progress2` 输出格式：
```
  1,234,567  45%   12.34MB/s    0:01:23
```

- 按空白分割各字段
- 提取字节数（去除逗号）、百分比（去除 %）、速率、剩余时间
- 非进度行返回 false

### 5. rsync 统计解析

rsync `--stats` 输出中包含：
```
Number of files: 1,234
Number of regular files transferred: 56
Total file size: 1,234,567 bytes
Total transferred file size: 123,456 bytes
```

实现 `ParseStats(output string) RsyncStats` 解析统计信息。

### 6. 错误处理

根据 rsync 退出码映射错误类型：
- 0：成功
- 23：部分传输（某些文件无法传输）
- 24：部分传输（源文件消失）
- 其他非零：失败

## 验收目标

1. 可正确拼装本地→本地、本地→SSH、SSH→本地的 rsync 命令参数
2. 执行器可运行 rsync 命令并获取退出码
3. `--info=progress2` 输出可被实时解析为 ProgressInfo
4. `--stats` 输出可被解析为 RsyncStats
5. Context 取消后 rsync 进程被正确终止
6. 为命令构建器和进度/统计解析编写单元测试（不需要实际执行 rsync）
