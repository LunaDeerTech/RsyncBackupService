<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"

import { changePassword, getCurrentUser, verifyPassword } from "../api/auth"
import { listInstances } from "../api/instances"
import { ApiError } from "../api/client"
import { createUser, listInstancePermissions, listUsers, resetUserPassword, updateInstancePermissions } from "../api/users"
import type { AuthUser, InstancePermission, InstanceSummary, PermissionPayload, UserSummary } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppDialog from "../components/ui/AppDialog.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppPasswordInput from "../components/ui/AppPasswordInput.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import AppSwitch from "../components/ui/AppSwitch.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime } from "../utils/formatters"

type PermissionDraft = {
	userId: number
	username: string
	isAdmin: boolean
	currentRole: string
	nextRole: string
}

const currentUser = ref<AuthUser | null>(null)
const users = ref<UserSummary[]>([])
const instances = ref<InstanceSummary[]>([])
const permissionRows = ref<PermissionDraft[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const isLoading = ref(false)
const isSavingPermissions = ref(false)
const resetDialogOpen = ref(false)
const resetTargetUser = ref<UserSummary | null>(null)
const resetPasswordError = ref("")
const isResettingPassword = ref(false)

const createUserForm = reactive({
	username: "",
	password: "",
	isAdmin: false,
})

const passwordForm = reactive({
	currentPassword: "",
	newPassword: "",
})

const permissionForm = reactive({
	instanceId: "",
})

const resetForm = reactive({
	newPassword: "",
	currentPassword: "",
})

const instanceOptions = computed(() => [
	{ value: "", label: "选择实例" },
	...instances.value.map((item) => ({ value: String(item.id), label: item.name })),
])

const roleOptions = [
	{ value: "", label: "保持现状" },
	{ value: "viewer", label: "viewer" },
	{ value: "admin", label: "admin" },
]

function mergePermissionDrafts(userItems: UserSummary[], permissionItems: InstancePermission[]): PermissionDraft[] {
	const permissionMap = new Map(permissionItems.map((item) => [item.user_id, item.role]))

	return userItems.map((user) => {
		const currentRole = permissionMap.get(user.id) ?? (user.is_admin ? "admin" : "")
		return {
			userId: user.id,
			username: user.username,
			isAdmin: user.is_admin,
			currentRole,
			nextRole: currentRole,
		}
	})
	}

async function loadBaseData(): Promise<void> {
	isLoading.value = true
	errorMessage.value = ""

	try {
		const [me, userItems, instanceItems] = await Promise.all([getCurrentUser(), listUsers(), listInstances()])
		currentUser.value = me
		users.value = userItems
		instances.value = instanceItems
		if (permissionForm.instanceId === "" && instanceItems.length > 0) {
			permissionForm.instanceId = String(instanceItems[0].id)
		}
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载设置页失败。"
	} finally {
		isLoading.value = false
	}
}

async function loadPermissions(): Promise<void> {
	if (permissionForm.instanceId === "") {
		permissionRows.value = []
		return
	}

	try {
		const permissions = await listInstancePermissions(Number.parseInt(permissionForm.instanceId, 10))
		permissionRows.value = mergePermissionDrafts(users.value, permissions)
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载实例权限失败。"
	}
}

async function submitCreateUser(): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await createUser({
			username: createUserForm.username.trim(),
			password: createUserForm.password,
			is_admin: createUserForm.isAdmin,
		})
		successMessage.value = "用户已创建。"
		createUserForm.username = ""
		createUserForm.password = ""
		createUserForm.isAdmin = false
		await loadBaseData()
		await loadPermissions()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "创建用户失败。"
	}
}

async function submitChangePassword(): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await changePassword({
			current_password: passwordForm.currentPassword,
			new_password: passwordForm.newPassword,
		})
		successMessage.value = "当前账户密码已修改。"
		passwordForm.currentPassword = ""
		passwordForm.newPassword = ""
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "修改密码失败。"
	}
}

function openResetDialog(user: UserSummary): void {
	resetTargetUser.value = user
	resetDialogOpen.value = true
	resetPasswordError.value = ""
	resetForm.newPassword = ""
	resetForm.currentPassword = ""
}

function closeResetDialog(): void {
	if (isResettingPassword.value) {
		return
	}

	resetDialogOpen.value = false
	resetTargetUser.value = null
	resetPasswordError.value = ""
	resetForm.newPassword = ""
	resetForm.currentPassword = ""
}

async function submitResetPassword(): Promise<void> {
	if (!resetTargetUser.value) {
		return
	}

	isResettingPassword.value = true
	resetPasswordError.value = ""

	try {
		const verification = await verifyPassword(resetForm.currentPassword)
		await resetUserPassword(resetTargetUser.value.id, {
			password: resetForm.newPassword,
			verify_token: verification.verify_token,
		})
		successMessage.value = `用户 ${resetTargetUser.value.username} 的密码已重置。`
		closeResetDialog()
	} catch (error) {
		resetPasswordError.value = error instanceof ApiError ? error.message : "重置用户密码失败。"
	} finally {
		isResettingPassword.value = false
	}
}

async function submitPermissions(): Promise<void> {
	if (permissionForm.instanceId === "") {
		return
	}

	isSavingPermissions.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		const payload: PermissionPayload[] = permissionRows.value
			.filter((row) => row.nextRole !== "")
			.map((row) => ({ user_id: row.userId, role: row.nextRole }))

		await updateInstancePermissions(Number.parseInt(permissionForm.instanceId, 10), payload)
		successMessage.value = "实例权限已更新。"
		await loadPermissions()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存实例权限失败。"
	} finally {
		isSavingPermissions.value = false
	}
}

watch(
	() => permissionForm.instanceId,
	() => {
		void loadPermissions()
	},
)

onMounted(async () => {
	await loadBaseData()
	await loadPermissions()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">SETTINGS</p>
				<h1 class="page-header__title">系统设置</h1>
				<p class="page-header__subtitle">聚焦用户管理、密码修改和实例权限设置，不扩展额外系统管理功能。</p>
			</div>
			<div class="page-header__actions">
				<AppButton variant="secondary" @click="loadBaseData">刷新设置</AppButton>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="设置页面操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="设置已更新" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<AppCard title="当前会话" description="确认当前登录用户和管理员权限。">
				<dl v-if="currentUser" class="page-detail-list">
					<div>
						<dt>用户名</dt>
						<dd>{{ currentUser.username }}</dd>
					</div>
					<div>
						<dt>管理员</dt>
						<dd>
							<AppTag :tone="currentUser.is_admin ? 'success' : 'warning'">{{ currentUser.is_admin ? '是' : '否' }}</AppTag>
						</dd>
					</div>
					<div>
						<dt>创建时间</dt>
						<dd>{{ formatDateTime(currentUser.created_at) }}</dd>
					</div>
					<div>
						<dt>更新时间</dt>
						<dd>{{ formatDateTime(currentUser.updated_at) }}</dd>
					</div>
				</dl>
				<AppEmpty v-else-if="!isLoading" title="当前用户信息不可用" compact />
			</AppCard>

			<AppCard title="修改当前密码" description="当前用户修改自身密码不需要 verify token，但必须提供现有密码。">
				<form class="page-stack" @submit.prevent="submitChangePassword">
					<AppFormField label="当前密码" required>
						<AppPasswordInput v-model="passwordForm.currentPassword" autocomplete="current-password" />
					</AppFormField>
					<AppFormField label="新密码" required>
						<AppPasswordInput v-model="passwordForm.newPassword" autocomplete="new-password" />
					</AppFormField>
					<AppButton type="submit">保存新密码</AppButton>
				</form>
			</AppCard>
		</section>

		<section class="page-two-column">
			<AppCard title="用户管理" description="创建新用户，并为现有用户执行管理员态密码重置。">
				<form class="page-stack" @submit.prevent="submitCreateUser">
					<div class="page-form-grid">
						<AppFormField label="用户名" required>
							<AppInput v-model="createUserForm.username" />
						</AppFormField>
						<AppFormField label="初始密码" required>
							<AppPasswordInput v-model="createUserForm.password" autocomplete="new-password" />
						</AppFormField>
						<AppFormField label="管理员账户">
							<AppSwitch v-model="createUserForm.isAdmin" />
						</AppFormField>
					</div>
					<AppButton type="submit">创建用户</AppButton>
				</form>

				<hr class="page-divider" />

				<AppTable
					:rows="users"
					:columns="[
						{ key: 'username', label: '用户' },
						{ key: 'is_admin', label: '管理员' },
						{ key: 'created_at', label: '创建时间' },
						{ key: 'actions', label: '操作' },
					]"
					row-key="id"
				>
					<template #cell-is_admin="{ value }">
						<AppTag :tone="value ? 'success' : 'default'">{{ value ? 'admin' : 'member' }}</AppTag>
					</template>
					<template #cell-created_at="{ value }">
						<span>{{ formatDateTime(String(value)) }}</span>
					</template>
					<template #cell-actions="{ row }">
						<AppButton size="sm" variant="secondary" @click="openResetDialog(row)">重置密码</AppButton>
					</template>
				</AppTable>

				<AppEmpty v-if="users.length === 0" title="当前没有用户" compact />
			</AppCard>

			<AppCard title="实例权限" description="选择实例后批量设置 viewer/admin；当前 API 不提供显式撤权，空值表示保持现状。">
				<div class="page-stack">
					<AppFormField label="实例" required>
						<AppSelect v-model="permissionForm.instanceId" :options="instanceOptions" />
					</AppFormField>

					<AppTable
						:rows="permissionRows"
						:columns="[
							{ key: 'username', label: '用户' },
							{ key: 'currentRole', label: '当前权限' },
							{ key: 'nextRole', label: '更新为' },
						]"
						row-key="userId"
					>
						<template #cell-currentRole="{ row }">
							<div class="page-action-row--wrap">
								<AppTag :tone="row.currentRole === 'admin' ? 'success' : row.currentRole === 'viewer' ? 'info' : 'default'">
									{{ row.currentRole || '未配置' }}
								</AppTag>
								<AppTag v-if="row.isAdmin" tone="warning">全局管理员</AppTag>
							</div>
						</template>
						<template #cell-nextRole="{ row }">
							<AppSelect v-model="row.nextRole" :options="roleOptions" />
						</template>
					</AppTable>

					<AppEmpty v-if="permissionRows.length === 0" title="请先选择实例" compact />

					<div class="page-action-row--wrap">
						<AppButton :loading="isSavingPermissions" @click="submitPermissions">保存实例权限</AppButton>
					</div>
				</div>
			</AppCard>
		</section>

		<AppDialog :open="resetDialogOpen" title="确认重置用户密码" tone="danger" @close="closeResetDialog">
			<div class="page-stack">
				<p class="page-danger-copy">即将重置 {{ resetTargetUser?.username || '该用户' }} 的密码。请提供你的当前密码以完成二次认证。</p>
				<AppFormField label="新密码" required>
					<AppPasswordInput v-model="resetForm.newPassword" autocomplete="new-password" />
				</AppFormField>
				<AppFormField label="当前管理员密码" required>
					<AppPasswordInput v-model="resetForm.currentPassword" autocomplete="current-password" />
				</AppFormField>
				<AppNotification v-if="resetPasswordError" title="密码未重置" tone="danger" :description="resetPasswordError" />
			</div>

			<template #actions>
				<AppButton variant="ghost" @click="closeResetDialog">取消</AppButton>
				<AppButton variant="danger" :loading="isResettingPassword" @click="submitResetPassword">确认重置</AppButton>
			</template>
		</AppDialog>
	</section>
</template>