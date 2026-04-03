import { fireEvent, render, screen, waitFor } from "@testing-library/vue"

import { changePassword, getCurrentUser } from "../api/auth"
import { formatDateTime } from "../utils/formatters"
import ProfileView from "./ProfileView.vue"

vi.mock("../api/auth", () => ({
	login: vi.fn(),
	getCurrentUser: vi.fn(),
	verifyPassword: vi.fn(),
	changePassword: vi.fn(),
}))

describe("ProfileView", () => {
	const viewerUser = {
		id: 2,
		username: "viewer",
		is_admin: false,
		created_at: "Wed, 02 Apr 2026 08:00:00 GMT",
		updated_at: "Wed, 02 Apr 2026 08:30:00 GMT",
	}

	beforeEach(() => {
		vi.mocked(getCurrentUser).mockReset()
		vi.mocked(changePassword).mockReset()
		vi.mocked(getCurrentUser).mockResolvedValue(viewerUser)
		vi.mocked(changePassword).mockResolvedValue()
	})

	it("loads the current session details and renders the password form", async () => {
		render(ProfileView)

		await waitFor(() => {
			expect(getCurrentUser).toHaveBeenCalledTimes(1)
			expect(screen.getByText("viewer")).toBeInTheDocument()
		})

		expect(screen.getByRole("heading", { name: "个人信息" })).toBeInTheDocument()
		expect(screen.getByText("查看会话信息和修改密码。")).toBeInTheDocument()
		expect(screen.getByText("普通用户")).toBeInTheDocument()
		expect(screen.getByText(formatDateTime(viewerUser.created_at))).toBeInTheDocument()
		expect(screen.getByText(formatDateTime(viewerUser.updated_at))).toBeInTheDocument()
		expect(screen.getByLabelText("当前密码")).toBeInTheDocument()
		expect(screen.getByLabelText("新密码")).toBeInTheDocument()
		expect(screen.getByRole("button", { name: "保存新密码" })).toBeInTheDocument()
	})

	it("submits a password change and resets the form after success", async () => {
		render(ProfileView)

		await waitFor(() => {
			expect(getCurrentUser).toHaveBeenCalledTimes(1)
		})

		const currentPassword = screen.getByLabelText("当前密码")
		const newPassword = screen.getByLabelText("新密码")

		await fireEvent.update(currentPassword, "old-secret")
		await fireEvent.update(newPassword, "new-secret")
		await fireEvent.click(screen.getByRole("button", { name: "保存新密码" }))

		await waitFor(() => {
			expect(changePassword).toHaveBeenCalledWith({
				current_password: "old-secret",
				new_password: "new-secret",
			})
		})

		expect(screen.getByText("密码已修改成功。")).toBeInTheDocument()
		expect(currentPassword).toHaveValue("")
		expect(newPassword).toHaveValue("")
	})
})