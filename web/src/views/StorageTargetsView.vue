<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../api/client"
import { createStorageTarget, deleteStorageTarget, listStorageTargets, testStorageTarget, updateStorageTarget } from "../api/storageTargets"
import { listSSHKeys } from "../api/sshKeys"
import type { SSHKeySummary, StorageTargetPayload, StorageTargetSummary } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppDialog from "../components/ui/AppDialog.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppModal from "../components/ui/AppModal.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime } from "../utils/formatters"

type TargetGroupKey = "rolling" | "cold"
type StorageTargetType = "rolling_local" | "rolling_ssh" | "cold_local" | "cold_ssh"

type TargetGroup = {
	key: TargetGroupKey
	title: string
	description: string
	createLabel: string
	emptyDescription: string
	items: StorageTargetSummary[]
}

const targets = ref<StorageTargetSummary[]>([])
const sshKeys = ref<SSHKeySummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const formError = ref("")
const modalOpen = ref(false)
const deleteDialogOpen = ref(false)
const deleteTargetId = ref<number | null>(null)
const deleteTargetName = ref("")
const testModalOpen = ref(false)
const testTarget = ref<StorageTargetSummary | null>(null)
const isSubmitting = ref(false)
const isTesting = ref(false)
const formModalTitleId = "storage-targets-form-modal-title"
const testModalTitleId = "storage-targets-test-modal-title"

const form = reactive({
	id: "",
	name: "",
	type: "rolling_local" as StorageTargetType,
	host: "",
	port: "22",
	user: "",
	sshKeyId: "",
	basePath: "",
})

const typeOptions = [
	{ value: "rolling_local", label: "滚动备份 / 本地" },
	{ value: "rolling_ssh", label: "滚动备份 / SSH" },
	{ value: "cold_local", label: "冷备份 / 本地" },
	{ value: "cold_ssh", label: "冷备份 / SSH" },
]

const createTypeOptions: Record<TargetGroupKey, Array<{ value: StorageTargetType; label: string }>> = {
	rolling: [
		{ value: "rolling_local", label: "本地路径" },
		{ value: "rolling_ssh", label: "SSH 主机" },
	],
	cold: [
		{ value: "cold_local", label: "本地路径" },
		{ value: "cold_ssh", label: "SSH 主机" },
	],
}

const sshKeyOptions = computed(() => [
	{ value: "", label: "选择 SSH 密钥" },
	...sshKeys.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.fingerprint}` })),
])

const isRemoteTarget = computed(() => form.type.endsWith("_ssh"))
const isCreatingTarget = computed(() => form.id === "")
const currentFormGroup = computed<TargetGroupKey>(() => (form.type.startsWith("cold_") ? "cold" : "rolling"))
const formTypeOptions = computed(() => (isCreatingTarget.value ? createTypeOptions[currentFormGroup.value] : typeOptions))
const formTypeFieldLabel = computed(() => (isCreatingTarget.value ? "接入方式" : "目标类型"))
const formModalTitle = computed(() => {
	if (!isCreatingTarget.value) {
		return "编辑存储目标"
	}

	return currentFormGroup.value === "cold" ? "新建冷备归档目标" : "新建滚动备份目标"
})
const formModalDescription = computed(() => {
	if (!isCreatingTarget.value) {
		return "本地目标只需基础路径，SSH 目标还需要连接信息。"
	}

	return currentFormGroup.value === "cold"
		? "创建冷备归档目标，可选择本地路径或 SSH 主机作为接入方式。"
		: "创建滚动备份目标，可选择本地路径或 SSH 主机作为接入方式。"
})
const testTargetConnection = computed(() => {
	if (!testTarget.value?.host) {
		return "本地目标"
	}

	const userPrefix = testTarget.value.user ? `${testTarget.value.user}@` : ""
	return `${userPrefix}${testTarget.value.host}:${testTarget.value.port}`
})

const groupedTargets = computed<TargetGroup[]>(() => [
	{
		key: "rolling",
		title: "滚动备份目标",
		description: "用于 rsync 快照与 link-dest 增量路径。",
		createLabel: "新建滚动备份目标",
		emptyDescription: "点击右上角「新建滚动备份目标」按钮添加目标。",
		items: targets.value.filter((item) => item.type.startsWith("rolling_")),
	},
	{
		key: "cold",
		title: "冷备归档目标",
		description: "用于 tar 归档或分卷文件的上传与保留。",
		createLabel: "新建冷备归档目标",
		emptyDescription: "点击右上角「新建冷备归档目标」按钮添加目标。",
		items: targets.value.filter((item) => item.type.startsWith("cold_")),
	},
])

function resetForm(defaultType: StorageTargetType = "rolling_local"): void {
	form.id = ""
	form.name = ""
	form.type = defaultType
	form.host = ""
	form.port = "22"
	form.user = ""
	form.sshKeyId = ""
	form.basePath = ""
	formError.value = ""
}

function openCreateModal(group: TargetGroupKey): void {
	resetForm(group === "cold" ? "cold_local" : "rolling_local")
	modalOpen.value = true
}

function openEditModal(target: StorageTargetSummary): void {
	form.id = String(target.id)
	form.name = target.name
	form.type = target.type
	form.host = target.host ?? ""
	form.port = String(target.port)
	form.user = target.user ?? ""
	form.sshKeyId = target.ssh_key_id ? String(target.ssh_key_id) : ""
	form.basePath = target.base_path
	formError.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}

	modalOpen.value = false
}

function openDeleteDialog(target: StorageTargetSummary): void {
	deleteTargetId.value = target.id
	deleteTargetName.value = target.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteTargetId.value = null
	deleteTargetName.value = ""
}

function openTestModal(target: StorageTargetSummary): void {
	testTarget.value = target
	testModalOpen.value = true
}

function closeTestModal(): void {
	if (isTesting.value) {
		return
	}

	testModalOpen.value = false
	testTarget.value = null
}

function buildPayload(): StorageTargetPayload {
	const payload: StorageTargetPayload = {
		name: form.name.trim(),
		type: form.type,
		base_path: form.basePath.trim(),
	}

	if (isRemoteTarget.value) {
		payload.host = form.host.trim()
		payload.port = Number.parseInt(form.port, 10) || 22
		payload.user = form.user.trim()
		payload.ssh_key_id = form.sshKeyId !== "" ? Number.parseInt(form.sshKeyId, 10) : null
	}

	return payload
}

async function loadData(): Promise<void> {
	errorMessage.value = ""

	try {
		const [targetItems, sshKeyItems] = await Promise.all([listStorageTargets(), listSSHKeys()])
		targets.value = targetItems
		sshKeys.value = sshKeyItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载存储目标失败。"
	}
}

async function submitForm(): Promise<void> {
	formError.value = ""
	successMessage.value = ""
	isSubmitting.value = true

	try {
		if (form.id === "") {
			await createStorageTarget(buildPayload())
			successMessage.value = "存储目标已创建。"
		} else {
			await updateStorageTarget(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "存储目标已更新。"
		}

		modalOpen.value = false
		resetForm()
		await loadData()
	} catch (error) {
		formError.value = error instanceof ApiError ? error.message : "保存存储目标失败。"
	} finally {
		isSubmitting.value = false
	}
}

async function confirmTest(): Promise<void> {
	if (!testTarget.value) {
		return
	}

	errorMessage.value = ""
	successMessage.value = ""
	isTesting.value = true

	try {
		await testStorageTarget(testTarget.value.id)
		successMessage.value = `存储目标「${testTarget.value.name}」连通性测试成功。`
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "存储目标测试失败。"
	} finally {
		isTesting.value = false
		closeTestModal()
	}
}

async function confirmDelete(): Promise<void> {
	if (deleteTargetId.value === null) {
		return
	}

	errorMessage.value = ""
	successMessage.value = ""

	try {
		await deleteStorageTarget(deleteTargetId.value)
		successMessage.value = `存储目标「${deleteTargetName.value}」已删除。`
		closeDeleteDialog()
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除存储目标失败。"
		closeDeleteDialog()
	}
}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">STORAGE TARGETS</p>
				<h1 class="page-header__title">存储目标</h1>
				<p class="page-header__subtitle">按备份类型管理目标路径，并执行连通性测试。</p>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="存储目标操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="存储目标操作成功" tone="success" :description="successMessage" />

		<div class="page-stack">
			<AppCard v-for="group in groupedTargets" :key="group.title">
				<template #header>
					<div class="storage-targets__card-header">
						<div class="storage-targets__card-heading">
							<h2 class="storage-targets__card-title">{{ group.title }}</h2>
							<p class="storage-targets__card-description">{{ group.description }}</p>
						</div>
						<AppButton size="sm" @click="openCreateModal(group.key)">{{ group.createLabel }}</AppButton>
					</div>
				</template>

				<AppTable
					v-if="group.items.length > 0"
					:rows="group.items"
					:columns="[
						{ key: 'name', label: '名称' },
						{ key: 'type', label: '类型' },
						{ key: 'base_path', label: '基础路径' },
						{ key: 'updated_at', label: '更新时间' },
						{ key: 'actions', label: '操作' },
					]"
					row-key="id"
				>
					<template #cell-type="{ row }">
						<div class="page-stack">
							<AppTag :tone="row.type.endsWith('_ssh') ? 'info' : 'default'">{{ row.type }}</AppTag>
							<span v-if="row.host" class="page-muted">{{ row.user }}@{{ row.host }}:{{ row.port }}</span>
						</div>
					</template>
					<template #cell-updated_at="{ value }">
						<span>{{ formatDateTime(String(value)) }}</span>
					</template>
					<template #cell-actions="{ row }">
						<div class="page-action-row--wrap">
							<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
							<AppButton size="sm" variant="ghost" @click="openTestModal(row)">测试</AppButton>
							<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
						</div>
					</template>
				</AppTable>
				<AppEmpty
					v-else
					title="当前没有目标"
					:description="group.emptyDescription"
					compact
				/>
			</AppCard>
		</div>

		<AppModal
			:open="modalOpen"
			:close-on-overlay="!isSubmitting"
			labelled-by="storage-targets-form-modal-title"
			width="34rem"
			@close="closeModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="formModalTitleId" class="page-modal-form__title">{{ formModalTitle }}</h2>
					<p class="page-muted">{{ formModalDescription }}</p>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="archive-primary" />
						</AppFormField>

						<AppFormField :label="formTypeFieldLabel" required>
							<AppSelect v-model="form.type" :options="formTypeOptions" />
						</AppFormField>

						<AppFormField label="基础路径" required>
							<AppInput v-model="form.basePath" placeholder="/srv/backup" />
						</AppFormField>
					</div>

					<div v-if="isRemoteTarget" class="page-form-grid">
						<AppFormField label="主机" required>
							<AppInput v-model="form.host" placeholder="192.0.2.20" />
						</AppFormField>
						<AppFormField label="端口">
							<AppInput v-model="form.port" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="用户" required>
							<AppInput v-model="form.user" placeholder="backup" />
						</AppFormField>
						<AppFormField label="SSH 密钥" required>
							<AppSelect v-model="form.sshKeyId" :options="sshKeyOptions" />
						</AppFormField>
					</div>

					<AppNotification
						v-if="form.type === 'rolling_ssh' || form.type === 'cold_ssh'"
						title="远程目标提示"
						tone="warning"
						description="SSH 目标的连通性测试会直接验证主机、用户和密钥组合。"
					/>

					<AppNotification v-if="formError" title="保存失败" tone="danger" :description="formError" />

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? '创建目标' : '保存修改' }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<AppModal
			:open="testModalOpen"
			:close-on-overlay="!isTesting"
			labelled-by="storage-targets-test-modal-title"
			width="30rem"
			@close="closeTestModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="testModalTitleId" class="page-modal-form__title">测试存储目标连通性</h2>
					<p class="page-muted">将对存储目标「{{ testTarget?.name ?? '' }}」执行一次即时连通性检查。</p>
				</header>

				<div class="page-stack">
					<dl class="page-detail-list">
						<div>
							<dt>目标类型</dt>
							<dd>{{ testTarget?.type ?? '—' }}</dd>
						</div>
						<div>
							<dt>基础路径</dt>
							<dd class="page-mono">{{ testTarget?.base_path ?? '—' }}</dd>
						</div>
						<div v-if="testTarget?.host">
							<dt>远程连接</dt>
							<dd>{{ testTargetConnection }}</dd>
						</div>
					</dl>

					<div class="page-action-row--wrap">
						<AppButton :loading="isTesting" @click="confirmTest">开始测试</AppButton>
						<AppButton variant="ghost" @click="closeTestModal">取消</AppButton>
					</div>
				</div>
			</section>
		</AppModal>

		<AppDialog :open="deleteDialogOpen" title="确认删除存储目标" tone="danger" @close="closeDeleteDialog">
			<p>即将删除存储目标「{{ deleteTargetName }}」。若该目标仍被策略引用或已经存在备份记录，系统会拒绝删除。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>

<style scoped>
.storage-targets__card-header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: var(--space-4);
	flex-wrap: wrap;
}

.storage-targets__card-heading {
	display: grid;
	gap: var(--space-2);
}

.storage-targets__card-title,
.storage-targets__card-description {
	margin: 0;
}

.storage-targets__card-title {
	color: var(--text-strong);
	font-size: 1.08rem;
	line-height: 1.15;
	letter-spacing: -0.03em;
}

.storage-targets__card-description {
	color: var(--text-muted);
	font-size: 0.92rem;
	line-height: 1.6;
}
</style>