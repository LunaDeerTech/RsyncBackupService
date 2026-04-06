# 15-01 构建、部署与验收

## 前序任务简报

系统全部功能已开发完成且经过安全加固：前后端完整闭环（13 个阶段 + 安全加固），覆盖备份/恢复引擎、任务调度、容灾率/风险事件、通知、仪表盘、管理页面、实时进度、CSRF/输入验证/脱敏。

## 当前任务目标

完善构建流程、Docker 部署支持，执行端到端功能验证，编写项目文档。

## 实现指导

### 1. 构建流程完善

**Makefile 更新**：

```makefile
.PHONY: build dev clean test docker

# 生产构建
build:
	cd web && npm ci && npm run build
	CGO_ENABLED=0 go build -o bin/rbs cmd/server/main.go

# 开发：分别启动前后端
dev-backend:
	go run cmd/server/main.go

dev-frontend:
	cd web && npm run dev

# 测试
test:
	go test ./...

# Docker 构建
docker:
	docker build -t rbs:latest .

# 清理
clean:
	rm -rf bin/ web/dist/
```

### 2. Dockerfile（多阶段构建）

```dockerfile
# 阶段一：前端构建
FROM node:20-alpine AS frontend
WORKDIR /app/web
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# 阶段二：后端编译
FROM golang:1.22-alpine AS backend
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/web/dist web/dist
RUN CGO_ENABLED=0 go build -o /rbs cmd/server/main.go

# 阶段三：最终镜像
FROM alpine:3.19
RUN apk add --no-cache rsync openssh-client ca-certificates tzdata
COPY --from=backend /rbs /usr/local/bin/rbs
EXPOSE 8080
VOLUME ["/data"]
ENV RBS_DATA_DIR=/data
CMD ["rbs"]
```

### 3. docker-compose.yml

```yaml
version: '3.8'
services:
  rbs:
    build: .
    # 或使用预构建镜像：image: rbs:latest
    ports:
      - "8080:8080"
    volumes:
      - rbs-data:/data
    environment:
      - RBS_JWT_SECRET=${RBS_JWT_SECRET}
      - RBS_DATA_DIR=/data
      - RBS_PORT=8080
      - RBS_WORKER_POOL_SIZE=3
      - RBS_LOG_LEVEL=info
    restart: unless-stopped

volumes:
  rbs-data:
```

### 4. 端到端功能验证清单

按以下顺序执行完整流程测试：

**基础流程**：
- [ ] 首次启动服务，数据库自动创建
- [ ] 注册第一个用户，自动成为 admin
- [ ] 使用日志中的密码登录
- [ ] 关闭注册开关，确认注册页显示关闭

**配置管理**：
- [ ] 创建 SSH 远程配置，测试连接成功
- [ ] 创建本地备份目标（滚动类型），手动健康检查通过
- [ ] 创建 SSH 备份目标（冷类型），手动健康检查通过

**实例与策略**：
- [ ] 创建本地源实例
- [ ] 为实例创建滚动备份策略（interval 60秒）
- [ ] 为实例创建冷备份策略（压缩+加密）
- [ ] 手动触发滚动备份，观察任务进度，等待完成
- [ ] 手动触发冷备份，等待完成
- [ ] 查看备份列表，确认记录正确

**调度与清理**：
- [ ] 等待调度器自动触发备份（interval 策略）
- [ ] 确认保留策略自动清理生效（设置保留 1 条后多次备份）

**恢复**：
- [ ] 执行滚动备份恢复到指定路径
- [ ] 执行加密冷备份恢复（需输入密钥）
- [ ] 冷备份下载功能正常

**容灾与风险**：
- [ ] 查看实例容灾率分数和等级
- [ ] 制造备份失败场景，确认风险事件自动生成
- [ ] 备份成功后确认风险事件自动解决
- [ ] 仪表盘正确展示系统状态

**通知**：
- [ ] 配置 SMTP 并发送测试邮件
- [ ] 订阅实例通知后，风险事件触发邮件

**权限**：
- [ ] 创建 viewer 用户
- [ ] viewer 登录后只能看到授权实例
- [ ] viewer 无法访问 admin 菜单

**响应式与主题**：
- [ ] 浅色/深色主题切换，所有页面样式正确
- [ ] 移动端（<1024px）导航抽屉和页面布局正常
- [ ] 平板端布局合理

### 5. 项目文档

**README.md**：

```markdown
# Rsync Backup Service (RBS)

基于 rsync 的自托管备份管理系统...

## 功能特性
- 滚动增量备份（基于 rsync --link-dest）
- 冷全量备份（支持压缩/加密/分卷）
- 计划调度（Interval / Cron）
- 备份恢复
- 容灾率评估
- 风险预警与邮件通知
- 审计日志
- 用户权限管理
- 响应式 Web 界面

## 快速启动

### 二进制运行
export RBS_JWT_SECRET="your-secret-key"
./rbs

### Docker 运行
docker-compose up -d

## 配置
| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| RBS_JWT_SECRET | 是 | - | JWT 签名密钥 |
| RBS_DATA_DIR | 否 | ./data | 数据存储目录 |
| RBS_PORT | 否 | 8080 | 监听端口 |

## 系统要求
- rsync >= 3.1
- openssh-client（SSH 备份场景）
```

**.env.example**：

```env
RBS_JWT_SECRET=change-me-to-a-random-string
RBS_DATA_DIR=./data
RBS_PORT=8080
RBS_WORKER_POOL_SIZE=3
RBS_LOG_LEVEL=info
```

## 验收目标

1. `make build` 成功产出单体二进制
2. `docker build` 成功构建镜像
3. `docker-compose up` 可启动服务
4. 上述端到端验证清单全部通过
5. README.md 内容完整准确
6. .env.example 文件存在且包含所有必要配置
