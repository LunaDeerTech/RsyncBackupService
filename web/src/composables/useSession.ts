import { computed } from "vue"

import { apiFetch } from "../api/client"
import { normalizeTokenPair, type ApiTokenPair, type LoginCredentials, type SessionTokens } from "../api/types"
import { useAuthStore } from "../stores/auth"

type SessionComposable = {
	accessToken: ReturnType<typeof computed<string | null>>
	refreshToken: ReturnType<typeof computed<string | null>>
	isAuthenticated: ReturnType<typeof computed<boolean>>
	login(credentials: LoginCredentials): Promise<void>
	logout(): void
	setSession(tokens: SessionTokens): void
	clearSession(): void
}

export function useSession(): SessionComposable {
	const auth = useAuthStore()
	const isAuthenticated = computed(() => auth.accessToken !== null)

	async function login(credentials: LoginCredentials): Promise<void> {
		const tokens = await apiFetch<ApiTokenPair>("/api/auth/login", {
			method: "POST",
			headers: {
				"Content-Type": "application/json",
			},
			body: JSON.stringify(credentials),
		})

		auth.setSession(normalizeTokenPair(tokens))
	}

	function logout(): void {
		auth.clearSession()
	}

	return {
		accessToken: computed(() => auth.accessToken),
		refreshToken: computed(() => auth.refreshToken),
		isAuthenticated,
		login,
		logout,
		setSession(tokens) {
			auth.setSession(tokens)
		},
		clearSession() {
			auth.clearSession()
		},
	}
}