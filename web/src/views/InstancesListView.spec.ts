import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { createInstance, listInstances, updateInstance } from "../api/instances"
import { listSSHKeys } from "../api/sshKeys"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import InstancesListView from "./InstancesListView.vue"

vi.mock("../api/instances", () => ({
	listInstances: vi.fn(),
	getInstanceDetail: vi.fn(),
	createInstance: vi.fn(),
	updateInstance: vi.fn(),
	deleteInstance: vi.fn(),
}))

vi.mock("../api/sshKeys", () => ({
	listSSHKeys: vi.fn(),
}))

describe("InstancesListView", () => {
	beforeEach(() => {
		history.replaceState(null, "", "/instances")
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		vi.mocked(listInstances).mockReset()
		vi.mocked(createInstance).mockReset()
		vi.mocked(updateInstance).mockReset()
		vi.mocked(listSSHKeys).mockReset()
		vi.mocked(listInstances).mockResolvedValue([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
		])
		vi.mocked(listSSHKeys).mockResolvedValue([
			{
				id: 2,
				name: "default-key",
				fingerprint: "SHA256:test",
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			},
		])
	})

	it("renders instances returned by the API", async () => {
		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(InstancesListView, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(listInstances).toHaveBeenCalledTimes(1)
			expect(screen.getByText("web-01")).toBeInTheDocument()
		})

		expect(screen.getByRole("button", { name: "新建实例" })).toBeInTheDocument()
		expect(screen.getByText(/192\.0\.2\.10:\/srv\/www/)).toBeInTheDocument()
		expect(screen.getByText("2 条策略")).toBeInTheDocument()
		expect(screen.getAllByText("中继模式").length).toBeGreaterThan(0)
	})

	it("opens and closes the create modal from the page header action", async () => {
		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(InstancesListView, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(listInstances).toHaveBeenCalledTimes(1)
		})

		expect(screen.queryByRole("dialog", { name: "新建实例" })).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "新建实例" }))

		const dialog = screen.getByRole("dialog", { name: "新建实例" })

		expect(dialog).toBeInTheDocument()
		expect(screen.getByLabelText("名称")).toBeInTheDocument()
		expect(screen.getByLabelText("源路径")).toBeInTheDocument()
		expect(screen.queryByLabelText("源主机")).not.toBeInTheDocument()
		expect(screen.getByRole("button", { name: "取消" })).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建实例" })).not.toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "新建实例" }))
		await fireEvent.keyDown(screen.getByRole("dialog", { name: "新建实例" }), { key: "Escape" })
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建实例" })).not.toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "新建实例" }))
		const overlay = screen.getByRole("dialog", { name: "新建实例" }).parentElement
		expect(overlay).not.toBeNull()
		await fireEvent.click(overlay as HTMLElement)
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建实例" })).not.toBeInTheDocument()
		})
	})

	it("shows remote fields for create and closes after submit", async () => {
		vi.mocked(createInstance).mockResolvedValue({
			id: 2,
			name: "db-01",
			source_type: "remote",
			source_host: "192.0.2.11",
			source_port: 22,
			source_user: "replica",
			source_ssh_key_id: 2,
			source_path: "/srv/db",
			exclude_patterns: [],
			enabled: true,
			created_by: 1,
			created_at: "Wed, 02 Apr 2026 10:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 10:00:00 GMT",
		})
		vi.mocked(listInstances).mockResolvedValueOnce([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
		])
		vi.mocked(listInstances).mockResolvedValueOnce([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
			{
				id: 2,
				name: "db-01",
				source_type: "remote",
				source_host: "192.0.2.11",
				source_port: 22,
				source_user: "replica",
				source_ssh_key_id: 2,
				source_path: "/srv/db",
				exclude_patterns: [],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 10:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 10:00:00 GMT",
				strategy_count: 0,
				last_backup_status: undefined,
				last_backup_at: null,
				relay_mode: false,
				relay_mode_uncertain: false,
			},
		])

		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(InstancesListView, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(screen.getByText("web-01")).toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "新建实例" }))
		await fireEvent.click(screen.getByRole("combobox", { name: "源类型" }))
		await fireEvent.click(screen.getByRole("option", { name: "远程主机" }))

		expect(screen.getByLabelText("源主机")).toBeInTheDocument()
		expect(screen.getByLabelText("源用户")).toBeInTheDocument()

		await fireEvent.update(screen.getByLabelText("名称"), "db-01")
		await fireEvent.update(screen.getByLabelText("源路径"), "/srv/db")
		await fireEvent.update(screen.getByLabelText("源主机"), "192.0.2.11")
		await fireEvent.update(screen.getByLabelText("源用户"), "replica")
		await fireEvent.click(screen.getByRole("button", { name: "创建实例" }))

		await waitFor(() => {
			expect(createInstance).toHaveBeenCalledWith(
				expect.objectContaining({
					name: "db-01",
					source_type: "remote",
					source_host: "192.0.2.11",
					source_user: "replica",
					source_path: "/srv/db",
				}),
			)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建实例" })).not.toBeInTheDocument()
			expect(screen.getByText("实例已创建。")).toBeInTheDocument()
			expect(screen.getByText("db-01")).toBeInTheDocument()
		})
	})

	it("keeps the create modal open while submitting", async () => {
		let resolveCreate: ((value: Awaited<ReturnType<typeof createInstance>>) => void) | undefined
		const createPromise = new Promise<Awaited<ReturnType<typeof createInstance>>>((resolve) => {
			resolveCreate = resolve
		})

		vi.mocked(createInstance).mockReturnValue(createPromise)
		vi.mocked(listInstances).mockResolvedValueOnce([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
		])
		vi.mocked(listInstances).mockResolvedValueOnce([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
			{
				id: 3,
				name: "logs-01",
				source_type: "local",
				source_port: 22,
				source_path: "/srv/logs",
				exclude_patterns: [],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 10:10:00 GMT",
				updated_at: "Wed, 02 Apr 2026 10:10:00 GMT",
				strategy_count: 0,
				last_backup_status: undefined,
				last_backup_at: null,
				relay_mode: false,
				relay_mode_uncertain: false,
			},
		])

		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(InstancesListView, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(screen.getByText("web-01")).toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "新建实例" }))
		await fireEvent.update(screen.getByLabelText("名称"), "logs-01")
		await fireEvent.update(screen.getByLabelText("源路径"), "/srv/logs")
		await fireEvent.click(screen.getByRole("button", { name: "创建实例" }))

		const submitButton = screen.getByRole("button", { name: "创建实例" })
		const dialog = screen.getByRole("dialog", { name: "新建实例" })
		const overlay = dialog.parentElement

		expect(submitButton).toBeDisabled()
		expect(submitButton).toHaveAttribute("aria-busy", "true")

		await fireEvent.keyDown(dialog, { key: "Escape" })
		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		if (overlay) {
			await fireEvent.click(overlay)
		}

		expect(screen.getByRole("dialog", { name: "新建实例" })).toBeInTheDocument()

		resolveCreate?.({
			id: 3,
			name: "logs-01",
			source_type: "local",
			source_port: 22,
			source_path: "/srv/logs",
			exclude_patterns: [],
			enabled: true,
			created_by: 1,
			created_at: "Wed, 02 Apr 2026 10:10:00 GMT",
			updated_at: "Wed, 02 Apr 2026 10:10:00 GMT",
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建实例" })).not.toBeInTheDocument()
			expect(screen.getByText("实例已创建。")).toBeInTheDocument()
			expect(screen.getByText("logs-01")).toBeInTheDocument()
		})
	})

	it("opens an edit modal with prefilled values and closes after save", async () => {
		vi.mocked(updateInstance).mockResolvedValue({
			id: 1,
			name: "web-02",
			source_type: "remote",
			source_host: "192.0.2.10",
			source_port: 22,
			source_user: "backup",
			source_ssh_key_id: 2,
			source_path: "/srv/www",
			exclude_patterns: ["node_modules"],
			enabled: true,
			created_by: 1,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 09:30:00 GMT",
		})
		vi.mocked(listInstances).mockResolvedValueOnce([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
		])
		vi.mocked(listInstances).mockResolvedValueOnce([
			{
				id: 1,
				name: "web-02",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 09:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
		])

		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(InstancesListView, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(screen.getByText("web-01")).toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "编辑" }))

		expect(screen.getByRole("dialog", { name: "编辑实例" })).toBeInTheDocument()
		expect(screen.getByLabelText("名称")).toHaveValue("web-01")
		expect(screen.getByLabelText("源主机")).toHaveValue("192.0.2.10")
		expect(screen.getByLabelText("源用户")).toHaveValue("backup")

		await fireEvent.update(screen.getByLabelText("名称"), "web-02")
		await fireEvent.click(screen.getByRole("button", { name: "保存修改" }))

		await waitFor(() => {
			expect(updateInstance).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					name: "web-02",
					source_type: "remote",
					source_host: "192.0.2.10",
				}),
			)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "编辑实例" })).not.toBeInTheDocument()
			expect(screen.getByText("实例已更新。")).toBeInTheDocument()
			expect(screen.getByText("web-02")).toBeInTheDocument()
		})
	})
})