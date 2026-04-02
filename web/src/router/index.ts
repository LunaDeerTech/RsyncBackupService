import { createRouter as createVueRouter, createWebHistory, type RouteRecordRaw, type Router } from "vue-router"

import { getCurrentUser } from "../api/auth"
import { ApiError } from "../api/client"
import AppShell from "../layout/AppShell.vue"
import { useAuthStore } from "../stores/auth"

const protectedRoutes: RouteRecordRaw[] = [
	{
		path: "",
		name: "dashboard",
		component: () => import("../views/DashboardView.vue"),
		meta: {
			requiresAuth: true,
			requiresAdmin: true,
			title: "仪表盘",
			description: "统计卡片、运行中任务、最近备份与存储概览。",
		},
	},
	{
		path: "instances",
		name: "instances",
		component: () => import("../views/InstancesListView.vue"),
		meta: {
			requiresAuth: true,
			title: "备份实例",
			description: "实例列表、基础筛选和实例创建入口。",
		},
	},
	{
		path: "instances/:id",
		name: "instance-detail",
		component: () => import("../views/InstanceDetailView.vue"),
		meta: {
			requiresAuth: true,
			title: "实例详情",
			description: "概览、策略、备份历史、恢复和通知订阅。",
		},
	},
	{
		path: "storage-targets",
		name: "storageTargets",
		component: () => import("../views/StorageTargetsView.vue"),
		meta: {
			requiresAuth: true,
			requiresAdmin: true,
			title: "存储目标",
			description: "管理本地与 SSH 存储目标，并执行连通性测试。",
		},
	},
	{
		path: "ssh-keys",
		name: "sshKeys",
		component: () => import("../views/SSHKeysView.vue"),
		meta: {
			requiresAuth: true,
			requiresAdmin: true,
			title: "SSH 密钥",
			description: "登记 SSH 密钥并对目标主机执行验证。",
		},
	},
	{
		path: "notifications",
		name: "notifications",
		component: () => import("../views/NotificationsView.vue"),
		meta: {
			requiresAuth: true,
			requiresAdmin: true,
			title: "通知渠道",
			description: "配置 SMTP 渠道并管理通知测试。",
		},
	},
	{
		path: "audit-logs",
		name: "auditLogs",
		component: () => import("../views/AuditLogsView.vue"),
		meta: {
			requiresAuth: true,
			requiresAdmin: true,
			title: "审计日志",
			description: "筛选关键操作并查看本页结果的时间线摘要。",
		},
	},
	{
		path: "settings",
		name: "settings",
		component: () => import("../views/SettingsView.vue"),
		meta: {
			requiresAuth: true,
			requiresAdmin: true,
			title: "系统设置",
			description: "用户管理、密码修改和实例权限设置。",
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