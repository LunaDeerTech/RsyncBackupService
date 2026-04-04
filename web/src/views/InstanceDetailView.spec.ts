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
	OverviewTab: { props: ["strategies"], template: '<div data-testid="overview-tab">{{ strategies[0]?.name ?? "概览内容" }}</div>' },
	StrategiesTab: { template: '<div data-testid="strategies-tab">策略内容</div>' },
	BackupsTab: { template: '<div data-testid="backups-tab">备份历史内容</div>' },
	RestoreTab: { template: '<div data-testid="restore-tab">恢复内容</div>' },
	SubscriptionsTab: { template: '<div data-testid="subscriptions-tab">订阅内容</div>' },
}

async function renderView() {
	const router = createRouter({
		history: createMemoryHistory(),
		routes: [
			{ path: "/instances", component: { template: "<div />" } },
			{ path: "/instances/:id", component: InstanceDetailView },
		],
	})

	await router.push("/instances/1")
	await router.isReady()

	const view = render(InstanceDetailView, {
		global: {
			plugins: [router],
			stubs: componentStubs,
		},
	})

	await waitFor(() => {
		expect(getInstanceDetail).toHaveBeenCalledWith(1)
		expect(screen.getByTestId("overview-tab")).toBeInTheDocument()
	})

	return {
		router,
		...view,
	}
}

describe("InstanceDetailView", () => {
	beforeEach(() => {
		vi.useFakeTimers()
		vi.setSystemTime(new Date("2026-04-04T12:00:00Z"))
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
		vi.mocked(listStrategies).mockResolvedValue([
			{
				id: 7,
				instance_id: 1,
				name: "每日增量",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: [
					"Sat, 04 Apr 2026 12:01:00 GMT",
					"Sat, 04 Apr 2026 12:02:00 GMT",
					"Sat, 04 Apr 2026 12:03:00 GMT",
				],
			},
		])
		vi.mocked(listStorageTargets).mockResolvedValue([])
	})

	afterEach(() => {
		vi.useRealTimers()
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

	it("refreshes strategy previews after the next scheduled run elapses", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		await renderView()

		expect(listStrategies).toHaveBeenCalledTimes(1)

		vi.mocked(listStrategies).mockResolvedValueOnce([
			{
				id: 7,
				instance_id: 1,
				name: "每日增量",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: [
					"Sat, 04 Apr 2026 12:02:00 GMT",
					"Sat, 04 Apr 2026 12:03:00 GMT",
					"Sat, 04 Apr 2026 12:04:00 GMT",
				],
			},
		])

		await vi.advanceTimersByTimeAsync(62_000)

		await waitFor(() => {
			expect(listStrategies).toHaveBeenCalledTimes(2)
		})
	})

	it("stops re-arming silent strategy refresh after the page unmounts", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const view = await renderView()
		expect(listStrategies).toHaveBeenCalledTimes(1)

		let resolveRefresh: ((value: any) => void) | null = null
		vi.mocked(listStrategies).mockImplementationOnce(
			() =>
				new Promise((resolve) => {
					resolveRefresh = resolve
				}),
		)

		await vi.advanceTimersByTimeAsync(62_000)

		view.unmount()
		resolveRefresh?.([
			{
				id: 7,
				instance_id: 1,
				name: "每日增量",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Sat, 04 Apr 2026 12:03:00 GMT"],
			},
		])
		await Promise.resolve()
		await vi.advanceTimersByTimeAsync(120_000)

		expect(listStrategies).toHaveBeenCalledTimes(2)
	})

	it("caps long preview refresh timers to the browser-safe timeout range", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const timeoutSpy = vi.spyOn(globalThis, "setTimeout")
		vi.mocked(listStrategies).mockResolvedValueOnce([
			{
				id: 7,
				instance_id: 1,
				name: "每月归档",
				backup_type: "rolling",
				cron_expr: "0 0 1 * *",
				interval_seconds: 0,
				retention_days: 30,
				retention_count: 12,
				cold_volume_size: null,
				max_execution_seconds: 7200,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Mon, 01 Jun 2026 00:00:00 GMT"],
			},
		])

		await renderView()

		expect(timeoutSpy.mock.calls.some((call) => call[1] === 2_147_483_647)).toBe(true)
		timeoutSpy.mockRestore()
	})

	it("ignores stale silent refresh responses after switching to another instance", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		vi.mocked(getInstanceDetail).mockImplementation(async (id) => ({
			id,
			name: id === 1 ? "web-01" : "web-02",
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
		}))

		vi.mocked(listStrategies).mockReset()
		vi.mocked(listStrategies).mockResolvedValueOnce([
			{
				id: 7,
				instance_id: 1,
				name: "实例一策略",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Sat, 04 Apr 2026 12:01:00 GMT"],
			},
		])

		const view = await renderView()
		await waitFor(() => {
			expect(screen.getByText("实例一策略")).toBeInTheDocument()
		})

		let resolveRefresh: ((value: any) => void) | null = null
		vi.mocked(listStrategies).mockImplementationOnce(
			() =>
				new Promise((resolve) => {
					resolveRefresh = resolve
				}),
		)

		await vi.advanceTimersByTimeAsync(62_000)

		vi.mocked(listStrategies).mockResolvedValueOnce([
			{
				id: 8,
				instance_id: 2,
				name: "实例二策略",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Sat, 04 Apr 2026 12:03:00 GMT"],
			},
		])

		await view.router.push("/instances/2")
		await waitFor(() => {
			expect(getInstanceDetail).toHaveBeenCalledWith(2)
			expect(screen.getByText("实例二策略")).toBeInTheDocument()
		})

		resolveRefresh?.([
			{
				id: 7,
				instance_id: 1,
				name: "实例一旧策略",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Sat, 04 Apr 2026 12:04:00 GMT"],
			},
		])
		await Promise.resolve()

		expect(screen.getByText("实例二策略")).toBeInTheDocument()
		expect(screen.queryByText("实例一旧策略")).not.toBeInTheDocument()
	})

	it("ignores stale initial load responses after switching to another instance", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		let resolveInstanceOneDetail: ((value: any) => void) | null = null
		let resolveInstanceOneStrategies: ((value: any) => void) | null = null

		vi.mocked(getInstanceDetail).mockReset()
		vi.mocked(getInstanceDetail).mockImplementation((id) => {
			if (id === 1) {
				return new Promise((resolve) => {
					resolveInstanceOneDetail = resolve
				})
			}

			return Promise.resolve({
				id,
				name: "web-02",
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
		})

		vi.mocked(listStrategies).mockReset()
		vi.mocked(listStrategies).mockImplementation((id) => {
			if (id === 1) {
				return new Promise((resolve) => {
					resolveInstanceOneStrategies = resolve
				})
			}

			return Promise.resolve([
				{
					id: 8,
					instance_id: 2,
					name: "实例二策略",
					backup_type: "rolling",
					cron_expr: "0 0 * * *",
					interval_seconds: 0,
					retention_days: 7,
					retention_count: 3,
					cold_volume_size: null,
					max_execution_seconds: 3600,
					storage_target_ids: [4],
					enabled: true,
					created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
					updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
					upcoming_runs: ["Sat, 04 Apr 2026 12:03:00 GMT"],
				},
			])
		})

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

		await router.push("/instances/2")

		await waitFor(() => {
			expect(getInstanceDetail).toHaveBeenCalledWith(2)
			expect(screen.getByText("实例二策略")).toBeInTheDocument()
		})

		resolveInstanceOneDetail?.({
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
		resolveInstanceOneStrategies?.([
			{
				id: 7,
				instance_id: 1,
				name: "实例一旧策略",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Sat, 04 Apr 2026 12:04:00 GMT"],
			},
		])
		await Promise.resolve()

		expect(screen.getByText("实例二策略")).toBeInTheDocument()
		expect(screen.queryByText("实例一旧策略")).not.toBeInTheDocument()
	})

	it("does not arm silent preview refresh from stale strategies while a new instance is still loading", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const view = await renderView()
		let resolveInstanceTwoDetail: ((value: any) => void) | null = null
		let resolveInstanceTwoStrategies: ((value: any) => void) | null = null

		vi.mocked(getInstanceDetail).mockImplementation((id) => {
			if (id === 2) {
				return new Promise((resolve) => {
					resolveInstanceTwoDetail = resolve
				})
			}

			return Promise.resolve({
				id,
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
		})

		vi.mocked(listStrategies).mockImplementationOnce(
			() =>
				new Promise((resolve) => {
					resolveInstanceTwoStrategies = resolve
				}),
		)

		await view.router.push("/instances/2")
		await vi.advanceTimersByTimeAsync(62_000)

		expect(listStrategies).toHaveBeenCalledTimes(2)

		resolveInstanceTwoDetail?.({
			id: 2,
			name: "web-02",
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
		resolveInstanceTwoStrategies?.([
			{
				id: 8,
				instance_id: 2,
				name: "实例二策略",
				backup_type: "rolling",
				cron_expr: "0 0 * * *",
				interval_seconds: 0,
				retention_days: 7,
				retention_count: 3,
				cold_volume_size: null,
				max_execution_seconds: 3600,
				storage_target_ids: [4],
				enabled: true,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
				upcoming_runs: ["Sat, 04 Apr 2026 12:03:00 GMT"],
			},
		])
		await Promise.resolve()
	})

	it("clears the previous instance while a switched route is loading and shows the error state on failure", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		const view = await renderView()
		await waitFor(() => {
			expect(screen.getByText("每日增量")).toBeInTheDocument()
		})

		vi.mocked(getInstanceDetail).mockImplementation((id) => {
			if (id === 2) {
				return Promise.reject(new Error("route switched load failed"))
			}

			return Promise.resolve({
				id,
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
		})
		vi.mocked(listStrategies).mockResolvedValue([])

		const navigation = view.router.push("/instances/2")
		await waitFor(() => {
			expect(screen.queryByText("每日增量")).not.toBeInTheDocument()
			expect(screen.getByText("正在加载实例详情…")).toBeInTheDocument()
		})
		await navigation

		await waitFor(() => {
			expect(screen.getByText("实例详情加载失败")).toBeInTheDocument()
			expect(screen.queryByText("每日增量")).not.toBeInTheDocument()
		})
	})

	it("backs off silent preview retries after a refresh failure instead of polling every second", async () => {
		useAuthStore().setCurrentUser({
			id: 1,
			username: "admin",
			is_admin: true,
			created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
			updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		})

		await renderView()
		vi.mocked(listStrategies).mockRejectedValueOnce(new Error("temporary preview refresh failure"))

		await vi.advanceTimersByTimeAsync(62_000)
		expect(listStrategies).toHaveBeenCalledTimes(2)

		await vi.advanceTimersByTimeAsync(5_000)
		expect(listStrategies).toHaveBeenCalledTimes(2)

		await vi.advanceTimersByTimeAsync(25_000)
		expect(listStrategies).toHaveBeenCalledTimes(3)
	})
})