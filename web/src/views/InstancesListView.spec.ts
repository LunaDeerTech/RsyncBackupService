import { render, screen, waitFor } from "@testing-library/vue"

import { listInstances } from "../api/instances"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import InstancesListView from "./InstancesListView.vue"

vi.mock("../api/instances", () => ({
	listInstances: vi.fn(),
	getInstanceDetail: vi.fn(),
	createInstance: vi.fn(),
	updateInstance: vi.fn(),
	deleteInstance: vi.fn(),
}))

describe("InstancesListView", () => {
	beforeEach(() => {
		history.replaceState(null, "", "/instances")
		useAuthStore().setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		vi.mocked(listInstances).mockReset()
		vi.mocked(listInstances).mockResolvedValue([
			{
				id: 1,
				name: "web-01",
				source_type: "remote",
				source_host: "192.0.2.10",
				source_port: 22,
				source_user: "backup",
				source_ssh_key_id: 2,
				source_path: "/srv/www",
				exclude_patterns: ["node_modules"],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				strategy_count: 2,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: true,
				relay_mode_uncertain: false,
			},
		])
	})

	it("renders instances returned by the API", async () => {
		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(InstancesListView, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(listInstances).toHaveBeenCalledTimes(1)
			expect(screen.getByText("web-01")).toBeInTheDocument()
		})

		expect(screen.getByRole("button", { name: "新建实例" })).toBeInTheDocument()
		expect(screen.getByText(/192\.0\.2\.10:\/srv\/www/)).toBeInTheDocument()
		expect(screen.getByText("2 条策略")).toBeInTheDocument()
		expect(screen.getAllByText("中继模式").length).toBeGreaterThan(0)
	})
})