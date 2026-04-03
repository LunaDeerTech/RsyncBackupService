import { fireEvent, render, screen, waitFor } from "@testing-library/vue"
import { createMemoryHistory, createRouter } from "vue-router"

import { getInstanceDetail } from "../api/instances"
import { listStorageTargets } from "../api/storageTargets"
import { listStrategies } from "../api/strategies"
import { useAuthStore } from "../stores/auth"
import InstanceDetailView from "./InstanceDetailView.vue"

vi.mock("../api/instances", () => ({
	getInstanceDetail: vi.fn(),
}))

vi.mock("../api/strategies", () => ({
	listStrategies: vi.fn(),
}))

vi.mock("../api/storageTargets", () => ({
	listStorageTargets: vi.fn(),
}))

const AppTabsStub = {
	props: ["modelValue", "tabs", "ariaLabel"],
	emits: ["update:modelValue"],
	template: `
		<div>
			<button
				v-for="tab in tabs"
				:key="tab.value"
				type="button"
				@click="$emit('update:modelValue', tab.value)"
			>
				{{ tab.label }}
			</button>
			<button type="button" @click="$emit('update:modelValue', 'subscriptions')">force-hidden-admin-tab</button>
		</div>
	`,
}

const componentStubs = {
	AppTabs: AppTabsStub,
	OverviewTab: { template: '<div data-testid="overview-tab">概览内容</div>' },
	StrategiesTab: { template: '<div data-testid="strategies-tab">策略内容</div>' },
	BackupsTab: { template: '<div data-testid="backups-tab">备份历史内容</div>' },
	RestoreTab: { template: '<div data-testid="restore-tab">恢复内容</div>' },
	SubscriptionsTab: { template: '<div data-testid="subscriptions-tab">订阅内容</div>' },
}

async function renderView(): Promise<void> {
	const router = createRouter({
		history: createMemoryHistory(),
		routes: [
			{ path: "/instances", component: { template: "<div />" } },
			{ path: "/instances/:id", component: InstanceDetailView },
		],
	})

	await router.push("/instances/1")
	await router.isReady()

	render(InstanceDetailView, {
		global: {
			plugins: [router],
			stubs: componentStubs,
		},
	})

	await waitFor(() => {
		expect(getInstanceDetail).toHaveBeenCalledWith(1)
		expect(screen.getByTestId("overview-tab")).toBeInTheDocument()
	})
}

describe("InstanceDetailView", () => {
	beforeEach(() => {
		const auth = useAuthStore()

		localStorage.clear()
		auth.clearSession()
		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})

		vi.mocked(getInstanceDetail).mockReset()
		vi.mocked(listStrategies).mockReset()
		vi.mocked(listStorageTargets).mockReset()

		vi.mocked(getInstanceDetail).mockResolvedValue({
			id: 1,
			name: "web-01",
			source_type: "remote",
			source_host: "192.0.2.10",
			source_port: 22,
			source_user: "backup",
			source_ssh_key_id: 2,
			source_path: "/srv/www",
			exclude_patterns: [],
			enabled: true,
			created_by: 1,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
		})
		vi.mocked(listStrategies).mockResolvedValue([])
		vi.mocked(listStorageTargets).mockResolvedValue([])
	})

	it("shows only overview and backups tabs for viewers", async () => {
		useAuthStore().setCurrentUser({
			id: 2,
			username: "viewer",
			is_admin: false,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		await renderView()

		expect(screen.getByRole("button", { name: "概览" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "备份历史" })).toBeInTheDocument()
		expect(screen.queryByRole("button", { name: "策略" })).not.toBeInTheDocument()
		expect(screen.queryByRole("button", { name: "恢复" })).not.toBeInTheDocument()
		expect(screen.queryByRole("button", { name: "通知订阅" })).not.toBeInTheDocument()
	})

	it("shows all five tabs for admins", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		await renderView()

		expect(screen.getByRole("button", { name: "概览" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "策略" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "备份历史" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "恢复" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "通知订阅" })).toBeInTheDocument()
	})

	it("falls back to a visible tab when a viewer is forced onto an admin-only tab", async () => {
		useAuthStore().setCurrentUser({
			id: 2,
			username: "viewer",
			is_admin: false,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		await renderView()

		await fireEvent.click(screen.getByRole("button", { name: "force-hidden-admin-tab" }))

		await waitFor(() => {
			expect(screen.getByTestId("overview-tab")).toBeInTheDocument()
			expect(screen.queryByTestId("subscriptions-tab")).not.toBeInTheDocument()
		})
	})
})