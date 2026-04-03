import { createRouter as createVueRouter, createMemoryHistory } from "vue-router"

import { getCurrentUser } from "../api/auth"
import { createRouter } from "./index"
import { useAuthStore } from "../stores/auth"

vi.mock("../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

describe("router legacy redirects", () => {
	const adminUser = {
		id: 1,
		username: "admin",
		is_admin: true,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	}

	const viewerUser = {
		id: 2,
		username: "viewer",
		is_admin: false,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	}

	beforeEach(() => {
		localStorage.clear()
		useAuthStore().clearSession()
		vi.mocked(getCurrentUser).mockReset()
		vi.mocked(getCurrentUser).mockResolvedValue(adminUser)
	})

	it("redirects removed management routes to /system", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(adminUser)

		const router = createRouter()
		await router.push("/ssh-keys")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/system")

		await router.push("/notifications")
		expect(router.currentRoute.value.path).toBe("/system")

		await router.push("/audit-logs")
		expect(router.currentRoute.value.path).toBe("/system")
	})

	it("redirects the removed settings route to /profile for non-admin users", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(viewerUser)

		const router = createRouter()
		await router.push("/settings")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/profile")
	})
})

describe("sidebar-compatible route matching", () => {
	it("keeps the instances section active on instance detail routes", async () => {
		const router = createVueRouter({
			history: createMemoryHistory(),
			routes: [
				{ path: "/instances", component: { template: "<div />" } },
				{ path: "/instances/:id", component: { template: "<div />" } },
				{ path: "/profile", component: { template: "<div />" } },
			],
		})

		await router.push("/instances/42")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/instances/42")
	})
})