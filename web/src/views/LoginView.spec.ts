import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { getCurrentUser, login } from "../api/auth"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import LoginView from "./LoginView.vue"

vi.mock("../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

describe("LoginView", () => {
	beforeEach(() => {
		localStorage.clear()
		history.replaceState(null, "", "/login")
		useAuthStore().clearSession()
		vi.mocked(login).mockReset()
		vi.mocked(getCurrentUser).mockReset()
		vi.mocked(login).mockResolvedValue({
			access_token: "access-token",
			refresh_token: "refresh-token",
		})
		vi.mocked(getCurrentUser).mockResolvedValue({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})
	})

	it("submits login form and stores returned token pair", async () => {
		const router = createRouter()
		await router.push("/login")
		await router.isReady()

		render(LoginView, {
			global: {
				plugins: [router],
			},
		})

		await fireEvent.update(screen.getByLabelText("用户名"), "admin")
		await fireEvent.update(screen.getByLabelText("密码"), "secret")
		await fireEvent.click(screen.getByRole("button", { name: "登录" }))

		await waitFor(() => {
			expect(login).toHaveBeenCalledWith({
				username: "admin",
				password: "secret",
			})
		})

		expect(useAuthStore().accessToken).toBe("access-token")
		expect(useAuthStore().refreshToken).toBe("refresh-token")
	})
})