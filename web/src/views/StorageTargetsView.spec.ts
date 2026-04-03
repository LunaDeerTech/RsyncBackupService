import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import {
	createStorageTarget,
	deleteStorageTarget,
	listStorageTargets,
	testStorageTarget,
	updateStorageTarget,
} from "../api/storageTargets"
import { listSSHKeys } from "../api/sshKeys"
import StorageTargetsView from "./StorageTargetsView.vue"

vi.mock("../api/storageTargets", () => ({
	listStorageTargets: vi.fn(),
	createStorageTarget: vi.fn(),
	updateStorageTarget: vi.fn(),
	deleteStorageTarget: vi.fn(),
	testStorageTarget: vi.fn(),
}))

vi.mock("../api/sshKeys", () => ({
	listSSHKeys: vi.fn(),
}))

const sshKeyItems = [
	{
		id: 9,
		name: "primary-key",
		fingerprint: "SHA256:test",
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	},
]

const rollingTarget = {
	id: 1,
	name: "archive-primary",
	type: "rolling_ssh",
	host: "192.0.2.20",
	port: 22,
	user: "backup",
	ssh_key_id: 9,
	base_path: "/srv/archive",
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 09:00:00 GMT",
}

async function renderView(): Promise<void> {
	render(StorageTargetsView)

	await waitFor(() => {
		expect(listStorageTargets).toHaveBeenCalledTimes(1)
		expect(listSSHKeys).toHaveBeenCalledTimes(1)
		expect(screen.getByText("archive-primary")).toBeInTheDocument()
	})
}

describe("StorageTargetsView", () => {
	beforeEach(() => {
		vi.mocked(listStorageTargets).mockReset()
		vi.mocked(createStorageTarget).mockReset()
		vi.mocked(updateStorageTarget).mockReset()
		vi.mocked(deleteStorageTarget).mockReset()
		vi.mocked(testStorageTarget).mockReset()
		vi.mocked(listSSHKeys).mockReset()

		vi.mocked(listStorageTargets).mockResolvedValue([rollingTarget])
		vi.mocked(listSSHKeys).mockResolvedValue(sshKeyItems)
	})

	it("opens the create modal, reveals ssh fields, and closes through every dismiss path", async () => {
		await renderView()

		expect(screen.getByText("点击上方「新建目标」按钮添加存储目标。")).toBeInTheDocument()
		expect(screen.queryByRole("dialog", { name: "新建存储目标" })).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "新建目标" }))

		expect(screen.getByRole("dialog", { name: "新建存储目标" })).toBeInTheDocument()
		expect(screen.getByLabelText("名称")).toBeInTheDocument()
		expect(screen.getByLabelText("基础路径")).toBeInTheDocument()
		expect(screen.queryByLabelText("主机")).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("combobox", { name: "目标类型" }))
		await fireEvent.click(screen.getByRole("option", { name: "滚动备份 / SSH" }))

		expect(screen.getByLabelText("主机")).toBeInTheDocument()
		expect(screen.getByLabelText("用户")).toBeInTheDocument()
		expect(screen.getByText("SSH 目标的连通性测试会直接验证主机、用户和密钥组合。")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("combobox", { name: "目标类型" }))
		await fireEvent.keyDown(screen.getByRole("combobox", { name: "目标类型" }), { key: "Escape" })
		expect(screen.getByRole("dialog", { name: "新建存储目标" })).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建存储目标" })).not.toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "新建目标" }))
		await fireEvent.keyDown(screen.getByRole("dialog", { name: "新建存储目标" }), { key: "Escape" })
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建存储目标" })).not.toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "新建目标" }))
		const overlay = screen.getByRole("dialog", { name: "新建存储目标" }).parentElement
		expect(overlay).not.toBeNull()
		await fireEvent.click(overlay as HTMLElement)
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建存储目标" })).not.toBeInTheDocument()
		})
	})

	it("creates a target and closes the modal after refreshing the grouped list", async () => {
		const createdTarget = {
			id: 2,
			name: "cold-vault",
			type: "cold_local",
			port: 22,
			base_path: "/srv/cold",
			created_at: "Wed, 02 Apr 2026 10:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 10:00:00 GMT",
		}

		vi.mocked(listStorageTargets).mockReset()
		vi.mocked(listStorageTargets).mockResolvedValueOnce([rollingTarget])
		vi.mocked(listStorageTargets).mockResolvedValueOnce([rollingTarget, createdTarget])
		vi.mocked(createStorageTarget).mockResolvedValue(createdTarget)

		await renderView()

		await fireEvent.click(screen.getByRole("button", { name: "新建目标" }))
		await fireEvent.update(screen.getByLabelText("名称"), "cold-vault")
		await fireEvent.click(screen.getByRole("combobox", { name: "目标类型" }))
		await fireEvent.click(screen.getByRole("option", { name: "冷备份 / 本地" }))
		await fireEvent.update(screen.getByLabelText("基础路径"), "/srv/cold")
		await fireEvent.click(screen.getByRole("button", { name: "创建目标" }))

		await waitFor(() => {
			expect(createStorageTarget).toHaveBeenCalledWith({
				name: "cold-vault",
				type: "cold_local",
				base_path: "/srv/cold",
			})
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建存储目标" })).not.toBeInTheDocument()
			expect(screen.getByText("存储目标已创建。")).toBeInTheDocument()
			expect(screen.getByText("cold-vault")).toBeInTheDocument()
		})
	})

	it("opens an edit modal with prefilled remote values and closes after save", async () => {
		const updatedTarget = {
			...rollingTarget,
			name: "archive-secondary",
			updated_at: "Wed, 02 Apr 2026 09:30:00 GMT",
		}

		vi.mocked(listStorageTargets).mockReset()
		vi.mocked(listStorageTargets).mockResolvedValueOnce([rollingTarget])
		vi.mocked(listStorageTargets).mockResolvedValueOnce([updatedTarget])
		vi.mocked(updateStorageTarget).mockResolvedValue(updatedTarget)

		await renderView()

		await fireEvent.click(screen.getByRole("button", { name: "编辑" }))

		expect(screen.getByRole("dialog", { name: "编辑存储目标" })).toBeInTheDocument()
		expect(screen.getByLabelText("名称")).toHaveValue("archive-primary")
		expect(screen.getByLabelText("主机")).toHaveValue("192.0.2.20")
		expect(screen.getByLabelText("用户")).toHaveValue("backup")

		await fireEvent.update(screen.getByLabelText("名称"), "archive-secondary")
		await fireEvent.click(screen.getByRole("button", { name: "保存修改" }))

		await waitFor(() => {
			expect(updateStorageTarget).toHaveBeenCalledWith(
				1,
				expect.objectContaining({
					name: "archive-secondary",
					type: "rolling_ssh",
					host: "192.0.2.20",
					user: "backup",
					ssh_key_id: 9,
				}),
			)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "编辑存储目标" })).not.toBeInTheDocument()
			expect(screen.getByText("存储目标已更新。")).toBeInTheDocument()
			expect(screen.getByText("archive-secondary")).toBeInTheDocument()
		})
	})

	it("requires delete confirmation before removing a target", async () => {
		vi.mocked(listStorageTargets).mockReset()
		vi.mocked(listStorageTargets).mockResolvedValueOnce([rollingTarget])
		vi.mocked(listStorageTargets).mockResolvedValueOnce([])
		vi.mocked(deleteStorageTarget).mockResolvedValue()

		await renderView()

		await fireEvent.click(screen.getByRole("button", { name: "删除" }))

		expect(screen.getByRole("dialog", { name: "确认删除存储目标" })).toBeInTheDocument()
		expect(screen.getByText("即将删除存储目标「archive-primary」。若该目标仍被策略引用或已经存在备份记录，系统会拒绝删除。")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认删除存储目标" })).not.toBeInTheDocument()
		})
		expect(deleteStorageTarget).not.toHaveBeenCalled()

		await fireEvent.click(screen.getByRole("button", { name: "删除" }))
		await fireEvent.click(screen.getByRole("button", { name: "确认删除" }))

		await waitFor(() => {
			expect(deleteStorageTarget).toHaveBeenCalledWith(1)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认删除存储目标" })).not.toBeInTheDocument()
			expect(screen.getByText("存储目标「archive-primary」已删除。")).toBeInTheDocument()
		})
	})

	it("runs the connectivity test from a dedicated operation modal", async () => {
		vi.mocked(testStorageTarget).mockResolvedValue()

		await renderView()

		await fireEvent.click(screen.getByRole("button", { name: "测试" }))

		expect(screen.getByRole("dialog", { name: "测试存储目标连通性" })).toBeInTheDocument()
		expect(screen.getByText("将对存储目标「archive-primary」执行一次即时连通性检查。"))
			.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "开始测试" }))

		await waitFor(() => {
			expect(testStorageTarget).toHaveBeenCalledWith(1)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "测试存储目标连通性" })).not.toBeInTheDocument()
			expect(screen.getByText("存储目标「archive-primary」连通性测试成功。")).toBeInTheDocument()
		})
	})
})