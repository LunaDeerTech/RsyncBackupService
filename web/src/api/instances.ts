import { ApiError, apiFetch } from "./client"
import { listBackups } from "./backups"
import { listStorageTargets } from "./storageTargets"
import { listStrategies } from "./strategies"
import type {
	BackupRecord,
	CreateInstancePayload,
	InstanceDetail,
	InstanceSummary,
	StorageTargetSummary,
	StrategySummary,
	UpdateInstancePayload,
} from "./types"

type RawInstance = InstanceDetail

function sortBackups(records: BackupRecord[]): BackupRecord[] {
	return [...records].sort((left, right) => Date.parse(right.started_at) - Date.parse(left.started_at))
}

function isRemoteStorageTarget(target: StorageTargetSummary | undefined): boolean {
	if (!target) {
		return false
	}

	return target.type === "rolling_ssh" || target.type === "cold_ssh"
}

function isRelayMode(
	instance: RawInstance,
	strategies: StrategySummary[],
	storageTargets: Map<number, StorageTargetSummary>,
): boolean {
	if (instance.source_type !== "remote") {
		return false
	}

	return strategies.some((strategy) =>
		strategy.storage_target_ids.some((storageTargetId) => isRemoteStorageTarget(storageTargets.get(storageTargetId))),
	)
}

function isRelayModeUncertain(
	instance: RawInstance,
	strategies: StrategySummary[],
	storageTargetsRestricted: boolean,
): boolean {
	if (instance.source_type !== "remote" || !storageTargetsRestricted) {
		return false
	}

	return strategies.some((strategy) => strategy.storage_target_ids.length > 0)
}

async function listRawInstances(): Promise<RawInstance[]> {
	return apiFetch<RawInstance[]>("/api/instances")
}

export async function listInstances(): Promise<InstanceSummary[]> {
	const instances = await listRawInstances()

	if (instances.length === 0) {
		return []
	}

	const storageTargetsPromise = listStorageTargets()
		.then((items) => ({ items, restricted: false }))
		.catch((error: unknown) => {
			if (error instanceof ApiError && error.status === 403) {
				return { items: [], restricted: true }
			}

			throw error
		})

	const [storageTargetResult, strategyGroups, backupGroups] = await Promise.all([
		storageTargetsPromise,
		Promise.all(instances.map((instance) => listStrategies(instance.id))),
		Promise.all(instances.map((instance) => listBackups(instance.id))),
	])

	const storageTargets = storageTargetResult.items
	const storageTargetMap = new Map(storageTargets.map((target) => [target.id, target]))

	return instances.map((instance, index) => {
		const strategies = strategyGroups[index] ?? []
		const lastBackup = sortBackups(backupGroups[index] ?? [])[0]

		return {
			...instance,
			strategy_count: strategies.length,
			last_backup_status: lastBackup?.status,
			last_backup_at: lastBackup?.started_at ?? null,
			relay_mode: isRelayMode(instance, strategies, storageTargetMap),
			relay_mode_uncertain: isRelayModeUncertain(instance, strategies, storageTargetResult.restricted),
		}
	})
}

export async function getInstanceDetail(id: number): Promise<InstanceDetail> {
	return apiFetch<InstanceDetail>(`/api/instances/${id}`)
}

export async function createInstance(payload: CreateInstancePayload): Promise<InstanceDetail> {
	return apiFetch<InstanceDetail>("/api/instances", {
		method: "POST",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function updateInstance(id: number, payload: UpdateInstancePayload): Promise<InstanceDetail> {
	return apiFetch<InstanceDetail>(`/api/instances/${id}`, {
		method: "PUT",
		headers: {
			"Content-Type": "application/json",
		},
		body: JSON.stringify(payload),
	})
}

export async function deleteInstance(id: number, verifyToken: string): Promise<void> {
	await apiFetch<void>(`/api/instances/${id}`, {
		method: "DELETE",
		headers: {
			"X-Verify-Token": verifyToken,
		},
	})
}