import { render, screen } from "@testing-library/vue"

import AppShell from "./AppShell.vue"

vi.mock("./SidebarNav.vue", () => ({
	default: {
		template: '<nav aria-label="主导航">Navigation</nav>',
	},
}))

describe("AppShell layout", () => {
	it("wraps the sidebar in a dedicated rail container for fixed-height positioning", () => {
		const { container } = render(AppShell, {
			global: {
				stubs: {
					RouterView: {
						template: '<div data-testid="router-view" />',
					},
				},
			},
		})

		const sidebarRail = container.querySelector(".app-shell__sidebar")

		expect(sidebarRail).not.toBeNull()
		expect(sidebarRail).toContainElement(screen.getByRole("navigation", { name: "主导航" }))
		expect(screen.getByTestId("router-view")).toBeInTheDocument()
	})
})