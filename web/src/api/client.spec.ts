import { apiFetch } from "./client"
import { useAuthStore } from "../stores/auth"

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: {
			"Content-Type": "application/json",
		},
	})
}

describe("apiFetch", () => {
	afterEach(() => {
		vi.restoreAllMocks()
		localStorage.clear()
		useAuthStore().clearSession()
	})

	it("injects the access token into requests", async () => {
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})

		const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
			jsonResponse({ healthy: true }),
		)

		await apiFetch<{ healthy: boolean }>("/api/system/status")

		expect(fetchSpy).toHaveBeenCalledTimes(1)
		expect(new Headers(fetchSpy.mock.calls[0]?.[1]?.headers).get("Authorization")).toBe(
			"Bearer access-token",
		)
	})

	it("refreshes the session and replays the original request when the access token expires", async () => {
		useAuthStore().setSession({
			accessToken: "stale-access",
			refreshToken: "refresh-token",
		})

		const fetchSpy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(jsonResponse({ error: "token expired" }, 401))
			.mockResolvedValueOnce(
				jsonResponse({
					access_token: "next-access",
					refresh_token: "next-refresh",
				}),
			)
			.mockResolvedValueOnce(jsonResponse({ healthy: true }))

		const payload = await apiFetch<{ healthy: boolean }>("/api/system/status")

		expect(payload.healthy).toBe(true)
		expect(fetchSpy).toHaveBeenCalledTimes(3)
		expect(fetchSpy.mock.calls[1]?.[0]).toBe("/api/auth/refresh")
		expect(useAuthStore().accessToken).toBe("next-access")
		expect(new Headers(fetchSpy.mock.calls[2]?.[1]?.headers).get("Authorization")).toBe(
			"Bearer next-access",
		)
	})

	it("reuses a single refresh request for concurrent 401 responses", async () => {
		useAuthStore().setSession({
			accessToken: "stale-access",
			refreshToken: "refresh-token",
		})

		let resolveRefresh: ((value: Response) => void) | null = null
		const refreshPromise = new Promise<Response>((resolve) => {
			resolveRefresh = resolve
		})

		const fetchSpy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(jsonResponse({ error: "token expired" }, 401))
			.mockResolvedValueOnce(jsonResponse({ error: "token expired" }, 401))
			.mockImplementationOnce(() => refreshPromise)
			.mockResolvedValueOnce(jsonResponse({ resource: "one" }))
			.mockResolvedValueOnce(jsonResponse({ resource: "two" }))

		const firstRequest = apiFetch<{ resource: string }>("/api/system/status")
		const secondRequest = apiFetch<{ resource: string }>("/api/system/dashboard")

		await Promise.resolve()
		resolveRefresh?.(
			jsonResponse({
				access_token: "next-access",
				refresh_token: "next-refresh",
			}),
		)

		const [first, second] = await Promise.all([firstRequest, secondRequest])

		expect(first.resource).toBe("one")
		expect(second.resource).toBe("two")
		expect(fetchSpy.mock.calls.filter((call) => call[0] === "/api/auth/refresh")).toHaveLength(1)
	})

	it("clears the session when refresh fails", async () => {
		useAuthStore().setSession({
			accessToken: "stale-access",
			refreshToken: "refresh-token",
		})

		vi.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(jsonResponse({ error: "token expired" }, 401))
			.mockResolvedValueOnce(jsonResponse({ error: "refresh token expired" }, 401))

		await expect(apiFetch("/api/system/status")).rejects.toMatchObject({
			message: "refresh token expired",
			status: 401,
		})
		expect(useAuthStore().accessToken).toBeNull()
		expect(useAuthStore().refreshToken).toBeNull()
	})

	it("does not refresh the session for verify token failures", async () => {
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})

		const fetchSpy = vi
			.spyOn(globalThis, "fetch")
			.mockResolvedValueOnce(jsonResponse({ error: "invalid verify token" }, 401))

		await expect(
			apiFetch("/api/instances/1", {
				method: "DELETE",
				headers: {
					"X-Verify-Token": "bad-token",
				},
			}),
		).rejects.toMatchObject({
			message: "invalid verify token",
			status: 401,
		})

		expect(fetchSpy).toHaveBeenCalledTimes(1)
	})
})