import { useAuthStore } from "../stores/auth"
import { normalizeTokenPair, type ApiErrorResponse, type ApiTokenPair } from "./types"

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL?.trim() ?? ""

let refreshRequest: Promise<void> | null = null

export class ApiError extends Error {
	status: number

	constructor(message: string, status: number) {
		super(message)
		this.name = "ApiError"
		this.status = status
	}
}

function resolveUrl(path: string): string {
	if (/^https?:\/\//.test(path)) {
		return path
	}

	if (API_BASE_URL === "") {
		return path.startsWith("/") ? path : `/${path}`
	}

	if (path.startsWith("/")) {
		return `${API_BASE_URL}${path}`
	}

	return `${API_BASE_URL}/${path}`
}

function buildRequestInit(init: RequestInit, accessToken: string | null): RequestInit {
	const headers = new Headers(init.headers)

	if (!headers.has("Accept")) {
		headers.set("Accept", "application/json")
	}

	if (accessToken !== null && !headers.has("Authorization")) {
		headers.set("Authorization", `Bearer ${accessToken}`)
	}

	return {
		...init,
		headers,
	}
}

function canRefresh(path: string, refreshToken: string | null): boolean {
	return refreshToken !== null && !path.endsWith("/api/auth/login") && !path.endsWith("/api/auth/refresh")
}

async function readErrorMessage(response: Response): Promise<string | null> {
	const contentType = response.headers.get("content-type") ?? ""

	if (contentType.includes("application/json")) {
		try {
			const body = (await response.clone().json()) as ApiErrorResponse
			if (typeof body.error === "string" && body.error.trim() !== "") {
				return body.error.trim()
			}
		} catch {
			return null
		}

		return null
	}

	try {
		const text = (await response.clone().text()).trim()
		return text === "" ? null : text
	} catch {
		return null
	}
}

async function shouldRefresh(response: Response, path: string, refreshToken: string | null): Promise<boolean> {
	if (response.status !== 401 || !canRefresh(path, refreshToken)) {
		return false
	}

	const errorMessage = await readErrorMessage(response)
	return errorMessage === "token expired" || errorMessage === "invalid token"
}

async function createApiError(response: Response): Promise<ApiError> {
	let message = response.statusText || "Request failed"
	const errorMessage = await readErrorMessage(response)
	if (errorMessage !== null) {
		message = errorMessage
	}

	return new ApiError(message, response.status)
}

async function parseResponse<T>(response: Response): Promise<T> {
	if (response.status === 204) {
		return undefined as T
	}

	const contentType = response.headers.get("content-type") ?? ""
	if (contentType.includes("application/json")) {
		return (await response.json()) as T
	}

	return (await response.text()) as T
}

async function requestOnce<T>(path: string, init: RequestInit = {}, canRetryRequest = true): Promise<T> {
	const auth = useAuthStore()
	const response = await fetch(resolveUrl(path), buildRequestInit(init, auth.accessToken))

	if (canRetryRequest && (await shouldRefresh(response, path, auth.refreshToken))) {
		await refreshSession()
		return requestOnce<T>(path, init, false)
	}

	if (!response.ok) {
		throw await createApiError(response)
	}

	return parseResponse<T>(response)
}

export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
	return requestOnce<T>(path, init, true)
}

export async function refreshSession(): Promise<void> {
	const auth = useAuthStore()

	if (auth.refreshToken === null) {
		auth.clearSession()
		throw new ApiError("authentication required", 401)
	}

	if (refreshRequest !== null) {
		return refreshRequest
	}

	refreshRequest = (async () => {
		const response = await fetch(resolveUrl("/api/auth/refresh"), {
			method: "POST",
			headers: {
				Accept: "application/json",
				"Content-Type": "application/json",
			},
			body: JSON.stringify({
				refresh_token: auth.refreshToken,
			}),
		})

		if (!response.ok) {
			auth.clearSession()
			throw await createApiError(response)
		}

		const tokens = normalizeTokenPair((await response.json()) as ApiTokenPair)
		auth.setSession(tokens)
	})()

	try {
		await refreshRequest
	} finally {
		refreshRequest = null
	}
}