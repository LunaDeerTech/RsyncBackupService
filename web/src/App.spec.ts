import { render, screen } from "@testing-library/vue"
import App from "./App.vue"
import { createRouter } from "./router"
import { useAuthStore } from "./stores/auth"

describe("App", () => {
	beforeEach(() => {
		localStorage.clear()
		history.replaceState(null, "", "/")
		useAuthStore().clearSession()
	})

	it("renders the application root", async () => {
		const router = createRouter()
		await router.push("/login")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getByTestId("app-root")).toBeInTheDocument()
		expect(screen.getByTestId("login-shell")).toBeInTheDocument()
	})

	it("redirects anonymous visitors to the login shell", async () => {
		const router = createRouter()
		await router.push("/")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(router.currentRoute.value.path).toBe("/login")
		expect(screen.getByRole("heading", { name: "Rsync Backup Service" })).toBeInTheDocument()
	})
})