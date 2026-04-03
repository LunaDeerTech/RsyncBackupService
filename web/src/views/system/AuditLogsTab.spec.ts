import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { listAuditLogs } from "../../api/audit"
import { listUsers } from "../../api/users"
import AuditLogsTab from "./AuditLogsTab.vue"

vi.mock("../../api/audit", () => ({
	listAuditLogs: vi.fn(),
}))

vi.mock("../../api/users", () => ({
	listUsers: vi.fn(),
	createUser: vi.fn(),
	resetUserPassword: vi.fn(),
	listInstancePermissions: vi.fn(),
	updateInstancePermissions: vi.fn(),
}))

const adminUser = {
	id: 1,
	username: "admin",
	is_admin: true,
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
}

const viewerUser = {
	id: 2,
	username: "viewer",
	is_admin: false,
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
}

const initialResponse = {
	items: [
		{
			id: 31,
			user_id: 1,
			username: "admin",
			action: "instances.restore",
			resource_type: "restore_records",
			resource_id: 9,
			detail: { status: "success" },
			ip_address: "192.0.2.10",
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		},
	],
	total: 41,
	page: 1,
	page_size: 20,
}

const filteredResponse = {
	items: [
		{
			id: 32,
			user_id: 2,
			username: "viewer",
			action: "instances.restore",
			resource_type: "restore_records",
			resource_id: 10,
			detail: { status: "queued" },
			ip_address: "192.0.2.11",
			created_at: "Wed, 02 Apr 2026 09:00:00 GMT",
		},
	],
	total: 22,
	page: 1,
	page_size: 20,
}

const pageTwoResponse = {
	items: [
		{
			id: 33,
			user_id: 2,
			username: "viewer",
			action: "instances.restore",
			resource_type: "restore_records",
			resource_id: 11,
			detail: { status: "running" },
			ip_address: "192.0.2.12",
			created_at: "Wed, 02 Apr 2026 10:00:00 GMT",
		},
	],
	total: 22,
	page: 2,
	page_size: 20,
}

async function renderTab(): Promise<void> {
	render(AuditLogsTab)

	await waitFor(() => {
		expect(listAuditLogs).toHaveBeenCalledWith({ page: 1, page_size: 20 })
		expect(listUsers).toHaveBeenCalledTimes(1)
		expect(screen.getByText("instances.restore")).toBeInTheDocument()
	})
}

describe("AuditLogsTab", () => {
	beforeEach(() => {
		vi.mocked(listAuditLogs).mockReset()
		vi.mocked(listUsers).mockReset()

		vi.mocked(listAuditLogs).mockResolvedValue(initialResponse)
		vi.mocked(listUsers).mockResolvedValue([adminUser, viewerUser])
	})

	it("renders the filter card and full-width table without the old timeline panel", async () => {
		await renderTab()

		expect(screen.getByRole("heading", { name: "筛选器" })).toBeInTheDocument()
		expect(screen.getByRole("heading", { name: "日志表格" })).toBeInTheDocument()
		expect(screen.queryByText("最近操作时间线")).not.toBeInTheDocument()
		expect(screen.getByText(/#9 · 192\.0\.2\.10/)).toBeInTheDocument()
	})

	it("applies filters and paginates the table", async () => {
		vi.mocked(listAuditLogs).mockReset()
		vi.mocked(listAuditLogs)
			.mockResolvedValueOnce(initialResponse)
			.mockResolvedValueOnce(filteredResponse)
			.mockResolvedValueOnce(pageTwoResponse)

		await renderTab()

		await fireEvent.update(screen.getByLabelText("动作"), "instances.restore")
		await fireEvent.update(screen.getByLabelText("资源类型"), "restore_records")
		await fireEvent.click(screen.getByRole("combobox", { name: "用户" }))
		await fireEvent.click(screen.getByRole("option", { name: "viewer" }))
		await fireEvent.click(screen.getByRole("button", { name: "应用筛选" }))

		await waitFor(() => {
			expect(listAuditLogs).toHaveBeenNthCalledWith(2, {
				action: "instances.restore",
				resource_type: "restore_records",
				user_id: 2,
				start_time: undefined,
				end_time: undefined,
				page: 1,
				page_size: 20,
			})
		})

		await fireEvent.click(screen.getByRole("button", { name: "下一页" }))

		await waitFor(() => {
			expect(listAuditLogs).toHaveBeenNthCalledWith(3, {
				action: "instances.restore",
				resource_type: "restore_records",
				user_id: 2,
				start_time: undefined,
				end_time: undefined,
				page: 2,
				page_size: 20,
			})
		})

		expect(screen.getByText(/#11 · 192\.0\.2\.12/)).toBeInTheDocument()
	})
})