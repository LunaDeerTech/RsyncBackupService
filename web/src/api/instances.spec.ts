import { ApiError } from "./client"
import { listBackups } from "./backups"
import { listInstances } from "./instances"
import { listStorageTargets } from "./storageTargets"
import { listStrategies } from "./strategies"

vi.mock("./backups", () => ({
	listBackups: vi.fn(),
}))

vi.mock("./storageTargets", () => ({
	listStorageTargets: vi.fn(),
}))

vi.mock("./strategies", () => ({
	listStrategies: vi.fn(),
}))

describe("listInstances", () => {
	const originalFetch = globalThis.fetch

	beforeEach(() => {
		vi.mocked(listStorageTargets).mockReset()
		vi.mocked(listStrategies).mockReset()
		vi.mocked(listBackups).mockReset()

		globalThis.fetch = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			headers: new Headers({
				"content-type": "application/json",
			}),
			json: async () => [
				{
					id: 1,
					name: "web-01",
					source_type: "remote",
					source_host: "192.0.2.10",
					source_port: 22,
					source_user: "backup",
					source_ssh_key_id: 2,
					source_path: "/srv/www",
					exclude_patterns: [],
					enabled: true,
					created_by: 1,
					created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
					updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				},
			],
		}) as typeof fetch
	})

	afterEach(() => {
		globalThis.fetch = originalFetch
	})

	it("marks relay mode as uncertain when storage targets are admin-only", async () => {
		vi.mocked(listStorageTargets).mockRejectedValue(new ApiError("forbidden", 403))
		vi.mocked(listStrategies).mockResolvedValue([
			{
				id: 7,
				instance_id: 1,
				name: "daily",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 0,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			},
		])
		vi.mocked(listBackups).mockResolvedValue([])

		await expect(listInstances()).resolves.toMatchObject([
			{
				id: 1,
				relay_mode: false,
				relay_mode_uncertain: true,
				strategy_count: 1,
			},
		])
	})

	it("surfaces strategy fan-out failures instead of silently reporting zero summaries", async () => {
		vi.mocked(listStorageTargets).mockResolvedValue([])
		vi.mocked(listStrategies).mockRejectedValue(new Error("strategy failed"))
		vi.mocked(listBackups).mockResolvedValue([])

		await expect(listInstances()).rejects.toThrow("strategy failed")
	})
})