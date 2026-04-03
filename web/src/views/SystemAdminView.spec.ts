import { fireEvent, render, screen } from "@testing-library/vue"

vi.mock("./system/UsersTab.vue", () => ({
	default: {
		template: '<div>users-tab-body</div>',
	},
}))

vi.mock("./system/SSHKeysTab.vue", () => ({
	default: {
		template: '<div>ssh-keys-tab-body</div>',
	},
}))

vi.mock("./system/NotificationChannelsTab.vue", () => ({
	default: {
		template: '<div>notification-channels-tab-body</div>',
	},
}))

vi.mock("./system/AuditLogsTab.vue", () => ({
	default: {
		template: '<div>audit-logs-tab-body</div>',
	},
}))

import SystemAdminView from "./SystemAdminView.vue"

describe("SystemAdminView", () => {
	it("renders the page header and switches between management tabs", async () => {
		const { container } = render(SystemAdminView)

		expect(container.querySelector(".page-header.page-header--inset.page-header--shell-aligned")).not.toBeNull()
		expect(container.querySelector(".page-header__content")).not.toBeNull()
		expect(screen.getByText("SYSTEM")).toBeInTheDocument()
		expect(screen.getByRole("heading", { name: "系统管理" })).toBeInTheDocument()
		expect(screen.getByText("用户管理、SSH 密钥、通知渠道与审计日志。")).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "用户管理" })).toBeInTheDocument()
		expect(screen.getByText("users-tab-body")).toBeInTheDocument()
		expect(screen.queryByText("ssh-keys-tab-body")).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("tab", { name: "SSH 密钥" }))
		expect(screen.getByText("ssh-keys-tab-body")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("tab", { name: "通知渠道" }))
		expect(screen.getByText("notification-channels-tab-body")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("tab", { name: "审计日志" }))
		expect(screen.getByText("audit-logs-tab-body")).toBeInTheDocument()
	})
})
