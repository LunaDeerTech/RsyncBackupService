# Rsync Backup Service (RBS)

基于 rsync 的自托管备份管理系统，提供滚动增量备份、冷全量备份、恢复、风险检测、通知和权限管理等完整闭环能力，并内置响应式 Web 管理界面。

## 功能特性

- 滚动增量备份，基于 rsync `--link-dest` 管理快照
- 冷全量备份，支持压缩、加密和下载
- 计划调度，支持 Interval 与 Cron 策略
- 备份恢复，支持滚动备份和加密冷备份恢复
- 容灾率评估与风险事件检测
- 邮件通知与测试发送
- 审计日志与操作留痕
- 用户权限管理与实例授权
- 响应式 Web 界面与浅色/深色主题

## 系统要求

- Go 1.22+
- Node.js 20+
- npm 10+
- rsync 3.1+
- openssh-client，用于 SSH 远程源和目标场景
- Docker 24+ 与 Docker Compose v2，可选

## 快速启动

### 二进制运行

1. 复制环境变量模板并修改密钥：

```bash
cp .env.example .env
```

2. 构建前后端单体二进制：

```bash
make build
```

3. 启动服务：

```bash
./bin/rbs
```

默认监听 `http://127.0.0.1:8080`。首次启动会自动创建数据目录、SQLite 数据库和必要子目录。

### 开发模式

前后端分离开发：

```bash
make dev-backend
make dev-frontend
```

后端在开发模式下设置 `RBS_DEV_MODE=true`，此时不会加载内嵌前端静态资源，便于 Vite 代理 `/api` 请求。

### Docker 运行

1. 准备环境变量：

```bash
cp .env.example .env
```

2. 修改 `.env` 中的 `RBS_JWT_SECRET`。

3. 构建并启动：

```bash
docker compose up -d --build
```

服务默认暴露在 `8080` 端口，持久化数据写入 Docker volume `rbs-data`，容器内路径为 `/data`。
如果所在环境访问 `proxy.golang.org` 不稳定，可在 `.env` 中调整 `RBS_GOPROXY`，例如改成 `direct` 或内网镜像。

## 常用命令

```bash
make build
make test
make docker
make clean
```

## 配置说明

### 核心配置

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| RBS_JWT_SECRET | 是 | - | JWT 签名密钥，同时用于派生应用内部加密密钥 |
| RBS_DATA_DIR | 否 | `./data` | 数据存储目录，包含数据库、密钥和中间文件 |
| RBS_PORT | 否 | `8080` | HTTP 服务监听端口 |
| RBS_WORKER_POOL_SIZE | 否 | `3` | 后台任务工作池大小 |
| RBS_LOG_LEVEL | 否 | `info` | 日志级别，支持 `debug`、`info`、`warn`、`error` |
| RBS_DEV_MODE | 否 | `false` | 开发模式，启用后禁用内嵌前端资源 |
| RBS_GOPROXY | 否 | `https://proxy.golang.org,direct` | Docker 构建后端镜像时使用的 Go 模块代理 |

### SMTP 配置

以下变量为可选，用于首个用户密码投递和风险通知邮件发送：

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| RBS_SMTP_HOST | 否 | - | SMTP 主机 |
| RBS_SMTP_PORT | 否 | - | SMTP 端口 |
| RBS_SMTP_USERNAME | 否 | - | SMTP 用户名 |
| RBS_SMTP_PASSWORD | 否 | - | SMTP 密码 |
| RBS_SMTP_FROM | 否 | - | 发件人地址 |

未配置 SMTP 时，系统会把首次生成的登录密码记录到日志中，便于手工交付。

## 数据与部署说明

- 默认数据目录会创建 `keys`、`relay`、`temp`、`logs` 等子目录。
- SQLite 数据库位于数据目录内，适合单机自托管部署。
- 生产容器镜像基于 Alpine，运行时包含 `rsync`、`openssh-client`、`ca-certificates` 和 `tzdata`。

## 验证与验收

基础构建与镜像验收建议执行以下命令：

```bash
make test
make build
docker build -t rbs:latest .
docker compose config
docker compose up -d
```

功能验收应覆盖以下场景：

- 首次启动与首个管理员注册
- 远程配置、目标健康检查、实例与策略创建
- 手动与调度触发的滚动备份、冷备份与保留清理
- 滚动恢复、加密冷备份恢复与冷备份下载
- 风险事件生成/自动解决、容灾率与仪表盘展示
- SMTP 测试发送与风险通知
- viewer 权限隔离
- 响应式布局与主题切换

## 许可证

本项目采用仓库根目录中的 LICENSE。