import { reactive } from "vue"

import type { AuthUser, SessionTokens } from "../api/types"

export const AUTH_STORAGE_KEY = "rbs.auth.session"

type AuthStore = {
	accessToken: string | null
	refreshToken: string | null
	currentUser: AuthUser | null
	setSession(tokens: SessionTokens): void
	setCurrentUser(user: AuthUser | null): void
	clearSession(): void
}

function readStoredSession(): SessionTokens | null {
	if (typeof localStorage === "undefined") {
		return null
	}

	const raw = localStorage.getItem(AUTH_STORAGE_KEY)
	if (raw === null) {
		return null
	}

	try {
		const parsed = JSON.parse(raw) as Partial<SessionTokens>
		if (typeof parsed.accessToken !== "string" || typeof parsed.refreshToken !== "string") {
			localStorage.removeItem(AUTH_STORAGE_KEY)
			return null
		}

		return {
			accessToken: parsed.accessToken,
			refreshToken: parsed.refreshToken,
		}
	} catch {
		localStorage.removeItem(AUTH_STORAGE_KEY)
		return null
	}
}

function persistSession(tokens: SessionTokens | null): void {
	if (typeof localStorage === "undefined") {
		return
	}

	if (tokens === null) {
		localStorage.removeItem(AUTH_STORAGE_KEY)
		return
	}

	localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(tokens))
}

function applySession(tokens: SessionTokens): void {
	const accessToken = tokens.accessToken.trim()
	const refreshToken = tokens.refreshToken.trim()

	if (accessToken === "" || refreshToken === "") {
		clearSessionState()
		return
	}

	authStore.accessToken = accessToken
	authStore.refreshToken = refreshToken
	persistSession({ accessToken, refreshToken })
}

function clearSessionState(): void {
	authStore.accessToken = null
	authStore.refreshToken = null
	authStore.currentUser = null
	persistSession(null)
}

const initialSession = readStoredSession()

const authStore = reactive<AuthStore>({
	accessToken: initialSession?.accessToken ?? null,
	refreshToken: initialSession?.refreshToken ?? null,
	currentUser: null,
	setSession(tokens) {
		applySession(tokens)
	},
	setCurrentUser(user) {
		authStore.currentUser = user
	},
	clearSession() {
		clearSessionState()
	},
})

export function useAuthStore(): AuthStore {
	return authStore
}