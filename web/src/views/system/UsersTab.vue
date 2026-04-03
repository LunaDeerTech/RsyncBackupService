<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { verifyPassword } from "../../api/auth"
import { ApiError } from "../../api/client"
import { listInstances } from "../../api/instances"
import { createUser, listInstancePermissions, listUsers, resetUserPassword, updateInstancePermissions } from "../../api/users"
import type { InstanceSummary, UserSummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppPasswordInput from "../../components/ui/AppPasswordInput.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppSwitch from "../../components/ui/AppSwitch.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatDateTime } from "../../utils/formatters"

type PermissionDraft = {
	instanceId: number
	instanceName: string
	currentRole: string
	role: string
}

const createModalTitleId = "system-users-create-modal-title"
const assignModalTitleId = "system-users-assign-modal-title"

const users = ref<UserSummary[]>([])
const instances = ref<InstanceSummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const isLoading = ref(true)

const createModalOpen = ref(false)
const isCreating = ref(false)
const createErrorMessage = ref("")
const createForm = reactive({
	username: "",
	password: "",
	isAdmin: false,
})

const resetDialogOpen = ref(false)
const resetTargetUser = ref<UserSummary | null>(null)
const resetPasswordError = ref("")
const isResettingPassword = ref(false)
const resetForm = reactive({
	newPassword: "",
	currentPassword: "",
})

const assignModalOpen = ref(false)
const assignTargetUser = ref<UserSummary | null>(null)
const assignPermissions = ref<PermissionDraft[]>([])
const isSavingPermissions = ref(false)

const assignErrorMessage = ref("")

const assignHasExplicitGlobalPermissions = computed(
	() => assignTargetUser.value?.is_admin === true && assignPermissions.value.some((permission) => permission.currentRole !== ""),
)

const assignHelpMessage = computed(() =>
	assignTargetUser.value?.is_admin
		? assignHasExplicitGlobalPermissions.value
			? "全局管理员默认拥有所有实例的 admin 权限，无需在此保存。若存在历史显式记录，界面会单独标记，当前 API 暂不支持移除。"
			: "全局管理员默认拥有所有实例的 admin 权限，无需在此保存。"
		: "当前 API 仅支持授予或更新权限，暂不支持移除已有实例权限。",
)

function resetCreateForm(): void {
	createForm.username = ""
	createForm.password = ""
	createForm.isAdmin = false
	createErrorMessage.value = ""
}

function resetResetDialogState(): void {
	resetDialogOpen.value = false
	resetTargetUser.value = null
	resetPasswordError.value = ""
	resetForm.newPassword = ""
	resetForm.currentPassword = ""
}

function resetAssignModalState(): void {
	assignModalOpen.value = false
	assignTargetUser.value = null
	assignPermissions.value = []
	assignErrorMessage.value = ""
}

async function loadPermissionDrafts(user: UserSummary): Promise<PermissionDraft[]> {
	return Promise.all(
		instances.value.map(async (instance) => {
			const permissions = await listInstancePermissions(instance.id)
			const userPermission = permissions.find((permission) => permission.user_id === user.id)
			const currentRole = userPermission?.role ?? ""
			return {
				instanceId: instance.id,
				instanceName: instance.name,
				currentRole,
				role: user.is_admin ? "admin" : currentRole,
			}
		}),
	)
}

function formatAssignFailureMessage(updatedInstances: string[], baseMessage: string, reloaded: boolean): string {
	const normalizedMessage = baseMessage.replace(/[。.!]+$/u, "")
	const prefix = updatedInstances.length > 0 ? `已更新 ${updatedInstances.join("、")}；` : ""
	const suffix = reloaded ? "当前权限已重新加载，请确认。" : "当前权限重新加载失败，请关闭弹层后重试。"
	return `${prefix}${normalizedMessage}。${suffix}`
}

async function loadData(): Promise<void> {
	isLoading.value = true
	errorMessage.value = ""

	try {
		const [userItems, instanceItems] = await Promise.all([listUsers(), listInstances()])
		users.value = userItems
		instances.value = instanceItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载用户数据失败。"
	} finally {
		isLoading.value = false
	}
}

function openCreateModal(): void {
	resetCreateForm()
	createModalOpen.value = true
}

function closeCreateModal(): void {
	if (isCreating.value) {
		return
	}

	createModalOpen.value = false
}

async function submitCreateUser(): Promise<void> {
	isCreating.value = true
	createErrorMessage.value = ""
	successMessage.value = ""

	try {
		await createUser({
			username: createForm.username.trim(),
			password: createForm.password,
			is_admin: createForm.isAdmin,
		})
		successMessage.value = "用户已创建。"
		createModalOpen.value = false
		resetCreateForm()
		await loadData()
	} catch (error) {
		createErrorMessage.value = error instanceof ApiError ? error.message : "创建用户失败。"
	} finally {
		isCreating.value = false
	}
}

function openResetDialog(user: UserSummary): void {
	resetTargetUser.value = user
	resetPasswordError.value = ""
	resetForm.newPassword = ""
	resetForm.currentPassword = ""
	resetDialogOpen.value = true
}

function closeResetDialog(): void {
	if (isResettingPassword.value) {
		return
	}

	resetResetDialogState()
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
		resetResetDialogState()
	} catch (error) {
		resetPasswordError.value = error instanceof ApiError ? error.message : "重置密码失败。"
	} finally {
		isResettingPassword.value = false
	}
}

async function openAssignModal(user: UserSummary): Promise<void> {
	errorMessage.value = ""
	assignErrorMessage.value = ""

	try {
		const permissionItems = await loadPermissionDrafts(user)

		assignTargetUser.value = user
		assignPermissions.value = permissionItems
		assignModalOpen.value = true
	} catch (error) {
		resetAssignModalState()
		errorMessage.value = error instanceof ApiError ? error.message : "加载权限数据失败。"
	}
}

function closeAssignModal(): void {
	if (isSavingPermissions.value) {
		return
	}

	resetAssignModalState()
}

async function submitAssignPermissions(): Promise<void> {
	if (!assignTargetUser.value) {
		return
	}

	isSavingPermissions.value = true
	assignErrorMessage.value = ""
	successMessage.value = ""

	try {
		if (assignTargetUser.value.is_admin) {
			assignErrorMessage.value = "全局管理员默认拥有所有实例的 admin 权限，无需在此保存。"
			return
		}

		if (assignPermissions.value.some((permission) => permission.currentRole !== "" && permission.role === "")) {
			assignErrorMessage.value = "当前 API 暂不支持移除已有实例权限，请保留 viewer 或 admin。"
			return
		}

		const permissionsToUpdate = assignPermissions.value.filter(
			(permission) => permission.role !== "" && permission.role !== permission.currentRole,
		)
		const updatedInstances: string[] = []

		for (const permission of permissionsToUpdate) {
			try {
				await updateInstancePermissions(permission.instanceId, [
					{ user_id: assignTargetUser.value.id, role: permission.role },
				])
				updatedInstances.push(permission.instanceName)
			} catch (error) {
				const updateMessage = error instanceof ApiError ? error.message : "保存权限失败"

				try {
					assignPermissions.value = await loadPermissionDrafts(assignTargetUser.value)
					assignErrorMessage.value = formatAssignFailureMessage(updatedInstances, updateMessage, true)
				} catch (reloadError) {
					assignErrorMessage.value = formatAssignFailureMessage(updatedInstances, updateMessage, false)
					if (reloadError instanceof ApiError) {
						errorMessage.value = reloadError.message
					}
				}
				return
			}
		}

		successMessage.value = `用户 ${assignTargetUser.value.username} 的实例权限已更新。`
		resetAssignModalState()
	} catch (error) {
		assignErrorMessage.value = error instanceof ApiError ? error.message : "保存权限失败。"
	} finally {
		isSavingPermissions.value = false
	}
}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="用户管理操作失败" tone="danger" :description="errorMessage" announce />
		<AppNotification v-if="successMessage" title="操作成功" tone="success" :description="successMessage" />

		<AppCard>
			<template #header>
				<div class="system-users__card-header">
					<div class="system-users__card-heading">
						<h2 class="system-users__card-title">用户列表</h2>
						<p class="system-users__card-description">创建用户、重置密码，并为单个用户分配实例角色。</p>
					</div>
					<AppButton size="sm" @click="openCreateModal">创建用户</AppButton>
				</div>
			</template>

			<AppTable
				:rows="users"
				:columns="[
					{ key: 'username', label: '用户名' },
					{ key: 'is_admin', label: '角色' },
					{ key: 'created_at', label: '创建时间' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-is_admin="{ value }">
					<AppTag :tone="value ? 'success' : 'default'">{{ value ? "admin" : "member" }}</AppTag>
				</template>
				<template #cell-created_at="{ value }">
					<span>{{ formatDateTime(String(value)) }}</span>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openAssignModal(row)">分配实例</AppButton>
						<AppButton size="sm" variant="ghost" @click="openResetDialog(row)">重置密码</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="!isLoading && users.length === 0" title="当前没有用户" compact />
		</AppCard>

		<AppModal
			:open="createModalOpen"
			:close-on-overlay="!isCreating"
			:labelled-by="createModalTitleId"
			width="28rem"
			@close="closeCreateModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="createModalTitleId" class="page-modal-form__title">创建用户</h2>
				</header>

				<form class="page-stack" @submit.prevent="submitCreateUser">
					<AppNotification v-if="createErrorMessage" title="创建用户失败" tone="danger" :description="createErrorMessage" announce />
					<AppFormField label="用户名" required>
						<AppInput v-model="createForm.username" />
					</AppFormField>
					<AppFormField label="初始密码" required>
						<AppPasswordInput v-model="createForm.password" autocomplete="new-password" />
					</AppFormField>
					<AppFormField label="管理员账户">
						<AppSwitch v-model="createForm.isAdmin" />
					</AppFormField>
					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isCreating">创建用户</AppButton>
						<AppButton type="button" variant="ghost" @click="closeCreateModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<AppModal
			:open="assignModalOpen"
			:close-on-overlay="!isSavingPermissions"
			:labelled-by="assignModalTitleId"
			width="36rem"
			@close="closeAssignModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="assignModalTitleId" class="page-modal-form__title">分配实例权限</h2>
					<p class="page-muted">为用户「{{ assignTargetUser?.username }}」选择可访问的备份实例及其角色。</p>
				</header>

				<div class="page-stack">
					<p class="page-muted">{{ assignHelpMessage }}</p>
					<div v-for="permission in assignPermissions" :key="permission.instanceId" class="system-admin__perm-row">
						<div class="system-admin__perm-copy">
							<span class="system-admin__perm-name">{{ permission.instanceName }}</span>
							<p v-if="assignTargetUser?.is_admin === true && permission.currentRole !== ''" class="system-admin__perm-note">
								已存在显式 {{ permission.currentRole }} 记录，当前生效权限仍为 admin。
							</p>
						</div>
						<AppSelect
							v-model="permission.role"
							:disabled="assignTargetUser?.is_admin === true"
							:options="[
								{ value: '', label: '无权限' },
								{ value: 'viewer', label: 'viewer' },
								{ value: 'admin', label: 'admin' },
							]"
						/>
					</div>
					<AppEmpty v-if="assignPermissions.length === 0" title="没有可分配的实例" compact />
					<AppNotification v-if="assignErrorMessage" title="保存权限失败" tone="danger" :description="assignErrorMessage" announce />
					<div class="page-action-row--wrap">
						<AppButton :loading="isSavingPermissions" :disabled="assignTargetUser?.is_admin === true" @click="submitAssignPermissions">保存权限</AppButton>
						<AppButton variant="ghost" @click="closeAssignModal">取消</AppButton>
					</div>
				</div>
			</section>
		</AppModal>

		<AppDialog :open="resetDialogOpen" title="确认重置用户密码" tone="danger" @close="closeResetDialog">
			<div class="page-stack">
				<p class="page-danger-copy">即将重置「{{ resetTargetUser?.username }}」的密码。请输入你的当前密码以完成二次认证。</p>
				<AppFormField label="新密码" required>
					<AppPasswordInput v-model="resetForm.newPassword" autocomplete="new-password" />
				</AppFormField>
				<AppFormField label="当前管理员密码" required>
					<AppPasswordInput v-model="resetForm.currentPassword" autocomplete="current-password" />
				</AppFormField>
				<AppNotification v-if="resetPasswordError" title="密码未重置" tone="danger" :description="resetPasswordError" announce />
			</div>

			<template #actions>
				<AppButton variant="ghost" @click="closeResetDialog">取消</AppButton>
				<AppButton variant="danger" :loading="isResettingPassword" @click="submitResetPassword">确认重置</AppButton>
			</template>
		</AppDialog>
	</section>
</template>

<style scoped>
.system-users__card-header {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.system-users__card-heading {
	display: grid;
	gap: var(--space-2);
}

.system-users__card-title,
.system-users__card-description {
	margin: 0;
}

.system-users__card-title {
	color: var(--text-strong);
	font-size: 1.08rem;
	line-height: 1.15;
	letter-spacing: -0.03em;
}

.system-users__card-description {
	color: var(--text-muted);
	font-size: 0.92rem;
	line-height: 1.6;
}

.system-admin__perm-row {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: var(--space-3);
	padding: var(--space-2) 0;
	border-bottom: var(--border-width) solid var(--border-default);
}

.system-admin__perm-copy {
	display: grid;
	gap: var(--space-1);
}

.system-admin__perm-name {
	font-weight: 600;
	color: var(--text-strong);
}

.system-admin__perm-note {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.82rem;
	line-height: 1.5;
}
</style>