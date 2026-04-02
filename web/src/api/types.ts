export interface ApiTokenPair {
	access_token: string
	refresh_token: string
}

export interface SessionTokens {
	accessToken: string
	refreshToken: string
}

export interface LoginCredentials {
	username: string
	password: string
}

export interface AuthUser {
	id: number
	username: string
	is_admin: boolean
	created_at: string
	updated_at: string
}

export interface ApiErrorResponse {
	error?: string
}

export function normalizeTokenPair(tokens: ApiTokenPair): SessionTokens {
	return {
		accessToken: tokens.access_token,
		refreshToken: tokens.refresh_token,
	}
}