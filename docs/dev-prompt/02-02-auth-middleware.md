# 02-02 认证与权限中间件

## 前序任务简报

用户数据层与认证接口已完成：`users` 表 CRUD 可用，注册（首个用户为 admin）、登录（JWT 生成 + 频率限制）、刷新令牌接口已就绪。JWT 工具可生成和解析 HS256 令牌。

## 当前任务目标

实现 JWT 认证中间件、角色检查中间件和实例权限检查中间件，为后续所有业务接口提供统一的权限控制。

## 实现指导

### 1. JWT 认证中间件

```go
// internal/middleware/auth.go
func Auth(jwtSecret string) func(http.Handler) http.Handler
```

- 从请求 Header 中提取 `Authorization: Bearer <token>`
- 解析并校验 JWT（签名、过期时间）
- 解析成功后将用户信息（UserID, Email, Role）注入请求 Context
- 认证失败返回 `{"code": 40101, "message": "unauthorized"}`

### 2. Context 工具

```go
// internal/middleware/context.go
type contextKey string

func SetUser(ctx context.Context, claims *crypto.Claims) context.Context
func GetUser(ctx context.Context) *crypto.Claims  // 获取当前认证用户，未认证返回 nil
func MustGetUser(ctx context.Context) *crypto.Claims // 获取当前认证用户，未认证 panic
```

### 3. 角色检查中间件

```go
func RequireAuth(next http.Handler) http.Handler     // 仅要求已认证
func RequireAdmin(next http.Handler) http.Handler     // 要求已认证 + admin 角色
```

- `RequireAuth`：检查 Context 中是否有用户信息，无则返回 401
- `RequireAdmin`：检查用户角色是否为 `admin`，否则返回 `{"code": 40301, "message": "forbidden"}`

### 4. 实例权限检查中间件

```go
func RequireInstanceAccess(db *store.DB) func(http.Handler) http.Handler
```

- 从 URL 路径参数提取实例 ID（`:id`）
- admin 角色直接放行
- viewer 角色查询 `instance_permissions` 表检查是否有权限
- 无权限返回 403

### 5. 中间件挂载方式

在路由注册时按需组合中间件链：

```go
// 公开路由（无需认证）
mux.Handle("POST /api/v1/auth/login", handler.Login)

// 需要认证的路由
mux.Handle("GET /api/v1/users/me", Auth(secret)(RequireAuth(handler.GetMe)))

// 需要 admin 的路由
mux.Handle("GET /api/v1/users", Auth(secret)(RequireAdmin(handler.ListUsers)))

// 需要实例权限的路由
mux.Handle("GET /api/v1/instances/{id}", Auth(secret)(RequireAuth(RequireInstanceAccess(db)(handler.GetInstance))))
```

### 6. 实例权限 Store 方法

```go
func (db *DB) GetInstancePermission(userID, instanceID int64) (*model.InstancePermission, error)
func (db *DB) SetInstancePermissions(instanceID int64, permissions []model.InstancePermission) error
func (db *DB) ListInstancePermissionsByUser(userID int64) ([]model.InstancePermission, error)
```

## 验收目标

1. 无 Token 请求受保护接口返回 401
2. 过期 Token 请求返回 401
3. 有效 Token + admin 角色可访问所有接口
4. 有效 Token + viewer 角色访问 admin 接口返回 403
5. viewer 无实例权限时，访问实例相关接口返回 403
6. viewer 被授权实例后可正常访问对应实例接口
7. 为中间件编写单元测试（mock HTTP 请求测试各种场景）
