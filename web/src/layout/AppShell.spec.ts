import { render, screen, waitFor, within } from "@testing-library/vue"

import App from "../App.vue"
import { getCurrentUser } from "../api/auth"
import { listInstances } from "../api/instances"
import { listSSHKeys } from "../api/sshKeys"
import { listStorageTargets } from "../api/storageTargets"
import { listUsers } from "../api/users"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import { useUiStore, THEME_STORAGE_KEY } from "../stores/ui"
import { formatDateTime } from "../utils/formatters"

vi.mock("../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

vi.mock("../api/storageTargets", () => ({
	listStorageTargets: vi.fn(),
	createStorageTarget: vi.fn(),
	updateStorageTarget: vi.fn(),
	deleteStorageTarget: vi.fn(),
	testStorageTarget: vi.fn(),
}))

vi.mock("../api/sshKeys", () => ({
	listSSHKeys: vi.fn(),
	createSSHKey: vi.fn(),
	deleteSSHKey: vi.fn(),
	testSSHKey: vi.fn(),
}))

vi.mock("../api/users", () => ({
	listUsers: vi.fn(),
	createUser: vi.fn(),
	resetUserPassword: vi.fn(),
	listInstancePermissions: vi.fn(),
	updateInstancePermissions: vi.fn(),
}))

vi.mock("../api/instances", () => ({
	listInstances: vi.fn(),
	getInstanceDetail: vi.fn(),
	createInstance: vi.fn(),
	updateInstance: vi.fn(),
	deleteInstance: vi.fn(),
}))

describe("AppShell", () => {
	const adminUser = {
		id: 1,
		username: "admin",
		is_admin: true,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	}

	const viewerUser = {
		id: 2,
		username: "viewer",
		is_admin: false,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
	}

	beforeEach(() => {
		localStorage.clear()
		history.replaceState(null, "", "/")
		document.documentElement.removeAttribute("data-theme")
		useAuthStore().clearSession()
		useUiStore().setTheme("light")
		vi.mocked(getCurrentUser).mockReset()
		vi.mocked(getCurrentUser).mockResolvedValue(adminUser)
		vi.mocked(listStorageTargets).mockReset()
		vi.mocked(listStorageTargets).mockResolvedValue([])
		vi.mocked(listSSHKeys).mockReset()
		vi.mocked(listSSHKeys).mockResolvedValue([])
		vi.mocked(listUsers).mockReset()
		vi.mocked(listUsers).mockResolvedValue([adminUser, viewerUser])
		vi.mocked(listInstances).mockReset()
		vi.mocked(listInstances).mockResolvedValue([
			{
				id: 11,
				name: "web-01",
				source_type: "local",
				source_port: 22,
				source_path: "/srv/web",
				exclude_patterns: [],
				enabled: true,
				created_by: 1,
				created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				updated_at: "Wed, 02 Apr 2026 08:00:00 GMT",
				strategy_count: 1,
				last_backup_status: "success",
				last_backup_at: "Wed, 02 Apr 2026 09:00:00 GMT",
				relay_mode: false,
				relay_mode_uncertain: false,
			},
		])
	})

	it("redirects anonymous users to /login", async () => {
		const router = createRouter()

		await router.push("/")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/login")
	})

	it("applies the dark theme to the document root", () => {
		const ui = useUiStore()

		ui.setTheme("dark")

		expect(document.documentElement.dataset.theme).toBe("dark")
		expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe("dark")
	})

	it("renders grouped navigation for admins without a shell top bar", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(adminUser)

		const router = createRouter()
		await router.push("/")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		const navigation = screen.getByRole("navigation", { name: "主导航" })
		const banner = document.querySelector(".top-bar")

		expect(screen.getByTestId("app-shell")).toBeInTheDocument()
		expect(banner).toBeNull()
		expect(within(navigation).getByText("工作区")).toBeInTheDocument()
		expect(within(navigation).getByText("管理")).toBeInTheDocument()
		expect(within(navigation).getByText("系统管理")).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "深色主题" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument()
		expect(screen.getByText("admin").closest("a")).toHaveAttribute("href", "/profile")
		expect(screen.queryByText("Operations Console")).not.toBeInTheDocument()
		expect(screen.queryByText("会话已验证")).not.toBeInTheDocument()
	})

	it("shows the dashboard page heading and refresh action once", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(adminUser)

		const router = createRouter()
		await router.push("/")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getAllByRole("heading", { name: "系统概览" })).toHaveLength(1)
		expect(screen.getAllByText("监控备份健康、运行任务与容量风险。")).toHaveLength(1)
		expect(screen.getByRole("button", { name: "刷新概览" })).toBeInTheDocument()
	})

	it("redirects non-admin users away from the admin dashboard", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(viewerUser)

		const router = createRouter()
		await router.push("/")
		await router.isReady()

		expect(router.currentRoute.value.path).toBe("/instances")
	})

	it("hides admin-only navigation groups for non-admin users", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(viewerUser)

		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		const navigation = screen.getByRole("navigation", { name: "主导航" })

		expect(within(navigation).getByText("工作区")).toBeInTheDocument()
		expect(within(navigation).queryByText("管理")).not.toBeInTheDocument()
		expect(within(navigation).getByText("备份实例")).toBeInTheDocument()
		expect(within(navigation).queryByText("仪表盘")).not.toBeInTheDocument()
		expect(within(navigation).queryByText("存储目标")).not.toBeInTheDocument()
		expect(within(navigation).queryByText("系统管理")).not.toBeInTheDocument()
		expect(screen.getByText("viewer").closest("a")).toHaveAttribute("href", "/profile")
	})

	it("renders the instances page header action for non-admin users", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(viewerUser)

		const router = createRouter()
		await router.push("/instances")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getByText("支持按名称、主机或路径筛选。")).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "新建实例" })).toBeInTheDocument()
	})

	it("renders the system admin users tab for administrators", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(adminUser)

		const router = createRouter()
		await router.push("/system")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(listUsers).toHaveBeenCalledTimes(1)
			expect(listInstances).toHaveBeenCalledTimes(1)
			expect(screen.getByRole("button", { name: "创建用户" })).toBeInTheDocument()
		})

		expect(screen.getByRole("tab", { name: "用户管理" })).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "SSH 密钥" })).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "通知渠道" })).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "审计日志" })).toBeInTheDocument()
		expect(screen.getAllByRole("heading", { name: "系统管理" })).toHaveLength(1)
		expect(screen.getAllByText("用户管理、SSH 密钥、通知渠道与审计日志。")).toHaveLength(1)
		expect(screen.getByText("用户列表")).toBeInTheDocument()
	})

	it("shows only one storage-targets title and keeps the grouped create actions visible", async () => {
		const auth = useAuthStore()

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(adminUser)

		const router = createRouter()
		await router.push("/storage-targets")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		expect(screen.getAllByRole("heading", { name: "存储目标" })).toHaveLength(1)
		expect(screen.getAllByText("按备份类型管理目标路径，并执行连通性测试。")).toHaveLength(1)
		expect(screen.getByRole("button", { name: "新建滚动备份目标" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "新建冷备归档目标" })).toBeInTheDocument()
	})

	it("renders the full profile route for authenticated users", async () => {
		const auth = useAuthStore()
		vi.mocked(getCurrentUser).mockResolvedValue(viewerUser)

		auth.setSession({
			accessToken: "access-token",
			refreshToken: "refresh-token",
		})
		auth.setCurrentUser(viewerUser)

		const router = createRouter()
		await router.push("/profile")
		await router.isReady()

		render(App, {
			global: {
				plugins: [router],
			},
		})

		await waitFor(() => {
			expect(screen.getAllByText(formatDateTime(viewerUser.created_at))).toHaveLength(2)
		})

		expect(screen.getAllByRole("heading", { name: "个人信息" })).toHaveLength(1)
		expect(screen.getAllByText("查看会话信息和修改密码。")).toHaveLength(1)
		expect(screen.getByText("当前会话")).toBeInTheDocument()
		expect(screen.getAllByText(/^viewer$/)).toHaveLength(2)
		expect(screen.getByLabelText("当前密码")).toBeInTheDocument()
		expect(screen.getByLabelText("新密码")).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "保存新密码" })).toBeInTheDocument()
	})
})