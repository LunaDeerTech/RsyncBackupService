import { fireEvent, render, screen, waitFor, within } from "@testing-library/vue"

import { ApiError } from "../../api/client"
import { createSSHKey, deleteSSHKey, listSSHKeys, testSSHKey } from "../../api/sshKeys"
import SSHKeysTab from "./SSHKeysTab.vue"

vi.mock("../../api/sshKeys", () => ({
	listSSHKeys: vi.fn(),
	createSSHKey: vi.fn(),
	deleteSSHKey: vi.fn(),
	testSSHKey: vi.fn(),
}))

const primaryKey = {
	id: 7,
	name: "primary-key",
	fingerprint: "SHA256:primary",
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
}

const secondaryKey = {
	id: 8,
	name: "secondary-key",
	fingerprint: "SHA256:secondary",
	created_at: "Wed, 02 Apr 2026 09:00:00 GMT",
}

function createPrivateKeyFile(content: string): File {
	const file = new File([content], "id_rsa", { type: "application/x-pem-file" })
	Object.defineProperty(file, "text", {
		value: vi.fn().mockResolvedValue(content),
	})
	return file
}

async function renderTab(): Promise<void> {
	render(SSHKeysTab)

	await waitFor(() => {
		expect(listSSHKeys).toHaveBeenCalledTimes(1)
		expect(screen.getByText("primary-key")).toBeInTheDocument()
	})
}

function getRowByText(value: string): HTMLElement {
	const cell = screen.getByText(value)
	const row = cell.closest("tr")
	expect(row).not.toBeNull()
	return row as HTMLElement
}

describe("SSHKeysTab", () => {
	beforeEach(() => {
		vi.mocked(listSSHKeys).mockReset()
		vi.mocked(createSSHKey).mockReset()
		vi.mocked(deleteSSHKey).mockReset()
		vi.mocked(testSSHKey).mockReset()

		vi.mocked(listSSHKeys).mockResolvedValue([primaryKey])
		vi.mocked(createSSHKey).mockResolvedValue(secondaryKey)
		vi.mocked(deleteSSHKey).mockResolvedValue()
		vi.mocked(testSSHKey).mockResolvedValue()
	})

	it("registers a new key from a modal instead of an inline form", async () => {
		vi.mocked(listSSHKeys).mockReset()
		vi.mocked(listSSHKeys)
			.mockResolvedValueOnce([primaryKey])
			.mockResolvedValueOnce([primaryKey, secondaryKey])
		const privateKey = "-----BEGIN RSA PRIVATE KEY-----\nsecondary\n-----END RSA PRIVATE KEY-----"

		await renderTab()

		await fireEvent.click(screen.getByRole("button", { name: "登记密钥" }))

		const dialog = screen.getByRole("dialog", { name: "登记 SSH 密钥" })
		expect(dialog).toBeInTheDocument()
		await fireEvent.update(screen.getByLabelText("名称"), "secondary-key")
		await fireEvent.change(screen.getByLabelText("私钥文件"), {
			target: {
				files: [createPrivateKeyFile(privateKey)],
			},
		})
		await fireEvent.click(within(dialog).getByRole("button", { name: "登记密钥" }))

		await waitFor(() => {
			expect(createSSHKey).toHaveBeenCalledWith({
				name: "secondary-key",
				private_key: privateKey,
			})
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "登记 SSH 密钥" })).not.toBeInTheDocument()
			expect(screen.getByText("SSH 密钥已登记。")).toBeInTheDocument()
			expect(screen.getByText("secondary-key")).toBeInTheDocument()
		})
	})

	it("keeps SSH key registration failures inside the modal", async () => {
		const privateKey = "-----BEGIN RSA PRIVATE KEY-----\nmissing\n-----END RSA PRIVATE KEY-----"
		vi.mocked(createSSHKey).mockRejectedValue(new ApiError("无效的 SSH 私钥", 400))

		await renderTab()

		await fireEvent.click(screen.getByRole("button", { name: "登记密钥" }))

		const dialog = screen.getByRole("dialog", { name: "登记 SSH 密钥" })
		await fireEvent.update(screen.getByLabelText("名称"), "secondary-key")
		await fireEvent.change(screen.getByLabelText("私钥文件"), {
			target: {
				files: [createPrivateKeyFile(privateKey)],
			},
		})
		await fireEvent.click(within(dialog).getByRole("button", { name: "登记密钥" }))

		await waitFor(() => {
			expect(within(dialog).getByRole("alert")).toHaveTextContent("无效的 SSH 私钥")
		})

		expect(screen.getByRole("dialog", { name: "登记 SSH 密钥" })).toBeInTheDocument()
	})

	it("tests connectivity from a dedicated modal", async () => {
		await renderTab()

		await fireEvent.click(within(getRowByText("primary-key")).getByRole("button", { name: "测试" }))

		expect(screen.getByRole("dialog", { name: "连通性验证" })).toBeInTheDocument()
		await fireEvent.update(screen.getByLabelText("主机"), "192.0.2.40")
		await fireEvent.update(screen.getByLabelText("用户"), "root")
		await fireEvent.click(screen.getByRole("button", { name: "执行验证" }))

		await waitFor(() => {
			expect(testSSHKey).toHaveBeenCalledWith(7, {
				host: "192.0.2.40",
				port: 22,
				user: "root",
			})
		})

		expect(screen.getByText("连通性验证成功。")).toBeInTheDocument()
	})

	it("requires delete confirmation before removing a key", async () => {
		vi.mocked(listSSHKeys).mockReset()
		vi.mocked(listSSHKeys).mockResolvedValueOnce([primaryKey]).mockResolvedValueOnce([])

		await renderTab()

		await fireEvent.click(within(getRowByText("primary-key")).getByRole("button", { name: "删除" }))

		expect(screen.getByRole("dialog", { name: "确认删除 SSH 密钥" })).toBeInTheDocument()
		expect(screen.getByText("即将删除 SSH 密钥「primary-key」。使用该密钥的实例和存储目标将无法正常连接。")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "确认删除" }))

		await waitFor(() => {
			expect(deleteSSHKey).toHaveBeenCalledWith(7)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认删除 SSH 密钥" })).not.toBeInTheDocument()
			expect(screen.getByText("SSH 密钥「primary-key」已删除。")).toBeInTheDocument()
		})
	})
})