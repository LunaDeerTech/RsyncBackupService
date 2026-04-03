import { render, screen } from "@testing-library/vue"
import { createMemoryHistory, createRouter } from "vue-router"

import { useAuthStore } from "../stores/auth"
import SidebarNav from "./SidebarNav.vue"

describe("SidebarNav", () => {
	beforeEach(() => {
		localStorage.clear()
		useAuthStore().clearSession()
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
})