import { apiFetch } from "./client"
import type { SSHKeyConnectionPayload, SSHKeyPayload, SSHKeySummary } from "./types"

export async function listSSHKeys(): Promise<SSHKeySummary[]> {
	return apiFetch<SSHKeySummary[]>("/api/ssh-keys")
}

export async function createSSHKey(payload: SSHKeyPayload): Promise<SSHKeySummary> {
	return apiFetch<SSHKeySummary>("/api/ssh-keys", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function deleteSSHKey(id: number): Promise<void> {
	await apiFetch<void>(`/api/ssh-keys/${id}`, {
		method: "DELETE",
	})
}

export async function testSSHKey(id: number, payload: SSHKeyConnectionPayload): Promise<void> {
	await apiFetch<void>(`/api/ssh-keys/${id}/test`, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}