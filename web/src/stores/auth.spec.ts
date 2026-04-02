import { useAuthStore, AUTH_STORAGE_KEY } from "./auth"

describe("useAuthStore", () => {
	beforeEach(() => {
		localStorage.clear()
		useAuthStore().clearSession()
	})

	it("persists the session tokens", () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})

		expect(auth.accessToken).toBe("access-token")
		expect(auth.refreshToken).toBe("refresh-token")
		expect(localStorage.getItem(AUTH_STORAGE_KEY)).toBe(
			JSON.stringify({
				accessToken: "access-token",
				refreshToken: "refresh-token",
			}),
		)
	})

	it("clears the persisted session", () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.clearSession()

		expect(auth.accessToken).toBeNull()
		expect(auth.refreshToken).toBeNull()
		expect(localStorage.getItem(AUTH_STORAGE_KEY)).toBeNull()
	})
})