# 01-04 前后端嵌入与构建流程

## 前序任务简报

后端：Go HTTP Server 已运行，具备 `/api/v1/health` 路由、统一 JSON 响应、CORS 和日志中间件。前端：Vue 3 + Vite + TypeScript + Tailwind CSS 工程已在 `web/` 下就绪，API 封装层和主题切换已实现，`npm run build` 可产出 `web/dist/`。

## 当前任务目标

将前端构建产物通过 Go `embed.FS` 嵌入后端二进制，配置 SPA fallback 路由，编写统一构建脚本，使项目可编译为单体可执行文件。

## 实现指导

### 1. embed.FS 嵌入

在 `web/` 目录（或 `cmd/server/`）中创建嵌入声明：

```go
//go:embed all:dist
var frontendFS embed.FS
```

- 在 HTTP Server 路由中注册静态文件服务
- 对 `/api` 前缀的请求走 API 路由
- 其余所有请求作为静态文件处理，文件不存在时返回 `index.html`（SPA fallback）
- 注意：嵌入目录需要在 `web/dist` 存在时才能编译成功，开发时可创建空目录占位

### 2. SPA Fallback 逻辑

```go
// 伪代码逻辑
func serveFrontend(fs embed.FS) http.Handler {
    // 1. 尝试从 embed.FS 读取请求路径对应的文件
    // 2. 如果文件存在（如 .js, .css, .png），直接返回
    // 3. 如果文件不存在，返回 index.html（Vue Router history 模式支持）
}
```

### 3. 构建脚本

创建 `Makefile`：

```makefile
.PHONY: build dev clean

# 完整构建：前端 → 后端 → 单体二进制
build:
	cd web && npm ci && npm run build
	go build -o bin/rbs cmd/server/main.go

# 开发模式提示
dev:
	@echo "前端: cd web && npm run dev"
	@echo "后端: go run cmd/server/main.go"

clean:
	rm -rf bin/ web/dist/
```

### 4. 开发模式兼容

- 当 `web/dist` 不存在或为空目录时，后端不应 panic
- 可通过环境变量（如 `RBS_DEV_MODE=true`）跳过前端静态文件服务
- 开发时前端通过 Vite dev server 独立运行，通过 Vite proxy 转发 API 请求到后端

## 验收目标

1. 执行 `make build` 成功产出 `bin/rbs` 单体二进制文件
2. 运行 `bin/rbs` 后，浏览器访问 `http://localhost:8080` 可见前端 Vue 页面
3. 访问 `http://localhost:8080/api/v1/health` 返回正确 JSON
4. 访问前端任意路由（如 `http://localhost:8080/login`）不返回 404，而是返回 `index.html`（SPA fallback）
5. 开发模式下前后端可分别独立运行和调试
