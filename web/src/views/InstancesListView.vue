<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../api/client"
import { createInstance, listInstances, updateInstance } from "../api/instances"
import { listSSHKeys } from "../api/sshKeys"
import type { CreateInstancePayload, InstanceSummary, SSHKeySummary, UpdateInstancePayload } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppModal from "../components/ui/AppModal.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import AppSwitch from "../components/ui/AppSwitch.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime, formatSource, formatStatusLabel, splitLines, statusTone } from "../utils/formatters"

const instances = ref<InstanceSummary[]>([])
const sshKeys = ref<SSHKeySummary[]>([])
const errorMessage = ref("")
const formError = ref("")
const successMessage = ref("")
const isLoading = ref(true)
const isSubmitting = ref(false)
const modalOpen = ref(false)
const query = ref("")
const enabledFilter = ref("all")
const modalTitleId = "instances-list-modal-title"

const form = reactive({
	id: "",
	name: "",
	sourceType: "local",
	sourceHost: "",
	sourcePort: "22",
	sourceUser: "",
	sourceSSHKeyID: "",
	sourcePath: "",
	excludePatterns: "",
	enabled: true,
})

const sshKeyOptions = computed(() => [
	{ value: "", label: "不使用 SSH 密钥" },
	...sshKeys.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.fingerprint}` })),
])

const filteredInstances = computed(() => {
	const trimmedQuery = query.value.trim().toLowerCase()

	return instances.value.filter((instance) => {
		const matchesQuery =
			trimmedQuery === "" ||
			[instance.name, instance.source_host ?? "", instance.source_path]
				.join(" ")
				.toLowerCase()
				.includes(trimmedQuery)

		const matchesEnabled =
			enabledFilter.value === "all" ||
			(enabledFilter.value === "enabled" ? instance.enabled : !instance.enabled)

		return matchesQuery && matchesEnabled
	})
})

const hasConfirmedRelayInstances = computed(() => filteredInstances.value.some((instance) => instance.relay_mode))
const hasPotentialRelayInstances = computed(() => filteredInstances.value.some((instance) => instance.relay_mode_uncertain))

function resetForm(): void {
	form.id = ""
	form.name = ""
	form.sourceType = "local"
	form.sourceHost = ""
	form.sourcePort = "22"
	form.sourceUser = ""
	form.sourceSSHKeyID = ""
	form.sourcePath = ""
	form.excludePatterns = ""
	form.enabled = true
	formError.value = ""
}

function openCreateModal(): void {
	resetForm()
	modalOpen.value = true
}

function openEditModal(instance: InstanceSummary): void {
	form.id = String(instance.id)
	form.name = instance.name
	form.sourceType = instance.source_type
	form.sourceHost = instance.source_host ?? ""
	form.sourcePort = String(instance.source_port)
	form.sourceUser = instance.source_user ?? ""
	form.sourceSSHKeyID = instance.source_ssh_key_id ? String(instance.source_ssh_key_id) : ""
	form.sourcePath = instance.source_path
	form.excludePatterns = instance.exclude_patterns.join("\n")
	form.enabled = instance.enabled
	formError.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}

	modalOpen.value = false
}

function buildPayload(): CreateInstancePayload | UpdateInstancePayload {
	const payload: CreateInstancePayload | UpdateInstancePayload = {
		name: form.name.trim(),
		source_type: form.sourceType,
		source_path: form.sourcePath.trim(),
		exclude_patterns: splitLines(form.excludePatterns),
		enabled: form.enabled,
	}

	if (form.sourceType === "remote") {
		payload.source_host = form.sourceHost.trim()
		payload.source_port = Number.parseInt(form.sourcePort, 10) || 22
		payload.source_user = form.sourceUser.trim()
		payload.source_ssh_key_id = form.sourceSSHKeyID !== "" ? Number.parseInt(form.sourceSSHKeyID, 10) : null
	}

	return payload
}

async function loadData(): Promise<void> {
	errorMessage.value = ""
	isLoading.value = true

	try {
		const [instanceItems, sshKeyItems] = await Promise.all([
			listInstances(),
			listSSHKeys().catch((error: unknown) => {
				if (error instanceof ApiError && error.status === 403) {
					return []
				}

				return []
			}),
		])
		instances.value = instanceItems
		sshKeys.value = sshKeyItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载实例列表失败。"
	} finally {
		isLoading.value = false
	}
}

async function submitForm(): Promise<void> {
	formError.value = ""
	successMessage.value = ""
	isSubmitting.value = true

	try {
		if (form.id === "") {
			await createInstance(buildPayload())
			successMessage.value = "实例已创建。"
		} else {
			await updateInstance(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "实例已更新。"
		}

		modalOpen.value = false
		resetForm()
		await loadData()
	} catch (error) {
		formError.value = error instanceof ApiError ? error.message : "保存实例失败。"
	} finally {
		isSubmitting.value = false
	}
}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header">
			<div>
				<h1 class="page-header__title">备份实例</h1>
				<p class="page-header__subtitle">管理源路径、源主机和实例级恢复入口。</p>
			</div>
			<AppButton @click="openCreateModal">新建实例</AppButton>
		</header>

		<AppNotification v-if="errorMessage" title="实例列表加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="实例已保存" tone="success" :description="successMessage" />
		<AppNotification
			v-if="hasConfirmedRelayInstances"
			title="中继模式"
			tone="warning"
			description="存在源与目标均为远程主机的实例组合。恢复或滚动同步时将使用本机缓存目录中继，请确认磁盘空间。"
		/>
		<AppNotification
			v-if="hasPotentialRelayInstances"
			title="可能经过中继缓存"
			tone="warning"
			description="当前账户无法读取目标类型。若远程源绑定了 SSH 目标，恢复或滚动同步会经过本机缓存目录，请预留磁盘空间。"
		/>

		<AppCard title="实例列表" description="支持按名称、主机或路径筛选。">
			<div class="page-form-grid">
				<AppFormField label="搜索">
					<AppInput v-model="query" placeholder="名称 / 主机 / 路径" />
				</AppFormField>
				<AppFormField label="启用状态">
					<AppSelect
						v-model="enabledFilter"
						:options="[
							{ value: 'all', label: '全部' },
							{ value: 'enabled', label: '已启用' },
							{ value: 'disabled', label: '已停用' },
						]"
					/>
				</AppFormField>
			</div>

			<AppTable
				:rows="filteredInstances"
				:columns="[
					{ key: 'name', label: '实例' },
					{ key: 'source_path', label: '源路径' },
					{ key: 'strategy_count', label: '策略' },
					{ key: 'last_backup_status', label: '最近状态' },
					{ key: 'enabled', label: '启用' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-name="{ row }">
					<div class="instances-list__name-cell">
						<RouterLink class="instances-list__detail-link" :to="`/instances/${row.id}`">{{ row.name }}</RouterLink>
						<AppTag v-if="row.relay_mode" tone="warning">中继模式</AppTag>
						<AppTag v-else-if="row.relay_mode_uncertain" tone="warning">可能中继</AppTag>
					</div>
				</template>
				<template #cell-source_path="{ row }">
					<div class="instances-list__source-cell">
						<span>{{ formatSource(row.source_type, row.source_path, row.source_host) }}</span>
						<span class="page-muted">{{ formatDateTime(row.updated_at) }}</span>
					</div>
				</template>
				<template #cell-strategy_count="{ value }">
					<span>{{ value }} 条策略</span>
				</template>
				<template #cell-last_backup_status="{ row }">
					<div class="instances-list__status-cell">
						<AppTag :tone="statusTone(row.last_backup_status)">{{ formatStatusLabel(row.last_backup_status) }}</AppTag>
						<span class="page-muted">{{ formatDateTime(row.last_backup_at) }}</span>
					</div>
				</template>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? "已启用" : "已停用" }}</AppTag>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<RouterLink class="instances-list__detail-link" :to="`/instances/${row.id}`">详情</RouterLink>
					</div>
				</template>
			</AppTable>
		</AppCard>

		<AppModal
			:open="modalOpen"
			:close-on-overlay="!isSubmitting"
			labelled-by="instances-list-modal-title"
			width="34rem"
			@close="closeModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="modalTitleId" class="page-modal-form__title">{{ form.id === '' ? '新建实例' : '编辑实例' }}</h2>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="prod-web" />
						</AppFormField>

						<AppFormField label="源类型" required>
							<AppSelect
								v-model="form.sourceType"
								:options="[
									{ value: 'local', label: '本地路径' },
									{ value: 'remote', label: '远程主机' },
								]"
							/>
						</AppFormField>

						<AppFormField label="源路径" required>
							<AppInput v-model="form.sourcePath" placeholder="/srv/data" />
						</AppFormField>

						<AppFormField label="启用实例">
							<AppSwitch v-model="form.enabled" />
						</AppFormField>
					</div>

					<div v-if="form.sourceType === 'remote'" class="page-form-grid">
						<AppFormField label="源主机" required>
							<AppInput v-model="form.sourceHost" placeholder="192.0.2.10" />
						</AppFormField>

						<AppFormField label="端口">
							<AppInput v-model="form.sourcePort" inputmode="numeric" />
						</AppFormField>

						<AppFormField label="源用户" required>
							<AppInput v-model="form.sourceUser" placeholder="backup" />
						</AppFormField>

						<AppFormField label="SSH 密钥">
							<AppSelect v-model="form.sourceSSHKeyID" :options="sshKeyOptions" />
						</AppFormField>
					</div>

					<AppFormField label="排除模式" description="每行一个模式，例如 node_modules 或 *.tmp。">
						<textarea v-model="form.excludePatterns" class="instances-list__textarea" rows="4" />
					</AppFormField>

					<AppNotification v-if="formError" title="保存失败" tone="danger" :description="formError" />

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? "创建实例" : "保存修改" }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>
	</section>
</template>

<style scoped>
.instances-list__name-cell,
.instances-list__source-cell,
.instances-list__status-cell {
	display: grid;
	gap: var(--space-2);
}

.instances-list__detail-link {
	color: var(--text-strong);
	font-weight: 700;
	text-decoration: none;
}

.instances-list__detail-link:hover,
.instances-list__detail-link:focus-visible {
	text-decoration: underline;
	text-decoration-thickness: 2px;
	text-underline-offset: 0.18rem;
}

.instances-list__textarea {
	width: 100%;
	min-height: 7rem;
	padding: 0.82rem 0.96rem;
	border: var(--border-width) solid var(--control-border);
	border-radius: var(--radius-control);
	background: var(--control-bg);
	color: var(--control-text);
	font: inherit;
	line-height: 1.5;
	resize: vertical;
}

.instances-list__textarea:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring), var(--control-shadow-focus);
	border-color: var(--control-border-hover);
}
</style>