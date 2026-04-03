import { render, screen } from "@testing-library/vue"

import AppModal from "./AppModal.vue"

describe("AppModal", () => {
	it("applies a configurable panel width", () => {
		render(AppModal, {
			props: {
				open: true,
				labelledBy: "modal-title",
				width: "32rem",
			},
			slots: {
				default: '<h2 id="modal-title">编辑策略</h2>',
			},
		})

		expect(screen.getByRole("dialog", { name: "编辑策略" })).toHaveStyle({
			width: "min(100%, 32rem)",
		})
	})

	it("constrains tall content with a scrollable panel", () => {
		render(AppModal, {
			props: {
				open: true,
				labelledBy: "modal-title",
			},
			slots: {
				default: '<h2 id="modal-title">长表单</h2>',
			},
		})

		expect(screen.getByRole("dialog", { name: "长表单" })).toHaveStyle({
			maxHeight: "min(calc(100dvh - 2rem), 100%)",
			overflow: "auto",
		})
	})
})