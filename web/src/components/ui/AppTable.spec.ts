import { render, screen } from "@testing-library/vue"

import AppTable from "./AppTable.vue"

describe("AppTable", () => {
	it("renders a dense table with stable headers and rows", () => {
		render(AppTable, {
			props: {
				dense: true,
				rows: [
					{ id: 1, name: "prod-main", status: "running" },
					{ id: 2, name: "archive", status: "idle" },
				],
				columns: [
					{ key: "name", label: "名称" },
					{ key: "status", label: "状态" },
				],
			},
		})

		const table = screen.getByRole("table")

		expect(table).toHaveAttribute("data-density", "dense")
		expect(screen.getByRole("columnheader", { name: "名称" })).toBeInTheDocument()
		expect(screen.getByRole("columnheader", { name: "状态" })).toBeInTheDocument()
		expect(screen.getByText("prod-main")).toBeInTheDocument()
		expect(screen.getByText("idle")).toBeInTheDocument()
	})
})