import { fireEvent, render, screen, waitFor, within } from "@testing-library/vue"

import { ApiError } from "../../api/client"
import {
	createNotificationChannel,
	deleteNotificationChannel,
	listNotificationChannels,
	testNotificationChannel,
	updateNotificationChannel,
} from "../../api/notifications"
import NotificationChannelsTab from "./NotificationChannelsTab.vue"

vi.mock("../../api/notifications", () => ({
	listNotificationChannels: vi.fn(),
	createNotificationChannel: vi.fn(),
	updateNotificationChannel: vi.fn(),
	deleteNotificationChannel: vi.fn(),
	testNotificationChannel: vi.fn(),
	listSubscriptions: vi.fn(),
	upsertSubscription: vi.fn(),
	deleteSubscription: vi.fn(),
}))

const smtpMain = {
	id: 5,
	name: "smtp-main",
	type: "smtp",
	enabled: true,
	config: {
		host: "smtp.example.com",
		port: 587,
		username: "mailer",
		from: "backup@example.com",
		tls: true,
	},
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
}

const smtpBackup = {
	id: 6,
	name: "smtp-backup",
	type: "smtp",
	enabled: false,
	config: {
		host: "smtp-backup.example.com",
		port: 2525,
		username: "mailer-backup",
		from: "ops@example.com",
		tls: false,
	},
	created_at: "Wed, 02 Apr 2026 09:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 09:10:00 GMT",
}

async function renderTab(): Promise<void> {
	render(NotificationChannelsTab)

	await waitFor(() => {
		expect(listNotificationChannels).toHaveBeenCalledTimes(1)
		expect(screen.getByText("smtp-main")).toBeInTheDocument()
	})
}

function getRowByText(value: string): HTMLElement {
	const cell = screen.getByText(value)
	const row = cell.closest("tr")
	expect(row).not.toBeNull()
	return row as HTMLElement
}

describe("NotificationChannelsTab", () => {
	beforeEach(() => {
		vi.mocked(listNotificationChannels).mockReset()
		vi.mocked(createNotificationChannel).mockReset()
		vi.mocked(updateNotificationChannel).mockReset()
		vi.mocked(deleteNotificationChannel).mockReset()
		vi.mocked(testNotificationChannel).mockReset()

		vi.mocked(listNotificationChannels).mockResolvedValue([smtpMain])
		vi.mocked(createNotificationChannel).mockResolvedValue(smtpBackup)
		vi.mocked(updateNotificationChannel).mockResolvedValue({
			...smtpMain,
			name: "smtp-main-updated",
		})
		vi.mocked(deleteNotificationChannel).mockResolvedValue()
		vi.mocked(testNotificationChannel).mockResolvedValue()
	})

	it("creates a new SMTP channel from a modal", async () => {
		vi.mocked(listNotificationChannels).mockReset()
		vi.mocked(listNotificationChannels)
			.mockResolvedValueOnce([smtpMain])
			.mockResolvedValueOnce([smtpMain, smtpBackup])

		await renderTab()

		await fireEvent.click(screen.getByRole("button", { name: "新建渠道" }))

		expect(screen.getByRole("dialog", { name: "新建 SMTP 渠道" })).toBeInTheDocument()
		await fireEvent.update(screen.getByLabelText("名称"), "smtp-backup")
		await fireEvent.update(screen.getByLabelText("SMTP Host"), "smtp-backup.example.com")
		await fireEvent.update(screen.getByLabelText("SMTP Port"), "2525")
		await fireEvent.update(screen.getByLabelText("From"), "ops@example.com")
		await fireEvent.update(screen.getByLabelText("用户名"), "mailer-backup")
		await fireEvent.update(screen.getByLabelText("密码"), "secret")
		await fireEvent.click(screen.getByRole("button", { name: "创建渠道" }))

		await waitFor(() => {
			expect(createNotificationChannel).toHaveBeenCalledWith({
				name: "smtp-backup",
				type: "smtp",
				config: {
					host: "smtp-backup.example.com",
					port: 2525,
					username: "mailer-backup",
					password: "secret",
					from: "ops@example.com",
					tls: true,
				},
				enabled: true,
			})
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "新建 SMTP 渠道" })).not.toBeInTheDocument()
			expect(screen.getByText("通知渠道已创建。")).toBeInTheDocument()
			expect(screen.getByText("smtp-backup")).toBeInTheDocument()
		})
	})

		it("submits edited SMTP values without overwriting the stored password", async () => {
			const updatedChannel = {
				...smtpMain,
				name: "smtp-main-updated",
				config: {
					...smtpMain.config,
					port: 2525,
				},
				updated_at: "Wed, 02 Apr 2026 09:45:00 GMT",
			}

			vi.mocked(listNotificationChannels).mockReset()
			vi.mocked(listNotificationChannels)
				.mockResolvedValueOnce([smtpMain])
				.mockResolvedValueOnce([updatedChannel])

			await renderTab()

			await fireEvent.click(within(getRowByText("smtp-main")).getByRole("button", { name: "编辑" }))

			const dialog = screen.getByRole("dialog", { name: "编辑 SMTP 渠道" })
			await fireEvent.update(screen.getByLabelText("名称"), "smtp-main-updated")
			await fireEvent.update(screen.getByLabelText("SMTP Port"), "2525")
			await fireEvent.click(within(dialog).getByRole("button", { name: "保存修改" }))

			await waitFor(() => {
				expect(updateNotificationChannel).toHaveBeenCalledWith(5, {
					name: "smtp-main-updated",
					type: "smtp",
					config: {
						host: "smtp.example.com",
						port: 2525,
						username: "mailer",
						password: undefined,
						from: "backup@example.com",
						tls: true,
					},
					enabled: true,
				})
			})

			await waitFor(() => {
				expect(screen.queryByRole("dialog", { name: "编辑 SMTP 渠道" })).not.toBeInTheDocument()
				expect(screen.getByText("通知渠道已更新。")).toBeInTheDocument()
				expect(screen.getByText("smtp-main-updated")).toBeInTheDocument()
			})
		})

		it("keeps notification save failures inside the modal", async () => {
			vi.mocked(updateNotificationChannel).mockRejectedValue(new ApiError("SMTP 认证失败", 400))

			await renderTab()

			await fireEvent.click(within(getRowByText("smtp-main")).getByRole("button", { name: "编辑" }))

			const dialog = screen.getByRole("dialog", { name: "编辑 SMTP 渠道" })
			await fireEvent.click(within(dialog).getByRole("button", { name: "保存修改" }))

			await waitFor(() => {
				expect(within(dialog).getByRole("alert")).toHaveTextContent("SMTP 认证失败")
			})

			expect(screen.getByRole("dialog", { name: "编辑 SMTP 渠道" })).toBeInTheDocument()
		})

	it("opens an edit modal with prefilled SMTP values and tests delivery from a separate modal", async () => {
		await renderTab()

		await fireEvent.click(within(getRowByText("smtp-main")).getByRole("button", { name: "编辑" }))

		expect(screen.getByRole("dialog", { name: "编辑 SMTP 渠道" })).toBeInTheDocument()
		expect(screen.getByLabelText("名称")).toHaveValue("smtp-main")
		expect(screen.getByLabelText("SMTP Host")).toHaveValue("smtp.example.com")
		expect(screen.getByLabelText("SMTP Port")).toHaveValue("587")
		expect(screen.getByLabelText("From")).toHaveValue("backup@example.com")
		expect(screen.getByLabelText("用户名")).toHaveValue("mailer")

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "编辑 SMTP 渠道" })).not.toBeInTheDocument()
		})

		await fireEvent.click(within(getRowByText("smtp-main")).getByRole("button", { name: "测试" }))
		expect(screen.getByRole("dialog", { name: "测试通知" })).toBeInTheDocument()
		await fireEvent.update(screen.getByLabelText("测试收件邮箱"), "ops@example.com")
		await fireEvent.click(screen.getByRole("button", { name: "发送测试" }))

		await waitFor(() => {
			expect(testNotificationChannel).toHaveBeenCalledWith(5, {
				email: "ops@example.com",
			})
		})

		expect(screen.getByText("测试通知已发送。")).toBeInTheDocument()
	})

	it("requires delete confirmation before removing a channel", async () => {
		vi.mocked(listNotificationChannels).mockReset()
		vi.mocked(listNotificationChannels).mockResolvedValueOnce([smtpMain]).mockResolvedValueOnce([])

		await renderTab()

		await fireEvent.click(within(getRowByText("smtp-main")).getByRole("button", { name: "删除" }))

		expect(screen.getByRole("dialog", { name: "确认删除通知渠道" })).toBeInTheDocument()
		expect(screen.getByText("即将删除通知渠道「smtp-main」。关联的订阅将无法继续发送通知。")).toBeInTheDocument()
		await fireEvent.click(screen.getByRole("button", { name: "确认删除" }))

		await waitFor(() => {
			expect(deleteNotificationChannel).toHaveBeenCalledWith(5)
		})

		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认删除通知渠道" })).not.toBeInTheDocument()
			expect(screen.getByText("通知渠道「smtp-main」已删除。")).toBeInTheDocument()
		})
	})
})