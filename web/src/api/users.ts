import { apiFetch } from "./client"
import type {
	CreateUserPayload,
	InstancePermission,
	PermissionPayload,
	ResetUserPasswordPayload,
	UserSummary,
} from "./types"

export async function listUsers(): Promise<UserSummary[]> {
	return apiFetch<UserSummary[]>("/api/users")
}

export async function createUser(payload: CreateUserPayload): Promise<UserSummary> {
	return apiFetch<UserSummary>("/api/users", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function resetUserPassword(userId: number, payload: ResetUserPasswordPayload): Promise<void> {
	await apiFetch<void>(`/api/users/${userId}/password`, {
		method: "PUT",
		headers: {
			"Content-Type": "application/json",
			"X-Verify-Token": payload.verify_token,
		},
		body: JSON.stringify({ password: payload.password }),
	})
}

export async function listInstancePermissions(instanceId: number): Promise<InstancePermission[]> {
	return apiFetch<InstancePermission[]>(`/api/instances/${instanceId}/permissions`)
}

export async function updateInstancePermissions(instanceId: number, payload: PermissionPayload[]): Promise<void> {
	await Promise.all(
		payload.map((entry) =>
			apiFetch<void>(`/api/instances/${instanceId}/permissions/${entry.user_id}`, {
				method: "PUT",
				headers: {
					"Content-Type": "application/json",
				},
				body: JSON.stringify({ role: entry.role }),
			}),
		),
	)
}