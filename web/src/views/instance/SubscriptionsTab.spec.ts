import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { deleteSubscription, listNotificationChannels, listSubscriptions, upsertSubscription } from "../../api/notifications"
import SubscriptionsTab from "./SubscriptionsTab.vue"

vi.mock("../../api/notifications", () => ({
	listNotificationChannels: vi.fn(),
	listSubscriptions: vi.fn(),
	upsertSubscription: vi.fn(),
	deleteSubscription: vi.fn(),
}))

const channel = {
	id: 11,
	name: "SMTP Primary",
	type: "smtp",
	enabled: true,
	config: null,
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
}

const subscription = {
	id: 21,
	user_id: 1,
	instance_id: 1,
	channel_id: 11,
	channel,
	events: ["backup_success", "backup_failed"],
	channel_config: { email: "ops@example.com" },
	enabled: true,
	created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
}

async function renderTab(): Promise<void> {
	render(SubscriptionsTab, {
		props: {
			instanceId: 1,
		},
	})

	await waitFor(() => {
		expect(listNotificationChannels).toHaveBeenCalledTimes(1)
		expect(listSubscriptions).toHaveBeenCalledWith(1)
		expect(screen.getByText("SMTP Primary")).toBeInTheDocument()
	})
}

describe("SubscriptionsTab", () => {
	beforeEach(() => {
		vi.mocked(listNotificationChannels).mockReset()
		vi.mocked(listSubscriptions).mockReset()
		vi.mocked(upsertSubscription).mockReset()
		vi.mocked(deleteSubscription).mockReset()

		vi.mocked(listNotificationChannels).mockResolvedValue([channel])
		vi.mocked(listSubscriptions).mockResolvedValue([subscription])
	})

	it("uses modal add and edit flows instead of an inline form", async () => {
		await renderTab()

		expect(screen.queryByText("添加或更新订阅")).not.toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "添加订阅" }))

		expect(screen.getByRole("dialog", { name: "添加订阅" })).toBeInTheDocument()
		expect(screen.getByLabelText("收件邮箱")).toHaveValue("")

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "添加订阅" })).not.toBeInTheDocument()
		})

		await fireEvent.click(screen.getByRole("button", { name: "编辑" }))

		expect(screen.getByRole("dialog", { name: "编辑订阅" })).toBeInTheDocument()
		expect(screen.getByRole("combobox", { name: "通知渠道" })).toBeDisabled()
		expect(screen.getByLabelText("收件邮箱")).toHaveValue("ops@example.com")
	})

	it("shows visible guidance when no notification channels are available", async () => {
		vi.mocked(listNotificationChannels).mockResolvedValue([])
		vi.mocked(listSubscriptions).mockResolvedValue([])

		render(SubscriptionsTab, {
			props: {
				instanceId: 1,
			},
		})

		await waitFor(() => {
			expect(screen.getByText("暂无可用通知渠道")).toBeInTheDocument()
			expect(screen.getAllByText("请先在系统管理 > 通知渠道中创建并启用至少一个 SMTP 渠道。").length).toBeGreaterThan(0)
		})

		expect(screen.getByRole("button", { name: "添加订阅" })).toBeDisabled()
		expect(screen.getByText("当前没有订阅")).toBeInTheDocument()
	})

	it("requires delete confirmation before removing a subscription", async () => {
		vi.mocked(listSubscriptions).mockReset()
		vi.mocked(listSubscriptions).mockResolvedValueOnce([subscription])
		vi.mocked(listSubscriptions).mockResolvedValueOnce([])
		vi.mocked(deleteSubscription).mockResolvedValue()

		await renderTab()

		await fireEvent.click(screen.getByRole("button", { name: "删除" }))

		expect(screen.getByRole("dialog", { name: "确认删除订阅" })).toBeInTheDocument()
		expect(screen.getByText("删除该订阅后将不再接收对应事件的通知。")).toBeInTheDocument()

		await fireEvent.click(screen.getByRole("button", { name: "取消" }))
		await waitFor(() => {
			expect(screen.queryByRole("dialog", { name: "确认删除订阅" })).not.toBeInTheDocument()
		})
		expect(deleteSubscription).not.toHaveBeenCalled()

		await fireEvent.click(screen.getByRole("button", { name: "删除" }))
		await fireEvent.click(screen.getByRole("button", { name: "确认删除" }))

		await waitFor(() => {
			expect(deleteSubscription).toHaveBeenCalledWith(21)
		})
	})
})