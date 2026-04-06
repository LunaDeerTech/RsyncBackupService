# Rsync Backup Service — 系统设计文档

---

## 1. 系统概述

Rsync Backup Service（以下简称 RBS）是一个基于 rsync 的自托管备份管理系统，提供滚动增量备份与冷全量备份两种模式，支持本地、SSH 远程和云存储（仅冷备份）三种存储类型，具备计划调度、任务执行、备份恢复、容灾评估、风险预警、审计日志和用户权限管理等完整能力，并通过 Web 界面进行管理和监控。

### 1.1 设计目标

- 提供可靠的增量/全量备份能力，基于 rsync `--link-dest` 实现空间高效的滚动快照；
- 支持本地、SSH 远程双向组合的备份拓扑，含双远程中继模式；
- 提供完整的计划调度、保留策略、自动清理能力；
- 通过容灾率量化评估实例可恢复能力，驱动风险预警；
- 单一可执行文件部署，前端嵌入后端，零外部依赖（仅需 rsync）；
- 支持桌面与移动端，支持浅色/深色主题；

### 1.2 系统边界

- RBS 不是文件同步工具，不提供双向同步或实时同步；
- RBS 不实现自有传输协议，始终通过 rsync 完成实际数据传输；
- 云存储接口首版仅预留，不做完整实现；

---

## 2. 整体架构

### 2.1 部署架构

```
┌──────────────────────────────────────────────────┐
│                  RBS 单体进程                      │
│  ┌────────────┐  ┌────────────────────────────┐  │
│  │  静态文件   │  │       Go HTTP Server       │  │
│  │ (Vue SPA)  │  │  ┌──────┐ ┌─────────────┐  │  │
│  │  embed.FS  │  │  │Router│ │ Middleware   │  │  │
│  └────────────┘  │  └──┬───┘ └─────────────┘  │  │
│                  │     │                       │  │
│                  │  ┌──▼──────────────────┐    │  │
│                  │  │   Business Layer    │    │  │
│                  │  │  (Service + Engine) │    │  │
│                  │  └──┬──────────────────┘    │  │
│                  │     │                       │  │
│                  │  ┌──▼───┐  ┌────────────┐   │  │
│                  │  │SQLite│  │ rsync CLI  │   │  │
│                  │  └──────┘  └────────────┘   │  │
│                  └────────────────────────────┘  │
└──────────────────────────────────────────────────┘
         │                          │
    ┌────▼────┐              ┌──────▼──────┐
    │  浏览器  │              │ 备份目标     │
    │ (SPA)   │              │ 本地/SSH/云  │
    └─────────┘              └─────────────┘
```

### 2.2 技术栈

| 层级 | 技术选型 | 说明 |
|------|---------|------|
| 后端 | Go (1.22+) | 标准库为主，最小化外部依赖 |
| 数据库 | SQLite (WAL 模式) | 嵌入式，单文件，通过 `modernc.org/sqlite` 纯 Go 驱动 |
| 前端 | Vue 3 + Vite + TypeScript | 单页应用 |
| 样式 | Tailwind CSS + 自定义 token | Balanced Flux 风格体系 |
| 备份引擎 | rsync (系统命令) | 通过 `os/exec` 调用 |
| 认证 | JWT (HS256) | 无状态令牌认证 |
| 部署 | 单一二进制 + `embed.FS` | 前端构建产物嵌入 Go 二进制 |

### 2.3 后端分层架构

```
cmd/server/           # 入口，启动 HTTP 服务与后台调度
internal/
  config/             # 配置加载（.env + 环境变量）
  model/              # 数据模型定义（struct）
  store/              # 数据访问层（SQLite CRUD）
  service/            # 业务逻辑层
  engine/             # 核心引擎（rsync 执行、调度器、任务队列）
  handler/            # HTTP handler（路由 + 请求/响应）
  middleware/         # 中间件（认证、日志、CORS）
  notify/             # 通知模块（邮件发送）
  audit/              # 审计日志写入
  crypto/             # 加密工具（备份加密、密码哈希）
  util/               # 通用工具函数
```

**依赖方向**：`handler → service → store`、`handler → service → engine`，禁止反向依赖。

---

## 3. 数据模型

### 3.1 ER 概览

```
User ──1:N──> InstancePermission <──N:1── Instance
Instance ──1:N──> Policy ──N:1──> BackupTarget
Instance ──1:N──> Backup
Instance ──1:N──> AuditLog
Policy ──1:N──> Backup
BackupTarget ──1:N──> TargetHealthCheck
RemoteConfig ──1:N──> (Instance.source / BackupTarget.storage)
```

### 3.2 核心表结构

#### users

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| email | TEXT UNIQUE | 邮箱，登录凭据 |
| name | TEXT | 显示名称 |
| password_hash | TEXT | bcrypt 哈希 |
| role | TEXT | `admin` / `viewer` |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### instances

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| name | TEXT UNIQUE | 实例名称 |
| source_type | TEXT | `local` / `ssh` |
| source_path | TEXT | 本地路径或远程路径 |
| remote_config_id | INTEGER NULL | 关联 remote_configs.id（SSH 源时） |
| status | TEXT | `idle` / `running` |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### policies

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| instance_id | INTEGER FK | 关联 instances.id |
| name | TEXT | 策略名称 |
| type | TEXT | `rolling` / `cold` |
| target_id | INTEGER FK | 关联 backup_targets.id |
| schedule_type | TEXT | `interval` / `cron` |
| schedule_value | TEXT | 间隔秒数或 cron 表达式 |
| enabled | BOOLEAN | 是否启用 |
| compression | BOOLEAN | 是否压缩（仅冷备份） |
| encryption | BOOLEAN | 是否加密（仅冷备份） |
| encryption_key_hash | TEXT NULL | 加密密钥哈希 |
| split_enabled | BOOLEAN | 是否分卷（仅冷备份） |
| split_size_mb | INTEGER NULL | 分卷大小（MB） |
| retention_type | TEXT | `time` / `count` |
| retention_value | INTEGER | 保留天数或保留数量 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### backup_targets

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| name | TEXT UNIQUE | 目标名称 |
| backup_type | TEXT | `rolling` / `cold` |
| storage_type | TEXT | `local` / `ssh` / `cloud` |
| storage_path | TEXT | 存储路径 |
| remote_config_id | INTEGER NULL | 关联 remote_configs.id |
| total_capacity_bytes | INTEGER NULL | 总容量 |
| used_capacity_bytes | INTEGER NULL | 已用容量 |
| last_health_check | DATETIME NULL | 上次健康检查时间 |
| health_status | TEXT | `healthy` / `degraded` / `unreachable` |
| health_message | TEXT | 健康检查详情 |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

#### backups

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| instance_id | INTEGER FK | 关联 instances.id |
| policy_id | INTEGER FK | 关联 policies.id |
| type | TEXT | `rolling` / `cold` |
| status | TEXT | `pending` / `running` / `success` / `failed` / `cancelled` |
| snapshot_path | TEXT | 备份快照路径 |
| backup_size_bytes | INTEGER | 备份占用大小（实际磁盘） |
| actual_size_bytes | INTEGER | 数据原始大小 |
| started_at | DATETIME NULL | 开始时间 |
| completed_at | DATETIME NULL | 完成时间 |
| duration_seconds | INTEGER | 持续时长 |
| error_message | TEXT | 失败原因 |
| rsync_stats | TEXT | rsync 输出统计（JSON） |
| created_at | DATETIME | 创建时间 |

#### tasks

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| instance_id | INTEGER FK | 关联 instances.id |
| backup_id | INTEGER NULL FK | 关联 backups.id（备份任务） |
| type | TEXT | `rolling` / `cold` / `restore` |
| status | TEXT | `queued` / `running` / `success` / `failed` / `cancelled` |
| progress | INTEGER | 进度百分比 0-100 |
| current_step | TEXT | 当前步骤描述 |
| started_at | DATETIME NULL | 开始时间 |
| completed_at | DATETIME NULL | 完成时间 |
| estimated_end | DATETIME NULL | 预计完成时间 |
| error_message | TEXT | 失败原因 |
| created_at | DATETIME | 创建时间 |

#### remote_configs

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| name | TEXT UNIQUE | 配置名称 |
| type | TEXT | `ssh` / `cloud` |
| host | TEXT | 主机地址（SSH） |
| port | INTEGER | 端口（SSH，默认 22） |
| username | TEXT | 用户名（SSH） |
| private_key_path | TEXT | 私钥文件在服务器上的存储路径 |
| cloud_provider | TEXT NULL | 云存储提供商（预留） |
| cloud_config | TEXT NULL | 云存储配置 JSON（预留） |
| created_at | DATETIME | 创建时间 |
| updated_at | DATETIME | 更新时间 |

> **安全要求**：`private_key_path` 存储的是服务器本地文件路径，私钥内容不经过 API 传输。上传私钥时通过 multipart 接口写入 `DATA_DIR/keys/` 目录，仅存储路径引用。

#### instance_permissions

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| user_id | INTEGER FK | 关联 users.id |
| instance_id | INTEGER FK | 关联 instances.id |
| permission | TEXT | `readonly` / `readonly_download` |
| created_at | DATETIME | 创建时间 |

**唯一约束**：`(user_id, instance_id)`

#### audit_logs

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| instance_id | INTEGER NULL FK | 关联 instances.id（可为空，系统级日志） |
| user_id | INTEGER NULL FK | 操作人 |
| action | TEXT | 操作类型 |
| detail | TEXT | 操作详情（JSON） |
| created_at | DATETIME | 创建时间 |

`action` 枚举值：`instance.create`、`instance.update`、`instance.delete`、`policy.create`、`policy.update`、`policy.delete`、`backup.trigger`、`backup.complete`、`backup.fail`、`restore.trigger`、`restore.complete`、`restore.fail`、`user.create`、`user.update`、`user.delete`、`target.create`、`target.update`、`target.delete`、`remote.create`、`remote.update`、`remote.delete`、`system.config.update`。

#### risk_events

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| instance_id | INTEGER NULL FK | 关联 instances.id |
| target_id | INTEGER NULL FK | 关联 backup_targets.id |
| severity | TEXT | `info` / `warning` / `critical` |
| source | TEXT | 风险来源类型 |
| message | TEXT | 风险描述 |
| resolved | BOOLEAN | 是否已解决 |
| created_at | DATETIME | 创建时间 |
| resolved_at | DATETIME NULL | 解决时间 |

`source` 枚举值：`backup_failed`、`backup_overdue`、`cold_backup_missing`、`target_unreachable`、`target_capacity_low`、`restore_failed`、`credential_error`。

#### notification_subscriptions

| 字段 | 类型 | 说明 |
|------|------|------|
| id | INTEGER PK | 自增主键 |
| user_id | INTEGER FK | 关联 users.id |
| instance_id | INTEGER FK | 关联 instances.id |
| enabled | BOOLEAN | 是否启用 |
| created_at | DATETIME | 创建时间 |

#### system_configs

| 字段 | 类型 | 说明 |
|------|------|------|
| key | TEXT PK | 配置键 |
| value | TEXT | 配置值（JSON） |
| updated_at | DATETIME | 更新时间 |

用于存储 SMTP 配置、注册开关等运行时可变的系统配置。

---

## 4. API 设计

### 4.1 通用约定

- 基础路径：`/api/v1`
- 认证方式：`Authorization: Bearer <JWT>`
- 请求格式：`application/json`（文件上传使用 `multipart/form-data`）
- 响应格式：统一 JSON 包装

```json
{
  "code": 0,
  "message": "ok",
  "data": { ... }
}
```

- 错误响应：

```json
{
  "code": 40001,
  "message": "invalid request",
  "data": null
}
```

- 分页参数：`?page=1&page_size=20`
- 排序参数：`?sort=created_at&order=desc`
- 筛选参数：按具体接口定义

### 4.2 认证接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| POST | `/api/v1/auth/login` | 登录，返回 JWT | 公开 |
| POST | `/api/v1/auth/register` | 注册 | 公开（可关闭） |
| POST | `/api/v1/auth/refresh` | 刷新令牌 | 已认证 |

### 4.3 用户接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/users` | 用户列表 | admin |
| POST | `/api/v1/users` | 创建用户 | admin |
| PUT | `/api/v1/users/:id` | 编辑用户 | admin |
| DELETE | `/api/v1/users/:id` | 删除用户 | admin |
| GET | `/api/v1/users/me` | 当前用户信息 | 已认证 |
| PUT | `/api/v1/users/me/password` | 修改密码 | 已认证 |
| PUT | `/api/v1/users/me/profile` | 修改名称 | 已认证 |
| GET | `/api/v1/users/me/subscriptions` | 通知订阅列表 | 已认证 |
| PUT | `/api/v1/users/me/subscriptions` | 更新通知订阅 | 已认证 |

### 4.4 实例接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/instances` | 实例列表（admin 全量/viewer 有权限的） | 已认证 |
| POST | `/api/v1/instances` | 创建实例 | admin |
| GET | `/api/v1/instances/:id` | 实例详情（含概览统计） | 已认证+实例权限 |
| PUT | `/api/v1/instances/:id` | 编辑实例 | admin |
| DELETE | `/api/v1/instances/:id` | 删除实例 | admin |
| GET | `/api/v1/instances/:id/stats` | 实例统计数据 | 已认证+实例权限 |
| GET | `/api/v1/instances/:id/disaster-recovery` | 容灾率详情 | 已认证+实例权限 |
| PUT | `/api/v1/instances/:id/permissions` | 配置实例访问权限 | admin |

### 4.5 策略接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/instances/:id/policies` | 策略列表 | 已认证+实例权限 |
| POST | `/api/v1/instances/:id/policies` | 创建策略 | admin |
| PUT | `/api/v1/instances/:id/policies/:pid` | 编辑策略 | admin |
| DELETE | `/api/v1/instances/:id/policies/:pid` | 删除策略 | admin |
| POST | `/api/v1/instances/:id/policies/:pid/trigger` | 手动触发策略 | admin |

### 4.6 备份接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/instances/:id/backups` | 备份列表 | 已认证+实例权限 |
| GET | `/api/v1/instances/:id/backups/:bid` | 备份详情 | 已认证+实例权限 |
| POST | `/api/v1/instances/:id/backups/:bid/restore` | 触发恢复 | admin（需密码二次验证） |
| GET | `/api/v1/instances/:id/backups/:bid/download` | 下载冷备份 | 已认证+下载权限 |

### 4.7 任务接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/tasks` | 全局任务列表（运行中 + 队列中） | admin |
| GET | `/api/v1/tasks/:id` | 任务详情（含进度） | 已认证+实例权限 |
| POST | `/api/v1/tasks/:id/cancel` | 取消任务 | admin |

### 4.8 备份目标接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/targets` | 目标列表 | admin |
| POST | `/api/v1/targets` | 创建目标 | admin |
| PUT | `/api/v1/targets/:id` | 编辑目标 | admin |
| DELETE | `/api/v1/targets/:id` | 删除目标 | admin |
| POST | `/api/v1/targets/:id/health-check` | 手动触发健康检查 | admin |

### 4.9 远程配置接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/remotes` | 远程配置列表 | admin |
| POST | `/api/v1/remotes` | 创建远程配置（私钥通过 multipart 上传） | admin |
| PUT | `/api/v1/remotes/:id` | 编辑远程配置 | admin |
| DELETE | `/api/v1/remotes/:id` | 删除远程配置 | admin |
| POST | `/api/v1/remotes/:id/test` | 测试连接 | admin |

### 4.10 审计日志接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/instances/:id/audit-logs` | 实例审计日志 | 已认证+实例权限 |
| GET | `/api/v1/audit-logs` | 全局审计日志 | admin |

### 4.11 系统配置接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/system/smtp` | 获取 SMTP 配置 | admin |
| PUT | `/api/v1/system/smtp` | 更新 SMTP 配置 | admin |
| POST | `/api/v1/system/smtp/test` | 发送测试邮件 | admin |
| GET | `/api/v1/system/registration` | 获取注册开关状态 | 公开 |
| PUT | `/api/v1/system/registration` | 设置注册开关 | admin |

### 4.12 仪表盘接口

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| GET | `/api/v1/dashboard/overview` | 总览卡片数据 | admin |
| GET | `/api/v1/dashboard/risks` | 风险事件列表 | admin |
| GET | `/api/v1/dashboard/trends` | 备份趋势数据 | admin |
| GET | `/api/v1/dashboard/focus-instances` | 重点关注实例 | admin |
| GET | `/api/v1/dashboard/upcoming-tasks` | 即将执行的计划任务 | admin |

---

## 5. 核心模块设计

### 5.1 rsync 执行引擎

#### 5.1.1 滚动备份流程

```
┌─────────┐    ┌────────────────────────────┐    ┌──────────┐
│  源     │───>│ rsync --link-dest=<last>   │───>│  目标    │
│ local/  │    │ -avz --delete --stats      │    │ local/   │
│ ssh     │    │ --log-file=...             │    │ ssh      │
└─────────┘    └────────────────────────────┘    └──────────┘
```

- 每次滚动备份创建独立快照目录：`<target_path>/<instance_name>/<timestamp>/`
- 使用 `--link-dest` 指向上一次成功快照，实现增量硬链接备份；
- 备份完成后更新 `latest` 符号链接指向新快照；

**rsync 参数模板**：

```bash
rsync -avz --delete --stats \
  --link-dest=<last_snapshot_path> \
  [--rsh="ssh -i <key_path> -p <port> -o StrictHostKeyChecking=accept-new"] \
  <source_path>/ \
  <target_path>/<instance_name>/<timestamp>/
```

#### 5.1.2 冷备份流程

```
源 ──rsync──> 临时目录 ──[压缩]──> [加密] ──[分卷]──> 目标路径
```

1. rsync 全量同步到临时目录；
2. 如果启用压缩：`tar + gzip/zstd` 压缩临时目录；
3. 如果启用加密：使用 AES-256-GCM 对压缩包进行对称加密；
4. 如果启用分卷：使用 `split` 分割为指定大小的分卷；
5. 将最终文件移动到目标路径；
6. 清理临时目录；

#### 5.1.3 双远程中继模式

当源和目标均为 SSH 远程时：

```
阶段1: rsync pull  远程源 → 本地中继目录 (DATA_DIR/relay/<instance_id>/)
阶段2: rsync push  本地中继目录 → 远程目标
```

- 中继目录仅保留最新一份，用于下次增量 `--link-dest` 引用；
- 两阶段均使用增量传输；
- 创建策略时前端标记为中继模式，提示磁盘空间需求；

#### 5.1.4 进度解析

通过 rsync `--info=progress2` 参数输出整体进度信息，实时解析 stdout 流：

```
  1,234,567  45%   12.34MB/s    0:01:23
```

解析字段：已传输字节数、进度百分比、传输速率、剩余时间。更新到 `tasks` 表的 `progress`、`current_step`、`estimated_end` 字段。

### 5.2 任务调度器

#### 5.2.1 调度器架构

```
┌──────────────────────────────────┐
│           Scheduler              │
│  ┌──────────────────────────┐   │
│  │  Cron Engine (goroutine) │   │   周期扫描已启用策略
│  └────────┬─────────────────┘   │   根据 schedule_type/value 计算下次触发
│           │ 触发                 │
│  ┌────────▼─────────────────┐   │
│  │    Task Queue (chan)      │   │   带优先级的任务队列
│  └────────┬─────────────────┘   │
│           │ 消费                 │
│  ┌────────▼─────────────────┐   │
│  │  Worker Pool (N workers) │   │   并发执行上限可配置
│  └──────────────────────────┘   │
└──────────────────────────────────┘
```

#### 5.2.2 调度规则

- `interval` 类型：基于上次执行结束时间 + 间隔秒数计算下次触发时间；
- `cron` 类型：标准 5 字段 cron 表达式，使用内嵌的 cron 解析器；
- 服务启动时扫描所有已启用策略，恢复调度计划；
- 手动触发的任务不影响下次计划时间；

#### 5.2.3 并发控制

- 同一实例同一时刻只允许一个备份/恢复任务运行；
- 如果实例已有任务运行，新触发的任务进入队列等待；
- Worker Pool 默认大小 3，可通过配置调整；

### 5.3 备份保留与清理

#### 5.3.1 保留策略

- **按时间**：删除完成时间早于 `now - retention_days` 的备份记录及其快照；
- **按数量**：按完成时间降序排列，保留最新 N 条，删除多余的备份记录及其快照；

#### 5.3.2 清理流程

1. 每次备份完成后触发该策略的清理检查；
2. 后台定时任务每 6 小时全量扫描一次，处理漏掉的清理；
3. 删除顺序：先删除磁盘快照文件，成功后更新数据库记录状态；
4. 删除失败记录到 audit_log，不阻塞后续清理；

### 5.4 恢复引擎

#### 5.4.1 恢复流程

```
冷备份: 目标路径 ──[解密]──> [解压] ──> rsync ──> 恢复路径
滚动备份: 快照目录 ──rsync──> 恢复路径
```

- 恢复到源位置：`rsync --delete` 将快照完整还原到原始源路径；
- 恢复到指定位置：`rsync` 将快照同步到用户指定路径：
- 恢复前需要用户输入实例名称和当前密码进行二次确认；

### 5.5 备份目标健康检查

#### 5.5.1 检查内容

| 目标类型 | 检查项 |
|----------|--------|
| 本地 | 路径存在性、可写性、磁盘容量（`statvfs`） |
| SSH 远程 | SSH 连通性、路径可写性、远程 `df` 获取容量 |
| 云存储 | API 连通性、Bucket 可写性、容量查询（预留） |

#### 5.5.2 检查调度

- 周期：每 30 分钟执行一次所有目标的健康检查；
- 支持手动触发单个目标的健康检查；
- 检查结果更新到 `backup_targets` 表；
- 状态变化时生成 `risk_events` 记录；

### 5.6 容灾率计算

容灾率是对实例可恢复能力的综合评估，公式如下：

```
容灾率 = 0.35 × 备份新鲜度 + 0.30 × 恢复点可用性 + 0.20 × 冗余与隔离度 + 0.15 × 执行稳定性
```

#### 5.6.1 计算实现

```go
type DisasterRecoveryScore struct {
    Total           float64  // 总分 0-100
    Level           string   // safe/caution/risk/danger
    Freshness       float64  // 备份新鲜度分项
    RecoveryPoints  float64  // 恢复点可用性分项
    Redundancy      float64  // 冗余与隔离度分项
    Stability       float64  // 执行稳定性分项
    Deductions      []string // 主要扣分原因
}
```

**备份新鲜度** (满分 100)：
- 基线 = 所有已启用策略中最短备份周期；
- 最近一次成功备份距今 ≤ 1 个周期：100 分；
- 1~2 个周期：线性递减至 60 分；
- 2~3 个周期：线性递减至 30 分；
- 超过 3 个周期或无自动计划：0~20 分；

**恢复点可用性** (满分 100)：
- 每个已启用策略至少有一个未过期、状态为 success 的备份：基础 80 分；
- 多个连续可用恢复点加分至 100；
- 任何已启用策略无可用恢复点：该策略贡献 0 分，按策略数加权平均；

**冗余与隔离度** (满分 100)：
- ≥2 个健康目标且至少一个为远程：100 分；
- ≥2 个健康目标均为本地：70 分；
- 1 个健康远程目标：60 分；
- 1 个健康本地目标：40 分；
- 目标不可达/容量不足：每个异常目标扣 20 分；

**执行稳定性** (满分 100)：
- 最近 10 次备份成功率 ×80 + 无阻塞风险加分 20；
- 连续失败 ≥3 次：扣至 20 分以下；
- 存在阻塞性风险（目标不可达、容量耗尽、凭证错误）：0 分；

#### 5.6.2 等级定义

| 分数区间 | 等级 | 说明 |
|----------|------|------|
| 85-100 | 安全 (safe) | 系统可恢复能力良好 |
| 70-84 | 注意 (caution) | 存在需关注的问题 |
| 40-69 | 风险 (risk) | 可恢复能力受损 |
| 0-39 | 危险 (danger) | 可恢复能力严重不足 |

#### 5.6.3 计算时机

- 备份完成后重新计算关联实例的容灾率；
- 目标健康检查状态变化后重新计算引用该目标的实例；
- 风险事件产生或解除后重新计算；
- 仪表盘请求时按需计算（带 5 分钟缓存）；

### 5.7 风险事件系统

#### 5.7.1 风险检测规则

| 来源 | 触发条件 | 等级 |
|------|----------|------|
| `backup_failed` | 备份任务执行失败 | warning（首次）/ critical（连续≥3次） |
| `backup_overdue` | 距上次成功备份超过计划周期的 2 倍 | warning / critical（超过 3 倍） |
| `cold_backup_missing` | 实例有滚动备份策略但无冷备份策略 | info |
| `target_unreachable` | 备份目标健康检查失败 | critical |
| `target_capacity_low` | 目标剩余容量 < 20% | warning / critical（< 5%） |
| `restore_failed` | 恢复任务执行失败 | critical |
| `credential_error` | SSH 连接认证失败 | critical |

#### 5.7.2 风险生命周期

1. **产生**：检测触发 → 创建 `risk_events` 记录 → 写入 audit_log → 发送通知（如已订阅）；
2. **升级**：同类型风险持续存在且加重时更新等级；
3. **解决**：条件消失后标记 `resolved=true`，记录 `resolved_at`；

### 5.8 通知模块

#### 5.8.1 通知渠道

首版仅实现邮件通知（SMTP），预留扩展接口。

#### 5.8.2 通知时机

- 风险事件产生或升级为 critical 时；
- 发送给已订阅该实例通知的用户；

#### 5.8.3 实现要点

- 异步发送，不阻塞业务流程；
- 发送失败重试 3 次，间隔指数退避；
- 未配置 SMTP 时降级为后台日志输出；

---

## 6. 认证与权限

### 6.1 JWT 认证

- 算法：HS256；
- 密钥：`RBS_JWT_SECRET` 环境变量；
- Access Token 有效期：24 小时；
- Refresh Token 有效期：7 天；
- Token Payload：

```json
{
  "sub": 1,
  "email": "admin@example.com",
  "role": "admin",
  "exp": 1700000000,
  "iat": 1699913600
}
```

### 6.2 权限模型

| 主体 | 范围 | 能力 |
|------|------|------|
| admin | 全局 | 所有操作 |
| viewer | 被授权的实例 | 查看实例详情、备份列表、审计日志 |
| viewer + download | 被授权的实例 | 上述 + 下载冷备份 |

### 6.3 权限检查中间件

```
请求 → JWT 解析 → 角色检查 → 实例权限检查（如涉及实例资源） → Handler
```

- admin 跳过实例权限检查；
- viewer 的实例级请求需查询 `instance_permissions` 表；

### 6.4 初始用户

- 第一个注册的用户自动成为 admin；
- 后续注册用户为 viewer；
- admin 角色唯一，不可删除；

---

## 7. 配置管理

### 7.1 启动配置（.env + 环境变量）

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `RBS_DATA_DIR` | 否 | `./data` | 数据目录（数据库、密钥、缓存、日志） |
| `RBS_PORT` | 否 | `8080` | HTTP 监听端口 |
| `RBS_JWT_SECRET` | 是 | - | JWT 签名密钥 |
| `RBS_WORKER_POOL_SIZE` | 否 | `3` | 并发任务 Worker 数 |
| `RBS_LOG_LEVEL` | 否 | `info` | 日志级别 |

优先级：环境变量 > `.env` 文件。

### 7.2 数据目录结构

```
DATA_DIR/
  rbs.db                 # SQLite 数据库
  keys/                  # SSH 私钥文件存储
    <uuid>.pem
  relay/                 # 双远程中继缓存
    <instance_id>/
  temp/                  # 冷备份临时目录
  logs/                  # 运行日志
```

### 7.3 运行时配置（存储在 system_configs 表）

| key | 说明 |
|-----|------|
| `smtp.host` | SMTP 服务器地址 |
| `smtp.port` | SMTP 端口 |
| `smtp.username` | SMTP 用户名 |
| `smtp.password` | SMTP 密码（AES 加密存储） |
| `smtp.from` | 发件人邮箱 |
| `registration.enabled` | 是否允许注册 |

---

## 8. 前端架构

### 8.1 工程结构

```
web/
  src/
    api/               # API 请求封装（按模块）
    assets/             # 静态资源
    components/         # 通用组件（Button、Input、Table、Modal 等）
    composables/        # 组合式函数（useAuth、useToast、useTask 等）
    layouts/            # 布局组件（AppLayout、AuthLayout）
    pages/              # 页面组件
      dashboard/
      instances/
      targets/
      system/
      profile/
      auth/
    router/             # Vue Router 路由配置
    stores/             # Pinia 状态管理
    styles/             # 全局样式、token 变量
    types/              # TypeScript 类型定义
    utils/              # 工具函数
    App.vue
    main.ts
```

### 8.2 路由结构

| 路径 | 页面 | 权限 |
|------|------|------|
| `/login` | 登录 | 公开 |
| `/register` | 注册 | 公开 |
| `/dashboard` | 仪表盘 | admin |
| `/instances` | 实例列表 | 已认证 |
| `/instances/:id` | 实例详情（tab 切换） | 已认证+实例权限 |
| `/targets` | 备份目标 | admin |
| `/system` | 系统配置（远程配置/用户管理/SMTP 配置 Tab 页） | admin |
| `/system/risks` | 风险事件 | admin |
| `/profile` | 个人中心 | 已认证 |

### 8.3 状态管理

使用 Pinia 管理以下全局状态：

- `useAuthStore`：当前用户信息、JWT 令牌、登录/登出逻辑；
- `useThemeStore`：主题切换（浅色/深色）；
- `useTaskStore`：全局运行中任务状态（轮询更新）；
- `useToastStore`：全局 toast 通知队列；

### 8.4 实时数据更新

- 任务进度：前端轮询 `/api/v1/tasks/:id`，间隔 2 秒；
- 仪表盘数据：进入页面时加载，后台每 30 秒刷新；
- 首版不实现 WebSocket，通过轮询满足实时性需求；

### 8.5 响应式与主题

- 断点：`sm: 640px`、`md: 768px`、`lg: 1024px`、`xl: 1280px`；
- 桌面端（≥1024px）：左侧固定导航栏 + 右侧内容区；
- 移动端（<1024px）：顶部菜单按钮 + 抽屉导航 + 全屏内容区；
- 主题通过 CSS 变量切换，样式 token 体系参见 [组件样式设计](./component-style-design.md)；

---

## 9. 安全设计

### 9.1 认证安全

- 密码使用 bcrypt 哈希存储，cost 因子 ≥ 12；
- JWT 密钥通过环境变量注入，不硬编码；
- 登录失败 5 次后锁定 15 分钟（基于 IP + 邮箱）；

### 9.2 API 安全

- 所有写操作要求 CSRF token 或自定义请求头验证；
- 恢复操作要求二次密码验证；
- 文件下载接口使用一次性临时令牌，防止 URL 泄露后的未授权下载；
- 分页接口限制单页最大返回数量，防止资源耗尽；

### 9.3 数据安全

- SSH 私钥文件权限设置为 `0600`，存储在 `DATA_DIR/keys/`；
- 私钥内容不通过任何 API 返回，仅存储路径引用；
- 备份加密使用 AES-256-GCM，密钥由用户提供，哈希存入数据库，原始密钥不存储；
- SMTP 密码在数据库中 AES 加密存储；

### 9.4 输入验证

- 所有 API 输入参数在 handler 层进行类型和范围校验；
- 文件路径参数进行路径遍历检测，禁止 `../` 等相对路径注入；
- SSH 连接参数校验格式合法性；

---

## 10. 构建与部署

### 10.1 构建流程

```bash
# 1. 构建前端
cd web && npm install && npm run build

# 2. 构建后端（嵌入前端产物）
go build -o rbs cmd/server/main.go
```

前端构建产物通过 Go `embed.FS` 嵌入二进制文件，服务启动后由 Go HTTP Server 直接提供静态文件服务。

### 10.2 Docker 部署

```dockerfile
# 多阶段构建
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/ .
RUN npm ci && npm run build

FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY . .
COPY --from=frontend /app/web/dist web/dist
RUN go build -o /rbs cmd/server/main.go

FROM alpine:3.19
RUN apk add --no-cache rsync openssh-client
COPY --from=backend /rbs /usr/local/bin/rbs
EXPOSE 8080
CMD ["rbs"]
```

### 10.3 运行要求

- 操作系统：Linux（推荐）/ macOS；
- 外部依赖：`rsync` (≥3.1)、`openssh-client`（SSH 备份场景）；
- 最低资源：1 核 CPU、256MB 内存；

---

## 11. 可扩展性设计

### 11.1 云存储接口

```go
type CloudStorageProvider interface {
    Upload(ctx context.Context, localPath string, remotePath string) error
    Download(ctx context.Context, remotePath string, localPath string) error
    Delete(ctx context.Context, remotePath string) error
    ListFiles(ctx context.Context, prefix string) ([]FileInfo, error)
    GetCapacity(ctx context.Context) (total, used int64, err error)
    TestConnection(ctx context.Context) error
}
```

首版仅定义接口，不做实现。后续可新增 S3、Azure Blob、WebDAV 等 Provider。

### 11.2 通知渠道扩展

```go
type NotificationChannel interface {
    Send(ctx context.Context, to string, subject string, body string) error
    TestConnection(ctx context.Context) error
}
```

首版仅实现 SMTP 邮件通道，可扩展 Webhook、Telegram、Slack 等。

