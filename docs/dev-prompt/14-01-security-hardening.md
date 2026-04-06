# 14-01 安全加固与输入验证

## 前序任务简报

系统全部功能已实现：前后端完整闭环，包括备份/恢复引擎、任务调度、容灾率/风险事件、通知、仪表盘、所有管理页面和任务进度实时展示。现在需要进行全面的安全加固。

## 当前任务目标

对系统进行全面安全加固，覆盖 CSRF 防护、输入验证、路径遍历检测、分页限制、下载安全和日志脱敏。

## 实现指导

### 1. CSRF 防护

```go
// internal/middleware/csrf.go
func CSRFProtection(next http.Handler) http.Handler
```

- 对所有非 GET/HEAD/OPTIONS 的请求检查自定义请求头
- 要求请求携带 `X-Requested-With: XMLHttpRequest` 或 `Content-Type: application/json`
- 不携带时返回 `{"code": 40301, "message": "csrf validation failed"}`
- 前端 Axios 默认会发送 `Content-Type: application/json`，无需额外修改
- 适用于 API 路由，不适用于文件下载

### 2. 分页限制

- 检查所有分页接口的 `page_size` 参数，确保在 `ParsePagination` 中已限制最大值为 100
- 审查所有列表 API，确保没有接口可不带分页返回全量数据（特殊场景如远程配置列表可例外，但需确认数量有限）

### 3. 输入验证加固

在所有 handler 层统一检查：

**通用验证**（封装为工具函数）：
```go
// internal/util/validate.go
func ValidateEmail(email string) error
func ValidatePath(path string) error           // 路径遍历检测
func ValidateCron(expr string) error
func ValidateSSHHost(host string) error
func ValidatePort(port int) error
func ValidatePassword(password string) error   // 长度 >= 8
```

**路径遍历检测**（`ValidatePath`）：
- 禁止包含 `..`（禁止 `../`、`..\\` 等变体）
- 禁止以 `~` 开头的路径（避免家目录展开歧义）
- 可考虑使用 `filepath.Clean` 后检查是否仍包含上级引用

**应用位置**：
- 实例创建/编辑：`source_path` 经过 `ValidatePath`
- 备份目标创建/编辑：`storage_path` 经过 `ValidatePath`
- 恢复请求：`target_path` 经过 `ValidatePath`
- 远程配置：`host` 经过 `ValidateSSHHost`，`port` 经过 `ValidatePort`
- 用户相关：`email` 经过 `ValidateEmail`，密码经过 `ValidatePassword`
- 策略：cron 表达式经过 `ValidateCron`

### 4. 文件下载安全

确认下载接口的一次性临时令牌机制：
- 令牌生成后 5 分钟过期
- 使用后立即删除（一次性）
- 令牌不可枚举（使用 UUID 或 crypto/rand 生成的随机字符串）
- 下载接口使用流式响应，设置正确的 `Content-Disposition` header

### 5. 日志与 API 响应脱敏

**审查项**：
- 确保所有日志中不出现：密码明文、私钥内容、JWT secret、加密密钥
- 确保所有 API 响应中不出现：`password_hash`、`private_key_path`、`encryption_key_hash`
- SMTP 密码在 API 返回时脱敏为 `"***"`
- 注册流程中生成的随机密码仅在日志中出现一次（SMTP 未配置时），且日志级别为 WARN

**Go struct 检查**：
- `model.User` 的 `PasswordHash` 字段有 `json:"-"` 标签
- `model.RemoteConfig` 的 `PrivateKeyPath` 字段有 `json:"-"` 标签
- `model.Policy` 的 `EncryptionKeyHash` 字段有 `json:"-"` 标签

### 6. 审计日志完整性检查

逐一审查以下操作是否已有审计日志记录：
- 实例 CRUD ✓
- 策略 CRUD ✓
- 备份触发/完成/失败 ✓
- 恢复触发/完成/失败 ✓
- 用户 CRUD ✓
- 目标 CRUD ✓
- 远程配置 CRUD ✓
- 系统配置更新 ✓
- **补充**（如遗漏）：
  - 登录失败（安全事件）→ 可选 `auth.login_failed`
  - 权限拒绝 → 可选 `auth.forbidden`

### 7. 安全响应头

在 CORS 中间件或全局中间件中添加安全响应头：
```go
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("X-XSS-Protection", "1; mode=block")
```

## 验收目标

1. 不携带 `Content-Type: application/json` 的 POST 请求被 CSRF 拦截
2. `page_size` 超过 100 时自动截断
3. 路径中包含 `..` 时请求被拒绝并返回明确错误
4. 所有 API 响应中无敏感字段泄露
5. 下载令牌使用后不可重用
6. 安全响应头正常返回
7. 为路径验证、邮箱验证编写单元测试
8. 手动测试各种注入场景：路径遍历、SQL 注入（SQLite 参数化查询已防护）确认安全
