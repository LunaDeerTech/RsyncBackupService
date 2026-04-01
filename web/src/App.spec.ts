import { render, screen } from "@testing-library/vue"
import App from "./App.vue"
import { createRouter } from "./router"

describe("App shell", () => {
	it("renders the application root", async () => {
		const router = createRouter()
		await router.push("/")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getByTestId("app-root")).toBeInTheDocument()
		expect(screen.getByText("Rsync Backup Service")).toBeInTheDocument()
	})

	it("assembles the base route", () => {
		const router = createRouter()

		expect(
			router.getRoutes().some((route) => route.path === "/" && route.name === "home"),
		).toBe(true)
	})
})