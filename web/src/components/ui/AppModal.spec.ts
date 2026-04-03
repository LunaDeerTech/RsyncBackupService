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
})