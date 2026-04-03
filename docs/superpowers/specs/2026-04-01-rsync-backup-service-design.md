# Rsync Backup Service — 设计文档

> 日期: 2026-04-01
> 状态: 已通过自审

---

## 1. 概述

基于 rsync 的备份管理服务，提供 Web 前端和 Go 后端，仅支持 Linux 平台。目标用户为个人或小团队，管理几台到十几台服务器的文件备份。

核心能力：

- 管理多个备份实例（本地/远程源）
- 滚动备份（rsync + 硬链接快照）到本地或 SSH 远程服务器
- 冷备份（打包压缩 + 可选分卷）到本地或 SSH 远程服务器，预留 S3/WebDAV/rclone 等扩展接口
- 灵活的调度策略（定时/间隔）和保留策略（天数/数量）
- 实时进度追踪与剩余时间预估
- 恢复到原路径或新位置，含风险提示与二次认证
- 用户级通知订阅（初期 SMTP 邮件）
- 审计日志
- 单二进制部署（前端嵌入）+ Docker 支持

## 2. 架构

### 2.1 架构方案：单体 + 内嵌调度器

单二进制进程包含所有组件，通过内部模块分层实现关注点分离。

```
┌─────────────────────────────────────────────┐
│                  单二进制                     │
│  ┌───────────────────────────────────────┐  │
│  │   Vue 3 SPA (embed.FS 嵌入)           │  │
│  └──────────────┬────────────────────────┘  │
│                 │ HTTP                       │
│  ┌──────────────▼────────────────────────┐  │
│  │   HTTP Layer (Gin)                    │  │
│  │   ├── REST API (JSON)                 │  │
│  │   ├── WebSocket (实时进度推送)          │  │
│  │   ├── JWT Auth Middleware             │  │
│  │   └── 审计日志 Middleware              │  │
│  └──────────────┬────────────────────────┘  │
│                 │                            │
│  ┌──────────────▼────────────────────────┐  │
│  │   Service Layer (业务逻辑)             │  │
│  │   ├── BackupService (备份实例管理)     │  │
│  │   ├── SchedulerService (调度)          │  │
│  │   ├── ExecutorService (任务执行)       │  │
│  │   ├── RestoreService (恢复)            │  │
│  │   ├── NotificationService (通知)       │  │
│  │   └── AuthService (认证)               │  │
│  └──────────────┬────────────────────────┘  │
│                 │                            │
│  ┌──────────────▼────────────────────────┐  │
│  │   Infrastructure Layer                │  │
│  │   ├── SQLite (GORM)                   │  │
│  │   ├── RsyncRunner (进程管理+输出解析)  │  │
│  │   ├── SSHManager (密钥连接管理)        │  │
│  │   └── StorageBackend (interface)       │  │
│  │       ├── LocalStorage                 │  │
│  │       ├── SSHStorage                   │  │
│  │       └── (future: S3, WebDAV, rclone) │  │
│  └───────────────────────────────────────┘  │
└─────────────────────────────────────────────┘
```

### 2.2 技术选型

| 组件 | 选型 | 理由 |
|------|------|------|
| 后端语言 | Go | 单二进制部署、并发模型好、性能高 |
| HTTP 框架 | Gin | 成熟、性能好、中间件生态丰富 |
| ORM | GORM | Go 主流 ORM，SQLite 支持好 |
| 数据库 | SQLite | 嵌入式、零部署、适合小规模 |
| 调度 | robfig/cron | Go 标准 cron 库，支持秒级精度 |
| WebSocket | gorilla/websocket | 实时进度推送 |
| 配置加载 | godotenv | .env 文件 + 环境变量 |
| 前端框架 | Vue 3 (Composition API) | 官方推荐，生态成熟 |
| 前端构建 | Vite | 快速构建，Vue 3 官方推荐 |
| 前端嵌入 | Go embed.FS | Go 1.16+ 原生，零依赖 |
| UI 组件库 | 自研 | 自由度高、风格鲜明 |

### 2.3 项目目录结构

```
rsync-backup-service/
├── cmd/
│   └── server/
│       └── main.go              # 入口：加载配置、初始化、启动
├── internal/
│   ├── api/                     # HTTP 层
│   │   ├── router.go            # 路由注册
│   │   ├── middleware/          # JWT、审计、CORS
│   │   ├── handler/             # 各资源的 Handler
│   │   └── ws/                  # WebSocket 进度推送
│   ├── service/                 # 业务逻辑层
│   ├── model/                   # 数据模型 (GORM models)
│   ├── repository/              # 数据访问层
│   ├── scheduler/               # Cron 调度器
│   ├── executor/                # 备份/恢复任务执行
│   │   ├── rsync.go             # rsync 命令构建与进程管理
│   │   ├── progress.go          # 输出解析与进度计算
│   │   └── snapshot.go          # 硬链接快照管理
│   ├── storage/                 # 存储后端接口与实现
│   │   ├── backend.go           # interface 定义
│   │   ├── local.go
│   │   └── ssh.go
│   ├── notify/                  # 通知接口与实现
│   │   ├── notifier.go          # interface 定义
│   │   └── smtp.go              # SMTP 邮件通知
│   └── config/                  # 配置加载 (.env)
├── web/                         # Vue 3 前端源码
│   ├── src/
│   │   ├── components/          # 自研 UI 组件库
│   │   ├── views/               # 页面
│   │   ├── composables/         # 组合式函数
│   │   ├── api/                 # API 调用封装
│   │   └── router/              # 前端路由
│   └── ...
├── docs/                        # 文档
├── .env.example                 # 环境变量模板
├── Dockerfile
├── docker-compose.yml
└── Makefile                     # 构建脚本
```

## 3. 数据模型

### 3.1 实体关系

```
User 1──N AuditLog (操作者)
User 1──N NotificationSubscription (订阅者)
User 1──N InstancePermission (权限)

BackupInstance 1──N Strategy
Strategy N──M StorageTarget (通过 strategy_storage_bindings)
Strategy 1──N BackupRecord
BackupInstance 1──N RestoreRecord

StorageTarget ──> SSHKey (SSH 类型时引用)
BackupInstance ──> SSHKey (远程源时引用)

NotificationChannel (全局渠道配置)
NotificationSubscription ──> User + Instance + Channel
```

核心关系：**BackupInstance**（备份什么）→ **Strategy**（怎么备份：类型+调度+保留+存储位置）→ **StorageTarget**（备份到哪里）。

### 3.2 表设计

#### users — 用户

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| username | TEXT UNIQUE | 用户名 |
| password_hash | TEXT | bcrypt 哈希 |
| is_admin | BOOLEAN | 超级管理员标记（首个用户为 true） |
| created_at | DATETIME | |
| updated_at | DATETIME | |

#### ssh_keys — SSH 密钥

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| name | TEXT | 密钥名称（显示用） |
| private_key_path | TEXT | 服务端托管后的私钥文件路径 |
| fingerprint | TEXT | 指纹（用于展示和校验） |
| created_at | DATETIME | |

> 前端上传私钥内容后，由服务端写入受控目录并设置 0600。数据库只保存服务端本地路径，不保存私钥内容本身。

#### backup_instances — 备份实例

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| name | TEXT | 实例名称 |
| source_type | TEXT | `local` / `remote` |
| source_host | TEXT | 远程主机（remote 时） |
| source_port | INTEGER | SSH 端口 |
| source_user | TEXT | SSH 用户名 |
| source_ssh_key_id | INTEGER FK | 关联 ssh_keys |
| source_path | TEXT | 源路径 |
| exclude_patterns | TEXT | 排除规则（JSON 数组） |
| enabled | BOOLEAN | 启用/禁用 |
| created_by | INTEGER FK | 创建者 user_id（用于自动分配权限） |
| created_at | DATETIME | |
| updated_at | DATETIME | |

#### storage_targets — 存储目标

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| name | TEXT | 存储名称 |
| type | TEXT | `rolling_local` / `rolling_ssh` / `cold_local` / `cold_ssh` |
| host | TEXT | 远程主机（SSH 类型时） |
| port | INTEGER | SSH 端口 |
| user | TEXT | SSH 用户名 |
| ssh_key_id | INTEGER FK | 关联 ssh_keys |
| base_path | TEXT | 存储根路径 |
| created_at | DATETIME | |
| updated_at | DATETIME | |

#### strategies — 备份策略

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| instance_id | INTEGER FK | 关联 backup_instances |
| name | TEXT | 策略名称 |
| backup_type | TEXT | `rolling` / `cold` |
| cron_expr | TEXT | Cron 表达式（与 interval 二选一） |
| interval_seconds | INTEGER | 间隔秒数 |
| retention_days | INTEGER | 保留天数（与 count 可同时配置） |
| retention_count | INTEGER | 保留数量 |
| cold_volume_size | TEXT | 冷备份分卷大小（如 `"1G"`, `"500M"`），NULL 不分卷 |
| max_execution_seconds | INTEGER | 最大执行时长（秒），超时自动终止，0 表示不限制 |
| enabled | BOOLEAN | |
| created_at | DATETIME | |
| updated_at | DATETIME | |

#### strategy_storage_bindings — 策略与存储目标绑定

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| strategy_id | INTEGER FK | 关联 strategies |
| storage_target_id | INTEGER FK | 关联 storage_targets |
| created_at | DATETIME | |

#### backup_records — 备份执行记录

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| instance_id | INTEGER FK | |
| storage_target_id | INTEGER FK | |
| strategy_id | INTEGER FK | NULL 表示手动触发 |
| backup_type | TEXT | `rolling` / `cold` |
| status | TEXT | `running` / `success` / `failed` / `cancelled` |
| snapshot_path | TEXT | 快照路径（滚动）或压缩包路径（冷） |
| bytes_transferred | INTEGER | 传输字节数 |
| files_transferred | INTEGER | 传输文件数 |
| total_size | INTEGER | 源总大小 |
| volume_count | INTEGER | 分卷数（1 = 未分卷） |
| started_at | DATETIME | |
| finished_at | DATETIME | |
| error_message | TEXT | 失败时的错误信息 |

#### restore_records — 恢复记录

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| instance_id | INTEGER FK | |
| backup_record_id | INTEGER FK | 从哪个备份恢复 |
| restore_target_path | TEXT | 恢复目标路径 |
| overwrite | BOOLEAN | 是否覆盖原路径 |
| status | TEXT | `running` / `success` / `failed` |
| started_at | DATETIME | |
| finished_at | DATETIME | |
| error_message | TEXT | |
| triggered_by | INTEGER FK | 操作人 user_id |

#### notification_channels — 通知渠道

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| name | TEXT | 渠道名称 |
| type | TEXT | `smtp` / `webhook` / `custom`（预留） |
| config | TEXT | JSON 配置（SMTP 服务器、端口、发件人等，不含收件人） |
| enabled | BOOLEAN | |
| created_at | DATETIME | |
| updated_at | DATETIME | |

#### notification_subscriptions — 通知订阅

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| user_id | INTEGER FK | 订阅用户 |
| instance_id | INTEGER FK | 订阅的备份实例 |
| channel_id | INTEGER FK | 通知渠道 |
| events | TEXT | JSON 数组：订阅的事件类型 |
| channel_config | TEXT | 用户级配置（如收件邮箱地址） |
| enabled | BOOLEAN | |
| created_at | DATETIME | |

#### instance_permissions — 实例权限

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| user_id | INTEGER FK | |
| instance_id | INTEGER FK | |
| role | TEXT | `admin`（全权）/ `viewer`（只读+订阅通知） |
| created_at | DATETIME | |

> 创建实例的用户自动获得 admin 权限。首个用户（超级管理员）拥有所有实例的权限。

#### audit_logs — 审计日志

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | |
| user_id | INTEGER FK | 操作人 |
| action | TEXT | 操作类型 |
| resource_type | TEXT | 资源类型 |
| resource_id | INTEGER | 资源 ID |
| detail | TEXT | JSON 详情 |
| ip_address | TEXT | 操作者 IP |
| created_at | DATETIME | |

## 4. 核心流程

### 4.1 滚动备份（rsync + 硬链接快照）

每次备份创建时间戳命名的快照目录，通过 `rsync --link-dest` 引用上一次快照，未变更文件通过硬链接共享，只有变更文件占用新空间。

**执行流程**：

1. 调度器触发 → 创建 BackupRecord(status=running)
2. 确定快照路径: `{base_path}/{instance_name}/{timestamp}/`
3. 查找最新快照作为 `--link-dest` 引用
4. 根据源/目标位置构建 rsync 命令（见下方场景矩阵）
5. 启动 rsync 子进程，实时解析 stdout 推送进度
6. 完成后更新 BackupRecord(status=success/failed)
7. 执行保留策略清理
8. 触发通知

**源/目标场景矩阵**：

| 源 | 目标 | rsync 模式 | --link-dest |
|----|------|-----------|-------------|
| local | local | 直接执行 | 本地路径，正常 |
| local | ssh | push 模式 | 远程路径，正常 |
| ssh | local | pull 模式 | 本地路径，正常 |
| ssh | ssh | **两阶段中继** | 见下文 |

**双远程中继模式**：

rsync 不支持远程到远程的直接传输。当源和目标都是远程时，服务所在主机作为中继：

- **阶段1**：rsync pull 远程源 → 本地中继缓存目录（`{data_dir}/relay_cache/{instance_id}/`），带 `--link-dest` 引用本地上次缓存
- **阶段2**：rsync push 本地中继缓存 → 远程目标，带 `--link-dest` 引用远程上次快照
- 缓存目录只保留最新一份，用于下次增量引用
- 两阶段都是增量传输，网络开销 ≈ 单次增量的 2 倍
- 前端标记此场景为中继模式，提示用户确保服务器有足够临时磁盘空间

### 4.2 冷备份（打包压缩 + 可选分卷 + 上传）

**执行流程**：

1. 调度器触发 → 创建 BackupRecord(status=running)
2. 若源为远程，先 rsync 同步到本地临时目录
3. 打包压缩:
   - 无分卷: `tar czf {name}_{timestamp}.tar.gz -C <source> .`
   - 有分卷: `tar czf - -C <source> . | split -b {volume_size} - {name}_{timestamp}.tar.gz.part_`
     → 生成 `.part_aa`, `.part_ab`, `.part_ac` ...
4. 上传到存储目标:
   - `cold_local`: 直接 mv 到目标路径
   - `cold_ssh`: rsync/scp 推送到远程
   - (future: S3 上传、WebDAV PUT 等)
5. 删除临时文件
6. 更新 BackupRecord（含 volume_count）
7. 执行保留策略清理
8. 触发通知

### 4.3 保留策略

```
清理逻辑(strategy, storage_target):
  if retention_count > 0:
    列出该 strategy+target 的所有快照/归档，按时间排序
    删除最旧的，直到数量 ≤ retention_count
  if retention_days > 0:
    删除所有超过 retention_days 天的快照/归档
  (两者可同时配置，取并集删除)
```

### 4.4 恢复流程

#### 从滚动备份恢复

1. 用户选择：备份实例 → 存储目标 → 快照版本（按时间倒序展示）
2. 指定恢复目标路径（默认原始源路径）+ 选择覆盖或新位置
3. 风险提示：
   - 覆盖模式: "即将用 {snapshot_time} 的快照覆盖 {path}，当前文件将被替换，此操作不可撤销。"
   - 新位置: "即将恢复 {snapshot_time} 的快照到 {new_path}，请确认路径有足够空间。"
4. 二次认证（输入密码）
5. 执行恢复（rsync 同步，根据源/目标位置自动选择模式，双远程走中继）
6. WebSocket 实时进度推送
7. 创建 RestoreRecord
8. 触发通知

#### 从冷备份恢复

1. 用户选择：备份实例 → 存储目标 → 归档文件
2. 指定恢复参数 + 风险提示 + 二次认证
3. 从存储目标下载归档到本地临时目录
4. 解压：
   - 无分卷: `tar xzf <archive> -C <restore_target>`
   - 有分卷: `cat <parts...> | tar xzf - -C <restore_target>`
5. 清理临时文件 → RestoreRecord → 通知

### 4.5 进度追踪

解析 rsync `--info=progress2` 输出获取整体进度：

```
rsync 输出格式: "1,234,567  45%  12.34MB/s  0:01:23"
解析提取: bytes_transferred, percentage, speed, elapsed
```

自算预估剩余时间：
- `remaining = (total_size - transferred) / current_speed`
- 使用滑动窗口平均速度，避免瞬时波动
- 通过 WebSocket 每秒推送一次给前端

## 5. API 设计

### 5.1 认证

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/auth/login` | 登录，返回 JWT access_token + refresh_token |
| POST | `/api/auth/refresh` | 用 refresh_token 续期 |
| POST | `/api/auth/verify` | 二次认证，返回一次性 verify_token（5 分钟有效） |
| GET | `/api/auth/me` | 当前用户信息 |
| PUT | `/api/auth/password` | 修改密码 |

### 5.2 备份实例

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/instances` | 列表（含状态摘要） |
| POST | `/api/instances` | 创建 |
| GET | `/api/instances/:id` | 详情 |
| PUT | `/api/instances/:id` | 更新 |
| DELETE | `/api/instances/:id` | 删除（需二次认证） |

### 5.3 策略

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/instances/:id/strategies` | 实例下的策略列表 |
| POST | `/api/instances/:id/strategies` | 创建策略 |
| PUT | `/api/strategies/:id` | 更新策略 |
| DELETE | `/api/strategies/:id` | 删除策略 |
| POST | `/api/strategies/:id/trigger` | 手动触发执行 |

### 5.4 存储目标

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/storage-targets` | 列表 |
| POST | `/api/storage-targets` | 创建 |
| PUT | `/api/storage-targets/:id` | 更新 |
| DELETE | `/api/storage-targets/:id` | 删除 |
| POST | `/api/storage-targets/:id/test` | 测试连通性 |

### 5.5 SSH 密钥

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/ssh-keys` | 列表（不返回路径详情） |
| POST | `/api/ssh-keys` | 注册密钥 |
| DELETE | `/api/ssh-keys/:id` | 删除 |
| POST | `/api/ssh-keys/:id/test` | 测试密钥连通性 |

### 5.6 备份记录与恢复

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/instances/:id/backups` | 备份历史（支持按策略/类型/状态筛选） |
| GET | `/api/instances/:id/snapshots` | 可用快照列表（用于恢复选择） |
| POST | `/api/instances/:id/restore` | 发起恢复（需二次认证） |
| GET | `/api/restore-records` | 恢复历史 |

### 5.7 任务与进度

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/tasks/running` | 当前运行中的任务 |
| POST | `/api/tasks/:id/cancel` | 取消任务 |
| WS | `/api/ws/progress` | WebSocket 实时进度推送 |

### 5.8 通知

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/notification-channels` | 渠道列表 |
| POST | `/api/notification-channels` | 创建渠道 |
| PUT | `/api/notification-channels/:id` | 更新 |
| DELETE | `/api/notification-channels/:id` | 删除 |
| POST | `/api/notification-channels/:id/test` | 发送测试通知 |
| GET | `/api/instances/:id/subscriptions` | 当前用户对该实例的订阅 |
| POST | `/api/instances/:id/subscriptions` | 创建/更新订阅 |
| DELETE | `/api/subscriptions/:id` | 取消订阅 |

### 5.9 审计日志

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/audit-logs` | 分页查询（按用户/操作类型/时间筛选） |

### 5.10 系统

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/system/status` | 系统状态（磁盘、运行时长、版本） |
| GET | `/api/system/dashboard` | 仪表盘数据（汇总统计） |

## 6. 安全

### 6.1 JWT 认证

- 登录返回 access_token（2h）+ refresh_token（7d）
- access_token 存 localStorage，API 请求通过 `Authorization: Bearer <token>` 携带
- refresh_token 用于续期
- JWT 密钥通过 `.env` 的 `RBS_JWT_SECRET` 配置

### 6.2 二次认证

用于敏感操作（删除实例、删除备份、恢复、删除存储目标）：

1. 前端弹出密码确认框
2. 调用 `POST /api/auth/verify` 获取一次性 verify_token（5 分钟有效）
3. 敏感 API 请求头携带 `X-Verify-Token`
4. 后端校验 verify_token 的有效性和时效性

### 6.3 SSH 密钥安全

- 数据库仅存密钥文件路径，不存内容
- API 不返回密钥文件内容和完整路径，仅返回名称和指纹
- 启动时校验密钥文件权限为 600

### 6.4 首次启动

- 检测到无用户时，通过 `.env` 中的 `RBS_ADMIN_USER` / `RBS_ADMIN_PASSWORD` 创建管理员
- 也可通过首次访问前端进入初始化向导

## 7. 通知系统

### 7.1 接口

```go
type Notifier interface {
    Type() string
    Send(ctx context.Context, event NotifyEvent) error
    Validate(config json.RawMessage) error
}

type NotifyEvent struct {
    Type       string    // backup_success, backup_failed, restore_complete, restore_failed
    Instance   string
    Strategy   string
    Message    string    // 人类可读摘要
    Detail     any
    OccurredAt time.Time
}
```

### 7.2 订阅模型

- 管理员配置全局通知渠道（如 SMTP 服务器信息）
- 用户在备份实例详情中自行订阅通知：选择渠道 + 事件类型 + 填写自己的收件信息（如邮箱）
- 任务完成后查询该实例的所有启用订阅，校验用户权限后发送

### 7.3 初期实现

**SMTP 邮件通知**：

- 渠道配置：SMTP 服务器、端口、用户名、密码（app password）、发件人、TLS 开关
- 用户订阅配置：收件邮箱地址
- 邮件内容：HTML 模板，包含事件类型、实例名、时间、摘要
- 超时 30 秒，失败重试 3 次（指数退避）

扩展：后续实现 WebhookNotifier、TelegramNotifier 等，只需实现 Notifier 接口。

## 8. 错误处理

### 8.1 rsync 进程管理

- 超时保护：每个任务可配置最大执行时长，超时自动 kill
- 进程异常退出：捕获 exit code，映射为可读错误信息
- 常见 rsync exit code 映射（12=SIGUSR1, 23=partial transfer, 24=vanished files 等）

### 8.2 任务冲突

- 同一实例+同一存储目标，同一时间只允许一个任务运行
- 手动触发时已有任务运行 → 返回冲突提示
- 调度触发时已有任务运行 → 跳过本次，记录日志

### 8.3 存储空间检查

- 备份前检查目标存储剩余空间（本地 `df`，远程 SSH 执行 `df`）
- 空间不足时记录警告但仍尝试执行（rsync 会自行报错）

## 9. 前端

### 9.1 页面路由

```
/login                    → 登录页
/                         → 仪表盘
/instances                → 备份实例列表
/instances/:id            → 实例详情（概览/策略/历史/恢复 Tabs）
/storage-targets          → 存储目标管理
/ssh-keys                 → SSH 密钥管理
/notifications            → 通知渠道管理
/audit-logs               → 审计日志
/settings                 → 系统设置（用户管理、密码修改）
```

### 9.2 核心页面

**仪表盘**：统计卡片（实例数、今日备份、成功/失败）+ 运行中任务列表（实时进度条、速率、预估剩余）+ 最近备份时间线 + 存储空间概览

**备份实例列表**：表格（名称、源路径、策略数、上次状态/时间、启用开关）+ 搜索/筛选 + 操作按钮

**实例详情（Tab 布局）**：
- 概览：基本信息、源配置、策略摘要
- 策略：策略列表、增删改，类型标签、调度规则、存储目标、保留配置
- 备份历史：分页表格，按策略/类型/状态筛选
- 恢复：选择快照/归档 → 配置参数 → 风险确认 → 执行
- 通知订阅：管理当前用户对该实例的通知订阅

**存储目标管理**：按类型分组展示，创建时动态表单，连通性测试

**审计日志**：时间线/表格视图，筛选（用户、操作类型、时间范围），详情展开

### 9.3 自研 UI 组件库

第一期组件清单：

Button, Input, Textarea, Select, Switch, Table, Modal, Dialog, Form, Card, Tag, Badge, Progress, Toast, Notification, Tabs, Breadcrumb, Sidebar, Layout, Icon (SVG), Empty, Spinner, PasswordInput, Timeline

## 10. 构建与部署

### 10.1 构建

```makefile
build:
  cd web && npm run build
  go build -o rsync-backup-service cmd/server/main.go

dev:
  cd web && npm run dev        # Vite :5173
  go run cmd/server/main.go    # API :8080
```

### 10.2 环境变量

```env
RBS_PORT=8080
RBS_DATA_DIR=/var/lib/rsync-backup
RBS_JWT_SECRET=<random-secret>
RBS_ADMIN_USER=admin
RBS_ADMIN_PASSWORD=<initial-password>
```

> SMTP 等通知渠道的配置通过前端管理，存储在 `notification_channels` 表中，不在 .env 中配置。

### 10.3 Docker

多阶段构建：Node.js 构建前端 → Go 编译后端 → Alpine 运行时（安装 rsync + openssh-client）

```yaml
# docker-compose.yml
services:
  rsync-backup:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/var/lib/rsync-backup
      - ~/.ssh:/root/.ssh:ro
    env_file: .env
    restart: unless-stopped
```

## 11. 存储后端扩展接口

```go
type StorageBackend interface {
    Type() string
    Upload(ctx context.Context, localPath string, remotePath string) error
    Download(ctx context.Context, remotePath string, localPath string) error
    List(ctx context.Context, prefix string) ([]StorageObject, error)
    Delete(ctx context.Context, path string) error
    SpaceAvailable(ctx context.Context, path string) (uint64, error)
    TestConnection(ctx context.Context) error
}
```

初期实现：LocalStorage、SSHStorage。未来 S3Storage、WebDAVStorage、RcloneStorage 只需实现此接口。

## 12. 权限模型

轻量实例级权限控制：

- `admin`：实例的完整管理权限（配置、触发、删除、恢复）
- `viewer`：只读访问（查看状态/历史）+ 订阅通知
- 创建实例的用户自动获得 admin 权限
- 首个用户（超级管理员）拥有所有实例的权限
- 超级管理员可为其他用户分配实例权限
