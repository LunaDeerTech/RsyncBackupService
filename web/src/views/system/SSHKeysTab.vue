<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../../api/client"
import { createSSHKey, deleteSSHKey, listSSHKeys, testSSHKey } from "../../api/sshKeys"
import type { SSHKeySummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFileInput from "../../components/ui/AppFileInput.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppTable from "../../components/ui/AppTable.vue"
import { formatDateTime } from "../../utils/formatters"

const createModalTitleId = "system-ssh-keys-create-modal-title"
const testModalTitleId = "system-ssh-keys-test-modal-title"

const keys = ref<SSHKeySummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")

const createModalOpen = ref(false)
const isCreating = ref(false)
const createErrorMessage = ref("")
const createFileInputKey = ref(0)
const createFile = ref<File | null>(null)
const createFileName = ref("")
const createForm = reactive({
	name: "",
})

const testModalOpen = ref(false)
const isTesting = ref(false)
const testResult = ref("")
const testForm = reactive({
	keyId: "",
	host: "",
	port: "22",
	user: "",
})

const deleteDialogOpen = ref(false)
const deleteKeyId = ref<number | null>(null)
const deleteKeyName = ref("")

const keyOptions = computed(() => [
	{ value: "", label: "选择 SSH 密钥" },
	...keys.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.fingerprint}` })),
])

const createFileDescription = computed(() =>
	createFileName.value !== "" ? `已选择：${createFileName.value}` : "上传后由服务端以 0600 权限托管到数据目录。",
)

function resetCreateForm(): void {
	createForm.name = ""
	createFile.value = null
	createFileName.value = ""
	createFileInputKey.value += 1
	createErrorMessage.value = ""
}

function onCreateFileSelect(file: File | null): void {
	createFile.value = file
	createFileName.value = file?.name ?? ""
}

function resetTestForm(): void {
	testForm.keyId = keys.value.length > 0 ? String(keys.value[0].id) : ""
	testForm.host = ""
	testForm.port = "22"
	testForm.user = ""
	testResult.value = ""
}

async function loadKeys(): Promise<void> {
	errorMessage.value = ""

	try {
		keys.value = await listSSHKeys()
		if (testForm.keyId === "" && keys.value.length > 0) {
			testForm.keyId = String(keys.value[0].id)
		}
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载 SSH 密钥失败。"
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

async function submitCreate(): Promise<void> {
	createErrorMessage.value = ""
	successMessage.value = ""
	if (createFile.value === null) {
		createErrorMessage.value = "请选择私钥文件。"
		return
	}

	isCreating.value = true

	try {
		const privateKey = await createFile.value.text()
		await createSSHKey({
			name: createForm.name.trim(),
			private_key: privateKey,
		})
		successMessage.value = "SSH 密钥已登记。"
		createModalOpen.value = false
		resetCreateForm()
		await loadKeys()
	} catch (error) {
		createErrorMessage.value = error instanceof ApiError ? error.message : "登记 SSH 密钥失败。"
	} finally {
		isCreating.value = false
	}
}

function openTestModal(keyId?: number): void {
	resetTestForm()
	testForm.keyId = keyId ? String(keyId) : testForm.keyId
	testModalOpen.value = true
	}

function closeTestModal(): void {
	if (isTesting.value) {
		return
	}

	testModalOpen.value = false
	}

async function submitTest(): Promise<void> {
	if (testForm.keyId === "") {
		return
	}

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

function openDeleteDialog(key: SSHKeySummary): void {
	deleteKeyId.value = key.id
	deleteKeyName.value = key.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteKeyId.value = null
	deleteKeyName.value = ""
}

async function confirmDelete(): Promise<void> {
	if (deleteKeyId.value === null) {
		return
	}

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
		<AppNotification v-if="errorMessage" title="SSH 密钥操作失败" tone="danger" :description="errorMessage" announce />
		<AppNotification v-if="successMessage" title="SSH 密钥已更新" tone="success" :description="successMessage" />

		<AppCard>
			<template #header>
				<div class="system-keys__card-header">
					<div class="system-keys__card-heading">
						<h2 class="system-keys__card-title">已登记密钥</h2>
						<p class="system-keys__card-description">列表不会暴露私钥路径，只显示名称与指纹。</p>
					</div>
					<AppButton size="sm" @click="openCreateModal">登记密钥</AppButton>
				</div>
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

		<AppModal
			:open="createModalOpen"
			:close-on-overlay="!isCreating"
			:labelled-by="createModalTitleId"
			width="28rem"
			@close="closeCreateModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="createModalTitleId" class="page-modal-form__title">登记 SSH 密钥</h2>
					<p class="page-muted">从本地选择私钥文件，上传后由服务端托管并派生指纹。</p>
				</header>

				<form class="page-stack" @submit.prevent="submitCreate">
					<AppNotification v-if="createErrorMessage" title="登记 SSH 密钥失败" tone="danger" :description="createErrorMessage" announce />
					<AppFormField label="名称" required>
						<AppInput v-model="createForm.name" placeholder="prod-root" />
					</AppFormField>
					<AppFormField label="私钥文件" required :description="createFileDescription">
						<AppFileInput :key="createFileInputKey" accept=".pem,.key,.rsa,.txt" @select="onCreateFileSelect" />
					</AppFormField>
					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isCreating">登记密钥</AppButton>
						<AppButton type="button" variant="ghost" @click="closeCreateModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<AppModal
			:open="testModalOpen"
			:close-on-overlay="!isTesting"
			:labelled-by="testModalTitleId"
			width="30rem"
			@close="closeTestModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="testModalTitleId" class="page-modal-form__title">连通性验证</h2>
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
						announce
					/>

					<div class="page-action-row--wrap">
						<AppButton type="submit" variant="secondary" :loading="isTesting">执行验证</AppButton>
						<AppButton type="button" variant="ghost" @click="closeTestModal">关闭</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<AppDialog :open="deleteDialogOpen" title="确认删除 SSH 密钥" tone="danger" @close="closeDeleteDialog">
			<p>即将删除 SSH 密钥「{{ deleteKeyName }}」。使用该密钥的实例和存储目标将无法正常连接。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>

<style scoped>
.system-keys__card-header {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.system-keys__card-heading {
	display: grid;
	gap: var(--space-2);
}

.system-keys__card-title,
.system-keys__card-description {
	margin: 0;
}

.system-keys__card-title {
	color: var(--text-strong);
	font-size: 1.08rem;
	line-height: 1.15;
	letter-spacing: -0.03em;
}

.system-keys__card-description {
	color: var(--text-muted);
	font-size: 0.92rem;
	line-height: 1.6;
}
</style>