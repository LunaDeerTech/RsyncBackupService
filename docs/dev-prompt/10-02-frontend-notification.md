# 10-02 前端通知配置页面

## 前序任务简报

通知模块后端已完成：SMTP 配置 API（含加密存储、测试发送）、邮件发送模块（异步 + 重试）、通知订阅 CRUD API、风险事件自动通知订阅用户均已就位。注册开关 API 也已实现。

## 当前任务目标

实现 SMTP 配置管理页面和个人中心的通知订阅管理。

## 实现指导

### 1. SMTP 配置页面（`/system/smtp`）

- 路由：`/system/smtp`，admin 权限
- 页面组件：`pages/system/SmtpConfigPage.vue`

**API 模块**：
```typescript
// api/system.ts
function getSmtpConfig(): Promise<SmtpConfig>
function updateSmtpConfig(data: SmtpConfig): Promise<void>
function testSmtp(to: string): Promise<{ success: boolean; message: string }>

interface SmtpConfig {
  host: string
  port: number
  username: string
  password: string
  from: string
}
```

**页面内容**：
- 表单字段（AppFormItem + AppInput）：
  - SMTP 服务器地址（必填）
  - 端口（必填，默认 587）
  - 用户名
  - 密码（password input，显示为脱敏值，修改时输入新密码）
  - 发件人邮箱（必填）
- 保存按钮
- 测试发送区域：收件人邮箱输入 + 测试发送按钮
- 测试结果 Toast 显示成功/失败

### 2. 通知订阅管理

在个人中心页面（`/profile`，阶段十二实现完整页面）中预留通知订阅组件：

**组件：NotificationSubscriptions.vue**

```typescript
// api/notifications.ts
function getMySubscriptions(): Promise<SubscriptionItem[]>
function updateMySubscriptions(subs: { instance_id: number; enabled: boolean }[]): Promise<void>

interface SubscriptionItem {
  id: number
  instance_id: number
  instance_name: string  // 后端 JOIN 返回
  enabled: boolean
}
```

**UI 设计**：
- 以网格或列表展示用户可访问的所有实例
- 每个实例一行/卡片：实例名称 + AppSwitch（订阅开关）
- 切换开关时自动保存（debounce） 或提供统一保存按钮
- 空状态：「暂无可订阅的实例」

### 3. 注册开关管理

可放在用户管理页面或 SMTP 配置页面旁，简单实现：

```typescript
// api/system.ts（补充）
function getRegistrationStatus(): Promise<{ enabled: boolean }>
function updateRegistrationStatus(enabled: boolean): Promise<void>
```

- 一个 AppSwitch + 说明文字「允许新用户注册」

## 验收目标

1. admin 可配置 SMTP 服务器信息并保存
2. 密码字段显示脱敏值，修改时可输入新密码
3. 测试发送按钮可向指定邮箱发送测试邮件
4. 通知订阅管理显示用户可访问的所有实例
5. 切换订阅开关后保存成功
6. 深色/浅色主题下样式正确
