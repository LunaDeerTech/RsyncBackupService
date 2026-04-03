<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../api/client"
import { createSSHKey, deleteSSHKey, listSSHKeys, testSSHKey } from "../api/sshKeys"
import type { SSHKeySummary } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppFileInput from "../components/ui/AppFileInput.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppTable from "../components/ui/AppTable.vue"
import { formatDateTime } from "../utils/formatters"

const keys = ref<SSHKeySummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const isSubmitting = ref(false)
const testingId = ref<number | null>(null)
const createFileInputKey = ref(0)
const createFile = ref<File | null>(null)
const createFileName = ref("")

const createForm = reactive({
	name: "",
})

const testForm = reactive({
	keyId: "",
	host: "",
	port: "22",
	user: "",
})

const keyOptions = computed(() => [
	{ value: "", label: "选择 SSH 密钥" },
	...keys.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.fingerprint}` })),
])

const createFileDescription = computed(() =>
	createFileName.value !== "" ? `已选择：${createFileName.value}` : "上传后由服务端以 0600 权限托管到数据目录。",
)

function onCreateFileSelect(file: File | null): void {
	createFile.value = file
	createFileName.value = file?.name ?? ""
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

async function submitCreate(): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""
	if (createFile.value === null) {
		errorMessage.value = "请选择私钥文件。"
		return
	}

	isSubmitting.value = true

	try {
		const privateKey = await createFile.value.text()
		await createSSHKey({
			name: createForm.name.trim(),
			private_key: privateKey,
		})
		successMessage.value = "SSH 密钥已登记。"
		createForm.name = ""
		createFile.value = null
		createFileName.value = ""
		createFileInputKey.value += 1
		await loadKeys()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "登记 SSH 密钥失败。"
	} finally {
		isSubmitting.value = false
	}
}

async function submitTest(): Promise<void> {
	if (testForm.keyId === "") {
		return
	}

	testingId.value = Number.parseInt(testForm.keyId, 10)
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await testSSHKey(Number.parseInt(testForm.keyId, 10), {
			host: testForm.host.trim(),
			port: Number.parseInt(testForm.port, 10) || 22,
			user: testForm.user.trim(),
		})
		successMessage.value = "SSH 连通性验证成功。"
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "SSH 密钥测试失败。"
	} finally {
		testingId.value = null
	}
}

async function handleDelete(keyId: number): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await deleteSSHKey(keyId)
		successMessage.value = `SSH 密钥 ${keyId} 已删除。`
		await loadKeys()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除 SSH 密钥失败。"
	}
}

onMounted(() => {
	void loadKeys()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">SSH KEYS</p>
				<h1 class="page-header__title">SSH 密钥</h1>
				<p class="page-header__subtitle">登记可复用的密钥，并针对远程主机执行一次连接验证。</p>
			</div>
			<div class="page-header__actions">
				<AppButton variant="secondary" @click="loadKeys">刷新</AppButton>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="SSH 密钥操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="SSH 密钥已更新" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<AppCard title="已登记密钥" description="列表不会暴露私钥路径，只显示名称与指纹。">
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
						<AppButton size="sm" variant="ghost" @click="handleDelete(row.id)">删除</AppButton>
					</template>
				</AppTable>
				<AppEmpty v-if="keys.length === 0" title="暂无 SSH 密钥" compact />
			</AppCard>

			<div class="page-stack">
				<AppCard title="登记 SSH 密钥" description="从本地选择私钥文件，上传后由服务端托管并派生指纹。">
					<form class="page-stack" @submit.prevent="submitCreate">
						<AppFormField label="名称" required>
							<AppInput v-model="createForm.name" placeholder="prod-root" />
						</AppFormField>
						<AppFormField label="私钥文件" required :description="createFileDescription">
							<AppFileInput :key="createFileInputKey" accept=".pem,.key,.rsa,.txt" @select="onCreateFileSelect" />
						</AppFormField>
						<AppButton type="submit" :loading="isSubmitting">登记密钥</AppButton>
					</form>
				</AppCard>

				<AppCard title="连通性验证" description="使用已登记密钥验证主机、端口和用户组合是否可达。">
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
						<AppButton type="submit" variant="secondary" :loading="testingId !== null">执行验证</AppButton>
					</form>
				</AppCard>
			</div>
		</section>
	</section>
</template>