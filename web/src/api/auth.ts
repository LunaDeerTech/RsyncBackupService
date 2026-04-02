import { apiFetch } from "./client"
import type {
	AuthUser,
	LoginCredentials,
	LoginResponse,
	VerifyResponse,
} from "./types"

type ChangePasswordPayload = {
	current_password: string
	new_password: string
}

export async function login(payload: LoginCredentials): Promise<LoginResponse> {
	return apiFetch<LoginResponse>("/api/auth/login", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function verifyPassword(password: string): Promise<VerifyResponse> {
	return apiFetch<VerifyResponse>("/api/auth/verify", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify({ password }),
	})
}

export async function getCurrentUser(): Promise<AuthUser> {
	return apiFetch<AuthUser>("/api/auth/me")
}

export async function changePassword(payload: ChangePasswordPayload): Promise<void> {
	await apiFetch<void>("/api/auth/password", {
		method: "PUT",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}