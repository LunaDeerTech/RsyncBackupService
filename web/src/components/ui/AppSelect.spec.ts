import { fireEvent, render, screen } from "@testing-library/vue"

import AppSelect from "./AppSelect.vue"

describe("AppSelect", () => {
	it("opens upward when there is not enough space below the trigger", async () => {
		render(AppSelect, {
			props: {
				modelValue: "",
				options: [
					{ value: "", label: "无权限" },
					{ value: "viewer", label: "viewer" },
					{ value: "admin", label: "admin" },
				],
			},
		})

		const trigger = screen.getByRole("combobox")
		Object.defineProperty(window, "innerHeight", {
			configurable: true,
			value: 720,
		})
		vi.spyOn(trigger, "getBoundingClientRect").mockReturnValue({
			width: 240,
			height: 44,
			top: 620,
			right: 260,
			bottom: 664,
			left: 20,
			x: 20,
			y: 620,
			toJSON: () => ({}),
		})

		await fireEvent.click(trigger)

		expect(screen.getByRole("listbox")).toHaveAttribute("data-placement", "top")
	})
})