# 02-01 用户数据层与认证接口

## 前序任务简报

阶段一已完成：Go 后端工程骨架就绪，HTTP Server 运行在配置端口，SQLite 数据库（WAL 模式）已建表（含 `users` 表），统一 JSON 响应、分页解析、CORS 和日志中间件已就位。前端 Vue 3 工程已初始化并可通过 `embed.FS` 嵌入后端二进制。`make build` 可产出单体可执行文件。

## 当前任务目标

实现用户数据模型与 store 层 CRUD，实现密码哈希工具，实现完整的注册/登录/刷新令牌认证 API。

## 实现指导

### 1. 用户模型（`internal/model`）

```go
type User struct {
    ID           int64     `json:"id"`
    Email        string    `json:"email"`
    Name         string    `json:"name"`
    PasswordHash string    `json:"-"`  // JSON 序列化时忽略
    Role         string    `json:"role"` // "admin" / "viewer"
    CreatedAt    time.Time `json:"created_at"`
    UpdatedAt    time.Time `json:"updated_at"`
}
```

### 2. 密码工具（`internal/crypto`）

```go
func HashPassword(password string) (string, error)    // bcrypt, cost >= 12
func CheckPassword(password, hash string) bool
```

### 3. Store 层（`internal/store`）

```go
func (db *DB) CreateUser(user *model.User) error
func (db *DB) GetUserByEmail(email string) (*model.User, error)
func (db *DB) GetUserByID(id int64) (*model.User, error)
func (db *DB) ListUsers() ([]model.User, error)
func (db *DB) UpdateUser(user *model.User) error
func (db *DB) DeleteUser(id int64) error
func (db *DB) CountUsers() (int64, error) // 用于判断首个注册用户
```

### 4. JWT 工具（`internal/crypto` 或 `internal/util`）

```go
type Claims struct {
    UserID int64  `json:"sub"`
    Email  string `json:"email"`
    Role   string `json:"role"`
}

func GenerateAccessToken(claims Claims, secret string) (string, error)   // HS256, 24h
func GenerateRefreshToken(claims Claims, secret string) (string, error)  // HS256, 7d
func ParseToken(tokenStr string, secret string) (*Claims, error)
```

- JWT Payload 字段：`sub`（用户 ID）、`email`、`role`、`exp`、`iat`
- 建议使用 `github.com/golang-jwt/jwt/v5` 库

### 5. 认证接口（`internal/handler`）

**POST `/api/v1/auth/register`**：
- 请求体：`{ "email": "user@example.com" }`
- 逻辑：验证邮箱格式 → 查重 → 查询用户总数（0 则为 admin，否则 viewer）→ 生成随机密码 → 创建用户 → 尝试通过 SMTP 发送密码到邮箱（SMTP 未配置时在后台日志输出密码）→ 返回成功提示
- 响应：`{ "code": 0, "message": "ok", "data": { "message": "请查收邮件获取密码" } }`

**POST `/api/v1/auth/login`**：
- 请求体：`{ "email": "...", "password": "..." }`
- 逻辑：验证参数 → 登录失败频率检查（5 次/15 分钟锁定，基于 IP+邮箱）→ 查询用户 → 校验密码 → 生成 Access Token + Refresh Token
- 响应：`{ "code": 0, "data": { "access_token": "...", "refresh_token": "...", "user": {...} } }`

**POST `/api/v1/auth/refresh`**：
- 请求体：`{ "refresh_token": "..." }`
- 逻辑：验证 refresh token 有效性 → 生成新 access token
- 响应：`{ "code": 0, "data": { "access_token": "..." } }`

### 6. 登录频率限制

- 使用内存 map 记录失败次数（无需持久化）：key 为 `ip:email`，value 为 `{count, lockUntil}`
- 同一 IP+邮箱 连续 5 次失败后锁定 15 分钟
- 登录成功或锁定过期后重置计数
- 加 mutex 保护并发访问

### 7. 随机密码生成

- 长度 12 位，包含大小写字母和数字
- 使用 `crypto/rand` 保证随机性

## 验收目标

1. 注册接口：首个用户注册后 role 为 `admin`，后续为 `viewer`
2. 注册成功后，后台日志输出生成的随机密码（SMTP 未配置时）
3. 使用日志中的密码可成功登录，返回有效的 JWT token
4. Access Token 包含正确的 `sub`、`email`、`role`、`exp` 字段
5. Refresh Token 可刷新 Access Token
6. 连续 5 次错误密码后，第 6 次返回锁定提示
7. 为密码工具、JWT 工具、store 层编写单元测试
