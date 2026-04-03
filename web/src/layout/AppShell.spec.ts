import { render, screen, within } from "@testing-library/vue"

import App from "../App.vue"
import { getCurrentUser } from "../api/auth"
import { createRouter } from "../router"
import { useAuthStore } from "../stores/auth"
import { useUiStore, THEME_STORAGE_KEY } from "../stores/ui"

vi.mock("../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
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

	it("renders grouped navigation and a simplified top bar for admins", async () => {
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
		expect(banner).not.toBeNull()
		expect(within(navigation).getByText("工作区")).toBeInTheDocument()
		expect(within(navigation).getByText("管理")).toBeInTheDocument()
		expect(within(navigation).getByText("系统管理")).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "深色主题" })).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "退出登录" })).toBeInTheDocument()
		expect(screen.getByText("admin").closest("a")).toHaveAttribute("href", "/profile")
		expect(banner).toHaveTextContent("运维仪表盘")
		expect(banner).toHaveTextContent("查看全局统计、运行中任务、最近备份与存储容量。")
		expect(banner).not.toHaveTextContent("Operations Console")
		expect(within(banner as HTMLElement).queryByText("会话已验证")).not.toBeInTheDocument()
		expect(within(banner as HTMLElement).queryByRole("button")).not.toBeInTheDocument()
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

	it("renders the system admin placeholder route for administrators", async () => {
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

		expect(screen.getByRole("tab", { name: "用户管理" })).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "SSH 密钥" })).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "通知渠道" })).toBeInTheDocument()
		expect(screen.getByRole("tab", { name: "审计日志" })).toBeInTheDocument()
		expect(screen.getByText("users — 此标签页内容将在后续阶段实现。")).toBeInTheDocument()
	})

	it("renders the profile placeholder route for authenticated users", async () => {
		const auth = useAuthStore()

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

		expect(screen.getByText("当前会话")).toBeInTheDocument()
		expect(screen.getByText("用户名：viewer")).toBeInTheDocument()
		expect(screen.getByText("角色：普通用户")).toBeInTheDocument()
		expect(screen.getByText("密码修改功能将在后续阶段实现。")).toBeInTheDocument()
	})
})