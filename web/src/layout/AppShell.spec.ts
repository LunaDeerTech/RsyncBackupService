import { render, screen } from "@testing-library/vue"

import App from "../App.vue"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import { useUiStore, THEME_STORAGE_KEY } from "../stores/ui"

describe("AppShell", () => {
	beforeEach(() => {
		localStorage.clear()
		history.replaceState(null, "", "/")
		document.documentElement.removeAttribute("data-theme")
		useAuthStore().clearSession()
		useUiStore().setTheme("light")
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
		expect(screen.getByRole("heading", { name: "仪表盘" })).toBeInTheDocument()
	})
})