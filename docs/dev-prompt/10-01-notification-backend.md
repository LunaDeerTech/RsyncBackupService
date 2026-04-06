# 10-01 通知模块后端

## 前序任务简报

阶段九已完成：容灾率计算引擎（四维评分 + 缓存 + 触发）和风险事件检测系统（7 种类型 + 等级升级 + 自动解决）均已就位。风险事件可在备份失败、目标不可达等场景自动生成。前端容灾率展示已在实例详情和列表中接入。

## 当前任务目标

实现 SMTP 邮件配置、邮件发送模块和通知订阅管理，当风险事件产生/升级时向订阅用户发送邮件通知。

## 实现指导

### 1. SMTP 配置存取

```go
// service/smtp.go
type SMTPConfig struct {
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Username string `json:"username"`
    Password string `json:"password"` // API 获取时脱敏为 "***"
    From     string `json:"from"`
}

func (s *Service) GetSMTPConfig() (*SMTPConfig, error)
func (s *Service) UpdateSMTPConfig(cfg *SMTPConfig) error
func (s *Service) TestSMTP(cfg *SMTPConfig, to string) error
```

- SMTP 密码在 `system_configs` 表中 AES 加密存储
- AES 密钥使用 `RBS_JWT_SECRET` 派生（SHA-256 截取前 32 字节）
- API 返回时密码字段脱敏

```go
// internal/crypto/aes.go（补充）
func AESEncrypt(plaintext string, key []byte) (string, error)  // 返回 base64 编码
func AESDecrypt(ciphertext string, key []byte) (string, error)
```

### 2. SMTP API

**GET `/api/v1/system/smtp`**（admin）：返回 SMTP 配置（密码脱敏）
**PUT `/api/v1/system/smtp`**（admin）：更新 SMTP 配置
**POST `/api/v1/system/smtp/test`**（admin）：
- 请求体：`{ "to": "test@example.com" }`
- 使用当前 SMTP 配置发送测试邮件
- 返回成功/失败信息

### 3. 邮件发送模块（`internal/notify`）

```go
type EmailSender struct {
    db      *store.DB
    aesKey  []byte
    mu      sync.Mutex
    queue   chan *EmailJob
}

type EmailJob struct {
    To      string
    Subject string
    Body    string
    Retries int
}

func NewEmailSender(db *store.DB, aesKey []byte) *EmailSender
func (s *EmailSender) Start(ctx context.Context)    // 启动发送 goroutine
func (s *EmailSender) Send(to, subject, body string) // 异步投入队列
```

发送实现：
- 使用 `net/smtp` 标准库发送邮件
- 异步发送，不阻塞业务流程
- 失败重试 3 次，间隔指数退避（5s、25s、125s）
- SMTP 未配置时降级为 `slog.Warn` 输出邮件内容到日志

### 4. 通知订阅

**订阅模型**：

```go
type NotificationSubscription struct {
    ID         int64 `json:"id"`
    UserID     int64 `json:"user_id"`
    InstanceID int64 `json:"instance_id"`
    Enabled    bool  `json:"enabled"`
}
```

**Store 层**：

```go
func (db *DB) ListSubscriptionsByUser(userID int64) ([]model.NotificationSubscription, error)
func (db *DB) UpdateSubscriptions(userID int64, subs []model.NotificationSubscription) error
func (db *DB) ListSubscribersByInstance(instanceID int64) ([]model.User, error) // 获取订阅了某实例的用户邮箱
```

**API 接口**：

**GET `/api/v1/users/me/subscriptions`**（已认证）：
- 返回当前用户的通知订阅列表（含实例名称）

**PUT `/api/v1/users/me/subscriptions`**（已认证）：
- 请求体：`{ "subscriptions": [{ "instance_id": 1, "enabled": true }, ...] }`
- 全量覆盖当前用户的订阅配置

### 5. 风险事件通知集成

在 RiskDetector 中，当风险事件**产生**或**升级为 critical** 时：

```go
func (rd *RiskDetector) notify(ctx context.Context, event *model.RiskEvent) {
    // 1. 查询订阅了该实例的用户列表
    // 2. 对每个用户调用 EmailSender.Send
    // 3. 邮件标题示例："[RBS 预警] 实例 my-app 出现备份失败风险"
    // 4. 邮件内容：风险类型、等级、描述、时间
}
```

### 6. 注册开关 API

**GET `/api/v1/system/registration`**（公开）：
- 返回 `{ "enabled": true/false }`

**PUT `/api/v1/system/registration`**（admin）：
- 请求体：`{ "enabled": true/false }`

## 验收目标

1. SMTP 配置可通过 API 读写，密码加密存储且 API 返回脱敏
2. 测试邮件可成功发送（需配置有效 SMTP）
3. SMTP 未配置时邮件内容降级为日志输出
4. 用户可查看和更新自己的通知订阅
5. 风险事件产生时自动向订阅用户发送邮件
6. 邮件发送失败自动重试 3 次
7. 注册开关 API 可正常控制注册页面行为
8. 为 AES 加密/解密编写单元测试
