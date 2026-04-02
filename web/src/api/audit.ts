import { apiFetch } from "./client"
import type { AuditLogListResponse, AuditLogQuery } from "./types"

export async function listAuditLogs(query: AuditLogQuery = {}): Promise<AuditLogListResponse> {
	const params = new URLSearchParams()

	for (const [key, value] of Object.entries(query)) {
		if (value === undefined || value === null || value === "") {
			continue
		}
		params.set(key, String(value))
	}

	const suffix = params.toString() === "" ? "" : `?${params.toString()}`
	return apiFetch<AuditLogListResponse>(`/api/audit-logs${suffix}`)
}