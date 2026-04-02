import { render, screen } from "@testing-library/vue"

import AppProgress from "./AppProgress.vue"

describe("AppProgress", () => {
	it("renders progress text alongside runtime metadata", () => {
		render(AppProgress, {
			props: {
				percentage: 45,
				speedText: "12.34MB/s",
				etaText: "1m 23s",
				tone: "running",
			},
		})

		const progressbar = screen.getByRole("progressbar")

		expect(progressbar).toHaveAttribute("aria-valuenow", "45")
		expect(screen.getByText("45%")).toBeInTheDocument()
		expect(screen.getByText("12.34MB/s")).toBeInTheDocument()
		expect(screen.getByText("1m 23s")).toBeInTheDocument()
	})

	it("forwards accessible naming to the progressbar element", () => {
		render(AppProgress, {
			props: {
				percentage: 72,
				speedText: "6.88MB/s",
			},
			attrs: {
				"aria-label": "prod-main 备份进度",
			},
		})

		expect(screen.getByRole("progressbar", { name: "prod-main 备份进度" })).toHaveAttribute("aria-valuenow", "72")
	})

	it("keeps the progressbar named when consumers replace the label slot", () => {
		render(AppProgress, {
			props: {
				percentage: 18,
				etaText: "2m 10s",
			},
			slots: {
				label: "edge-cache 重试中",
			},
		})

		expect(screen.getByRole("progressbar", { name: "edge-cache 重试中" })).toHaveAttribute("aria-valuenow", "18")
	})
})