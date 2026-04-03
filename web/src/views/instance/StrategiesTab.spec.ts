import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { createStrategy, deleteStrategy, listStrategies, updateStrategy } from "../../api/strategies"
import { listStorageTargets } from "../../api/storageTargets"
import StrategiesTab from "./StrategiesTab.vue"

vi.mock("../../api/strategies", () => ({
	listStrategies: vi.fn(),
	createStrategy: vi.fn(),
	updateStrategy: vi.fn(),
	deleteStrategy: vi.fn(),
}))

vi.mock("../../api/storageTargets", () => ({
	listStorageTargets: vi.fn(),
}))

const strategy = {
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
}

async function renderTab(): Promise<void> {
	render(StrategiesTab, {
		props: {
			instanceId: 1,
		},
	})

	await waitFor(() => {
		expect(listStrategies).toHaveBeenCalledWith(1)
		expect(listStorageTargets).toHaveBeenCalledTimes(1)
		expect(screen.getByText("每日增量")).toBeInTheDocument()
	})
}

describe("StrategiesTab", () => {
	beforeEach(() => {
		vi.mocked(listStrategies).mockReset()
		vi.mocked(createStrategy).mockReset()
		vi.mocked(updateStrategy).mockReset()
		vi.mocked(deleteStrategy).mockReset()
		vi.mocked(listStorageTargets).mockReset()

		vi.mocked(listStrategies).mockResolvedValue([strategy])
		vi.mocked(listStorageTargets).mockResolvedValue([
			{
				id: 4,
				name: "archive-primary",
				type: "rolling_ssh",
				host: "192.0.2.20",
				port: 22,
				user: "backup",
				ssh_key_id: 9,
				base_path: "/srv/archive",
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 09:00:00 GMT",
			},
		])
	})

	it("uses modal create and edit flows instead of an inline form", async () => {
		await renderTab()

		expect(screen.queryByText("新建 / 编辑策略")).not.toBeInTheDocument()
		expect(screen.queryByRole("dialog", { name: "新建策略" })).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "新建策略" }))

		expect(screen.getByRole("dialog", { name: "新建策略" })).toBeInTheDocument()
		expect(screen.getByLabelText("策略名称")).toHaveValue("")

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建策略" })).not.toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "编辑" }))

		expect(screen.getByRole("dialog", { name: "编辑策略" })).toBeInTheDocument()
		expect(screen.getByLabelText("策略名称")).toHaveValue("每日增量")
	})

	it("requires delete confirmation before removing a strategy", async () => {
		vi.mocked(listStrategies).mockReset()
		vi.mocked(listStrategies).mockResolvedValueOnce([strategy])
		vi.mocked(listStrategies).mockResolvedValueOnce([])
		vi.mocked(deleteStrategy).mockResolvedValue()

		await renderTab()

		await fireEvent.click(screen.getByRole("button", { name: "删除" }))

		expect(screen.getByRole("dialog", { name: "确认删除策略" })).toBeInTheDocument()
		expect(screen.getByText("即将删除策略「每日增量」。若该策略已经产生备份记录，系统会拒绝删除。")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认删除策略" })).not.toBeInTheDocument()
		})
		expect(deleteStrategy).not.toHaveBeenCalled()

		await fireEvent.click(screen.getByRole("button", { name: "删除" }))
		await fireEvent.click(screen.getByRole("button", { name: "确认删除" }))

		await waitFor(() => {
			expect(deleteStrategy).toHaveBeenCalledWith(7)
		})
	})
})