import { render, screen } from "@testing-library/vue"

import SystemAdminView from "./SystemAdminView.vue"

describe("SystemAdminView", () => {
	it("renders the shared page header above the management tabs", () => {
		const { container } = render(SystemAdminView)

		expect(container.querySelector(".page-header.page-header--inset.page-header--shell-aligned")).not.toBeNull()
		expect(container.querySelector(".page-header__content")).not.toBeNull()
		expect(screen.getByText("SYSTEM")).toBeInTheDocument()
		expect(screen.getByRole("heading", { name: "系统管理" })).toBeInTheDocument()
		expect(screen.getByText("用户管理、SSH 密钥、通知渠道与审计日志。")).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "用户管理" })).toBeInTheDocument()
	})
})
