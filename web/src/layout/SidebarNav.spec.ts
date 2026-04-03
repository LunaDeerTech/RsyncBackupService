import { render, screen } from "@testing-library/vue"
import { createMemoryHistory, createRouter } from "vue-router"

import { useAuthStore } from "../stores/auth"
import SidebarNav from "./SidebarNav.vue"

describe("SidebarNav", () => {
	beforeEach(() => {
		localStorage.clear()
		useAuthStore().clearSession()
	})

	it("replaces the demo-style brand copy with the product identity", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser({
			id: 2,
			username: "viewer",
			is_admin: false,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const router = createRouter({
			history: createMemoryHistory(),
			routes: [
				{ path: "/instances", component: { template: "<div />" } },
				{ path: "/profile", component: { template: "<div />" } },
			],
		})

		await router.push("/instances")
		await router.isReady()

		render(SidebarNav, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getByText("Rsync Backup Service")).toBeInTheDocument()
		expect(screen.getByText("运维备份控制台")).toBeInTheDocument()
		expect(screen.queryByText("BALANCED FLUX")).not.toBeInTheDocument()
	})

	it("keeps the instances navigation item active on instance detail routes", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser({
			id: 2,
			username: "viewer",
			is_admin: false,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const router = createRouter({
			history: createMemoryHistory(),
			routes: [
				{ path: "/instances", component: { template: "<div />" } },
				{ path: "/instances/:id", component: { template: "<div />" } },
				{ path: "/profile", component: { template: "<div />" } },
			],
		})

		await router.push("/instances/42")
		await router.isReady()

		render(SidebarNav, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getByRole("link", { name: "备份实例 INSTANCES" })).toHaveClass("sidebar-nav__link--active")
	})

	it("prevents horizontal scrolling inside the navigation groups when links animate on hover", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const router = createRouter({
			history: createMemoryHistory(),
			routes: [
				{ path: "/", component: { template: "<div />" } },
				{ path: "/instances", component: { template: "<div />" } },
				{ path: "/storage-targets", component: { template: "<div />" } },
				{ path: "/system", component: { template: "<div />" } },
				{ path: "/profile", component: { template: "<div />" } },
			],
		})

		await router.push("/")
		await router.isReady()

		const { container } = render(SidebarNav, {
			global: {
				plugins: [router],
			},
		})

		const groups = container.querySelector(".sidebar-nav__groups") as HTMLElement
		expect(window.getComputedStyle(groups).overflowX).toBe("hidden")
	})
})