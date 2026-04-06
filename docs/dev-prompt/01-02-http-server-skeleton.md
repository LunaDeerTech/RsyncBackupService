# 01-02 HTTP 服务骨架

## 前序任务简报

已完成后端工程初始化：Go module 已建立、分层目录结构已创建、配置加载模块（支持 .env 和环境变量）已实现、SQLite 数据库连接（WAL 模式）与全部 12 张表的建表迁移已完成。`cmd/server/main.go` 可编译运行，启动后自动创建数据目录并初始化数据库。

## 当前任务目标

启动 Go 标准库 HTTP Server，建立统一的请求/响应规范，实现基础中间件，为后续所有 API 接口提供基础框架。

## 实现指导

### 1. 路由与 HTTP Server

- 使用 Go 标准库 `net/http` 的 `http.ServeMux`（Go 1.22+ 支持方法匹配）
- 在 `internal/handler/` 中组织路由注册
- 基础路由：`GET /api/v1/health` 返回健康检查响应

```go
// internal/handler/router.go
func NewRouter(/* 依赖注入 */) http.Handler

// internal/handler/health.go
func (h *Handler) Health(w http.ResponseWriter, r *http.Request)
```

### 2. 统一 JSON 响应

实现响应工具函数：

```go
// internal/handler/response.go
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data"`
}

func JSON(w http.ResponseWriter, status int, data interface{})
func Error(w http.ResponseWriter, status int, code int, message string)
```

- 成功响应：`{"code": 0, "message": "ok", "data": {...}}`
- 错误响应：`{"code": 40001, "message": "invalid request", "data": null}`
- 常见错误码约定：`40001` 请求无效、`40101` 未认证、`40301` 权限不足、`40401` 资源不存在、`50001` 服务器内部错误

### 3. 分页参数解析

```go
// internal/handler/pagination.go
type Pagination struct {
    Page     int    // 默认 1
    PageSize int    // 默认 20，最大 100
    Sort     string // 排序字段
    Order    string // "asc" 或 "desc"，默认 "desc"
}

type PaginatedResponse struct {
    Items      interface{} `json:"items"`
    Total      int64       `json:"total"`
    Page       int         `json:"page"`
    PageSize   int         `json:"page_size"`
    TotalPages int         `json:"total_pages"`
}

func ParsePagination(r *http.Request) Pagination
```

### 4. CORS 中间件

```go
// internal/middleware/cors.go
func CORS(next http.Handler) http.Handler
```

- 开发期允许所有来源（`*`）
- 允许方法：GET, POST, PUT, DELETE, OPTIONS
- 允许头部：Content-Type, Authorization
- OPTIONS 预检请求直接返回 204

### 5. 请求日志中间件

```go
// internal/middleware/logger.go
func Logger(next http.Handler) http.Handler
```

- 记录：方法、路径、状态码、耗时
- 使用 Go 标准库 `log/slog` 结构化日志

### 6. 入口整合

在 `main.go` 中：加载配置 → 初始化数据库 → 构建 Router（挂载中间件链）→ 启动 HTTP Server 监听 `RBS_PORT`。

## 验收目标

1. 服务启动后，`GET /api/v1/health` 返回 `{"code": 0, "message": "ok", "data": {"status": "healthy"}}`
2. 请求不存在的 API 路径返回统一格式的 404 错误 JSON
3. 控制台日志输出每个请求的方法、路径、状态码和耗时
4. CORS 中间件对 OPTIONS 请求正确返回 204
5. 分页参数解析：缺省值正确、`page_size` 超过 100 时截断为 100
