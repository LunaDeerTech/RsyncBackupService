import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import AppSelect from "./AppSelect.vue"

describe("AppSelect", () => {
	it("renders the opened listbox outside the trigger container to avoid clipping", async () => {
		const { container } = render(AppSelect, {
			props: {
				modelValue: "",
				options: [
					{ value: "", label: "无权限" },
					{ value: "viewer", label: "viewer" },
					{ value: "admin", label: "admin" },
				],
			},
		})

		await fireEvent.click(screen.getByRole("combobox"))

		const listbox = screen.getByRole("listbox")
		expect(container.querySelector(".app-select")?.contains(listbox)).toBe(false)
		expect(document.body.contains(listbox)).toBe(true)
	})

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

		await waitFor(() => {
			expect(screen.getByRole("listbox")).toHaveAttribute("data-placement", "top")
		})
	})

	it("prevents horizontal scrolling inside the floating listbox", async () => {
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

		await fireEvent.click(screen.getByRole("combobox"))

		const listbox = screen.getByRole("listbox")
		const computedStyle = window.getComputedStyle(listbox)

		expect(computedStyle.boxSizing).toBe("border-box")
		expect(computedStyle.overflowX).toBe("hidden")
	})

	it("keeps each floating option constrained to the listbox width", async () => {
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

		await fireEvent.click(screen.getByRole("combobox"))

		expect(screen.getByRole("option", { name: "无权限" })).toHaveStyle({
			boxSizing: "border-box",
		})
	})
})