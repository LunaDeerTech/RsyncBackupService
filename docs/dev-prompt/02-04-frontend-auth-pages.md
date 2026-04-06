# 02-04 前端登录与注册页面

## 前序任务简报

后端认证系统完整就绪：注册（`POST /auth/register`）、登录（`POST /auth/login`、含频率限制）、刷新令牌（`POST /auth/refresh`）、用户信息（`GET /users/me`）接口均已实现。JWT 认证与角色/实例权限中间件已挂载。前端工程骨架已就绪（Vue 3 + Vite + TypeScript + Tailwind CSS + Pinia），API 封装层和主题切换已实现。

## 当前任务目标

实现前端认证相关页面（登录/注册）、认证状态管理（Pinia store）和路由守卫。

## 实现指导

### 1. AuthLayout 布局组件

- 用于登录/注册等无需侧边栏的认证页面
- 居中卡片式布局：垂直水平居中的表单卡片
- 顶部显示应用 Logo / 名称
- 底部可选主题切换入口
- 响应式：移动端卡片占满宽度，桌面端限宽（如 `max-w-md`）

### 2. 登录页（`/login`）

- 路由：`/login`
- 表单字段：邮箱（email input）、密码（password input）
- 提交按钮：登录
- 表单验证：邮箱格式、密码非空
- 错误提示：登录失败时在表单下方显示错误信息（如"邮箱或密码错误"、"账号已锁定"）
- 登录成功后：存储 token → 获取用户信息 → 跳转（admin → `/dashboard`，viewer → `/instances`）
- 底部链接：跳转注册页

### 3. 注册页（`/register`）

- 路由：`/register`
- 表单字段：邮箱（email input）
- 提交按钮：注册
- 提交成功后显示提示：「注册成功，请查收邮件获取密码」
- 注册页需先检查注册开关状态（`GET /api/v1/system/registration`），关闭时显示「注册已关闭」
- 底部链接：跳转登录页

### 4. 认证 Store（`stores/auth.ts`）

```typescript
// useAuthStore
interface AuthState {
  accessToken: string | null
  refreshToken: string | null
  user: User | null
}

// 核心方法
login(email: string, password: string): Promise<void>
register(email: string): Promise<void>
logout(): void
refreshAccessToken(): Promise<void>
fetchCurrentUser(): Promise<void>

// 计算属性
isAuthenticated: boolean
isAdmin: boolean
```

- Token 持久化到 `localStorage`
- 应用启动时（`App.vue` 的 `onMounted`）检查本地 Token 有效性 → 有效则获取用户信息，无效则清除

### 5. 路由守卫

```typescript
// router/index.ts
router.beforeEach((to, from, next) => {
  // 1. 公开路由（login, register）：已登录则跳转首页
  // 2. 受保护路由：未登录跳转 /login
  // 3. admin 路由：viewer 角色跳转 /instances
})
```

- 路由 meta 标记：`{ requiresAuth: true, requiresAdmin: true }`

### 6. API 模块

```typescript
// api/auth.ts
function login(email: string, password: string): Promise<LoginResponse>
function register(email: string): Promise<void>
function refreshToken(refreshToken: string): Promise<{ access_token: string }>
function getMe(): Promise<User>
function getRegistrationStatus(): Promise<{ enabled: boolean }>
```

### 7. Token 自动刷新

在 API 请求拦截器中：
- 收到 401 响应时，尝试用 refresh token 刷新 access token
- 刷新成功后重试原请求
- 刷新失败则登出并跳转登录页
- 注意防止多个并发请求同时触发刷新（加锁，排队等待）

## 验收目标

1. 访问任意受保护页面时自动跳转到登录页
2. 输入正确邮箱密码后成功登录，admin 跳转 `/dashboard`，viewer 跳转 `/instances`
3. 登录失败时表单下方显示错误提示
4. 刷新页面后登录状态保持（Token 从 localStorage 恢复）
5. 注册页可提交邮箱，成功后显示提示信息
6. 登出后清除 Token，跳转登录页
7. Token 过期后自动刷新，无感续期
