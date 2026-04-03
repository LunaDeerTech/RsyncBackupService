import { createRouter as createVueRouter, createWebHistory, type RouteRecordRaw, type Router } from "vue-router"

import { getCurrentUser } from "../api/auth"
import { ApiError } from "../api/client"
import AppShell from "../layout/AppShell.vue"
import { useAuthStore } from "../stores/auth"

const DashboardView = () => import("../views/DashboardView.vue")
const InstancesListView = () => import("../views/InstancesListView.vue")
const InstanceDetailView = () => import("../views/InstanceDetailView.vue")
const StorageTargetsView = () => import("../views/StorageTargetsView.vue")
const SystemAdminView = () => import("../views/SystemAdminView.vue")
const ProfileView = () => import("../views/ProfileView.vue")

async function ensureCurrentUser() {
	const auth = useAuthStore()

	if (auth.accessToken === null) {
		auth.setCurrentUser(null)
		return null
	}

	if (auth.currentUser !== null) {
		return auth.currentUser
	}

	try {
		const currentUser = await getCurrentUser()
		auth.setCurrentUser(currentUser)
		return currentUser
	} catch (error) {
		if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
			auth.clearSession()
			return null
		}

		return null
	}
}

const protectedRoutes: RouteRecordRaw[] = [
	{
		path: "instances",
		name: "instances",
		component: InstancesListView,
		meta: {
			title: "备份实例",
			description: "管理源路径、源主机和实例级恢复入口。",
		},
	},
	{
		path: "",
		name: "dashboard",
		component: DashboardView,
		meta: {
			title: "运维仪表盘",
			description: "查看全局统计、运行中任务、最近备份与存储容量。",
			requiresAdmin: true,
		},
	},
	{
		path: "instances/:id",
		name: "instance-detail",
		component: InstanceDetailView,
		meta: {
			title: "实例详情",
			description: "查看实例配置、策略、备份历史与恢复操作。",
		},
	},
	{
		path: "storage-targets",
		name: "storage-targets",
		component: StorageTargetsView,
		meta: {
			title: "存储目标",
			description: "按备份类型管理目标路径，并执行连通性测试。",
			requiresAdmin: true,
		},
	},
	{
		path: "system",
		name: "system",
		component: SystemAdminView,
		meta: {
			title: "系统管理",
			description: "用户管理、SSH 密钥、通知渠道与审计日志。",
			requiresAdmin: true,
		},
	},
	{
		path: "profile",
		name: "profile",
		component: ProfileView,
		meta: {
			title: "个人信息",
			description: "查看会话信息和修改密码。",
		},
	},
]

const previewRoutes: RouteRecordRaw[] = import.meta.env.DEV
	? [
			{
				path: "/ui-preview",
				name: "ui-preview",
				component: () => import("../views/UiActionsPreviewView.vue"),
				meta: {
					title: "Task 11 组件预览",
					description: "输入、操作与危险交互组件预览页。",
				},
			},
			{
				path: "/ui-data-preview",
				name: "ui-data-preview",
				component: () => import("../views/UiDataDisplayPreviewView.vue"),
				meta: {
					title: "Task 12 组件预览",
					description: "表格、状态与反馈组件预览页。",
				},
			},
		]
	: []

const legacyRoutes: RouteRecordRaw[] = [
	{
		path: "/ssh-keys",
		redirect: "/system",
	},
	{
		path: "/notifications",
		redirect: "/system",
	},
	{
		path: "/audit-logs",
		redirect: "/system",
	},
	{
		path: "/settings",
		name: "legacy-settings",
		meta: {
			requiresAuth: true,
		},
		beforeEnter: async () => {
			const auth = useAuthStore()

			if (auth.accessToken === null) {
				return {
					name: "login",
					query: {
						redirect: "/settings",
					},
				}
			}

			const currentUser = auth.currentUser ?? (await ensureCurrentUser())
			return currentUser?.is_admin === true ? "/system" : "/profile"
		},
	},
]

const routes: RouteRecordRaw[] = [
	...previewRoutes,
	{
		path: "/login",
		name: "login",
		component: () => import("../views/LoginView.vue"),
		meta: {
			anonymousOnly: true,
			title: "登录",
			description: "通过用户名和密码进入 Rsync Backup Service。",
		},
	},
	...legacyRoutes,
	{
		path: "/",
		component: AppShell,
		meta: {
			requiresAuth: true,
		},
		children: protectedRoutes,
	},
	{
		path: "/:pathMatch(.*)*",
		redirect: "/",
	},
]

export function createRouter(): Router {
	const router = createVueRouter({
		history: createWebHistory(),
		routes,
	})

	router.beforeEach(async (to) => {
		const auth = useAuthStore()
		const isAuthenticated = auth.accessToken !== null
		const requiresAuth = to.matched.some((record) => record.meta.requiresAuth === true)
		const requiresAdmin = to.matched.some((record) => record.meta.requiresAdmin === true)
		const anonymousOnly = to.matched.some((record) => record.meta.anonymousOnly === true)

		if (requiresAuth && !isAuthenticated) {
			return {
				name: "login",
				query: {
					redirect: to.fullPath,
				},
			}
		}

		if (requiresAdmin) {
			const currentUser = await ensureCurrentUser()
			if (currentUser === null) {
				return {
					name: "login",
					query: {
						redirect: to.fullPath,
					},
				}
			}

			if (!currentUser.is_admin) {
				return "/instances"
			}
		}

		if (anonymousOnly && isAuthenticated) {
			const currentUser = auth.currentUser ?? (await ensureCurrentUser())
			const redirect = typeof to.query.redirect === "string" && to.query.redirect.startsWith("/")
				? to.query.redirect
				: currentUser?.is_admin === true
					? "/"
					: "/instances"
			return redirect
		}

		return true
	})

	return router
}