import { defineComponent, h } from "vue"
import { createRouter as createVueRouter, createWebHistory, type RouteRecordRaw, type Router } from "vue-router"

const HomeView = defineComponent({
	name: "HomeView",
	render() {
		return h("main", { class: "shell-home", "data-testid": "home-view" }, [
			h("section", { class: "shell-home__panel" }, [
				h("p", { class: "shell-home__eyebrow" }, "Balanced Flux UI Shell"),
				h("h1", { class: "shell-home__title" }, "Rsync Backup Service"),
				h(
					"p",
					{ class: "shell-home__body" },
					"Task 01 scaffolds the Vue application shell, base route, and theme token entry points.",
				),
			]),
		])
	},
})

const routes: RouteRecordRaw[] = [
	{
		path: "/",
		name: "home",
		component: HomeView,
	},
]

export function createRouter(): Router {
	return createVueRouter({
		history: createWebHistory(),
		routes,
	})
}