<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"

import { ApiError } from "../../api/client"
import { createStrategy, deleteStrategy, listStrategies, updateStrategy } from "../../api/strategies"
import { listStorageTargets } from "../../api/storageTargets"
import type { StorageTargetSummary, StrategyPayload, StrategySummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppSwitch from "../../components/ui/AppSwitch.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatBackupType, formatSchedule } from "../../utils/formatters"

const props = defineProps<{ instanceId: number }>()

const strategies = ref<StrategySummary[]>([])
const storageTargets = ref<StorageTargetSummary[]>([])
const errorMessage = ref("")
const formError = ref("")
const successMessage = ref("")
const modalOpen = ref(false)
const deleteDialogOpen = ref(false)
const deleteStrategyId = ref<number | null>(null)
const deleteStrategyName = ref("")
const isSubmitting = ref(false)
const formModalTitleId = "strategies-tab-modal-title"

const form = reactive({
	id: "",
	name: "",
	backupType: "rolling",
	scheduleMode: "interval",
	cronExpr: "",
	intervalSeconds: "3600",
	retentionDays: "7",
	retentionCount: "3",
	coldVolumeSize: "",
	maxExecutionSeconds: "0",
	enabled: true,
})

const selectedTargetIds = ref<string[]>([])

const compatibleTargets = computed(() =>
	storageTargets.value.filter((target) =>
		form.backupType === "cold" ? target.type.startsWith("cold_") : target.type.startsWith("rolling_"),
	),
)

const compatibleTargetIds = computed(() => new Set(compatibleTargets.value.map((target) => String(target.id))))

function toggleTarget(id: string): void {
	selectedTargetIds.value = selectedTargetIds.value.includes(id)
		? selectedTargetIds.value.filter((item) => item !== id)
		: [...selectedTargetIds.value, id]
}

function resetForm(): void {
	form.id = ""
	form.name = ""
	form.backupType = "rolling"
	form.scheduleMode = "interval"
	form.cronExpr = ""
	form.intervalSeconds = "3600"
	form.retentionDays = "7"
	form.retentionCount = "3"
	form.coldVolumeSize = ""
	form.maxExecutionSeconds = "0"
	form.enabled = true
	selectedTargetIds.value = []
	formError.value = ""
}

function openCreateModal(): void {
	resetForm()
	modalOpen.value = true
}

function openEditModal(strategy: StrategySummary): void {
	form.id = String(strategy.id)
	form.name = strategy.name
	form.backupType = strategy.backup_type
	form.scheduleMode = strategy.cron_expr ? "cron" : "interval"
	form.cronExpr = strategy.cron_expr ?? ""
	form.intervalSeconds = String(strategy.interval_seconds)
	form.retentionDays = String(strategy.retention_days)
	form.retentionCount = String(strategy.retention_count)
	form.coldVolumeSize = strategy.cold_volume_size ?? ""
	form.maxExecutionSeconds = String(strategy.max_execution_seconds)
	form.enabled = strategy.enabled
	selectedTargetIds.value = strategy.storage_target_ids.map((id) => String(id))
	formError.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}

	modalOpen.value = false
}

function openDeleteDialog(strategy: StrategySummary): void {
	deleteStrategyId.value = strategy.id
	deleteStrategyName.value = strategy.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteStrategyId.value = null
	deleteStrategyName.value = ""
}

function buildPayload(): StrategyPayload {
	return {
		name: form.name.trim(),
		backup_type: form.backupType,
		cron_expr: form.scheduleMode === "cron" && form.cronExpr.trim() !== "" ? form.cronExpr.trim() : null,
		interval_seconds: form.scheduleMode === "interval" ? Number.parseInt(form.intervalSeconds, 10) || 0 : 0,
		retention_days: Number.parseInt(form.retentionDays, 10) || 0,
		retention_count: Number.parseInt(form.retentionCount, 10) || 0,
		cold_volume_size: form.backupType === "cold" && form.coldVolumeSize.trim() !== "" ? form.coldVolumeSize.trim() : null,
		max_execution_seconds: Number.parseInt(form.maxExecutionSeconds, 10) || 0,
		storage_target_ids: selectedTargetIds.value.map((id) => Number.parseInt(id, 10)),
		enabled: form.enabled,
	}
}

async function loadData(): Promise<void> {
	errorMessage.value = ""
	try {
		const [strategyItems, storageTargetItems] = await Promise.all([
			listStrategies(props.instanceId),
			listStorageTargets().catch((error: unknown) => {
				if (error instanceof ApiError && error.status === 403) {
					return []
				}

				throw error
			}),
		])
		strategies.value = strategyItems
		storageTargets.value = storageTargetItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载策略失败。"
	}
}

async function submitForm(): Promise<void> {
	formError.value = ""
	successMessage.value = ""
	isSubmitting.value = true

	try {
		if (form.id === "") {
			await createStrategy(props.instanceId, buildPayload())
			successMessage.value = "策略已创建。"
		} else {
			await updateStrategy(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "策略已更新。"
		}

		modalOpen.value = false
		resetForm()
		await loadData()
	} catch (error) {
		formError.value = error instanceof ApiError ? error.message : "保存策略失败。"
	} finally {
		isSubmitting.value = false
	}
}


async function confirmDelete(): Promise<void> {
	if (deleteStrategyId.value === null) {
		return
	}

	try {
		await deleteStrategy(deleteStrategyId.value)
		successMessage.value = `策略「${deleteStrategyName.value}」已删除。`
		closeDeleteDialog()
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除策略失败。"
		closeDeleteDialog()
	}
}

watch(
	() => form.backupType,
	() => {
		selectedTargetIds.value = selectedTargetIds.value.filter((id) => compatibleTargetIds.value.has(id))
	},
)

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="策略加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="策略已保存" tone="success" :description="successMessage" />

		<div class="page-action-row--wrap page-actions-end">
			<AppButton @click="openCreateModal">新建策略</AppButton>
		</div>

		<AppCard title="策略列表" description="每个策略绑定备份类型、调度和目标集合。">
			<AppTable
				:rows="strategies"
				:columns="[
					{ key: 'name', label: '名称' },
					{ key: 'backup_type', label: '类型' },
					{ key: 'schedule', label: '调度' },
					{ key: 'storage_target_ids', label: '目标数' },
					{ key: 'enabled', label: '启用' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-backup_type="{ value }">
					<span>{{ formatBackupType(String(value)) }}</span>
				</template>
				<template #cell-schedule="{ row }">
					<span>{{ formatSchedule(row) }}</span>
				</template>
				<template #cell-storage_target_ids="{ value }">
					<span>{{ value.length }} 个目标</span>
				</template>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? "启用" : "停用" }}</AppTag>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="strategies.length === 0" title="尚未配置策略" compact />
		</AppCard>

		<AppModal
			:open="modalOpen"
			:close-on-overlay="!isSubmitting"
			:labelled-by="formModalTitleId"
			width="36rem"
			@close="closeModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="formModalTitleId" class="page-modal-form__title">{{ form.id === '' ? '新建策略' : '编辑策略' }}</h2>
					<p class="page-muted">滚动与冷备份使用不同的目标类型。</p>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="策略名称" required>
							<AppInput v-model="form.name" />
						</AppFormField>
						<AppFormField label="备份类型">
							<AppSelect
								v-model="form.backupType"
								:options="[
									{ value: 'rolling', label: '滚动备份' },
									{ value: 'cold', label: '冷备份' },
								]"
							/>
						</AppFormField>
						<AppFormField label="启用策略">
							<AppSwitch v-model="form.enabled" />
						</AppFormField>
					</div>

					<div class="page-form-grid">
						<AppFormField label="调度模式">
							<AppSelect
								v-model="form.scheduleMode"
								:options="[
									{ value: 'interval', label: '固定间隔' },
									{ value: 'cron', label: 'Cron 表达式' },
								]"
							/>
						</AppFormField>
						<AppFormField v-if="form.scheduleMode === 'interval'" label="间隔秒数">
							<AppInput v-model="form.intervalSeconds" inputmode="numeric" />
						</AppFormField>
						<AppFormField v-else label="Cron 表达式">
							<AppInput v-model="form.cronExpr" placeholder="0 0 * * *" />
						</AppFormField>
						<AppFormField label="最大执行秒数">
							<AppInput v-model="form.maxExecutionSeconds" inputmode="numeric" />
						</AppFormField>
					</div>

					<div class="page-form-grid">
						<AppFormField label="保留天数">
							<AppInput v-model="form.retentionDays" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="保留数量">
							<AppInput v-model="form.retentionCount" inputmode="numeric" />
						</AppFormField>
						<AppFormField v-if="form.backupType === 'cold'" label="冷备卷大小">
							<AppInput v-model="form.coldVolumeSize" placeholder="1G" />
						</AppFormField>
					</div>

					<div>
						<p class="page-section__title">存储目标</p>
						<p class="page-muted">仅展示与当前备份类型兼容的目标。</p>
						<div class="page-checkbox-list">
							<label v-for="target in compatibleTargets" :key="target.id" class="page-checkbox">
								<input
									type="checkbox"
									:checked="selectedTargetIds.includes(String(target.id))"
									@change="toggleTarget(String(target.id))"
								/>
								<span>{{ target.name }} · {{ target.type }}</span>
							</label>
						</div>
					</div>

					<AppNotification v-if="formError" title="保存失败" tone="danger" :description="formError" />

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? "创建策略" : "保存修改" }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<AppDialog :open="deleteDialogOpen" title="确认删除策略" tone="danger" @close="closeDeleteDialog">
			<p>即将删除策略「{{ deleteStrategyName }}」。若该策略已经产生备份记录，系统会拒绝删除。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>

<style scoped>
.page-actions-end {
	justify-content: flex-end;
}
</style>