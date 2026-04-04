import { render, screen, waitFor } from "@testing-library/vue"

import { listBackups, listSnapshots } from "../../api/backups"
import { listStrategies } from "../../api/strategies"
import BackupsTab from "./BackupsTab.vue"

vi.mock("../../api/backups", () => ({
	listBackups: vi.fn(),
	listSnapshots: vi.fn(),
}))

vi.mock("../../api/strategies", () => ({
	listStrategies: vi.fn(),
}))

describe("BackupsTab", () => {
	beforeEach(() => {
		vi.mocked(listBackups).mockReset()
		vi.mocked(listSnapshots).mockReset()
		vi.mocked(listStrategies).mockReset()

		vi.mocked(listSnapshots).mockResolvedValue([
			{
				id: 12,
				instance_id: 1,
				storage_target_id: 4,
				strategy_id: 7,
				backup_type: "rolling",
				status: "success",
				snapshot_path: "/backup/web-01/2026-04-02T08-00-00",
				bytes_transferred: 0,
				files_transferred: 12,
				total_size: 2048,
				volume_count: 1,
				started_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				finished_at: "Wed, 02 Apr 2026 08:01:00 GMT",
				error_message: "",
			},
		])
		vi.mocked(listStrategies).mockResolvedValue([
			{
				id: 7,
				instance_id: 1,
				name: "每日增量",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
			},
		])
	})

	it("loads only available backups through the snapshots API", async () => {
		render(BackupsTab, {
			props: {
				instanceId: 1,
			},
		})

		await waitFor(() => {
			expect(listSnapshots).toHaveBeenCalledWith(1, {
				backup_type: undefined,
				strategy_id: undefined,
			})
			expect(listStrategies).toHaveBeenCalledWith(1)
			expect(screen.getByText("/backup/web-01/2026-04-02T08-00-00")).toBeInTheDocument()
		})

		expect(listBackups).not.toHaveBeenCalled()
		expect(screen.queryByLabelText("状态")).not.toBeInTheDocument()
		expect(screen.getByText("2.00 KB")).toBeInTheDocument()
		expect(screen.getByText("仅显示当前仍存在的快照与归档；历史执行记录不在此处展示。"))
			.toBeInTheDocument()
	})
})