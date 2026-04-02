import { apiFetch } from "./client"
import type { StrategyPayload, StrategySummary } from "./types"

export async function listStrategies(instanceId: number): Promise<StrategySummary[]> {
	return apiFetch<StrategySummary[]>(`/api/instances/${instanceId}/strategies`)
}

export async function createStrategy(instanceId: number, payload: StrategyPayload): Promise<StrategySummary> {
	return apiFetch<StrategySummary>(`/api/instances/${instanceId}/strategies`, {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function updateStrategy(strategyId: number, payload: StrategyPayload): Promise<StrategySummary> {
	return apiFetch<StrategySummary>(`/api/strategies/${strategyId}`, {
		method: "PUT",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function deleteStrategy(strategyId: number): Promise<void> {
	await apiFetch<void>(`/api/strategies/${strategyId}`, {
		method: "DELETE",
	})
}