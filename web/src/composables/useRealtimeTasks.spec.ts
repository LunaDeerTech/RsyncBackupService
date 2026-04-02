import { useAuthStore } from "../stores/auth"
import { listRunningTasks } from "../api/system"
import { useRealtimeTasks } from "./useRealtimeTasks"

vi.mock("../api/system", () => ({
	getDashboard: vi.fn(),
	getSystemStatus: vi.fn(),
	listRunningTasks: vi.fn(),
	cancelTask: vi.fn(),
}))

class MockWebSocket {
	static instances: MockWebSocket[] = []

	url: string
	readyState = 0
	onopen: ((event: Event) => void) | null = null
	onmessage: ((event: MessageEvent<string>) => void) | null = null
	onclose: ((event: CloseEvent) => void) | null = null

	constructor(url: string) {
		this.url = url
		MockWebSocket.instances.push(this)
	}

	close(): void {
		this.readyState = 3
	}

	emitOpen(): void {
		this.readyState = 1
		this.onopen?.(new Event("open"))
	}

	emitMessage(payload: unknown): void {
		this.onmessage?.(
			new MessageEvent("message", {
				data: JSON.stringify(payload),
			}),
		)
	}

	reset(): void {
		MockWebSocket.instances = []
	}
}

describe("useRealtimeTasks", () => {
	const originalWebSocket = globalThis.WebSocket

	beforeEach(() => {
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		vi.mocked(listRunningTasks).mockReset()
		vi.mocked(listRunningTasks).mockResolvedValue([
			{
				task_id: "task-1",
				instance_id: 1,
				storage_target_id: 2,
				started_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				percentage: 20,
				speed_text: "12 MB/s",
				remaining_text: "2m10s",
				status: "running",
			},
		])
		MockWebSocket.instances = []
		globalThis.WebSocket = MockWebSocket as unknown as typeof WebSocket
	})

	afterEach(() => {
		globalThis.WebSocket = originalWebSocket
	})

	it("loads running tasks and updates them from websocket progress events", async () => {
		const realtime = useRealtimeTasks()

		await realtime.connect()

		expect(listRunningTasks).toHaveBeenCalledTimes(1)
		expect(realtime.tasks.value).toHaveLength(1)
		expect(MockWebSocket.instances[0]?.url).toContain("/api/ws/progress?access_token=access-token")

		MockWebSocket.instances[0]?.emitOpen()
		MockWebSocket.instances[0]?.emitMessage({
			task_id: "task-1",
			instance_id: 1,
			percentage: 68,
			speed_text: "18 MB/s",
			remaining_text: "51s",
			status: "running",
		})

		expect(realtime.tasks.value[0]).toMatchObject({
			taskId: "task-1",
			percentage: 68,
			speedText: "18 MB/s",
			etaText: "51s",
		})

		realtime.disconnect()
		expect(MockWebSocket.instances[0]?.readyState).toBe(3)
	})
})