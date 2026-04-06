# 01-03 前端工程初始化

## 前序任务简报

后端已完成：Go 工程骨架已就绪，HTTP Server 在 `RBS_PORT`（默认 8080）监听，`/api/v1/health` 可返回 JSON 响应，具备统一的请求/响应格式、分页解析、CORS 和日志中间件。SQLite 数据库已建表。

## 当前任务目标

在 `web/` 目录下创建 Vue 3 + TypeScript + Vite 前端工程，配置 Tailwind CSS 和样式 token 体系，建立前端分层目录结构，实现 API 请求封装与基础主题切换。

## 实现指导

### 1. Vite + Vue 3 + TypeScript 初始化

- 在项目根级 `web/` 目录下创建 Vite 项目，模板选择 `vue-ts`
- 安装核心依赖：`vue-router`、`pinia`、`axios`

### 2. Tailwind CSS 配置

- 安装 Tailwind CSS（v3 或 v4，推荐 v4）及其 Vite 插件
- 配置扫描路径：`./src/**/*.{vue,ts,tsx}`
- 在 Tailwind 配置中定义断点与设计文档一致：`sm: 640px`、`md: 768px`、`lg: 1024px`、`xl: 1280px`
- 配置支持深色模式：`class` 策略（通过根元素 `data-theme` 属性切换）

### 3. CSS Token 变量体系

参照 `docs/component-style-design.md` 的色彩体系，创建全局样式文件定义 CSS 变量。至少包含：

- 品牌色：`--primary-300/500/600`
- 语义色：`--success-500`、`--warning-500`、`--error-500`
- 表面色：`--surface-base`、`--surface-raised`、`--surface-overlay`
- 文本色：`--text-primary`、`--text-secondary`、`--text-muted`
- 边框色：`--border-default`、`--border-subtle`
- 浅色主题与深色主题各一套变量集
- 通过 `[data-theme="dark"]` 选择器切换深色主题变量

### 4. 前端目录结构

```
web/src/
  api/           # API 请求封装（按模块）
  assets/        # 静态资源
  components/    # 通用组件
  composables/   # 组合式函数
  layouts/       # 布局组件
  pages/         # 页面组件
  router/        # 路由配置
  stores/        # Pinia 状态管理
  styles/        # 全局样式、token 变量
  types/         # TypeScript 类型定义
  utils/         # 工具函数
  App.vue
  main.ts
```

### 5. API 请求封装

```typescript
// api/client.ts
// 创建 Axios 实例，配置 baseURL 为 '/api/v1'
// 请求拦截器：自动注入 Authorization: Bearer <token>
// 响应拦截器：
//   - 解包 { code, message, data } 格式，code !== 0 时抛出业务错误
//   - 401 时清除 token 并跳转登录页
//   - 网络错误统一处理

// api/types.ts
interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

interface PaginatedData<T> {
  items: T[]
  total: number
  page: number
  page_size: number
  total_pages: number
}
```

### 6. 主题切换

```typescript
// stores/theme.ts
// useThemeStore:
//   - theme: 'light' | 'dark'
//   - toggleTheme(): 切换主题，更新 document.documentElement 的 data-theme 属性
//   - 初始化时从 localStorage 读取偏好，默认跟随系统 prefers-color-scheme
```

### 7. 路由基础配置

- 创建空路由表，预留 `/login`、`/register`、`/dashboard`、`/instances` 等路径作为注释占位
- 暂时配置一个根路由 `/` 显示"RBS - Coming Soon"占位页面

## 验收目标

1. `cd web && npm install && npm run dev` 可启动开发服务器
2. 浏览器访问 Vite 开发地址可见占位页面
3. 主题切换功能可用：点击切换后 CSS 变量正确变化、页面背景/文字颜色随之切换
4. API 请求封装可用：手动调用 health 接口（配置 Vite proxy 代理到后端 8080）能获取响应
5. `npm run build` 产物输出到 `web/dist/`
