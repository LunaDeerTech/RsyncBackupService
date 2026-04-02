import { render, screen } from "@testing-library/vue"

import AppNotification from "./AppNotification.vue"

describe("AppNotification", () => {
	it("renders informational notifications with semantic content without announcing by default", () => {
		render(AppNotification, {
			props: {
				title: "备份已开始",
				tone: "info",
				description: "系统正在同步最新快照。",
			},
		})

		const notification = screen.getByText("备份已开始").closest("article")

		expect(notification).toHaveAttribute("data-tone", "info")
		expect(screen.getByText("信息")).toBeInTheDocument()
		expect(screen.getByText("系统正在同步最新快照。")).toBeInTheDocument()
		expect(notification).not.toHaveAttribute("role")
	})

	it("keeps danger notifications isolated from brand color tokens and supports opt-in announcement", () => {
		render(AppNotification, {
			props: {
				title: "备份失败",
				tone: "danger",
				description: "SSH 连接中断，任务已终止。",
				announce: true,
			},
		})

		const notification = screen.getByRole("alert")

		expect(notification).toHaveAttribute("data-tone", "danger")
		expect(notification).toHaveStyle({
			"--app-notification-accent": "var(--error-500)",
			"--app-notification-surface": "color-mix(in srgb, var(--error-500) 30%, var(--surface-panel-solid))",
		})
	})
})