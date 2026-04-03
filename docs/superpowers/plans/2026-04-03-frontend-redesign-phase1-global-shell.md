# Phase 1: 全局骨架重构

> **For agentic workers:** Use superpowers:executing-plans to implement this phase. Each section is a sequential unit of work.

**目标：** 重构路由配置、导航数据结构、侧边栏和顶部栏，建立新的全局骨架。创建 `ProfileView` 和 `SystemAdminView` 占位页面，确保所有路由可导航。

**前置条件：** 无。本 phase 是整个重构的起点。

**设计规格来源：** `docs/superpowers/specs/2026-04-03-frontend-layout-redesign.md` 第 2、3 节。

---

## 1. 扩展 AppModal 宽度支持

当前 `web/src/components/ui/AppModal.vue` 的面板宽度硬编码为 `width: min(100%, 36rem)`。后续 phase 需要 Modal 支持可配置宽度。

**文件：** `web/src/components/ui/AppModal.vue`

**修改内容：**

1. 在 props 接口中增加 `width` 属性：

```typescript
interface AppModalProps {
	open: boolean
	labelledBy?: string
	describedBy?: string
	tone?: "default" | "danger"
	closeOnOverlay?: boolean
	width?: string
}
```

2. 设置默认值：

```typescript
const props = withDefaults(defineProps<AppModalProps>(), {
	labelledBy: undefined,
	describedBy: undefined,
	tone: "default",
	closeOnOverlay: true,
	width: "36rem",
})
```

3. 在面板元素上绑定内联样式：

```vue
<div
	ref="panelRef"
	class="app-modal__panel"
	role="dialog"
	aria-modal="true"
	:data-tone="tone"
	:aria-labelledby="labelledBy"
	:aria-describedby="describedBy"
	:style="{ width: `min(100%, ${width})` }"
	tabindex="-1"
>
```

4. 移除 CSS 中的 `width` 声明（改为内联控制）：

在 `.app-modal__panel` 的样式中删除 `width: min(100%, 36rem);` 这一行。

---

## 2. 重构路由配置

**文件：** `web/src/router/index.ts`

**当前状态：** 8 条 protected routes（dashboard, instances, instances/:id, storage-targets, ssh-keys, notifications, audit-logs, settings）。

**目标状态：**

- 删除 `/ssh-keys`、`/notifications`、`/audit-logs`、`/settings` 四条路由
- 新增 `/system`（requiresAdmin: true）和 `/profile` 两条路由
- 管理员默认页 `/` 保持 DashboardView
- 普通用户登录后守卫重定向至 `/instances`

**详细修改：**

1. 导入新页面组件（懒加载）：

```typescript
const SystemAdminView = () => import("../views/SystemAdminView.vue")
const ProfileView = () => import("../views/ProfileView.vue")
```

2. 替换 `protectedRoutes` 数组为：

```typescript
const protectedRoutes: RouteRecordRaw[] = [
	{
		path: "/",
		component: AppShell,
		meta: { requiresAuth: true },
		children: [
			{
				path: "",
				name: "dashboard",
				component: DashboardView,
				meta: { title: "运维仪表盘", description: "查看全局统计、运行中任务、最近备份与存储容量。", requiresAdmin: true },
			},
			{
				path: "instances",
				name: "instances",
				component: InstancesListView,
				meta: { title: "备份实例", description: "管理源路径、源主机和实例级恢复入口。" },
			},
			{
				path: "instances/:id",
				name: "instance-detail",
				component: InstanceDetailView,
				meta: { title: "实例详情", description: "查看实例配置、策略、备份历史与恢复操作。" },
			},
			{
				path: "storage-targets",
				name: "storage-targets",
				component: StorageTargetsView,
				meta: { title: "存储目标", description: "按备份类型管理目标路径，并执行连通性测试。", requiresAdmin: true },
			},
			{
				path: "system",
				name: "system",
				component: SystemAdminView,
				meta: { title: "系统管理", description: "用户管理、SSH 密钥、通知渠道与审计日志。", requiresAdmin: true },
			},
			{
				path: "profile",
				name: "profile",
				component: ProfileView,
				meta: { title: "个人信息", description: "查看会话信息和修改密码。" },
			},
		],
	},
]
```

3. 修改路由守卫中的管理员检查逻辑。当前守卫在 `requiresAdmin` 不通过时回退到 `/`。对于普通用户，`/` 本身现在也是 `requiresAdmin`，所以需要改为回退到 `/instances`：

```typescript
if (to.meta.requiresAdmin === true && auth.currentUser?.is_admin !== true) {
	return "/instances"
}
```

4. 同样地，登录成功后的跳转逻辑需要区分管理员和普通用户。在登录页或 `anonymousOnly` 守卫中，当用户已登录时：
   - 管理员跳转 `/`
   - 普通用户跳转 `/instances`

检查当前守卫代码中 `to.meta.anonymousOnly` 分支的 `return "/"` 是否需要修改为根据角色跳转。如果当前实现是简单的 `return "/"`，需改为：

```typescript
if (to.meta.anonymousOnly === true && auth.accessToken !== null) {
	return auth.currentUser?.is_admin ? "/" : "/instances"
}
```

**注意：** 保持开发预览路由（`/ui-preview`、`/ui-data-preview`）不变。

---

## 3. 重构导航数据

**文件：** `web/src/router/navigation.ts`

**当前状态：** 7 个扁平 `NavigationItem` 条目。

**目标状态：** 支持分组的导航结构，分为「工作区」和「管理」两组。

**详细修改：**

1. 更新类型定义：

```typescript
export type NavigationGroup = {
	label: string
	requiresAdmin?: boolean
	items: NavigationItem[]
}

export type NavigationItem = {
	label: string
	to: string
	caption?: string
	requiresAdmin?: boolean
}
```

2. 替换导出的导航数据为分组结构：

```typescript
export const navigationGroups: NavigationGroup[] = [
	{
		label: "工作区",
		items: [
			{
				label: "仪表盘",
				to: "/",
				caption: "DASHBOARD",
				requiresAdmin: true,
			},
			{
				label: "备份实例",
				to: "/instances",
				caption: "INSTANCES",
			},
		],
	},
	{
		label: "管理",
		requiresAdmin: true,
		items: [
			{
				label: "存储目标",
				to: "/storage-targets",
				caption: "STORAGE",
			},
			{
				label: "系统管理",
				to: "/system",
				caption: "SYSTEM",
			},
		],
	},
]
```

3. 如果旧代码导出的是 `navigationItems`，需要同时更新所有引用该导出名的地方（主要是 `SidebarNav.vue`，下一步处理）。

---

## 4. 重写 SidebarNav

**文件：** `web/src/layout/SidebarNav.vue`

**当前状态：** 品牌区 + 扁平导航链接列表，按 `requiresAdmin` 过滤。

**目标状态：** 品牌区 + 分组导航（带分组标题） + 底部固定区域（主题切换 + 退出 + 用户信息）。

**完整重写内容：**

```vue
<script setup lang="ts">
import { computed } from "vue"
import { useRouter } from "vue-router"

import { useSession } from "../composables/useSession"
import { navigationGroups } from "../router/navigation"
import { useAuthStore } from "../stores/auth"
import { useUiStore } from "../stores/ui"

const router = useRouter()
const session = useSession()
const auth = useAuthStore()
const ui = useUiStore()

const isAdmin = computed(() => auth.currentUser?.is_admin === true)
const username = computed(() => auth.currentUser?.username ?? "用户")
const userInitial = computed(() => username.value.charAt(0).toUpperCase())

const visibleGroups = computed(() =>
	navigationGroups
		.filter((group) => !group.requiresAdmin || isAdmin.value)
		.map((group) => ({
			...group,
			items: group.items.filter((item) => !item.requiresAdmin || isAdmin.value),
		}))
		.filter((group) => group.items.length > 0),
)

const themeLabel = computed(() => (ui.theme === "light" ? "深色主题" : "浅色主题"))

function toggleTheme(): void {
	ui.setTheme(ui.theme === "light" ? "dark" : "light")
}

async function handleLogout(): Promise<void> {
	session.logout()
	await router.push("/login")
}
</script>

<template>
	<nav class="sidebar-nav" aria-label="主导航">
		<div class="sidebar-nav__brand">
			<p class="sidebar-nav__brand-eyebrow">BALANCED FLUX</p>
			<p class="sidebar-nav__brand-title">Rsync Backup</p>
		</div>

		<div class="sidebar-nav__groups">
			<div v-for="group in visibleGroups" :key="group.label" class="sidebar-nav__group">
				<p class="sidebar-nav__group-label">{{ group.label }}</p>
				<div class="sidebar-nav__links">
					<RouterLink
						v-for="item in group.items"
						:key="item.to"
						:to="item.to"
						class="sidebar-nav__link"
						active-class="sidebar-nav__link--active"
						:exact="item.to === '/'"
					>
						<span class="sidebar-nav__link-label">{{ item.label }}</span>
						<span v-if="item.caption" class="sidebar-nav__link-caption">{{ item.caption }}</span>
					</RouterLink>
				</div>
			</div>
		</div>

		<div class="sidebar-nav__footer">
			<div class="sidebar-nav__footer-actions">
				<button type="button" class="sidebar-nav__footer-btn" :aria-label="themeLabel" @click="toggleTheme">
					{{ ui.theme === "light" ? "🌙" : "☀️" }}
				</button>
				<button type="button" class="sidebar-nav__footer-btn" aria-label="退出登录" @click="handleLogout">
					退出
				</button>
			</div>
			<RouterLink to="/profile" class="sidebar-nav__user">
				<span class="sidebar-nav__user-avatar">{{ userInitial }}</span>
				<span class="sidebar-nav__user-name">{{ username }}</span>
			</RouterLink>
		</div>
	</nav>
</template>

<style scoped>
.sidebar-nav {
	display: flex;
	flex-direction: column;
	height: 100%;
	padding: var(--space-5);
	gap: var(--space-4);
}

.sidebar-nav__brand {
	display: grid;
	gap: var(--space-1);
}

.sidebar-nav__brand-eyebrow {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.68rem;
	font-weight: 700;
	letter-spacing: 0.12em;
	text-transform: uppercase;
}

.sidebar-nav__brand-title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1.05rem;
	font-weight: 800;
	letter-spacing: -0.02em;
}

.sidebar-nav__groups {
	display: flex;
	flex-direction: column;
	gap: var(--space-5);
	flex: 1;
	overflow-y: auto;
}

.sidebar-nav__group {
	display: flex;
	flex-direction: column;
	gap: var(--space-2);
}

.sidebar-nav__group-label {
	margin: 0;
	padding: 0 var(--space-3);
	color: var(--text-muted);
	font-size: 0.72rem;
	font-weight: 700;
	letter-spacing: 0.1em;
	text-transform: uppercase;
}

.sidebar-nav__links {
	display: flex;
	flex-direction: column;
	gap: var(--space-1);
}

.sidebar-nav__link {
	display: grid;
	gap: var(--space-1);
	padding: var(--space-3);
	border: var(--border-width) solid transparent;
	border-radius: var(--radius-card);
	background: transparent;
	color: var(--text-default);
	text-decoration: none;
	transition:
		transform var(--duration-fast) ease,
		background-color var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.sidebar-nav__link:hover {
	transform: translateX(2px);
	background: var(--nav-link-bg-hover);
	border-color: var(--border-default);
	color: var(--text-strong);
}

.sidebar-nav__link:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.sidebar-nav__link--active {
	background: var(--nav-link-bg-active);
	border-color: color-mix(in srgb, var(--primary-500) 28%, var(--border-default));
	color: var(--nav-link-text-active);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.sidebar-nav__link-label {
	font-weight: 700;
	font-size: 0.96rem;
}

.sidebar-nav__link-caption {
	font-size: 0.78rem;
	letter-spacing: 0.05em;
	text-transform: uppercase;
	opacity: 0.84;
}

.sidebar-nav__footer {
	display: flex;
	flex-direction: column;
	gap: var(--space-3);
	padding-top: var(--space-4);
	border-top: var(--border-width) solid var(--border-default);
}

.sidebar-nav__footer-actions {
	display: flex;
	gap: var(--space-2);
	align-items: center;
}

.sidebar-nav__footer-btn {
	padding: var(--space-2) var(--space-3);
	border: var(--border-width) solid var(--border-default);
	border-radius: var(--radius-button);
	background: transparent;
	color: var(--text-muted);
	font: inherit;
	font-size: 0.84rem;
	font-weight: 600;
	cursor: pointer;
	transition:
		background-color var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.sidebar-nav__footer-btn:hover {
	background: var(--nav-link-bg-hover);
	color: var(--text-strong);
}

.sidebar-nav__footer-btn:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.sidebar-nav__user {
	display: flex;
	align-items: center;
	gap: var(--space-3);
	padding: var(--space-2);
	border-radius: var(--radius-card);
	text-decoration: none;
	color: var(--text-default);
	transition:
		background-color var(--duration-fast) ease;
}

.sidebar-nav__user:hover {
	background: var(--nav-link-bg-hover);
}

.sidebar-nav__user:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.sidebar-nav__user-avatar {
	display: inline-grid;
	place-items: center;
	width: 2rem;
	height: 2rem;
	border-radius: 999px;
	background: color-mix(in srgb, var(--primary-500) 18%, var(--surface-elevated));
	color: var(--text-strong);
	font-size: 0.84rem;
	font-weight: 700;
}

.sidebar-nav__user-name {
	font-size: 0.9rem;
	font-weight: 600;
}

@media (max-width: 960px) {
	.sidebar-nav {
		padding: var(--space-4);
	}

	.sidebar-nav__links {
		flex-direction: row;
		flex-wrap: wrap;
		gap: var(--space-2);
	}

	.sidebar-nav__footer {
		flex-direction: row;
		justify-content: space-between;
		align-items: center;
	}
}
</style>
```

**RouterLink 的 `exact` 属性说明：** 仪表盘路径为 `/`，需要 exact 匹配以避免所有路径都高亮。Vue Router 4 中使用 `:exact="true"` 或改用 `exact-active-class`。注意：Vue Router 4 实际上没有 `exact` prop，而是通过 `activeClass` vs `exactActiveClass` 区分。你需要确保仪表盘链接使用 `exact-active-class` 而非 `active-class`。如果其他链接也需要精确匹配，修改 `active-class` 的绑定方式：

- 对 `/` 路径使用 `exact-active-class="sidebar-nav__link--active"` 且不设 `active-class`
- 对其他路径使用 `active-class="sidebar-nav__link--active"`

或者更简洁的做法：全部使用 `RouterLink` 的默认行为，通过 CSS `.router-link-exact-active` 控制仪表盘，`.router-link-active` 控制其他项。但为了简洁，建议统一使用 `exact-active-class`（所有导航项都用精确匹配），因为侧边栏导航只需精确匹配当前页面。

---

## 5. 精简 TopBar

**文件：** `web/src/layout/TopBar.vue`

**当前状态：** 页面标题 + 描述 + "Operations Console" 眉批 + "会话已验证" badge + 主题切换按钮 + 退出按钮。

**目标状态：** 仅保留页面标题和描述，不承担任何操作功能。

**完整重写内容：**

```vue
<script setup lang="ts">
import { computed } from "vue"
import { useRoute } from "vue-router"

const route = useRoute()

const title = computed(() =>
	typeof route.meta.title === "string" ? route.meta.title : "Rsync Backup Service",
)

const subtitle = computed(() =>
	typeof route.meta.description === "string" ? route.meta.description : "",
)
</script>

<template>
	<header class="top-bar">
		<h1 class="top-bar__title">{{ title }}</h1>
		<p v-if="subtitle" class="top-bar__subtitle">{{ subtitle }}</p>
	</header>
</template>

<style scoped>
.top-bar {
	display: grid;
	gap: var(--space-2);
	padding: var(--space-5);
	border: var(--border-width) solid var(--shell-panel-border);
	border-radius: var(--radius-card);
	background: var(--shell-topbar-bg);
	box-shadow: var(--shell-panel-shadow);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
}

.top-bar__title {
	margin: 0;
	color: var(--text-strong);
	font-size: clamp(1.35rem, 2.2vw, 2rem);
	font-weight: 700;
	line-height: 1.05;
	letter-spacing: -0.04em;
}

.top-bar__subtitle {
	margin: 0;
	max-width: 48rem;
	color: var(--text-muted);
	font-size: 0.96rem;
}

@media (max-width: 960px) {
	.top-bar {
		padding: var(--space-4);
	}
}
</style>
```

**注意：** 移除了对 `useSession`、`useUiStore`、`useRouter` 的导入，以及所有操作逻辑。这些功能已迁移到 SidebarNav 底部。

---

## 6. 创建 SystemAdminView 占位页面

**文件：** `web/src/views/SystemAdminView.vue`（新建）

创建一个包含 4 Tab 结构的占位页面。后续 Phase 5 将填充完整内容。

```vue
<script setup lang="ts">
import { ref } from "vue"

import AppTabs from "../components/ui/AppTabs.vue"

const activeTab = ref("users")

const tabs = [
	{ value: "users", label: "用户管理" },
	{ value: "ssh-keys", label: "SSH 密钥" },
	{ value: "notifications", label: "通知渠道" },
	{ value: "audit-logs", label: "审计日志" },
]
</script>

<template>
	<section class="page-view">
		<header class="page-header">
			<div>
				<h1 class="page-header__title">系统管理</h1>
				<p class="page-header__subtitle">用户管理、SSH 密钥、通知渠道与审计日志。</p>
			</div>
		</header>

		<AppTabs v-model="activeTab" :tabs="tabs" aria-label="系统管理标签" />

		<p class="page-muted">{{ activeTab }} — 此标签页内容将在后续阶段实现。</p>
	</section>
</template>
```

---

## 7. 创建 ProfileView 占位页面

**文件：** `web/src/views/ProfileView.vue`（新建）

创建一个包含会话信息和修改密码区域的占位页面。后续 Phase 5 将填充完整内容。

```vue
<script setup lang="ts">
import { computed } from "vue"

import { useAuthStore } from "../stores/auth"
import AppCard from "../components/ui/AppCard.vue"

const auth = useAuthStore()
const username = computed(() => auth.currentUser?.username ?? "—")
const isAdmin = computed(() => auth.currentUser?.is_admin === true)
</script>

<template>
	<section class="page-view">
		<header class="page-header">
			<div>
				<h1 class="page-header__title">个人信息</h1>
				<p class="page-header__subtitle">查看会话信息和修改密码。</p>
			</div>
		</header>

		<AppCard title="当前会话" description="你的登录账户信息。">
			<p>用户名：{{ username }}</p>
			<p>角色：{{ isAdmin ? "管理员" : "普通用户" }}</p>
		</AppCard>

		<p class="page-muted">密码修改功能将在后续阶段实现。</p>
	</section>
</template>
```

---

## 8. 验证与提交

1. 确认前端编译通过：

```bash
npm --prefix web run build
```

2. 确认 TypeScript 无错误。

3. 如果有前端测试引用了旧的 `navigationItems` 导出名或旧路由路径，需要同步更新测试文件。搜索 `web/src` 中对以下内容的引用并修复：
   - `navigationItems`（旧导出名）→ `navigationGroups`
   - `/ssh-keys`、`/notifications`、`/audit-logs`、`/settings`（旧路由路径）

4. 提交：

```bash
git add -A
git commit -m "refactor(web): Phase 1 — restructure global shell (router, nav, sidebar, topbar)"
```

---

## 9. 启动服务并测试

启动完整服务：

```bash
make run
```

同时在另一个终端启动前端开发服务器：

```bash
npm --prefix web run dev
```

然后使用 `askQuestion` 工具向用户提出以下测试问题：

**问题标题：** Phase 1 全局骨架重构测试

**测试清单（请用户逐项确认）：**

1. **侧边栏导航** — 管理员账户登录后，侧边栏是否显示「工作区」（仪表盘、备份实例）和「管理」（存储目标、系统管理）两个分组？
2. **侧边栏底部** — 是否在侧边栏底部看到主题切换按钮（🌙/☀️）、退出按钮、以及带头像字母和用户名的个人信息入口？
3. **TopBar** — 顶部栏是否仅显示页面标题和描述？是否已移除 "Operations Console" 眉批、"会话已验证" Badge、主题切换和退出按钮？
4. **主题切换** — 点击侧边栏底部的主题按钮，是否正常切换深色/浅色主题？
5. **退出登录** — 点击侧边栏底部的「退出」按钮，是否正常退出并跳转到登录页？
6. **个人信息** — 点击侧边栏底部的用户头像区域，是否跳转到 `/profile` 页面？
7. **系统管理** — 点击侧边栏的「系统管理」，是否跳转到 `/system` 页面并显示 4 个 Tab？
8. **普通用户视角** — 使用非管理员账户登录后，侧边栏是否只显示「工作区 > 备份实例」？是否看不到仪表盘、存储目标、系统管理？
9. **普通用户重定向** — 普通用户直接访问 `/` 时是否被重定向到 `/instances`？
10. **其他页面正常** — 仪表盘、实例列表、实例详情、存储目标等现有页面是否仍然正常渲染（即使布局尚未改动）？
