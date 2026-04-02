import { getCurrentInstance, onBeforeUnmount, ref, type Ref } from "vue"

import { ApiError } from "../api/client"
import { listRunningTasks } from "../api/system"
import type { ProgressEvent, RunningTaskStatus } from "../api/types"
import { useAuthStore } from "../stores/auth"

export interface RunningTaskViewModel {
	taskId: string
	instanceId: number
	storageTargetId?: number
	startedAt?: string
	percentage: number
	speedText: string
	etaText: string
	status: string
}

function mapRunningTask(task: RunningTaskStatus): RunningTaskViewModel {
	return {
		taskId: task.task_id,
		instanceId: task.instance_id,
		storageTargetId: task.storage_target_id,
		startedAt: task.started_at,
		percentage: task.percentage,
		speedText: task.speed_text,
		etaText: task.remaining_text,
		status: task.status,
	}
}

function mapProgressEvent(event: ProgressEvent, current?: RunningTaskViewModel): RunningTaskViewModel {
	return {
		taskId: event.task_id,
		instanceId: event.instance_id,
		storageTargetId: current?.storageTargetId,
		startedAt: current?.startedAt,
		percentage: event.percentage,
		speedText: event.speed_text,
		etaText: event.remaining_text,
		status: event.status,
	}
}

function isTerminalStatus(status: string): boolean {
	return status === "success" || status === "failed" || status === "cancelled"
}

function sortTasks(tasks: RunningTaskViewModel[]): RunningTaskViewModel[] {
	return [...tasks].sort((left, right) => {
		const leftTime = left.startedAt ? Date.parse(left.startedAt) : 0
		const rightTime = right.startedAt ? Date.parse(right.startedAt) : 0
		return rightTime - leftTime
	})
}

function resolveRealtimeUrl(accessToken: string): string {
	const apiBaseUrl = import.meta.env.VITE_API_BASE_URL?.trim()
	const baseUrl = apiBaseUrl === "" || apiBaseUrl === undefined ? window.location.origin : apiBaseUrl
	const endpoint = new URL("/api/ws/progress", baseUrl)

	endpoint.protocol = endpoint.protocol === "https:" ? "wss:" : "ws:"
	endpoint.searchParams.set("access_token", accessToken)

	return endpoint.toString()
}

export function useRealtimeTasks(): {
	tasks: Ref<RunningTaskViewModel[]>
	connect(): void
	disconnect(): void
} {
	const tasks = ref<RunningTaskViewModel[]>([])
	let socket: WebSocket | null = null
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null
	let manualDisconnect = false

	function clearReconnectTimer(): void {
		if (reconnectTimer !== null) {
			clearTimeout(reconnectTimer)
			reconnectTimer = null
		}
	}

	function upsertTask(nextTask: RunningTaskViewModel): void {
		const nextTasks = tasks.value.filter((task) => task.taskId !== nextTask.taskId)

		if (!isTerminalStatus(nextTask.status)) {
			nextTasks.push(nextTask)
		}

		tasks.value = sortTasks(nextTasks)
	}

	async function primeTasks(): Promise<void> {
		try {
			const runningTasks = await listRunningTasks()
			tasks.value = sortTasks(runningTasks.map(mapRunningTask))
		} catch (error) {
			if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
				tasks.value = []
				return
			}

			throw error
		}
	}

	function scheduleReconnect(): void {
		if (manualDisconnect || reconnectTimer !== null) {
			return
		}

		reconnectTimer = setTimeout(() => {
			reconnectTimer = null
			openSocket()
		}, 1000)
	}

	function openSocket(): void {
		const auth = useAuthStore()
		if (socket !== null || auth.accessToken === null) {
			return
		}

		socket = new WebSocket(resolveRealtimeUrl(auth.accessToken))

		socket.onopen = () => {
			void primeTasks().catch(() => {
				tasks.value = []
			})
		}

		socket.onmessage = (messageEvent) => {
			try {
				const payload = JSON.parse(String(messageEvent.data)) as ProgressEvent
				const current = tasks.value.find((task) => task.taskId === payload.task_id)
				upsertTask(mapProgressEvent(payload, current))
			} catch {
				// Ignore malformed events and keep the current task list stable.
			}
		}

		socket.onclose = () => {
			socket = null
			scheduleReconnect()
		}

		socket.onerror = () => {
			socket?.close()
		}
	}

	function connect(): void {
		manualDisconnect = false
		clearReconnectTimer()
		void primeTasks().catch(() => {
			tasks.value = []
		})
		openSocket()
	}

	function disconnect(): void {
		manualDisconnect = true
		clearReconnectTimer()

		if (socket !== null) {
			const activeSocket = socket
			socket = null
			activeSocket.close()
		}
	}

	if (getCurrentInstance()) {
		onBeforeUnmount(() => {
			disconnect()
		})
	}

	return {
		tasks,
		connect,
		disconnect,
	}
}