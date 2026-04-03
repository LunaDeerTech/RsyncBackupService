<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../api/client"
import { createStorageTarget, deleteStorageTarget, listStorageTargets, testStorageTarget, updateStorageTarget } from "../api/storageTargets"
import { listSSHKeys } from "../api/sshKeys"
import type { SSHKeySummary, StorageTargetPayload, StorageTargetSummary } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime } from "../utils/formatters"

type TargetGroup = {
	title: string
	description: string
	items: StorageTargetSummary[]
}

const targets = ref<StorageTargetSummary[]>([])
const sshKeys = ref<SSHKeySummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const formError = ref("")
const testingId = ref<number | null>(null)
const isSubmitting = ref(false)

const form = reactive({
	id: "",
	name: "",
	type: "rolling_local",
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

const sshKeyOptions = computed(() => [
	{ value: "", label: "选择 SSH 密钥" },
	...sshKeys.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.fingerprint}` })),
])

const isRemoteTarget = computed(() => form.type.endsWith("_ssh"))

const groupedTargets = computed<TargetGroup[]>(() => [
	{
		title: "滚动备份目标",
		description: "用于 rsync 快照与 link-dest 增量路径。",
		items: targets.value.filter((item) => item.type.startsWith("rolling_")),
	},
	{
		title: "冷备归档目标",
		description: "用于 tar 归档或分卷文件的上传与保留。",
		items: targets.value.filter((item) => item.type.startsWith("cold_")),
	},
])

function resetForm(): void {
	form.id = ""
	form.name = ""
	form.type = "rolling_local"
	form.host = ""
	form.port = "22"
	form.user = ""
	form.sshKeyId = ""
	form.basePath = ""
	formError.value = ""
}

function editTarget(target: StorageTargetSummary): void {
	form.id = String(target.id)
	form.name = target.name
	form.type = target.type
	form.host = target.host ?? ""
	form.port = String(target.port)
	form.user = target.user ?? ""
	form.sshKeyId = target.ssh_key_id ? String(target.ssh_key_id) : ""
	form.basePath = target.base_path
	formError.value = ""
	window.scrollTo({ top: 0, behavior: "smooth" })
}

function buildPayload(): StorageTargetPayload {
	return {
		name: form.name.trim(),
		type: form.type,
		host: isRemoteTarget.value ? form.host.trim() : undefined,
		port: isRemoteTarget.value ? Number.parseInt(form.port, 10) || 22 : undefined,
		user: isRemoteTarget.value ? form.user.trim() : undefined,
		ssh_key_id: isRemoteTarget.value && form.sshKeyId !== "" ? Number.parseInt(form.sshKeyId, 10) : null,
		base_path: form.basePath.trim(),
	}
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

		resetForm()
		await loadData()
	} catch (error) {
		formError.value = error instanceof ApiError ? error.message : "保存存储目标失败。"
	} finally {
		isSubmitting.value = false
	}
}

async function handleTest(targetId: number): Promise<void> {
	testingId.value = targetId
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await testStorageTarget(targetId)
		successMessage.value = `存储目标 ${targetId} 连通性测试成功。`
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "存储目标测试失败。"
	} finally {
		testingId.value = null
	}
}

async function handleDelete(targetId: number): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await deleteStorageTarget(targetId)
		successMessage.value = `存储目标 ${targetId} 已删除。`
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除存储目标失败。"
	}
}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<div class="page-action-row">
			<AppButton variant="secondary" @click="resetForm">新建目标</AppButton>
		</div>

		<AppNotification v-if="errorMessage" title="存储目标操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="存储目标已更新" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<div class="page-stack">
				<AppCard v-for="group in groupedTargets" :key="group.title" :title="group.title" :description="group.description">
					<AppTable
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
								<AppButton size="sm" variant="secondary" @click="editTarget(row)">编辑</AppButton>
								<AppButton size="sm" variant="ghost" :loading="testingId === row.id" @click="handleTest(row.id)">测试</AppButton>
								<AppButton size="sm" variant="ghost" @click="handleDelete(row.id)">删除</AppButton>
							</div>
						</template>
					</AppTable>
					<AppEmpty v-if="group.items.length === 0" title="当前没有目标" compact />
				</AppCard>
			</div>

			<AppCard :title="form.id === '' ? '新建存储目标' : '编辑存储目标'" description="本地目标只需基础路径，SSH 目标还需要连接信息。">
				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="archive-primary" />
						</AppFormField>

						<AppFormField label="目标类型" required>
							<AppSelect v-model="form.type" :options="typeOptions" />
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
						<AppButton type="button" variant="ghost" @click="resetForm">重置</AppButton>
					</div>
				</form>
			</AppCard>
		</section>
	</section>
</template>