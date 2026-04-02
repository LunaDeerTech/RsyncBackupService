import { defineComponent, h } from "vue"
import { createRouter as createVueRouter, createWebHistory, type RouteRecordRaw, type Router } from "vue-router"

import AppShell from "../layout/AppShell.vue"
import LoginShell from "../layout/LoginShell.vue"
import { useAuthStore } from "../stores/auth"
import { primaryNavigation, type NavigationItem } from "./navigation"

function createPlaceholderView(item: NavigationItem) {
	return defineComponent({
		name: `${item.name}View`,
		render() {
			return h("section", { class: "page-placeholder", "data-testid": `page-${item.name}` }, [
				h("div", { class: "page-placeholder__card" }, [
					h("p", { class: "page-placeholder__eyebrow" }, item.eyebrow),
					h("h1", { class: "page-placeholder__title" }, item.label),
					h("p", { class: "page-placeholder__body" }, item.description),
				]),
				h("div", { class: "page-placeholder__card page-placeholder__card--secondary" }, [
					h("h2", { class: "page-placeholder__subtitle" }, "Task 10 交付内容"),
					h("ul", { class: "page-placeholder__list" }, [
						h("li", null, "统一 API client 已集中处理 token 注入与 refresh 重放。"),
						h("li", null, "主布局、侧栏和顶栏已经为后续页面预留承载位置。"),
						h("li", null, "业务组件、数据表格和实时视图将在后续任务逐步接入。"),
					]),
				]),
			])
		},
	})
}

const protectedRoutes: RouteRecordRaw[] = primaryNavigation.map((item) => ({
	path: item.childPath,
	name: item.name,
	component: createPlaceholderView(item),
	meta: {
		requiresAuth: true,
		title: item.label,
		description: item.description,
	},
}))

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
		]
	: []

const routes: RouteRecordRaw[] = [
	...previewRoutes,
	{
		path: "/login",
		name: "login",
		component: LoginShell,
		meta: {
			anonymousOnly: true,
			title: "登录",
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

	router.beforeEach((to) => {
		const auth = useAuthStore()
		const isAuthenticated = auth.accessToken !== null
		const requiresAuth = to.matched.some((record) => record.meta.requiresAuth === true)
		const anonymousOnly = to.matched.some((record) => record.meta.anonymousOnly === true)

		if (requiresAuth && !isAuthenticated) {
			return {
				name: "login",
				query: {
					redirect: to.fullPath,
				},
			}
		}

		if (anonymousOnly && isAuthenticated) {
			const redirect = typeof to.query.redirect === "string" && to.query.redirect.startsWith("/") ? to.query.redirect : "/"
			return redirect
		}

		return true
	})

	return router
}