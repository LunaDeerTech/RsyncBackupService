import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { verifyPassword } from "../../api/auth"
import { listRestoreRecords, listSnapshots, startRestore } from "../../api/backups"
import RestoreTab from "./RestoreTab.vue"

vi.mock("../../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

vi.mock("../../api/backups", () => ({
	listBackups: vi.fn(),
	listSnapshots: vi.fn(),
	listRestoreRecords: vi.fn(),
	startRestore: vi.fn(),
}))

describe("RestoreTab", () => {
	beforeEach(() => {
		vi.mocked(listSnapshots).mockReset()
		vi.mocked(listRestoreRecords).mockReset()
		vi.mocked(startRestore).mockReset()
		vi.mocked(verifyPassword).mockReset()
		vi.mocked(listSnapshots).mockResolvedValue([
			{
				id: 12,
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
		])
		vi.mocked(listRestoreRecords).mockResolvedValue([])
		vi.mocked(verifyPassword).mockResolvedValue({ verify_token: "verify-token" })
		vi.mocked(startRestore).mockResolvedValue({
			id: 99,
			instance_id: 1,
			backup_record_id: 12,
			restore_target_path: "/srv/www",
			overwrite: true,
			status: "running",
			started_at: "Wed, 02 Apr 2026 09:00:00 GMT",
			finished_at: null,
			error_message: "",
			triggered_by: 1,
		})
	})

	it("requires password confirmation before restore submit", async () => {
		render(RestoreTab, {
			props: {
				instanceId: 1,
				instance: {
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
			},
		})

		await waitFor(() => {
			expect(listSnapshots).toHaveBeenCalledWith(1)
			expect(screen.getAllByText(/存储目标 #4/).length).toBeGreaterThan(0)
		})

		await fireEvent.click(screen.getByRole("button", { name: "开始恢复" }))

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "确认恢复" })).toBeInTheDocument()
			expect(screen.getByText(/二次认证/)).toBeInTheDocument()
			expect(screen.getAllByText(/存储目标 #4/).length).toBeGreaterThan(0)
		})
	})
})