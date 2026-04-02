import { render, screen } from "@testing-library/vue"

import App from "../App.vue"
import { getCurrentUser } from "../api/auth"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import { useUiStore, THEME_STORAGE_KEY } from "../stores/ui"

vi.mock("../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

describe("AppShell", () => {
	beforeEach(() => {
		localStorage.clear()
		history.replaceState(null, "", "/")
		document.documentElement.removeAttribute("data-theme")
		useAuthStore().clearSession()
		useUiStore().setTheme("light")
		vi.mocked(getCurrentUser).mockReset()
		vi.mocked(getCurrentUser).mockResolvedValue({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})
	})

	it("redirects anonymous users to /login", async () => {
		const router = createRouter()

		await router.push("/")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/login")
	})

	it("applies the dark theme to the document root", () => {
		const ui = useUiStore()

		ui.setTheme("dark")

		expect(document.documentElement.dataset.theme).toBe("dark")
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark")
	})

	it("renders the authenticated app shell", async () => {
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})

		const router = createRouter()
		await router.push("/")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getByTestId("app-shell")).toBeInTheDocument()
		expect(screen.getByRole("navigation", { name: "主导航" })).toBeInTheDocument()
		expect(screen.getByRole("heading", { name: "运维仪表盘" })).toBeInTheDocument()
	})

	it("redirects non-admin users away from the admin dashboard", async () => {
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		vi.mocked(getCurrentUser).mockResolvedValue({
			id: 2,
			username: "viewer",
			is_admin: false,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const router = createRouter()
		await router.push("/")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/instances")
	})
})