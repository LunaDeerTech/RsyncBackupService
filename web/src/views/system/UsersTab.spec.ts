import { fireEvent, render, screen, waitFor, within } from "@testing-library/vue"

import { verifyPassword } from "../../api/auth"
import { ApiError } from "../../api/client"
import { listInstances } from "../../api/instances"
import {
	createUser,
	listInstancePermissions,
	listUsers,
	resetUserPassword,
	updateInstancePermissions,
} from "../../api/users"
import UsersTab from "./UsersTab.vue"

vi.mock("../../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

vi.mock("../../api/instances", () => ({
	listInstances: vi.fn(),
	getInstanceDetail: vi.fn(),
	createInstance: vi.fn(),
	updateInstance: vi.fn(),
	deleteInstance: vi.fn(),
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

const opsUser = {
	id: 2,
	username: "ops",
	is_admin: false,
	created_at: "Wed, 02 Apr 2026 08:10:00 GMT",
	updated_at: "Wed, 02 Apr 2026 08:10:00 GMT",
}

const newUser = {
	id: 3,
	username: "new-user",
	is_admin: true,
	created_at: "Wed, 02 Apr 2026 09:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 09:00:00 GMT",
}

const instances = [
	{
		id: 101,
		name: "web-01",
		source_type: "local",
		source_port: 22,
		source_path: "/srv/web",
		exclude_patterns: [],
		enabled: true,
		created_by: 1,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		strategy_count: 1,
		last_backup_status: "success",
		last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
		relay_mode: false,
		relay_mode_uncertain: false,
	},
	{
		id: 202,
		name: "db-01",
		source_type: "remote",
		source_host: "192.0.2.30",
		source_port: 22,
		source_user: "backup",
		source_path: "/srv/db",
		exclude_patterns: [],
		enabled: true,
		created_by: 1,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		strategy_count: 1,
		last_backup_status: "success",
		last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
		relay_mode: false,
		relay_mode_uncertain: false,
	},
]

async function renderTab(): Promise<void> {
	render(UsersTab)

	await waitFor(() => {
		expect(listUsers).toHaveBeenCalledTimes(1)
		expect(listInstances).toHaveBeenCalledTimes(1)
		expect(screen.getByText("ops")).toBeInTheDocument()
	})
}

function getRowByText(value: string): HTMLElement {
	const cell = screen
		.getAllByText(value)
		.find((candidate) => candidate.closest("td")?.getAttribute("data-column-key") === "username")
	expect(cell).toBeDefined()
	const row = cell.closest("tr")
	expect(row).not.toBeNull()
	return row as HTMLElement
}

describe("UsersTab", () => {
	beforeEach(() => {
		vi.mocked(listUsers).mockReset()
		vi.mocked(createUser).mockReset()
		vi.mocked(listInstancePermissions).mockReset()
		vi.mocked(resetUserPassword).mockReset()
		vi.mocked(updateInstancePermissions).mockReset()
		vi.mocked(listInstances).mockReset()
		vi.mocked(verifyPassword).mockReset()

		vi.mocked(listUsers).mockResolvedValue([adminUser, opsUser])
		vi.mocked(listInstances).mockResolvedValue(instances)
		vi.mocked(createUser).mockResolvedValue(newUser)
		vi.mocked(verifyPassword).mockResolvedValue({ verify_token: "verify-token" })
		vi.mocked(resetUserPassword).mockResolvedValue()
		vi.mocked(updateInstancePermissions).mockResolvedValue()
		vi.mocked(listInstancePermissions).mockImplementation(async (instanceId) => {
			if (instanceId === 101) {
				return [{ user_id: 1, username: "admin", instance_id: 101, role: "admin" }]
			}

			if (instanceId === 202) {
				return []
			}

			return []
		})
	})

	it("opens the create modal and creates a new administrator user", async () => {
		vi.mocked(listUsers).mockReset()
		vi.mocked(listUsers)
			.mockResolvedValueOnce([adminUser, opsUser])
			.mockResolvedValueOnce([adminUser, opsUser, newUser])

		await renderTab()

		expect(screen.queryByRole("dialog", { name: "创建用户" })).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "创建用户" }))

		const dialog = screen.getByRole("dialog", { name: "创建用户" })
		expect(dialog).toBeInTheDocument()
		await fireEvent.update(screen.getByLabelText("用户名"), "  new-user  ")
		await fireEvent.update(screen.getByLabelText("初始密码"), "pass-123")
		await fireEvent.click(screen.getByRole("switch", { name: "管理员账户" }))
		await fireEvent.click(within(dialog).getByRole("button", { name: "创建用户" }))

		await waitFor(() => {
			expect(createUser).toHaveBeenCalledWith({
				username: "new-user",
				password: "pass-123",
				is_admin: true,
			})
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "创建用户" })).not.toBeInTheDocument()
			expect(screen.getByText("用户已创建。")).toBeInTheDocument()
			expect(screen.getByText("new-user")).toBeInTheDocument()
		})
	})

	it("keeps create-user failures inside the modal", async () => {
		vi.mocked(createUser).mockRejectedValue(new ApiError("用户名已存在", 409))

		await renderTab()

		await fireEvent.click(screen.getByRole("button", { name: "创建用户" }))

		const dialog = screen.getByRole("dialog", { name: "创建用户" })
		await fireEvent.update(screen.getByLabelText("用户名"), "ops")
		await fireEvent.update(screen.getByLabelText("初始密码"), "pass-123")
		await fireEvent.click(within(dialog).getByRole("button", { name: "创建用户" }))

		await waitFor(() => {
			expect(within(dialog).getByRole("alert")).toHaveTextContent("用户名已存在")
		})

		expect(screen.getByRole("dialog", { name: "创建用户" })).toBeInTheDocument()
	})

	it("opens the assign modal and saves per-instance roles", async () => {
		await renderTab()

		await fireEvent.click(within(getRowByText("ops")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(listInstancePermissions).toHaveBeenCalledWith(101)
			expect(listInstancePermissions).toHaveBeenCalledWith(202)
		})

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
		})
		expect(screen.getByText("为用户「ops」选择可访问的备份实例及其角色。")).toBeInTheDocument()

		const webRow = screen.getByText("web-01").closest(".system-admin__perm-row")
		const dbRow = screen.getByText("db-01").closest(".system-admin__perm-row")
		expect(webRow).not.toBeNull()
		expect(dbRow).not.toBeNull()

		await fireEvent.click(within(webRow as HTMLElement).getByRole("combobox"))
		await fireEvent.click(screen.getByRole("option", { name: "viewer" }))
		await fireEvent.click(within(dbRow as HTMLElement).getByRole("combobox"))
		await fireEvent.click(screen.getByRole("option", { name: "admin" }))
		await fireEvent.click(screen.getByRole("button", { name: "保存权限" }))

		await waitFor(() => {
			expect(updateInstancePermissions).toHaveBeenNthCalledWith(1, 101, [{ user_id: 2, role: "viewer" }])
			expect(updateInstancePermissions).toHaveBeenNthCalledWith(2, 202, [{ user_id: 2, role: "admin" }])
		})

		expect(screen.getByText("用户 ops 的实例权限已更新。")).toBeInTheDocument()
	})

	it("keeps permission loading failures out of the assign modal", async () => {
		vi.mocked(listInstancePermissions).mockImplementation(async (instanceId) => {
			if (instanceId === 101) {
				throw new ApiError("实例权限接口超时", 504)
			}

			return []
		})

		await renderTab()

		await fireEvent.click(within(getRowByText("ops")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "分配实例权限" })).not.toBeInTheDocument()
			expect(screen.getByText("实例权限接口超时")).toBeInTheDocument()
		})
	})

	it("shows effective admin access for global administrators even without explicit instance rows", async () => {
		await renderTab()

		await fireEvent.click(within(getRowByText("admin")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
		})

		const dbRow = screen.getByText("db-01").closest(".system-admin__perm-row")
		expect(dbRow).not.toBeNull()
		expect(within(dbRow as HTMLElement).getByRole("combobox")).toHaveTextContent("admin")
	})

	it("shows stale explicit grants for global administrators without hiding effective access", async () => {
		vi.mocked(listInstancePermissions).mockImplementation(async (instanceId) => {
			if (instanceId === 101) {
				return [{ user_id: 1, username: "admin", instance_id: 101, role: "viewer" }]
			}

			return []
		})

		await renderTab()

		await fireEvent.click(within(getRowByText("admin")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
		})

		const webRow = screen.getByText("web-01").closest(".system-admin__perm-row")
		expect(webRow).not.toBeNull()
		expect(within(webRow as HTMLElement).getByRole("combobox")).toHaveTextContent("admin")
		expect(within(webRow as HTMLElement).getByText("已存在显式 viewer 记录，当前生效权限仍为 admin。")).toBeInTheDocument()
	})

	it("treats global administrator assignments as read-only", async () => {
		await renderTab()

		await fireEvent.click(within(getRowByText("admin")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
		})

		const dbRow = screen.getByText("db-01").closest(".system-admin__perm-row")
		expect(dbRow).not.toBeNull()

		expect(within(dbRow as HTMLElement).getByRole("combobox")).toBeDisabled()
		expect(screen.getByRole("button", { name: "保存权限" })).toBeDisabled()
		expect(screen.getAllByText(/全局管理员默认拥有所有实例的 admin 权限，无需在此保存。/).length)
			.toBeGreaterThan(0)
	})

	it("blocks saving a revoke that the current backend API cannot perform", async () => {
		vi.mocked(listInstancePermissions).mockImplementation(async (instanceId) => {
			if (instanceId === 101) {
				return [{ user_id: 2, username: "ops", instance_id: 101, role: "viewer" }]
			}

			return []
		})

		await renderTab()

		await fireEvent.click(within(getRowByText("ops")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
		})

		const webRow = screen.getByText("web-01").closest(".system-admin__perm-row")
		expect(webRow).not.toBeNull()

		await fireEvent.click(within(webRow as HTMLElement).getByRole("combobox"))
		await fireEvent.click(screen.getByRole("option", { name: "无权限" }))
		await fireEvent.click(screen.getByRole("button", { name: "保存权限" }))

		expect(updateInstancePermissions).not.toHaveBeenCalled()
		expect(screen.getByText("当前 API 暂不支持移除已有实例权限，请保留 viewer 或 admin。"))
			.toBeInTheDocument()
	})

	it("reloads current permissions after a partial save failure", async () => {
		const permissionStore: Record<number, Array<{ user_id: number; username: string; instance_id: number; role: string }>> = {
			101: [{ user_id: 2, username: "ops", instance_id: 101, role: "viewer" }],
			202: [],
		}

		vi.mocked(listInstancePermissions).mockImplementation(async (instanceId) => permissionStore[instanceId] ?? [])
		vi.mocked(updateInstancePermissions).mockImplementation(async (instanceId, payload) => {
			if (instanceId === 101) {
				permissionStore[101] = [{
					user_id: payload[0].user_id,
					username: "ops",
					instance_id: 101,
					role: payload[0].role,
				}]
				return
			}

			throw new ApiError("db-01 更新失败", 500)
		})

		await renderTab()

		await fireEvent.click(within(getRowByText("ops")).getByRole("button", { name: "分配实例" }))

		await waitFor(() => {
			expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
		})

		const webRow = screen.getByText("web-01").closest(".system-admin__perm-row")
		const dbRow = screen.getByText("db-01").closest(".system-admin__perm-row")
		expect(webRow).not.toBeNull()
		expect(dbRow).not.toBeNull()

		await fireEvent.click(within(webRow as HTMLElement).getByRole("combobox"))
		await fireEvent.click(screen.getByRole("option", { name: "admin" }))
		await fireEvent.click(within(dbRow as HTMLElement).getByRole("combobox"))
		await fireEvent.click(screen.getByRole("option", { name: "admin" }))
		await fireEvent.click(screen.getByRole("button", { name: "保存权限" }))

		await waitFor(() => {
			expect(updateInstancePermissions).toHaveBeenNthCalledWith(1, 101, [{ user_id: 2, role: "admin" }])
			expect(updateInstancePermissions).toHaveBeenNthCalledWith(2, 202, [{ user_id: 2, role: "admin" }])
		})

		await waitFor(() => {
			expect(screen.getByText("已更新 web-01；db-01 更新失败。当前权限已重新加载，请确认。"))
				.toBeInTheDocument()
		})

		expect(within(webRow as HTMLElement).getByRole("combobox")).toHaveTextContent("admin")
		expect(within(dbRow as HTMLElement).getByRole("combobox")).toHaveTextContent("无权限")
		expect(screen.getByRole("dialog", { name: "分配实例权限" })).toBeInTheDocument()
	})

	it("resets a managed user password through the danger dialog", async () => {
		await renderTab()

		await fireEvent.click(within(getRowByText("ops")).getByRole("button", { name: "重置密码" }))

		expect(screen.getByRole("dialog", { name: "确认重置用户密码" })).toBeInTheDocument()
		await fireEvent.update(screen.getByLabelText("新密码"), "new-secret")
		await fireEvent.update(screen.getByLabelText("当前管理员密码"), "admin-secret")
		await fireEvent.click(screen.getByRole("button", { name: "确认重置" }))

		await waitFor(() => {
			expect(verifyPassword).toHaveBeenCalledWith("admin-secret")
			expect(resetUserPassword).toHaveBeenCalledWith(2, {
				password: "new-secret",
				verify_token: "verify-token",
			})
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认重置用户密码" })).not.toBeInTheDocument()
		})
		expect(screen.getByText("用户 ops 的密码已重置。")).toBeInTheDocument()
	})
})