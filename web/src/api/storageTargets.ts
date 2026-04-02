import { apiFetch } from "./client"
import type { StorageTargetPayload, StorageTargetSummary } from "./types"

export async function listStorageTargets(): Promise<StorageTargetSummary[]> {
	return apiFetch<StorageTargetSummary[]>("/api/storage-targets")
}

export async function createStorageTarget(payload: StorageTargetPayload): Promise<StorageTargetSummary> {
	return apiFetch<StorageTargetSummary>("/api/storage-targets", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function updateStorageTarget(id: number, payload: StorageTargetPayload): Promise<StorageTargetSummary> {
	return apiFetch<StorageTargetSummary>(`/api/storage-targets/${id}`, {
		method: "PUT",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function deleteStorageTarget(id: number): Promise<void> {
	await apiFetch<void>(`/api/storage-targets/${id}`, {
		method: "DELETE",
	})
}

export async function testStorageTarget(id: number): Promise<void> {
	await apiFetch<void>(`/api/storage-targets/${id}/test`, {
		method: "POST",
	})
}