import { apiFetch } from "./client"
import type { BackupRecord, RestorePayload, RestoreRecord } from "./types"

export interface ListBackupsOptions {
	backup_type?: string
	status?: string
	strategy_id?: number
}

function buildQuery(options: ListBackupsOptions = {}): string {
	const query = new URLSearchParams()

	for (const [key, value] of Object.entries(options)) {
		if (value === undefined || value === null || value === "") {
			continue
		}
		query.set(key, String(value))
	}

	const encoded = query.toString()
	return encoded === "" ? "" : `?${encoded}`
}

export async function listBackups(instanceId: number, options: ListBackupsOptions = {}): Promise<BackupRecord[]> {
	return apiFetch<BackupRecord[]>(`/api/instances/${instanceId}/backups${buildQuery(options)}`)
}

export async function listSnapshots(instanceId: number, options: ListBackupsOptions = {}): Promise<BackupRecord[]> {
	return apiFetch<BackupRecord[]>(`/api/instances/${instanceId}/snapshots${buildQuery(options)}`)
}

export async function startRestore(payload: RestorePayload): Promise<RestoreRecord> {
	return apiFetch<RestoreRecord>(`/api/instances/${payload.instance_id}/restore`, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
			"X-Verify-Token": payload.verify_token,
		},
		body: JSON.stringify({
			backup_record_id: payload.backup_record_id,
			restore_target_path: payload.restore_target_path,
			overwrite: payload.overwrite,
		}),
	})
}

export async function listRestoreRecords(instanceId?: number): Promise<RestoreRecord[]> {
	const query = typeof instanceId === "number" ? `?instance_id=${instanceId}` : ""
	return apiFetch<RestoreRecord[]>(`/api/restore-records${query}`)
}