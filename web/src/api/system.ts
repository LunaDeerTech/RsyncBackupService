import { apiFetch } from "./client"
import type { DashboardSummary, RunningTaskStatus, SystemStatus } from "./types"

export async function getSystemStatus(): Promise<SystemStatus> {
	return apiFetch<SystemStatus>("/api/system/status")
}

export async function getDashboard(): Promise<DashboardSummary> {
	return apiFetch<DashboardSummary>("/api/system/dashboard")
}

export async function listRunningTasks(): Promise<RunningTaskStatus[]> {
	return apiFetch<RunningTaskStatus[]>("/api/tasks/running")
}

export async function cancelTask(taskId: string): Promise<void> {
	await apiFetch<void>(`/api/tasks/${taskId}/cancel`, {
		method: "POST",
	})
}