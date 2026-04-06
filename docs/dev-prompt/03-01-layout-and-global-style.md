# 03-01 布局组件与全局样式系统

## 前序任务简报

阶段二已完成：后端认证系统（注册/登录/刷新/用户管理）全部就绪，JWT + 角色 + 实例权限中间件已挂载。前端登录/注册页面可用，`AuthLayout` 布局、`useAuthStore`（Pinia）和路由守卫已实现，Token 自动刷新机制就绪。

## 当前任务目标

实现应用主布局框架（`AppLayout`）和完整的全局样式 token 体系，为后续所有业务页面提供统一的视觉基础和导航框架。

## 实现指导

### 1. AppLayout 布局组件

**桌面端（≥1024px）**：
- 左侧固定导航栏（宽度约 240px）
  - 顶部：应用 Logo / 名称「RBS」
  - 导航菜单项（带图标 + 文字）：
    - 仪表盘（admin 可见）→ `/dashboard`
    - 实例列表 → `/instances`
    - 备份目标（admin 可见）→ `/targets`
    - 远程配置（admin 可见）→ `/system/remotes`
    - 用户管理（admin 可见）→ `/system/users`
    - SMTP 配置（admin 可见）→ `/system/smtp`
    - 风险事件（admin 可见）→ `/system/risks`
  - 底部：个人中心入口 + 登出按钮
  - 当前路由高亮对应菜单项
- 右侧内容区：
  - 顶部栏：页面标题区域 + 主题切换按钮
  - 主内容区域：`<router-view />`
  - 内容区域可滚动，导航栏固定

**移动端（<1024px）**：
- 导航栏收为抽屉式菜单
- 顶部固定栏：菜单按钮（汉堡图标）+ 应用名称 + 主题切换
- 点击菜单按钮打开左侧抽屉，展示与桌面端相同的导航菜单
- 点击遮罩或菜单项后关闭抽屉
- 内容区域全宽

### 2. 导航权限控制

- 根据 `useAuthStore` 中的用户角色动态显示/隐藏菜单项
- viewer 仅可见「实例列表」和「个人中心」
- admin 可见所有菜单项

### 3. 全局 CSS Token 系统

在 `web/src/styles/` 下创建完整的 CSS 变量定义，参照 `docs/component-style-design.md`：

**浅色主题变量**（`:root` 或 `[data-theme="light"]`）：

```css
/* 品牌色 */
--primary-300: #9BEAFF;
--primary-500: #63D9FF;
--primary-600: #2FC7F0;
--accent-mint-400: #7EF2D4;

/* 语义色 */
--success-500: #5DCC96;
--warning-500: #F5BE58;
--error-500: #F06060;
--info-500: #63D9FF;

/* 表面色 */
--surface-base: #F8FAFC;
--surface-raised: #FFFFFF;
--surface-overlay: #FFFFFF;
--surface-sunken: #F1F5F9;

/* 文本色 */
--text-primary: #0F172A;
--text-secondary: #475569;
--text-muted: #94A3B8;

/* 边框色 */
--border-default: #E2E8F0;
--border-subtle: #F1F5F9;
--border-focus: var(--primary-500);

/* 阴影 */
--shadow-sm: 0 1px 2px rgba(0,0,0,0.05);
--shadow-md: 0 4px 6px rgba(0,0,0,0.07);
--shadow-lg: 0 10px 15px rgba(0,0,0,0.1);
```

**深色主题变量**（`[data-theme="dark"]`）：

```css
--surface-base: #0B1120;
--surface-raised: #131C2E;
--surface-overlay: #1A2540;
--surface-sunken: #070D18;

--text-primary: #E8EDF5;
--text-secondary: #94A3B8;
--text-muted: #64748B;

--border-default: #1E293B;
--border-subtle: #162032;
/* ... 其他深色主题对应值 */
```

**通用 token**：

```css
/* 圆角 */
--radius-sm: 6px;
--radius-md: 8px;
--radius-lg: 12px;
--radius-xl: 14px;

/* 间距 */
--spacing-1: 4px;
--spacing-2: 8px;
--spacing-3: 12px;
--spacing-4: 16px;
--spacing-6: 24px;
--spacing-8: 32px;

/* 过渡 */
--transition-fast: 150ms ease;
--transition-normal: 250ms ease;
```

### 4. 图标方案

- 选择并安装图标库：推荐 Lucide Icons（`lucide-vue-next`）
- 封装统一的 Icon 使用方式，确保图标大小和颜色跟随 token

### 5. 全局排版

- 定义基础字号体系：`text-xs`（12px）、`text-sm`（14px）、`text-base`（16px）、`text-lg`（18px）、`text-xl`（20px）
- 正文使用 `text-sm` 或 `text-base`
- 标题使用 `text-lg` / `text-xl` + `font-semibold`

## 验收目标

1. admin 登录后看到完整的侧边导航栏，所有菜单项可见
2. viewer 登录后仅可见「实例列表」和「个人中心」
3. 点击菜单项正确跳转路由，当前项高亮
4. 移动端（<1024px）导航栏自动收为汉堡菜单 + 抽屉
5. 主题切换按钮点击后，整个界面（导航栏 + 内容区）颜色切换正确
6. CSS 变量在浅色/深色主题下各有一套完整定义
