<div align="center">

<p>
	<img src="web/public/brand/logo-final.svg" alt="Rsync Backup Service logo" width="96" />
</p>

<h1>Rsync Backup Service (RBS)</h1>

<p><strong>让自托管备份，不止能跑，还能看、能管、能恢复。</strong></p>

<p>🌀 滚动增量备份 · 🧊 冷全量备份 · ♻️ 恢复与下载 · 🚨 风险预警 · 🔐 权限审计 · ☁️ 云盘支持</p>

<p>
	<img src="https://img.shields.io/badge/self--hosted-ready-1f6feb?style=for-the-badge" alt="self-hosted ready" />
	<img src="https://img.shields.io/badge/rsync-powered-2ea043?style=for-the-badge" alt="rsync powered" />
	<img src="https://img.shields.io/badge/web-dashboard-f59e0b?style=for-the-badge" alt="web dashboard" />
	<img src="https://img.shields.io/badge/restore-built--in-e85aad?style=for-the-badge" alt="restore built in" />
	<img src="https://img.shields.io/badge/cloud-support-00bfff?style=for-the-badge" alt="cloud support" />
</p>

</div>

RBS 是一套面向真实运维场景的备份管理系统。它把备份执行、恢复流程、风险感知、权限控制和审计追踪整合进一个 Web 控制台，让备份从“有脚本能跑”升级为“有系统能管”。

## ✨ 核心亮点

- 🌀 **备份模式：** 支持滚动增量备份、冷全量备份、本地与 SSH 异地备份，以及面向不同业务场景的多种备份策略。
- ⏱ **任务调度：** 支持手动触发、Cron / Interval 定时执行、自动保留清理，减少人工值守成本。
- ♻️ **恢复能力：** 支持原地恢复、自定义路径恢复、加密冷备恢复和下载，让备份真正具备可用性。
- 🚨 **风险感知：** 支持健康检查、风险事件检测、仪表盘概览和 SMTP 通知，帮助团队更早发现异常。
- 🔐 **协作控制：** 支持用户管理、实例级权限、远程配置和审计日志，适合长期维护和多人协作。
- ☁️ **云盘支持：** 通过 [OpenList](https://github.com/OpenListTeam/OpenList) 支持主流云存储服务，方便异地备份和数据迁移。
- 🚀 **部署体验：** 支持单服务部署、内嵌 Web 控制台和默认 SQLite，自托管环境上手直接。

## 🎯 适合这些场景

- 想把业务目录、配置目录、文件型数据纳入稳定备份体系
- 想同时拥有“快速回滚快照”和“长期离线副本”
- 想统一管理本地与 SSH 远程备份源、备份目标
- 想把恢复操作、权限控制、审计记录纳入标准流程
- 想对失败、过久未备份、容量异常等风险持续感知

## 🛠️ 典型使用流程

1. 配置远程连接和备份目标
2. 创建实例并指定数据源
3. 配置滚动或冷备份策略
4. 按计划自动执行，或随时手动触发
5. 在仪表盘和风险中心持续观察状态
6. 需要时直接恢复或生成下载链接

## 📌 当前版本重点支持

- 自托管单机部署
- 本地与 SSH 场景下的备份源和备份目标
- 单体服务内嵌 Web 控制台

当前版本重点聚焦本地与 SSH 生产场景，云存储接口暂未作为主打能力对外宣传。

## 🚀 快速体验

### Docker 部署

创建 `docker-compose.yml` 文件，内容如下：

```yaml
services:
  rbs:
    image: ghcr.io/lunadeertech/rsyncbackupservice:latest
    ports:
      - "8080:8080"
    volumes:
      - rbs-data:/data
    environment:
      RBS_JWT_SECRET: change_this_to_a_secure_random_string
      RBS_DATA_DIR: /data
      RBS_PORT: "8080"
      RBS_WORKER_POOL_SIZE: "3"
      RBS_LOG_LEVEL: info
    restart: unless-stopped

volumes:
  rbs-data:
```

修改 `RBS_JWT_SECRET` 为一个安全的随机字符串，并根据需要调整其他环境变量。然后运行：

```bash
docker compose up -d
```

在没有配置 SMTP 的情况下，注册密码会通过后端日志输出，首次登录后建议立即修改密码。

### 预构建二进制包

首先创建 `.env` 文件：

```bash
cat >> .env <<EOF
RBS_JWT_SECRET=change_this_to_a_secure_random_string
RBS_DATA_DIR=./data
RBS_PORT=8080
RBS_WORKER_POOL_SIZE=3
RBS_LOG_LEVEL=info
EOF
```

前往 [Releases](https://github.com/LunaDeerTech/RsyncBackupService/releases) 下载适合你系统的预构建二进制包，解压后直接运行：

```bash
./rbs
```

### 本地构建运行

```bash
cp .env.example .env
make build
./bin/rbs
```

启动前请至少设置好 `.env` 中的 `RBS_JWT_SECRET`。

默认访问地址：`http://127.0.0.1:8080`



## ⚙️ 开发常用命令

```bash
make build
make test
make docker
make clean
```

## 🧱 运行环境

- Go 1.22+
- Node.js 20+
- npm 10+
- rsync 3.1+
- openssh-client
- Docker 24+ 与 Docker Compose v2（可选）

## 📦 部署形态

RBS 采用 Go 单服务加内嵌 Vue SPA 的交付方式，默认使用 SQLite 持久化数据。部署轻、依赖少、维护直接，是它面向自托管团队的重要优势。

## 📄 许可证

本项目采用仓库根目录中的 LICENSE。