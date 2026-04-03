import { render, screen, waitFor, within } from "@testing-library/vue"

import { listBackups } from "../../api/backups"
import { listRunningTasks } from "../../api/system"
import OverviewTab from "./OverviewTab.vue"

vi.mock("../../api/backups", () => ({
	listBackups: vi.fn(),
}))

vi.mock("../../api/system", () => ({
	listRunningTasks: vi.fn(),
}))

const instance = {
	id: 1,
	name: "web-01",
	source_type: "remote",
	source_host: "192.0.2.10",
	source_port: 22,
	source_user: "backup",
	source_ssh_key_id: 2,
	source_path: "/srv/www",
	exclude_patterns: ["node_modules", "tmp"],
	enabled: true,
	created_by: 1,
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
} as const

const strategies = [
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
	{
		id: 8,
		instance_id: 1,
		name: "每周冷备",
		backup_type: "cold",
		cron_expr: null,
		interval_seconds: 86400,
		retention_days: 30,
		retention_count: 2,
		cold_volume_size: "1G",
		max_execution_seconds: 7200,
		storage_target_ids: [5],
		enabled: false,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
	},
]

describe("OverviewTab", () => {
	beforeEach(() => {
		vi.mocked(listBackups).mockReset()
		vi.mocked(listRunningTasks).mockReset()
	})

	it("renders KPI cards, filtered running tasks, and recent activity from live data", async () => {
		vi.mocked(listBackups).mockResolvedValue([
			{
				id: 101,
				instance_id: 1,
				storage_target_id: 4,
				strategy_id: 7,
				backup_type: "rolling",
				status: "success",
				snapshot_path: "/backup/web-01/2026-04-02T08-00-00",
				bytes_transferred: 1024,
				files_transferred: 12,
				total_size: 2048,
				volume_count: 1,
				started_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				finished_at: "Wed, 02 Apr 2026 08:01:00 GMT",
				error_message: "",
			},
			{
				id: 102,
				instance_id: 1,
				storage_target_id: 5,
				strategy_id: null,
				backup_type: "cold",
				status: "failed",
				snapshot_path: "/backup/web-01/2026-04-01T08-00-00.tar",
				bytes_transferred: 0,
				files_transferred: 0,
				total_size: 1024,
				volume_count: 1,
				started_at: "Tue, 01 Apr 2026 08:00:00 GMT",
				finished_at: "Tue, 01 Apr 2026 08:02:00 GMT",
				error_message: "SSH timeout",
			},
		])
		vi.mocked(listRunningTasks).mockResolvedValue([
			{
				task_id: "task-1",
				instance_id: 1,
				storage_target_id: 4,
				started_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				percentage: 62,
				speed_text: "12 MB/s",
				remaining_text: "约 3 分钟",
				status: "running",
			},
			{
				task_id: "task-2",
				instance_id: 9,
				storage_target_id: 8,
				started_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				percentage: 24,
				speed_text: "8 MB/s",
				remaining_text: "约 12 分钟",
				status: "running",
			},
		])

		render(OverviewTab, {
			props: {
				instance,
				strategies,
				canViewRunningTasks: true,
				relayMode: true,
				relayModeHint: "需要预留缓存空间。",
				relayModeTitle: "中继模式",
			},
		})

		await waitFor(() => {
			expect(within(screen.getByText("备份总数").closest("section") as HTMLElement).getByText("2")).toBeInTheDocument()
			expect(screen.getByText("任务 task-1")).toBeInTheDocument()
		})

		expect(within(screen.getByText("活跃策略").closest("section") as HTMLElement).getByText("1")).toBeInTheDocument()
		expect(within(screen.getByText("备份总数").closest("section") as HTMLElement).getByText("2")).toBeInTheDocument()
		expect(within(screen.getByText("累计容量").closest("section") as HTMLElement).getByText("3.00 KB")).toBeInTheDocument()
		expect(within(screen.getByText("成功率").closest("section") as HTMLElement).getByText("50%")).toBeInTheDocument()
		expect(screen.getByText("任务 task-1")).toBeInTheDocument()
		expect(screen.queryByText("任务 task-2")).not.toBeInTheDocument()
		expect(screen.getByText(/失败：SSH timeout/)).toBeInTheDocument()
		expect(screen.getByText("滚动备份 · 每日增量 → 目标 #4")).toBeInTheDocument()
		expect(screen.getByText("中继模式")).toBeInTheDocument()
	})

	it("shows empty states when there are no running tasks or backup records", async () => {
		vi.mocked(listBackups).mockResolvedValue([])
		vi.mocked(listRunningTasks).mockResolvedValue([])

		render(OverviewTab, {
			props: {
				instance,
				strategies: [],
				canViewRunningTasks: true,
				relayMode: false,
			},
		})

		await waitFor(() => {
			expect(screen.getByText("没有运行中任务")).toBeInTheDocument()
			expect(screen.getByText("暂无备份记录")).toBeInTheDocument()
		})
	})

	it("does not call the running-task API when the current user cannot view it", async () => {
		vi.mocked(listBackups).mockResolvedValue([])
		vi.mocked(listRunningTasks).mockResolvedValue([])

		render(OverviewTab, {
			props: {
				instance,
				strategies: [],
				canViewRunningTasks: false,
				relayMode: false,
			},
		})

		await waitFor(() => {
			expect(screen.getByText("无法显示运行中任务")).toBeInTheDocument()
			expect(screen.getByText("当前账户没有实时任务查看权限。")).toBeInTheDocument()
		})

		expect(listRunningTasks).not.toHaveBeenCalled()
	})
})