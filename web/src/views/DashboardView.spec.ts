import { render, screen, waitFor } from "@testing-library/vue"
import { ref } from "vue"

import { getDashboard, getSystemStatus } from "../api/system"
import { useRealtimeTasks } from "../composables/useRealtimeTasks"
import DashboardView from "./DashboardView.vue"

vi.mock("../api/system", () => ({
	cancelTask: vi.fn(),
	getDashboard: vi.fn(),
	getSystemStatus: vi.fn(),
}))

vi.mock("../composables/useRealtimeTasks", () => ({
	useRealtimeTasks: vi.fn(),
}))

describe("DashboardView", () => {
	beforeEach(() => {
		vi.mocked(getDashboard).mockResolvedValue({
			instance_count: 4,
			today_backup_count: 2,
			success_count: 7,
			failed_count: 1,
			running_tasks: [],
			recent_backups: [],
			storage_overview: [],
		})
		vi.mocked(getSystemStatus).mockResolvedValue({
			version: "0.1.0",
			data_dir: "/var/lib/rsync-backup",
			uptime_seconds: 3600,
			disk_total_bytes: 1024,
			disk_free_bytes: 512,
		})
		vi.mocked(useRealtimeTasks).mockReturnValue({
			tasks: ref([]),
			connect: vi.fn(),
			disconnect: vi.fn(),
		})
	})

	it("renders the dashboard header as module label, title, and task-oriented subtitle", async () => {
		const { container } = render(DashboardView)

		await waitFor(() => {
			expect(getDashboard).toHaveBeenCalledTimes(1)
			expect(getSystemStatus).toHaveBeenCalledTimes(1)
		})

		const header = container.querySelector(".page-header.page-header--inset.page-header--shell-aligned")
		const refreshButton = screen.getByRole("button", { name: "刷新概览" })

		expect(header).not.toBeNull()
		expect(refreshButton.closest(".page-header__actions")).not.toBeNull()
		expect(screen.getByText("DASHBOARD")).toBeInTheDocument()
		expect(screen.getByRole("heading", { name: "系统概览" })).toBeInTheDocument()
		expect(screen.getByText("监控备份健康、运行任务与容量风险。")).toBeInTheDocument()
	})
})