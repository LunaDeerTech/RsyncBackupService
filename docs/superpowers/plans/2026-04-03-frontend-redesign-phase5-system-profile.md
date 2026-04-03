# Phase 5: 系统管理与个人信息

> **For agentic workers:** Use superpowers:executing-plans to implement this phase. Each section is a sequential unit of work.

**目标：** 实现 `SystemAdminView` 的 4 个 Tab（用户管理、SSH 密钥、通知渠道、审计日志）完整功能，以及 `ProfileView` 的完整功能（会话信息 + 修改密码）。

**前置条件：** Phase 1（全局骨架，含 SystemAdminView 和 ProfileView 占位页面）已完成。

**设计规格来源：** `docs/superpowers/specs/2026-04-03-frontend-layout-redesign.md` 第 5.5、5.6 节。

**内容来源：** 4 个已有独立页面提供所有现成逻辑：
- `web/src/views/SettingsView.vue`（用户管理 + 权限 + 密码修改）
- `web/src/views/SSHKeysView.vue`（SSH 密钥）
- `web/src/views/NotificationsView.vue`（通知渠道）
- `web/src/views/AuditLogsView.vue`（审计日志）

---

## 1. 实现 ProfileView 完整功能

**文件：** `web/src/views/ProfileView.vue`（替换 Phase 1 的占位内容）

从 `SettingsView.vue` 迁移「当前会话」和「修改密码」功能。

### 完整内容：

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { changePassword, getCurrentUser } from "../api/auth"
import { ApiError } from "../api/client"
import type { AuthUser } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppPasswordInput from "../components/ui/AppPasswordInput.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime } from "../utils/formatters"

const currentUser = ref<AuthUser | null>(null)
const errorMessage = ref("")
const successMessage = ref("")
const isLoading = ref(true)

const passwordForm = reactive({
	currentPassword: "",
	newPassword: "",
})

async function loadUser(): Promise<void> {
	isLoading.value = true
	errorMessage.value = ""

	try {
		currentUser.value = await getCurrentUser()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载用户信息失败。"
	} finally {
		isLoading.value = false
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
		successMessage.value = "密码已修改成功。"
		passwordForm.currentPassword = ""
		passwordForm.newPassword = ""
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "修改密码失败。"
	}
}

onMounted(() => {
	void loadUser()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="个人信息操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="操作成功" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<AppCard title="当前会话" description="确认当前登录用户信息。">
				<dl v-if="currentUser" class="page-detail-list">
					<div>
						<dt>用户名</dt>
						<dd>{{ currentUser.username }}</dd>
					</div>
					<div>
						<dt>角色</dt>
						<dd>
							<AppTag :tone="currentUser.is_admin ? 'success' : 'default'">{{ currentUser.is_admin ? '管理员' : '普通用户' }}</AppTag>
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
				<p v-else-if="!isLoading" class="page-muted">当前用户信息不可用。</p>
			</AppCard>

			<AppCard title="修改密码" description="修改当前登录用户的密码。">
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
	</section>
</template>
```

---

## 2. 实现 SystemAdminView 完整功能

**文件：** `web/src/views/SystemAdminView.vue`（替换 Phase 1 的占位内容）

该页面整合 4 个独立 Tab 视图。由于单文件会很长（600+ 行），建议将每个 Tab 拆分为独立组件：

- `web/src/views/system/UsersTab.vue`
- `web/src/views/system/SSHKeysTab.vue`
- `web/src/views/system/NotificationChannelsTab.vue`
- `web/src/views/system/AuditLogsTab.vue`

### 2.1 SystemAdminView 外壳

```vue
<script setup lang="ts">
import { ref } from "vue"

import AppTabs from "../components/ui/AppTabs.vue"
import AuditLogsTab from "./system/AuditLogsTab.vue"
import NotificationChannelsTab from "./system/NotificationChannelsTab.vue"
import SSHKeysTab from "./system/SSHKeysTab.vue"
import UsersTab from "./system/UsersTab.vue"

const activeTab = ref("users")

const tabs = [
	{ value: "users", label: "用户管理" },
	{ value: "ssh-keys", label: "SSH 密钥" },
	{ value: "notifications", label: "通知渠道" },
	{ value: "audit-logs", label: "审计日志" },
]
</script>

<template>
	<section class="page-view">
		<AppTabs v-model="activeTab" :tabs="tabs" aria-label="系统管理标签" />

		<UsersTab v-if="activeTab === 'users'" />
		<SSHKeysTab v-else-if="activeTab === 'ssh-keys'" />
		<NotificationChannelsTab v-else-if="activeTab === 'notifications'" />
		<AuditLogsTab v-else-if="activeTab === 'audit-logs'" />
	</section>
</template>
```

**注意：** 不再需要页面级标题 header，因为 TopBar 已经通过 route.meta 显示标题。

---

### 2.2 UsersTab

**文件：** `web/src/views/system/UsersTab.vue`（新建）

从 `SettingsView.vue` 迁移用户管理（创建用户 + 用户列表 + 重置密码 Dialog）和实例权限分配功能。

**关键变更：**
- 创建用户：从内嵌表单改为 Modal
- 用户列表：全宽表格
- 实例权限分配：从矩阵式改为「分配实例」按钮 → Modal（以用户为中心）
- 重置密码：保持 danger Dialog

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"

import { verifyPassword } from "../../api/auth"
import { listInstances } from "../../api/instances"
import { ApiError } from "../../api/client"
import { createUser, listInstancePermissions, listUsers, resetUserPassword, updateInstancePermissions } from "../../api/users"
import type { InstancePermission, InstanceSummary, PermissionPayload, UserSummary } from "../../api/types"
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

const users = ref<UserSummary[]>([])
const instances = ref<InstanceSummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const isLoading = ref(true)

// Create user Modal
const createModalOpen = ref(false)
const isCreating = ref(false)
const createForm = reactive({
	username: "",
	password: "",
	isAdmin: false,
})

// Reset password Dialog
const resetDialogOpen = ref(false)
const resetTargetUser = ref<UserSummary | null>(null)
const resetPasswordError = ref("")
const isResettingPassword = ref(false)
const resetForm = reactive({
	newPassword: "",
	currentPassword: "",
})

// Assign instances Modal
const assignModalOpen = ref(false)
const assignTargetUser = ref<UserSummary | null>(null)
const assignPermissions = ref<{ instanceId: number; instanceName: string; role: string }[]>([])
const isSavingPermissions = ref(false)

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

// --- Create user ---
function openCreateModal(): void {
	createForm.username = ""
	createForm.password = ""
	createForm.isAdmin = false
	createModalOpen.value = true
}

function closeCreateModal(): void {
	if (isCreating.value) return
	createModalOpen.value = false
}

async function submitCreateUser(): Promise<void> {
	isCreating.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await createUser({
			username: createForm.username.trim(),
			password: createForm.password,
			is_admin: createForm.isAdmin,
		})
		successMessage.value = "用户已创建。"
		createModalOpen.value = false
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "创建用户失败。"
	} finally {
		isCreating.value = false
	}
}

// --- Reset password ---
function openResetDialog(user: UserSummary): void {
	resetTargetUser.value = user
	resetForm.newPassword = ""
	resetForm.currentPassword = ""
	resetPasswordError.value = ""
	resetDialogOpen.value = true
}

function closeResetDialog(): void {
	if (isResettingPassword.value) return
	resetDialogOpen.value = false
	resetTargetUser.value = null
}

async function submitResetPassword(): Promise<void> {
	if (!resetTargetUser.value) return

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
		resetPasswordError.value = error instanceof ApiError ? error.message : "重置密码失败。"
	} finally {
		isResettingPassword.value = false
	}
}

// --- Assign instances ---
async function openAssignModal(user: UserSummary): Promise<void> {
	assignTargetUser.value = user
	errorMessage.value = ""

	try {
		const permissions = await listInstancePermissions(0)
		// 获取该用户在所有实例上的权限
		// 注意：当前 API 是 listInstancePermissions(instanceId)，按实例查询。
		// 为了以用户为中心展示，需要遍历所有实例获取该用户的权限。
		// 如果数据量不大，可以逐实例查询并合并。
		// 这里采用简化策略：列出所有实例 + 为每个实例查询权限
		const permMap = new Map<number, string>()
		for (const inst of instances.value) {
			try {
				const perms = await listInstancePermissions(inst.id)
				const userPerm = perms.find((p) => p.user_id === user.id)
				if (userPerm) {
					permMap.set(inst.id, userPerm.role)
				}
			} catch {
				// 跳过无法访问的实例
			}
		}

		assignPermissions.value = instances.value.map((inst) => ({
			instanceId: inst.id,
			instanceName: inst.name,
			role: permMap.get(inst.id) ?? "",
		}))

		assignModalOpen.value = true
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载权限数据失败。"
	}
}

function closeAssignModal(): void {
	if (isSavingPermissions.value) return
	assignModalOpen.value = false
	assignTargetUser.value = null
}

async function submitAssignPermissions(): Promise<void> {
	if (!assignTargetUser.value) return

	isSavingPermissions.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		// 对每个有角色的实例更新权限
		for (const perm of assignPermissions.value) {
			if (perm.role !== "") {
				await updateInstancePermissions(perm.instanceId, [
					{ user_id: assignTargetUser.value.id, role: perm.role },
				])
			}
		}
		successMessage.value = `用户 ${assignTargetUser.value.username} 的实例权限已更新。`
		closeAssignModal()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存权限失败。"
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
		<AppNotification v-if="errorMessage" title="用户管理操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="操作成功" tone="success" :description="successMessage" />

		<AppCard title="用户列表">
			<template #header-actions>
				<AppButton size="sm" @click="openCreateModal">创建用户</AppButton>
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
					<AppTag :tone="value ? 'success' : 'default'">{{ value ? 'admin' : 'member' }}</AppTag>
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
			<AppEmpty v-if="users.length === 0" title="当前没有用户" compact />
		</AppCard>

		<!-- 创建用户 Modal -->
		<AppModal :open="createModalOpen" width="28rem" @close="closeCreateModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">创建用户</h2>
				</header>
				<form class="page-stack" @submit.prevent="submitCreateUser">
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

		<!-- 分配实例 Modal -->
		<AppModal :open="assignModalOpen" width="36rem" @close="closeAssignModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">分配实例权限</h2>
					<p class="page-muted">为用户「{{ assignTargetUser?.username }}」选择可访问的备份实例及其角色。</p>
				</header>

				<div class="page-stack">
					<div v-for="perm in assignPermissions" :key="perm.instanceId" class="system-admin__perm-row">
						<span class="system-admin__perm-name">{{ perm.instanceName }}</span>
						<AppSelect
							v-model="perm.role"
							:options="[
								{ value: '', label: '无权限' },
								{ value: 'viewer', label: 'viewer' },
								{ value: 'admin', label: 'admin' },
							]"
						/>
					</div>
					<AppEmpty v-if="assignPermissions.length === 0" title="没有可分配的实例" compact />
				</div>

				<div class="page-action-row--wrap">
					<AppButton :loading="isSavingPermissions" @click="submitAssignPermissions">保存权限</AppButton>
					<AppButton variant="ghost" @click="closeAssignModal">取消</AppButton>
				</div>
			</section>
		</AppModal>

		<!-- 重置密码 Dialog -->
		<AppDialog :open="resetDialogOpen" title="确认重置用户密码" tone="danger" @close="closeResetDialog">
			<div class="page-stack">
				<p class="page-danger-copy">即将重置「{{ resetTargetUser?.username }}」的密码。请输入你的当前密码以完成二次认证。</p>
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

<style scoped>
.system-admin__perm-row {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: var(--space-3);
	padding: var(--space-2) 0;
	border-bottom: var(--border-width) solid var(--border-default);
}

.system-admin__perm-name {
	font-weight: 600;
	color: var(--text-strong);
}
</style>
```

**关于 `#header-actions` slot：** 如果 `AppCard` 不支持该 slot，将「创建用户」按钮放在 `<AppCard>` 上方。

**关于权限批量查询性能：** `openAssignModal` 中对每个实例逐一调用 `listInstancePermissions` 可能较慢。如果实例数量少于 20 个，这是可接受的。如果需要优化，可以在后续迭代中添加批量查询 API。

---

### 2.3 SSHKeysTab

**文件：** `web/src/views/system/SSHKeysTab.vue`（新建）

从 `SSHKeysView.vue` 迁移，改为 Modal 模式。

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../../api/client"
import { createSSHKey, deleteSSHKey, listSSHKeys, testSSHKey } from "../../api/sshKeys"
import type { SSHKeySummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppTable from "../../components/ui/AppTable.vue"
import { formatDateTime } from "../../utils/formatters"

const keys = ref<SSHKeySummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")

// Create Modal
const createModalOpen = ref(false)
const isCreating = ref(false)
const createForm = reactive({
	name: "",
	privateKeyPath: "",
})

// Test Modal
const testModalOpen = ref(false)
const isTesting = ref(false)
const testResult = ref("")
const testForm = reactive({
	keyId: "",
	host: "",
	port: "22",
	user: "",
})

// Delete Dialog
const deleteDialogOpen = ref(false)
const deleteKeyId = ref<number | null>(null)
const deleteKeyName = ref("")

const keyOptions = computed(() => [
	{ value: "", label: "选择 SSH 密钥" },
	...keys.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.fingerprint}` })),
])

async function loadKeys(): Promise<void> {
	errorMessage.value = ""

	try {
		keys.value = await listSSHKeys()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载 SSH 密钥失败。"
	}
}

// --- Create ---
function openCreateModal(): void {
	createForm.name = ""
	createForm.privateKeyPath = ""
	createModalOpen.value = true
}

function closeCreateModal(): void {
	if (isCreating.value) return
	createModalOpen.value = false
}

async function submitCreate(): Promise<void> {
	isCreating.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await createSSHKey({
			name: createForm.name.trim(),
			private_key_path: createForm.privateKeyPath.trim(),
		})
		successMessage.value = "SSH 密钥已登记。"
		createModalOpen.value = false
		await loadKeys()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "登记 SSH 密钥失败。"
	} finally {
		isCreating.value = false
	}
}

// --- Test ---
function openTestModal(keyId?: number): void {
	testForm.keyId = keyId ? String(keyId) : (keys.value.length > 0 ? String(keys.value[0].id) : "")
	testForm.host = ""
	testForm.port = "22"
	testForm.user = ""
	testResult.value = ""
	testModalOpen.value = true
}

function closeTestModal(): void {
	if (isTesting.value) return
	testModalOpen.value = false
}

async function submitTest(): Promise<void> {
	if (testForm.keyId === "") return

	isTesting.value = true
	testResult.value = ""

	try {
		await testSSHKey(Number.parseInt(testForm.keyId, 10), {
			host: testForm.host.trim(),
			port: Number.parseInt(testForm.port, 10) || 22,
			user: testForm.user.trim(),
		})
		testResult.value = "连通性验证成功。"
	} catch (error) {
		testResult.value = error instanceof ApiError ? error.message : "SSH 密钥测试失败。"
	} finally {
		isTesting.value = false
	}
}

// --- Delete ---
function openDeleteDialog(key: SSHKeySummary): void {
	deleteKeyId.value = key.id
	deleteKeyName.value = key.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteKeyId.value = null
}

async function confirmDelete(): Promise<void> {
	if (deleteKeyId.value === null) return

	try {
		await deleteSSHKey(deleteKeyId.value)
		successMessage.value = `SSH 密钥「${deleteKeyName.value}」已删除。`
		closeDeleteDialog()
		await loadKeys()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除 SSH 密钥失败。"
		closeDeleteDialog()
	}
}

onMounted(() => {
	void loadKeys()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="SSH 密钥操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="SSH 密钥已更新" tone="success" :description="successMessage" />

		<AppCard title="已登记密钥" description="列表不会暴露私钥路径，只显示名称与指纹。">
			<template #header-actions>
				<AppButton size="sm" @click="openCreateModal">登记密钥</AppButton>
			</template>

			<AppTable
				:rows="keys"
				:columns="[
					{ key: 'name', label: '名称' },
					{ key: 'fingerprint', label: '指纹' },
					{ key: 'created_at', label: '创建时间' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-fingerprint="{ value }">
					<span class="page-mono">{{ value }}</span>
				</template>
				<template #cell-created_at="{ value }">
					<span>{{ formatDateTime(String(value)) }}</span>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openTestModal(row.id)">测试</AppButton>
						<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="keys.length === 0" title="暂无 SSH 密钥" compact />
		</AppCard>

		<!-- 登记密钥 Modal -->
		<AppModal :open="createModalOpen" width="28rem" @close="closeCreateModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">登记 SSH 密钥</h2>
					<p class="page-muted">仅保存路径与派生指纹，私钥内容不会传入前端。</p>
				</header>
				<form class="page-stack" @submit.prevent="submitCreate">
					<AppFormField label="名称" required>
						<AppInput v-model="createForm.name" placeholder="prod-root" />
					</AppFormField>
					<AppFormField label="私钥路径" required>
						<AppInput v-model="createForm.privateKeyPath" placeholder="/var/lib/rsync-backup/keys/prod" />
					</AppFormField>
					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isCreating">登记密钥</AppButton>
						<AppButton type="button" variant="ghost" @click="closeCreateModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<!-- 测试连通性 Modal -->
		<AppModal :open="testModalOpen" width="30rem" @close="closeTestModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">连通性验证</h2>
					<p class="page-muted">使用已登记密钥验证远程主机是否可达。</p>
				</header>
				<form class="page-stack" @submit.prevent="submitTest">
					<div class="page-form-grid">
						<AppFormField label="SSH 密钥" required>
							<AppSelect v-model="testForm.keyId" :options="keyOptions" />
						</AppFormField>
						<AppFormField label="主机" required>
							<AppInput v-model="testForm.host" placeholder="192.0.2.40" />
						</AppFormField>
						<AppFormField label="端口">
							<AppInput v-model="testForm.port" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="用户" required>
							<AppInput v-model="testForm.user" placeholder="root" />
						</AppFormField>
					</div>

					<AppNotification
						v-if="testResult"
						:title="testResult.includes('成功') ? '验证成功' : '验证失败'"
						:tone="testResult.includes('成功') ? 'success' : 'danger'"
						:description="testResult"
					/>

					<div class="page-action-row--wrap">
						<AppButton type="submit" variant="secondary" :loading="isTesting">执行验证</AppButton>
						<AppButton type="button" variant="ghost" @click="closeTestModal">关闭</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<!-- 删除确认 Dialog -->
		<AppDialog :open="deleteDialogOpen" title="确认删除 SSH 密钥" tone="danger" @close="closeDeleteDialog">
			<p>即将删除 SSH 密钥「{{ deleteKeyName }}」。使用该密钥的实例和存储目标将无法正常连接。</p>
			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>
```

---

### 2.4 NotificationChannelsTab

**文件：** `web/src/views/system/NotificationChannelsTab.vue`（新建）

从 `NotificationsView.vue` 迁移，改为 Modal 模式。

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../../api/client"
import {
	createNotificationChannel,
	deleteNotificationChannel,
	listNotificationChannels,
	testNotificationChannel,
	updateNotificationChannel,
} from "../../api/notifications"
import type { NotificationChannelPayload, NotificationChannelSummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSwitch from "../../components/ui/AppSwitch.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatDateTime } from "../../utils/formatters"

const channels = ref<NotificationChannelSummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")

// Edit/Create Modal
const modalOpen = ref(false)
const isSubmitting = ref(false)
const form = reactive({
	id: "",
	name: "",
	enabled: true,
	host: "",
	port: "587",
	username: "",
	password: "",
	from: "",
	tls: true,
})

// Test Modal
const testModalOpen = ref(false)
const isTesting = ref(false)
const testChannelId = ref<number | null>(null)
const testEmail = ref("")
const testResult = ref("")

// Delete Dialog
const deleteDialogOpen = ref(false)
const deleteChannelId = ref<number | null>(null)
const deleteChannelName = ref("")

const smtpChannels = computed(() => channels.value.filter((item) => item.type === "smtp"))

function readConfig(channel: NotificationChannelSummary): Record<string, unknown> {
	const config = channel.config
	if (config === null || Array.isArray(config) || typeof config !== "object") return {}
	return config as Record<string, unknown>
}

function buildPayload(): NotificationChannelPayload {
	return {
		name: form.name.trim(),
		type: "smtp",
		config: {
			host: form.host.trim(),
			port: Number.parseInt(form.port, 10) || 587,
			username: form.username.trim(),
			password: form.password.trim() === "" ? undefined : form.password.trim(),
			from: form.from.trim(),
			tls: form.tls,
		},
		enabled: form.enabled,
	}
}

async function loadChannels(): Promise<void> {
	errorMessage.value = ""
	try {
		channels.value = await listNotificationChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载通知渠道失败。"
	}
}

// --- Create/Edit ---
function openCreateModal(): void {
	form.id = ""
	form.name = ""
	form.enabled = true
	form.host = ""
	form.port = "587"
	form.username = ""
	form.password = ""
	form.from = ""
	form.tls = true
	modalOpen.value = true
}

function openEditModal(channel: NotificationChannelSummary): void {
	const config = readConfig(channel)
	form.id = String(channel.id)
	form.name = channel.name
	form.enabled = channel.enabled
	form.host = typeof config.host === "string" ? config.host : ""
	form.port = typeof config.port === "number" ? String(config.port) : "587"
	form.username = typeof config.username === "string" ? config.username : ""
	form.password = ""
	form.from = typeof config.from === "string" ? config.from : ""
	form.tls = typeof config.tls === "boolean" ? config.tls : true
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) return
	modalOpen.value = false
}

async function submitForm(): Promise<void> {
	isSubmitting.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		if (form.id === "") {
			await createNotificationChannel(buildPayload())
			successMessage.value = "通知渠道已创建。"
		} else {
			await updateNotificationChannel(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "通知渠道已更新。"
		}

		modalOpen.value = false
		await loadChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存通知渠道失败。"
	} finally {
		isSubmitting.value = false
	}
}

// --- Test ---
function openTestModal(channelId: number): void {
	testChannelId.value = channelId
	testEmail.value = ""
	testResult.value = ""
	testModalOpen.value = true
}

function closeTestModal(): void {
	if (isTesting.value) return
	testModalOpen.value = false
}

async function submitTest(): Promise<void> {
	if (testChannelId.value === null || testEmail.value.trim() === "") return

	isTesting.value = true
	testResult.value = ""

	try {
		await testNotificationChannel(testChannelId.value, { email: testEmail.value.trim() })
		testResult.value = "测试通知已发送。"
	} catch (error) {
		testResult.value = error instanceof ApiError ? error.message : "测试通知发送失败。"
	} finally {
		isTesting.value = false
	}
}

// --- Delete ---
function openDeleteDialog(channel: NotificationChannelSummary): void {
	deleteChannelId.value = channel.id
	deleteChannelName.value = channel.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteChannelId.value = null
}

async function confirmDelete(): Promise<void> {
	if (deleteChannelId.value === null) return

	try {
		await deleteNotificationChannel(deleteChannelId.value)
		successMessage.value = `通知渠道「${deleteChannelName.value}」已删除。`
		closeDeleteDialog()
		await loadChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除通知渠道失败。"
		closeDeleteDialog()
	}
}

onMounted(() => {
	void loadChannels()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="通知渠道操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="通知渠道已更新" tone="success" :description="successMessage" />

		<AppCard title="SMTP 渠道列表">
			<template #header-actions>
				<AppButton size="sm" @click="openCreateModal">新建渠道</AppButton>
			</template>

			<AppTable
				:rows="smtpChannels"
				:columns="[
					{ key: 'name', label: '名称' },
					{ key: 'type', label: '类型' },
					{ key: 'enabled', label: '状态' },
					{ key: 'updated_at', label: '更新时间' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? '启用' : '停用' }}</AppTag>
				</template>
				<template #cell-updated_at="{ value }">
					<span>{{ formatDateTime(String(value)) }}</span>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<AppButton size="sm" variant="ghost" @click="openTestModal(row.id)">测试</AppButton>
						<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="smtpChannels.length === 0" title="暂无通知渠道" compact />
		</AppCard>

		<!-- 新建/编辑 Modal -->
		<AppModal :open="modalOpen" width="34rem" @close="closeModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">{{ form.id === '' ? '新建 SMTP 渠道' : '编辑 SMTP 渠道' }}</h2>
				</header>
				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="smtp-main" />
						</AppFormField>
						<AppFormField label="SMTP Host" required>
							<AppInput v-model="form.host" placeholder="smtp.example.com" />
						</AppFormField>
						<AppFormField label="SMTP Port" required>
							<AppInput v-model="form.port" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="From" required>
							<AppInput v-model="form.from" placeholder="backup@example.com" />
						</AppFormField>
					</div>
					<div class="page-form-grid">
						<AppFormField label="用户名" required>
							<AppInput v-model="form.username" />
						</AppFormField>
						<AppFormField label="密码">
							<AppInput v-model="form.password" type="password" autocomplete="new-password" />
						</AppFormField>
						<AppFormField label="启用 TLS">
							<AppSwitch v-model="form.tls" />
						</AppFormField>
						<AppFormField label="启用渠道">
							<AppSwitch v-model="form.enabled" />
						</AppFormField>
					</div>
					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? '创建渠道' : '保存修改' }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<!-- 测试通知 Modal -->
		<AppModal :open="testModalOpen" width="28rem" @close="closeTestModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">测试通知</h2>
					<p class="page-muted">通过该渠道发送一封测试邮件。</p>
				</header>
				<div class="page-stack">
					<AppFormField label="测试收件邮箱" required>
						<AppInput v-model="testEmail" placeholder="ops@example.com" />
					</AppFormField>
					<AppNotification
						v-if="testResult"
						:title="testResult.includes('成功') || testResult.includes('已发送') ? '发送成功' : '发送失败'"
						:tone="testResult.includes('成功') || testResult.includes('已发送') ? 'success' : 'danger'"
						:description="testResult"
					/>
					<div class="page-action-row--wrap">
						<AppButton variant="secondary" :loading="isTesting" :disabled="testEmail.trim() === ''" @click="submitTest">发送测试</AppButton>
						<AppButton variant="ghost" @click="closeTestModal">关闭</AppButton>
					</div>
				</div>
			</section>
		</AppModal>

		<!-- 删除确认 Dialog -->
		<AppDialog :open="deleteDialogOpen" title="确认删除通知渠道" tone="danger" @close="closeDeleteDialog">
			<p>即将删除通知渠道「{{ deleteChannelName }}」。关联的订阅将无法继续发送通知。</p>
			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>
```

---

### 2.5 AuditLogsTab

**文件：** `web/src/views/system/AuditLogsTab.vue`（新建）

从 `AuditLogsView.vue` 迁移。主要变化：移除右侧时间线卡片，改为全宽表格 + 上方筛选栏。

```vue
<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { listAuditLogs } from "../../api/audit"
import { ApiError } from "../../api/client"
import { listUsers } from "../../api/users"
import type { AuditLogItem, AuditLogQuery, UserSummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatDateTime, stringifyJson } from "../../utils/formatters"

const logs = ref<AuditLogItem[]>([])
const users = ref<UserSummary[]>([])
const total = ref(0)
const errorMessage = ref("")
const isLoading = ref(false)

const filters = reactive({
	action: "",
	resourceType: "",
	userId: "",
	startTime: "",
	endTime: "",
	page: 1,
})

const userOptions = computed(() => [
	{ value: "", label: "全部用户" },
	...users.value.map((item) => ({ value: String(item.id), label: item.username })),
])

function toISODateTime(value: string): string | undefined {
	const trimmed = value.trim()
	if (trimmed === "") return undefined
	const date = new Date(trimmed)
	return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function buildQuery(): AuditLogQuery {
	return {
		action: filters.action.trim() || undefined,
		resource_type: filters.resourceType.trim() || undefined,
		user_id: filters.userId === "" ? undefined : Number.parseInt(filters.userId, 10),
		start_time: toISODateTime(filters.startTime),
		end_time: toISODateTime(filters.endTime),
		page: filters.page,
		page_size: 20,
	}
}

async function loadData(): Promise<void> {
	isLoading.value = true
	errorMessage.value = ""

	try {
		const [response, userItems] = await Promise.all([listAuditLogs(buildQuery()), listUsers()])
		logs.value = response.items
		total.value = response.total
		users.value = userItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载审计日志失败。"
	} finally {
		isLoading.value = false
	}
}

function resetFilters(): void {
	filters.action = ""
	filters.resourceType = ""
	filters.userId = ""
	filters.startTime = ""
	filters.endTime = ""
	filters.page = 1
	void loadData()
}

function previousPage(): void {
	if (filters.page <= 1) return
	filters.page -= 1
	void loadData()
}

function nextPage(): void {
	const maxPage = Math.max(1, Math.ceil(total.value / 20))
	if (filters.page >= maxPage) return
	filters.page += 1
	void loadData()
}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="审计日志加载失败" tone="danger" :description="errorMessage" />

		<AppCard title="筛选器" description="按动作、资源类型、用户和时间范围过滤。">
			<form class="page-stack" @submit.prevent="loadData">
				<div class="page-form-grid">
					<AppFormField label="动作">
						<AppInput v-model="filters.action" placeholder="instances.restore" />
					</AppFormField>
					<AppFormField label="资源类型">
						<AppInput v-model="filters.resourceType" placeholder="restore_records" />
					</AppFormField>
					<AppFormField label="用户">
						<AppSelect v-model="filters.userId" :options="userOptions" />
					</AppFormField>
					<AppFormField label="开始时间">
						<AppInput v-model="filters.startTime" type="datetime-local" />
					</AppFormField>
					<AppFormField label="结束时间">
						<AppInput v-model="filters.endTime" type="datetime-local" />
					</AppFormField>
				</div>
				<div class="page-action-row--wrap">
					<AppButton type="submit" :loading="isLoading">应用筛选</AppButton>
					<AppButton type="button" variant="ghost" @click="resetFilters">重置</AppButton>
				</div>
			</form>
		</AppCard>

		<AppCard title="日志表格" :description="`当前第 ${filters.page} 页，共 ${total} 条记录。`">
			<AppTable
				:rows="logs"
				:columns="[
					{ key: 'created_at', label: '时间' },
					{ key: 'username', label: '用户' },
					{ key: 'action', label: '动作' },
					{ key: 'resource_type', label: '资源' },
					{ key: 'detail', label: '详情' },
				]"
				row-key="id"
			>
				<template #cell-created_at="{ value }">
					<span>{{ formatDateTime(String(value)) }}</span>
				</template>
				<template #cell-action="{ value }">
					<AppTag tone="default">{{ String(value) }}</AppTag>
				</template>
				<template #cell-resource_type="{ row }">
					<div class="page-stack">
						<span>{{ row.resource_type }}</span>
						<span class="page-muted">#{{ row.resource_id }} · {{ row.ip_address }}</span>
					</div>
				</template>
				<template #cell-detail="{ value }">
					<pre class="audit-log__detail">{{ stringifyJson(value) }}</pre>
				</template>
			</AppTable>

			<AppEmpty v-if="!isLoading && logs.length === 0" title="当前没有匹配日志" compact />

			<template #footer>
				<div class="page-action-row--wrap">
					<AppButton variant="ghost" :disabled="filters.page === 1" @click="previousPage">上一页</AppButton>
					<AppButton variant="secondary" :disabled="filters.page * 20 >= total" @click="nextPage">下一页</AppButton>
				</div>
			</template>
		</AppCard>
	</section>
</template>

<style scoped>
.audit-log__detail {
	margin: 0;
	padding: 0.75rem 0.85rem;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-control);
	background: color-mix(in srgb, var(--surface-elevated) 94%, var(--surface-panel-solid));
	color: var(--text-strong);
	font-family: "JetBrains Mono", "Fira Code", ui-monospace, monospace;
	font-size: 0.8rem;
	line-height: 1.55;
	white-space: pre-wrap;
	word-break: break-word;
}
</style>
```

---

## 3. 创建 system 目录

确保 `web/src/views/system/` 目录存在。如果不存在，创建时文件系统会自动创建。

---

## 4. 验证与提交

1. 确认编译通过：

```bash
npm --prefix web run build
```

2. 提交：

```bash
git add -A
git commit -m "refactor(web): Phase 5 — SystemAdminView (4 tabs) + ProfileView complete"
```

---

## 5. 启动服务并测试

启动完整服务：

```bash
make run
```

同时启动前端开发服务器：

```bash
npm --prefix web run dev
```

然后使用 `askQuestion` 工具向用户提出以下测试问题：

**问题标题：** Phase 5 系统管理与个人信息测试

**测试清单（请用户逐项确认）：**

1. **个人信息页** — 点击侧边栏底部用户头像区域，是否跳转到 `/profile` 页面？是否正确显示用户名、角色标签、创建时间和更新时间？
2. **修改密码** — 在个人信息页中修改密码是否正常工作？输入当前密码和新密码后提交是否成功？
3. **系统管理 — Tab 切换** — 在 `/system` 页面，是否可以在 4 个 Tab（用户管理、SSH 密钥、通知渠道、审计日志）之间正常切换？
4. **用户管理 — 列表** — 用户管理 Tab 是否显示全宽用户表格？
5. **用户管理 — 创建** — 点击「创建用户」是否弹出 Modal？填写用户名、密码、管理员开关后是否创建成功？
6. **用户管理 — 分配实例** — 点击用户行的「分配实例」按钮，是否弹出 Modal 列出所有备份实例？每个实例是否有角色选择下拉（无权限/viewer/admin）？保存后是否生效？
7. **用户管理 — 重置密码** — 点击「重置密码」是否弹出 danger Dialog？输入新密码和管理员密码后是否成功重置？
8. **SSH 密钥 — 全宽表格** — SSH 密钥 Tab 是否显示全宽密钥表格（名称、指纹、创建时间、操作）？
9. **SSH 密钥 — 登记/测试/删除** — 登记密钥（Modal）、测试连通性（Modal）、删除密钥（Dialog）是否正常工作？
10. **通知渠道 — 全宽表格** — 通知渠道 Tab 是否显示全宽 SMTP 渠道表格？
11. **通知渠道 — 创建/编辑/测试/删除** — 新建渠道（Modal）、编辑渠道（Modal 预填）、测试通知（Modal）、删除渠道（Dialog）是否正常工作？
12. **审计日志 — 全宽表格** — 审计日志 Tab 是否为全宽布局（筛选器 + 表格）？是否移除了右侧时间线卡片？
13. **审计日志 — 筛选和分页** — 筛选器（动作、资源类型、用户、时间范围）和分页是否正常工作？
14. **普通用户** — 普通用户是否无法导航到 `/system` 页面？
