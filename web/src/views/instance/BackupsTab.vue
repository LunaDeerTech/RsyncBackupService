<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"

import { ApiError } from "../../api/client"
import { listSnapshots } from "../../api/backups"
import { listStrategies } from "../../api/strategies"
import type { BackupRecord, StrategySummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatBackupType, formatBytes, formatDateTime, formatStatusLabel, statusTone } from "../../utils/formatters"

const props = defineProps<{ instanceId: number }>()

const backups = ref<BackupRecord[]>([])
const strategies = ref<StrategySummary[]>([])
const errorMessage = ref("")
const isLoading = ref(true)
const filters = reactive({
	backupType: "",
	strategyId: "",
})

async function loadBackupsData(): Promise<void> {
	errorMessage.value = ""
	isLoading.value = true

	try {
		const [backupItems, strategyItems] = await Promise.all([
			listSnapshots(props.instanceId, {
				backup_type: filters.backupType || undefined,
				strategy_id: filters.strategyId === "" ? undefined : Number.parseInt(filters.strategyId, 10),
			}),
			listStrategies(props.instanceId),
		])
		backups.value = backupItems
		strategies.value = strategyItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载备份历史失败。"
	} finally {
		isLoading.value = false
	}
}

onMounted(() => {
	void loadBackupsData()
})
</script>

<template>
	<section class="page-view">
		<AppCard title="备份历史" description="仅显示当前仍存在的快照与归档；历史执行记录不在此处展示。">
			<div class="page-form-grid">
				<AppFormField label="备份类型">
					<AppSelect
						v-model="filters.backupType"
						:options="[
							{ value: '', label: '全部类型' },
							{ value: 'rolling', label: '滚动备份' },
							{ value: 'cold', label: '冷备份' },
						]"
					/>
				</AppFormField>
				<AppFormField label="策略">
					<AppSelect
						v-model="filters.strategyId"
						:options="[
							{ value: '', label: '全部策略' },
							...strategies.map((item) => ({ value: String(item.id), label: item.name })),
						]"
					/>
				</AppFormField>
				<div class="page-action-row--wrap">
					<AppButton variant="secondary" @click="loadBackupsData">刷新列表</AppButton>
				</div>
			</div>

			<AppNotification v-if="errorMessage" title="加载备份历史失败" tone="danger" :description="errorMessage" />

			<AppTable
				:rows="backups"
				:columns="[
					{ key: 'snapshot_path', label: '快照 / 归档' },
					{ key: 'backup_type', label: '类型' },
					{ key: 'total_size', label: '大小' },
					{ key: 'started_at', label: '开始时间' },
				]"
				row-key="id"
			>
				<template #cell-backup_type="{ value }">
					<span>{{ formatBackupType(String(value)) }}</span>
				</template>
				<template #cell-total_size="{ row }">
					<span>
						{{ formatBytes(row.total_size > 0 ? row.total_size : row.bytes_transferred) }}
						<template v-if="row.total_size > 0 && row.bytes_transferred > 0 && row.bytes_transferred !== row.total_size">
							 / {{ formatBytes(row.bytes_transferred) }}
						</template>
					</span>
				</template>
				<template #cell-started_at="{ row }">
					<div class="page-stack">
						<span>{{ formatDateTime(row.started_at) }}</span>
						<span class="page-muted">完成 {{ formatDateTime(row.finished_at) }}</span>
					</div>
				</template>
			</AppTable>

			<AppEmpty v-if="!isLoading && backups.length === 0" title="暂无可用备份" compact />
		</AppCard>
	</section>
</template>