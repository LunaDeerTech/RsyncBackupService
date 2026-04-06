# Rsync Backup Service — 开发计划

---

## 概述

本文档基于 [system-design.md](./system-design.md) 将 Rsync Backup Service（RBS）项目拆分为可实施的开发阶段与具体步骤。每个阶段以可验证的里程碑为目标，前一阶段的完成是后一阶段的前置条件。

整体路线：**基础设施 → 核心后端 → 前端框架 → 功能逐层叠加 → 高级特性 → 集成与部署**。

---

## 阶段一：项目脚手架与基础设施

**目标**：搭建前后端工程骨架，建立开发、构建、嵌入流程，确保"零功能"状态下可编译运行。

### 步骤 1.1 — 后端工程初始化

- 初始化 Go module（`go mod init`），创建 `cmd/server/main.go` 入口；
- 按设计文档建立 `internal/` 分层目录结构：`config`、`model`、`store`、`service`、`engine`、`handler`、`middleware`、`notify`、`audit`、`crypto`、`util`；
- 引入 `modernc.org/sqlite` 依赖；
- 实现配置加载模块（`internal/config`）：读取 `.env` 文件与环境变量，支持 `RBS_DATA_DIR`、`RBS_PORT`、`RBS_JWT_SECRET`、`RBS_WORKER_POOL_SIZE`、`RBS_LOG_LEVEL`；
- 实现数据目录自动创建：`DATA_DIR/`、`keys/`、`relay/`、`temp/`、`logs/`；

### 步骤 1.2 — 数据库初始化与迁移

- 实现 SQLite 连接管理（WAL 模式），封装 `store.DB` 结构；
- 编写完整建表 DDL，覆盖设计文档所有表：`users`、`instances`、`policies`、`backup_targets`、`backups`、`tasks`、`remote_configs`、`instance_permissions`、`audit_logs`、`risk_events`、`notification_subscriptions`、`system_configs`；
- 实现简易版本迁移机制（基于 `system_configs` 表中的 `schema_version`），服务启动时自动执行；

### 步骤 1.3 — HTTP 服务骨架

- 启动 Go HTTP Server，注册基础路由（`/api/v1/health`）；
- 实现统一 JSON 响应包装（`code` / `message` / `data`）；
- 实现统一错误响应格式；
- 实现分页参数解析工具（`page`、`page_size`、`sort`、`order`）；
- 添加 CORS 中间件（开发期允许跨域）；
- 添加请求日志中间件；

### 步骤 1.4 — 前端工程初始化

- 在 `web/` 目录下使用 Vite + Vue 3 + TypeScript 脚手架创建项目；
- 安装 Tailwind CSS，配置 token 基线色彩体系（参照 [component-style-design.md](./component-style-design.md) 中的深色/浅色 token）；
- 建立前端目录结构：`api/`、`components/`、`composables/`、`layouts/`、`pages/`、`router/`、`stores/`、`styles/`、`types/`、`utils/`；
- 实现 API 请求封装层（Axios / Fetch），统一处理 JWT 注入、响应解包、错误处理；
- 实现基础主题切换机制（CSS 变量 + `useThemeStore`）；

### 步骤 1.5 — 前后端嵌入与构建流程

- 后端实现 `embed.FS` 嵌入 `web/dist` 静态文件；
- Go HTTP Server 配置 SPA fallback（非 `/api` 路径返回 `index.html`）；
- 编写 Makefile / shell 脚本：`make build`（前端构建 → 后端编译 → 单体二进制）；
- 验证里程碑：编译运行后浏览器访问可见 Vue 空白页、`/api/v1/health` 返回 200；

---

## 阶段二：认证与用户管理

**目标**：实现完整的用户注册、登录、JWT 认证与权限中间件，支撑后续所有业务接口的权限控制。

### 步骤 2.1 — 用户数据层

- 实现 `model.User` 结构体定义；
- 实现 `store` 层 User CRUD：`CreateUser`、`GetUserByEmail`、`GetUserByID`、`ListUsers`、`UpdateUser`、`DeleteUser`；
- 实现密码工具（`internal/crypto`）：bcrypt 哈希（cost ≥ 12）、密码校验；

### 步骤 2.2 — 认证接口

- 实现 `POST /api/v1/auth/register`：
  - 第一个注册用户自动为 admin，后续为 viewer；
  - 生成随机密码，尝试通过 SMTP 发送到邮箱；SMTP 未配置时在后台日志输出密码；
- 实现 `POST /api/v1/auth/login`：
  - 校验邮箱密码，返回 Access Token + Refresh Token；
  - 登录失败 5 次锁定 15 分钟（IP + 邮箱维度）；
- 实现 `POST /api/v1/auth/refresh`：刷新 Access Token；
- JWT 工具：生成（HS256, 24h/7d）、解析、校验；

### 步骤 2.3 — 认证与权限中间件

- 实现 JWT 认证中间件：解析 `Authorization: Bearer` header，注入用户信息到请求上下文；
- 实现角色检查中间件：`RequireAdmin`、`RequireAuth`；
- 实现实例权限检查中间件：查询 `instance_permissions` 表，admin 跳过；

### 步骤 2.4 — 用户管理接口

- 实现 `GET/POST/PUT/DELETE /api/v1/users`（admin）；
- 实现 `GET /api/v1/users/me`、`PUT /api/v1/users/me/password`、`PUT /api/v1/users/me/profile`（已认证）；

### 步骤 2.5 — 前端登录与注册页面

- 实现 `AuthLayout` 布局组件（居中卡片式登录界面）；
- 实现登录页（`/login`）：邮箱 + 密码表单、错误提示、登录成功后跳转；
- 实现注册页（`/register`）：邮箱表单、提交后提示检查邮箱；
- 实现 `useAuthStore`（Pinia）：存储 JWT、用户信息、登录/登出逻辑、Token 自动刷新；
- 实现路由守卫：未登录跳转登录页、已登录跳转对应首页（admin → 仪表盘 / viewer → 实例列表）；

---

## 阶段三：前端布局框架与基础组件库

**目标**：实现主布局框架与通用 UI 组件，为后续所有页面提供统一基础。

### 步骤 3.1 — 布局组件

- 实现 `AppLayout`：左侧固定导航栏 + 右侧内容区；
- 导航栏菜单项：仪表盘（admin）、实例列表、备份目标（admin）、系统配置（admin）、个人中心；
- 移动端响应式：导航栏收为抽屉式菜单（<1024px 断点）；
- 顶部栏：主题切换按钮、用户头像/下拉菜单；

### 步骤 3.2 — 基础 UI 组件

按照 [component-style-design.md](./component-style-design.md) 的规范，实现以下通用组件：

- **操作型组件**：Button（primary / outline / danger / ghost）、Input（text / password / number）、Select、Switch、Checkbox；
- **数据展示组件**：Table（分页、排序、行选中）、Badge / Tag（状态标签）、ProgressBar；
- **反馈组件**：Modal（对话框）、Toast（通知，`useToastStore`）、Confirm（确认弹窗）；
- **布局组件**：Card、Tabs、Drawer、EmptyState；
- **表单组件**：FormItem（标签 + 输入 + 错误提示）、FormGroup；

### 步骤 3.3 — 全局样式系统

- 定义 CSS 变量文件：深色/浅色 token 完整定义；
- 实现主题切换：通过根元素 `data-theme` 属性切换 CSS 变量集；
- 统一排版规范：字号、字重、间距；
- 统一图标方案：选定图标库（Lucide / Heroicons），封装 Icon 组件；

---

## 阶段四：远程配置与备份目标管理

**目标**：实现 SSH 连接配置与备份目标 CRUD，为后续备份引擎提供基础设施依赖。

### 步骤 4.1 — 远程配置后端

- 实现 `model.RemoteConfig` 结构体；
- 实现 `store` 层 RemoteConfig CRUD；
- 实现 `service` 层：创建时通过 multipart 接收私钥文件，存储到 `DATA_DIR/keys/<uuid>.pem`，权限设为 `0600`；
- 实现 `POST /api/v1/remotes/:id/test`：SSH 连接测试（建立连接并执行 `echo ok`）；
- 实现 `GET/POST/PUT/DELETE /api/v1/remotes`；
- 安全约束：API 返回的 RemoteConfig 不包含 `private_key_path` 字段；

### 步骤 4.2 — 备份目标后端

- 实现 `model.BackupTarget` 结构体；
- 实现 `store` 层 BackupTarget CRUD；
- 实现 `service` 层：创建/编辑时校验存储类型与备份类型的合法组合（滚动仅支持 local/ssh，冷支持 local/ssh/cloud）；
- 实现 `GET/POST/PUT/DELETE /api/v1/targets`；
- 实现 `POST /api/v1/targets/:id/health-check`（手动触发）；

### 步骤 4.3 — 备份目标健康检查

- 实现健康检查引擎（`engine/health_checker.go`）：
  - 本地目标：路径存在性、可写性、`statvfs` 获取容量；
  - SSH 远程目标：SSH 连通性、路径可写性、远程 `df` 获取容量；
- 后台定时任务：每 30 分钟执行全部目标健康检查；
- 检查结果更新 `backup_targets` 表（`health_status`、`health_message`、`total_capacity_bytes`、`used_capacity_bytes`、`last_health_check`）；

### 步骤 4.4 — 前端远程配置页面

- 实现远程配置列表（系统配置 `/system` 的远程配置 Tab）：表格展示，支持新增/编辑/删除；
- 新增/编辑 Modal：名称、类型（SSH/云存储）、SSH 表单（host/port/username/私钥上传/私钥密码）、测试连接按钮；
- 云存储表单暂留占位；

### 步骤 4.5 — 前端备份目标页面

- 实现备份目标列表页（`/targets`）：表格展示名称、备份类型、存储类型、容量条形图、健康状态、操作；
- 新增/编辑 Modal：名称、备份类型、存储类型（联动可选项）、存储路径、关联远程配置；
- 手动触发健康检查按钮；

---

## 阶段五：实例管理与策略配置

**目标**：实现备份实例的完整生命周期管理与策略 CRUD，为备份引擎提供任务来源。

### 步骤 5.1 — 实例后端

- 实现 `model.Instance` 结构体；
- 实现 `store` 层 Instance CRUD，包含实例权限查询；
- 实现 `service` 层：创建实例时校验数据源类型、关联远程配置有效性；
- 实现 `GET/POST/PUT/DELETE /api/v1/instances`；
- 实现 `GET /api/v1/instances/:id`（含概览统计）；
- 实现 `PUT /api/v1/instances/:id/permissions`（配置 viewer 权限）；

### 步骤 5.2 — 策略后端

- 实现 `model.Policy` 结构体；
- 实现 `store` 层 Policy CRUD；
- 实现 `service` 层：
  - 创建/编辑策略时校验：目标存在性、目标类型与策略类型兼容性、cron 表达式合法性；
  - 冷备份选项（压缩/加密/分卷）仅在 `type=cold` 时生效；
  - 加密密钥哈希存储，原始密钥不做持久化；
- 实现 `GET/POST/PUT/DELETE /api/v1/instances/:id/policies`；
- 实现 `POST /api/v1/instances/:id/policies/:pid/trigger`（手动触发）；

### 步骤 5.3 — 前端实例列表页

- 实现实例列表页（`/instances`）：表格展示名称、数据源、状态（运行中/空闲）、上次备份结果+时间；
- 创建实例 Modal：名称、数据源类型（本地/SSH）、路径、关联远程配置（SSH 时）；
- admin 可见所有，viewer 仅可见有权限的实例；

### 步骤 5.4 — 前端实例详情页

- 实现实例详情页（`/instances/:id`），使用 Tab 组织：
  - **概览 Tab**：实例名称、数据源、状态、上次备份、容灾率（后续阶段接入）、当前任务进度（如有）、备份总数/容量条形图、成功率、最近 5 条备份记录、最近 5 条计划任务；
  - **策略 Tab**：策略列表表格（名称/类型/目标/上次执行/成功率/操作），新增/编辑策略 Modal（完整表单）；
  - **备份 Tab**：备份列表（完成时间/类型/大小/持续时间/操作），恢复与下载入口（后续阶段实现操作逻辑）；
  - **审计 Tab**：审计日志列表（后续阶段接入数据）；
  - **设置 Tab**（admin）：基础信息编辑、访问权限配置（用户列表 + 权限选择）；

---

## 阶段六：备份引擎核心

**目标**：实现 rsync 执行引擎、任务队列与调度器，完成滚动备份与冷备份的端到端流程。

### 步骤 6.1 — rsync 命令执行器

- 实现 rsync 命令构建器（`engine/rsync.go`）：
  - 根据源/目标类型（local/ssh）和参数模板拼装 rsync 命令；
  - 支持 `--link-dest`、`--rsh`（SSH 连接参数）、`--delete`、`--stats`、`--info=progress2`；
- 实现命令执行与 stdout/stderr 流式读取；
- 实现进度解析：解析 `--info=progress2` 输出，提取进度百分比、传输速率、剩余时间；

### 步骤 6.2 — 滚动备份流程

- 实现滚动备份执行器（`engine/rolling_backup.go`）：
  1. 创建快照目录：`<target_path>/<instance_name>/<timestamp>/`；
  2. 查找上一次成功快照路径作为 `--link-dest` 参数；
  3. 执行 rsync 命令；
  4. 成功后更新 `latest` 符号链接；
  5. 写入 `backups` 记录（状态、大小、时长、rsync 统计）；
- 实现双远程中继模式：分两阶段 pull + push，中继目录 `DATA_DIR/relay/<instance_id>/`；

### 步骤 6.3 — 冷备份流程

- 实现冷备份执行器（`engine/cold_backup.go`）：
  1. rsync 全量同步到 `DATA_DIR/temp/<task_id>/`；
  2. 可选压缩：`tar + gzip/zstd`；
  3. 可选加密：AES-256-GCM 对称加密；
  4. 可选分卷：按指定大小 split；
  5. 移动最终文件到目标路径；
  6. 清理临时目录；
  7. 写入 `backups` 记录；

### 步骤 6.4 — 任务队列与 Worker Pool

- 实现 `model.Task` 结构体与 `store` 层 CRUD；
- 实现任务队列（`engine/task_queue.go`）：
  - 基于 channel 的优先级队列；
  - 同一实例同一时刻仅允许一个备份/恢复任务运行，其余排队；
- 实现 Worker Pool（`engine/worker_pool.go`）：
  - 可配置 worker 数量（默认 3）；
  - Worker 消费队列，执行备份/恢复任务，更新任务状态和进度；
- 实现 `GET /api/v1/tasks`、`GET /api/v1/tasks/:id`、`POST /api/v1/tasks/:id/cancel`；

### 步骤 6.5 — 调度器

- 实现调度器（`engine/scheduler.go`）：
  - 周期扫描所有启用策略，根据 `schedule_type` / `schedule_value` 计算下次触发时间；
  - `interval` 类型：上次执行结束时间 + 间隔秒数；
  - `cron` 类型：标准 5 字段 cron 解析；
  - 到达触发时间时创建 `task` 记录并投入任务队列；
- 服务启动时从数据库恢复所有启用策略的调度计划；
- 手动触发不影响自动计划的下次触发时间；

### 步骤 6.6 — 备份保留与自动清理

- 实现保留策略清理（`engine/retention.go`）：
  - 按时间保留：删除 `completed_at < now - retention_days` 的备份；
  - 按数量保留：保留最新 N 条，删除多余备份；
- 清理流程：先删磁盘文件/快照，再更新数据库；
- 触发时机：每次备份完成后检查该策略，后台定时任务每 6 小时全量扫描；
- 删除失败写入 audit_log，不阻塞后续清理；

---

## 阶段七：恢复引擎

**目标**：实现备份恢复的完整流程。

### 步骤 7.1 — 恢复执行器

- 实现恢复执行器（`engine/restore.go`）：
  - 滚动备份恢复：rsync 快照目录 → 恢复路径（`--delete` 用于恢复到源位置）；
  - 冷备份恢复：解密 → 解压 → rsync 到恢复路径；
- 恢复任务同样通过任务队列调度；

### 步骤 7.2 — 恢复接口与安全验证

- 实现 `POST /api/v1/instances/:id/backups/:bid/restore`：
  - 请求体：恢复类型（源位置/指定位置）、目标路径（指定位置时必填）、实例名称确认、用户密码二次验证；
  - 创建恢复任务投入队列；
- 实现 `GET /api/v1/instances/:id/backups/:bid/download`：
  - 仅冷备份可下载；
  - 生成一次性临时下载令牌，防止 URL 泄露；

### 步骤 7.3 — 前端恢复交互

- 在备份列表的操作列实现恢复按钮：
  - 打开恢复确认 Modal：选择恢复类型、输入目标路径（指定位置时）、输入实例名称确认、输入当前密码；
  - 提交后跳转回概览 Tab，显示恢复任务进度；
- 冷备份下载按钮：点击后触发文件下载；

---

## 阶段八：审计日志

**目标**：实现操作审计日志的记录与查询，覆盖所有关键业务操作。

### 步骤 8.1 — 审计日志写入

- 实现审计日志模块（`internal/audit`）：
  - 提供 `LogAction(ctx, instanceID, userID, action, detail)` 方法；
  - 在各 service 层操作的关键节点插入审计记录：
    - 实例：create / update / delete；
    - 策略：create / update / delete；
    - 备份：trigger / complete / fail；
    - 恢复：trigger / complete / fail；
    - 用户：create / update / delete；
    - 目标：create / update / delete；
    - 远程配置：create / update / delete；
    - 系统配置：update；

### 步骤 8.2 — 审计日志查询接口

- 实现 `GET /api/v1/audit-logs`（全局，admin）；
- 实现 `GET /api/v1/instances/:id/audit-logs`（实例级）；
- 支持筛选参数：时间范围（`start_date`、`end_date`）、操作类型（`action`）；
- 支持分页；

### 步骤 8.3 — 前端审计日志

- 实例详情页审计 Tab 接入数据：表格展示时间、操作类型、操作人、详情；支持时间范围和操作类型筛选；
- 全局审计日志页面（如后续需要，可在系统配置中添加入口）；

---

## 阶段九：容灾率与风险事件

**目标**：实现容灾率计算引擎与风险事件系统，为仪表盘和运维预警提供数据支撑。

### 步骤 9.1 — 容灾率计算引擎

- 实现 `DisasterRecoveryScore` 结构体与计算逻辑（`service/disaster_recovery.go`）：
  - 备份新鲜度（0.35 权重）：基于最短启用策略周期 vs 最近成功备份时间；
  - 恢复点可用性（0.30 权重）：每个启用策略是否存在未过期、成功的恢复点；
  - 冗余与隔离度（0.20 权重）：目标数量、类型分布、健康状态评估；
  - 执行稳定性（0.15 权重）：最近 10 次成功率、连续失败、阻塞性风险；
- 等级映射：85-100 安全 / 70-84 注意 / 40-69 风险 / 0-39 危险；
- 实现 `GET /api/v1/instances/:id/disaster-recovery`；

### 步骤 9.2 — 容灾率计算触发

- 备份完成后重新计算关联实例容灾率；
- 目标健康状态变化后重新计算引用该目标的实例；
- 风险事件产生/解除后重新计算；
- 结果带 5 分钟缓存，避免频繁计算；

### 步骤 9.3 — 风险事件检测与生命周期

- 实现风险检测引擎（`engine/risk_detector.go`）：
  - `backup_failed`：备份失败时触发，连续 ≥3 次升级为 critical；
  - `backup_overdue`：距上次成功备份超过计划周期 2 倍触发 warning，3 倍触发 critical；
  - `cold_backup_missing`：有滚动策略但无冷备份策略，触发 info；
  - `target_unreachable`：健康检查失败，触发 critical；
  - `target_capacity_low`：剩余容量 <20% warning，<5% critical；
  - `restore_failed`：恢复失败，触发 critical；
  - `credential_error`：SSH 认证失败，触发 critical；
- 实现风险生命周期：产生 → 升级 → 解决（`resolved=true`）；
- 风险产生时写入 audit_log，触发通知（如已订阅）；

### 步骤 9.4 — 前端容灾率展示

- 实例详情概览 Tab 接入容灾率数据：分数、等级（色彩标识）、四项分项得分、扣分原因列表；
- 实例列表增加容灾率列；

---

## 阶段十：通知模块

**目标**：实现 SMTP 邮件通知与通知订阅管理。

### 步骤 10.1 — SMTP 配置与邮件发送

- 实现 SMTP 配置存取（`system_configs` 表）：SMTP 密码 AES 加密存储；
- 实现 `GET/PUT /api/v1/system/smtp`；
- 实现 `POST /api/v1/system/smtp/test`（发送测试邮件）；
- 实现邮件发送模块（`internal/notify`）：异步发送、失败重试 3 次（指数退避）、未配置 SMTP 降级为日志输出；

### 步骤 10.2 — 通知订阅

- 实现 `GET/PUT /api/v1/users/me/subscriptions`；
- 风险事件产生/升级为 critical 时，向订阅了该实例的用户发送邮件通知；

### 步骤 10.3 — 前端 SMTP 配置页

- 实现 SMTP 配置（系统配置 `/system` 的 SMTP Tab）：表单（服务器/端口/用户名/密码/发件人）+ 测试发送按钮；

### 步骤 10.4 — 前端通知订阅

- 个人中心页面实现通知订阅管理：网格展示所有可访问实例，每个实例带开关控制是否订阅；

---

## 阶段十一：仪表盘

**目标**：实现管理员仪表盘，提供系统全局总览与风险预警。

### 步骤 11.1 — 仪表盘后端接口

- 实现 `GET /api/v1/dashboard/overview`：运行中任务数、异常实例数、待处理风险数、系统综合容灾率、备份目标健康度摘要；
- 实现 `GET /api/v1/dashboard/risks`：风险事件列表（未解决）；
- 实现 `GET /api/v1/dashboard/trends`：最近备份结果趋势（24h/7d）、实例健康分布；
- 实现 `GET /api/v1/dashboard/focus-instances`：容灾率最低或风险最多的 5~8 个实例；
- 实现 `GET /api/v1/dashboard/upcoming-tasks`：即将执行的计划任务列表；

### 步骤 11.2 — 前端仪表盘页面

- 实现仪表盘页面（`/dashboard`）：
  - 顶部总览卡片区：运行中任务数、异常实例数、待处理风险数、系统容灾率（分数 + 等级 + 趋势）、备份目标健康度；
  - 运行态与风险区：当前任务与队列列表、风险提醒列表；
  - 趋势与分布区：实例健康分布图、最近备份结果趋势图、即将执行的计划任务日历/列表；
  - 重点关注实例区：按容灾率/风险排序的实例卡片，点击跳转实例详情；
  - 快捷入口区：实例列表、备份目标、远程配置、审计日志；
- 数据刷新策略：进入页面加载，后台每 30 秒轮询刷新；

---

## 阶段十二：系统配置与用户管理页面

**目标**：补齐管理后台的系统配置功能页面。

### 步骤 12.1 — 用户管理页面

- 实现用户管理（系统配置 `/system` 的用户管理 Tab）：用户列表表格（邮箱/角色/创建时间/操作）；
- 新增用户 Modal：邮箱输入，提交后生成随机密码并发送；
- 编辑用户 Modal；
- 删除用户确认（admin 不可删除）；

### 步骤 12.2 — 注册开关

- 实现 `GET/PUT /api/v1/system/registration`；
- 用户管理页面或系统配置区域增加注册开关控制；

### 步骤 12.3 — 风险事件页面

- 实现风险事件列表页面（`/system/risks`）：表格展示严重等级、来源、关联实例/目标、描述、是否解决、时间；
- 支持按等级和来源类型筛选；

### 步骤 12.4 — 个人中心页面

- 实现个人中心页面（`/profile`）：
  - 修改密码表单；
  - 修改名称表单；
  - 通知订阅管理（步骤 10.4）；

---

## 阶段十三：前端任务进度实时展示

**目标**：实现任务进度的实时展示，提升用户对运行中任务的可观察性。

### 步骤 13.1 — 任务进度轮询

- 实现 `useTaskStore`（Pinia）：管理全局运行中任务状态；
- 实现轮询逻辑：每 2 秒请求 `/api/v1/tasks/:id`，获取 `progress`、`current_step`、`estimated_end`；
- 任务完成/失败时停止轮询，触发 Toast 通知；

### 步骤 13.2 — 进度 UI 展示

- 实例详情概览 Tab：运行中任务显示进度条、当前步骤、开始时间、预计完成时间、剩余时间；
- 仪表盘当前任务列表：实时进度展示；
- 全局顶部栏：运行中任务数指示器（可选）；

---

## 阶段十四：安全加固与输入验证

**目标**：全面加固系统安全性，覆盖设计文档的安全要求。

### 步骤 14.1 — API 安全

- 实现 CSRF 防护：自定义请求头验证（如 `X-Requested-With`）；
- 实现分页接口最大返回数量限制（如 `page_size` 最大 100）；
- 文件下载接口实现一次性临时令牌机制；

### 步骤 14.2 — 输入验证

- 在所有 handler 层实现参数类型和范围校验；
- 文件路径参数实现路径遍历检测（禁止 `../` 等）；
- SSH 连接参数格式合法性校验；
- cron 表达式合法性校验；

### 步骤 14.3 — 日志与审计补全

- 检查所有关键操作是否已有审计日志；
- 登录失败、权限拒绝等安全事件记录日志；
- 确保敏感信息（密码、密钥）不出现在日志和 API 响应中；

---

## 阶段十五：构建、部署与验收

**目标**：完成生产级构建流程、Docker 部署支持与端到端验证。

### 步骤 15.1 — 构建流程完善

- 完善 Makefile：`make dev`（前后端联调）、`make build`（生产构建）、`make clean`；
- 编写 Dockerfile（多阶段构建）：前端编译 → 后端编译 → 最终镜像（Alpine + rsync + openssh-client）；
- 编写 `docker-compose.yml`（含卷挂载 DATA_DIR 配置）；

### 步骤 15.2 — 端到端功能验证

- 验证完整流程：注册 → 登录 → 创建远程配置 → 创建备份目标 → 创建实例 → 配置策略 → 手动触发备份 → 等待完成 → 查看备份记录 → 执行恢复 → 查看审计日志；
- 验证调度器自动触发备份；
- 验证保留策略自动清理；
- 验证健康检查与风险事件生成；
- 验证容灾率计算与仪表盘展示；
- 验证通知邮件发送；
- 验证 viewer 权限边界；
- 验证移动端响应式布局；
- 验证浅色/深色主题切换；

### 步骤 15.3 — 文档与交付

- 编写 README.md：项目介绍、快速启动、配置说明、Docker 部署；
- 编写 `.env.example` 示例配置文件；
- 确保项目可通过 `make build` 产出单体二进制，或 `docker build` 产出可用镜像；

---

## 阶段依赖关系

```
阶段一（脚手架）
  └── 阶段二（认证与用户）
       └── 阶段三（布局与组件库）
            ├── 阶段四（远程配置与备份目标）
            │    └── 阶段五（实例与策略）
            │         └── 阶段六（备份引擎）
            │              ├── 阶段七（恢复引擎）
            │              ├── 阶段八（审计日志）
            │              └── 阶段九（容灾率与风险）
            │                   └── 阶段十（通知）
            │                        └── 阶段十一（仪表盘）
            ├── 阶段十二（系统配置页面）
            └── 阶段十三（任务进度展示）
                 └── 阶段十四（安全加固）
                      └── 阶段十五（构建与验收）
```

---

## 技术风险与注意事项

| 风险项 | 影响 | 应对策略 |
|--------|------|----------|
| rsync `--info=progress2` 在不同版本行为差异 | 进度解析不准确 | 兼容性测试，降级为不显示进度 |
| 双远程中继模式需要本地磁盘空间 | 磁盘耗尽 | 前端创建策略时提示，后台监控中继目录大小 |
| SQLite WAL 模式下高并发写入性能 | 任务队列堵塞 | Worker Pool 控制并发数，关键写操作加 mutex |
| SSH 私钥适配多种格式（RSA/ED25519/ECDSA） | 连接失败 | 测试连接接口前置校验，支持主流格式 |
| 冷备份加密密钥丢失导致数据不可恢复 | 数据丢失 | 前端强提示"密钥丢失无法恢复"，恢复时要求输入密钥 |
| 前端组件库自研维护成本 | 开发效率 | 按需实现，优先覆盖实际使用的组件 |
