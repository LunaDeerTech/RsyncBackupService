# 01-01 后端工程初始化与数据库

## 前序任务简报

这是项目的第一个任务，无前序依赖。我们正在构建一个名为 Rsync Backup Service（RBS）的自托管备份管理系统，技术栈为 Go 1.22+ 后端 + SQLite（WAL 模式）+ Vue 3 前端（后续任务）。

## 当前任务目标

搭建 Go 后端工程骨架，建立分层目录结构，实现配置加载与数据目录初始化，完成 SQLite 数据库连接与全部建表。

## 实现指导

### 1. Go Module 初始化

- `go mod init rsync-backup-service`
- 创建入口文件 `cmd/server/main.go`，暂时只需要打印启动信息即可

### 2. 目录结构

按以下分层创建 `internal/` 目录：

```
internal/
  config/       # 配置加载
  model/        # 数据模型 struct
  store/        # 数据访问层（SQLite CRUD）
  service/      # 业务逻辑层
  engine/       # 核心引擎（rsync 执行、调度器、任务队列）
  handler/      # HTTP handler
  middleware/   # 中间件
  notify/       # 通知模块
  audit/        # 审计日志
  crypto/       # 加密工具
  util/         # 通用工具
```

每个目录下放一个占位 `.go` 文件（package 声明即可）。

### 3. 配置加载（`internal/config`）

实现 `Config` 结构体与 `Load()` 函数：

```go
type Config struct {
    DataDir        string // RBS_DATA_DIR, 默认 "./data"
    Port           string // RBS_PORT, 默认 "8080"
    JWTSecret      string // RBS_JWT_SECRET, 必填
    WorkerPoolSize int    // RBS_WORKER_POOL_SIZE, 默认 3
    LogLevel       string // RBS_LOG_LEVEL, 默认 "info"
}

func Load() (*Config, error)
```

- 优先级：环境变量 > `.env` 文件
- `.env` 文件解析：逐行读取 `KEY=VALUE` 格式，忽略 `#` 开头的注释行，`=` 左侧为 key 右侧为 value
- 支持值两侧的引号去除（单引号/双引号）

### 4. 数据目录初始化

在配置加载后，自动创建以下目录结构：

```
DATA_DIR/
  keys/        # SSH 私钥存储
  relay/       # 双远程中继缓存
  temp/        # 冷备份临时目录
  logs/        # 运行日志
```

### 5. 数据库初始化（`internal/store`）

- 引入 `modernc.org/sqlite` 纯 Go SQLite 驱动
- 实现数据库连接管理：

```go
type DB struct {
    *sql.DB
}

func New(dataDir string) (*DB, error)  // 打开 DATA_DIR/rbs.db，启用 WAL 模式
func (db *DB) Migrate() error          // 执行建表与版本迁移
func (db *DB) Close() error
```

- 启用 WAL 模式：`PRAGMA journal_mode=WAL`
- 启用外键约束：`PRAGMA foreign_keys=ON`

### 6. 完整建表 DDL

需要创建以下 12 张表，字段定义严格参照系统设计文档 §3.2：

- `users`：用户表（id, email, name, password_hash, role, created_at, updated_at）
- `instances`：备份实例（id, name, source_type, source_path, remote_config_id, status, created_at, updated_at）
- `policies`：策略（id, instance_id, name, type, target_id, schedule_type, schedule_value, enabled, compression, encryption, encryption_key_hash, split_enabled, split_size_mb, retention_type, retention_value, created_at, updated_at）
- `backup_targets`：备份目标（id, name, backup_type, storage_type, storage_path, remote_config_id, total_capacity_bytes, used_capacity_bytes, last_health_check, health_status, health_message, created_at, updated_at）
- `backups`：备份记录（id, instance_id, policy_id, type, status, snapshot_path, backup_size_bytes, actual_size_bytes, started_at, completed_at, duration_seconds, error_message, rsync_stats, created_at）
- `tasks`：任务（id, instance_id, backup_id, type, status, progress, current_step, started_at, completed_at, estimated_end, error_message, created_at）
- `remote_configs`：远程配置（id, name, type, host, port, username, private_key_path, cloud_provider, cloud_config, created_at, updated_at）
- `instance_permissions`：实例权限（id, user_id, instance_id, permission, created_at），唯一约束 (user_id, instance_id)
- `audit_logs`：审计日志（id, instance_id, user_id, action, detail, created_at）
- `risk_events`：风险事件（id, instance_id, target_id, severity, source, message, resolved, created_at, resolved_at）
- `notification_subscriptions`：通知订阅（id, user_id, instance_id, enabled, created_at）
- `system_configs`：系统配置（key, value, updated_at）

### 7. 版本迁移机制

- 使用 `system_configs` 表存储 `schema_version`
- 服务启动时读取当前版本号，依次执行未应用的迁移
- 首版 schema_version = 1，对应全量建表

### 8. 入口整合

在 `cmd/server/main.go` 中按顺序执行：加载配置 → 创建数据目录 → 打开数据库 → 执行迁移 → 打印启动完成日志。

## 验收目标

1. `go build ./cmd/server/` 编译成功
2. 运行后自动创建 `DATA_DIR/` 及子目录
3. `rbs.db` 文件已创建，包含全部 12 张表
4. `PRAGMA journal_mode` 查询返回 `wal`
5. 重复启动不报错（迁移幂等）
6. 缺少 `RBS_JWT_SECRET` 环境变量时启动报错并退出
7. 为配置加载与数据库迁移编写单元测试
