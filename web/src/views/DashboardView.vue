<script setup lang="ts">
import { computed, onMounted, ref } from "vue"

import { ApiError } from "../api/client"
import { cancelTask, getDashboard, getSystemStatus } from "../api/system"
import type { DashboardSummary, RunningTaskStatus, SystemStatus } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppProgress from "../components/ui/AppProgress.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTimeline from "../components/ui/AppTimeline.vue"
import AppTag from "../components/ui/AppTag.vue"
import { useRealtimeTasks } from "../composables/useRealtimeTasks"
import {
	formatBackupType,
	formatBytes,
	formatDateTime,
	formatStatusLabel,
	statusTone,
} from "../utils/formatters"

type DashboardTaskRow = {
	taskId: string
	instanceId: number
	storageTargetId?: number
	percentage: number
	speedText: string
	etaText: string
	status: string
}

const dashboard = ref<DashboardSummary | null>(null)
const systemStatus = ref<SystemStatus | null>(null)
const errorMessage = ref("")
const isLoading = ref(true)
const isCancelling = ref<string | null>(null)
const realtime = useRealtimeTasks()

function mapDashboardTask(task: RunningTaskStatus): DashboardTaskRow {
	return {
		taskId: task.task_id,
		instanceId: task.instance_id,
		storageTargetId: task.storage_target_id,
		percentage: task.percentage,
		speedText: task.speed_text,
		etaText: task.remaining_text,
		status: task.status,
	}
}

const taskRows = computed<DashboardTaskRow[]>(() => {
	if (realtime.tasks.value.length > 0) {
		return realtime.tasks.value.map((task) => ({
			taskId: task.taskId,
			instanceId: task.instanceId,
			storageTargetId: task.storageTargetId,
			percentage: task.percentage,
			speedText: task.speedText,
			etaText: task.etaText,
			status: task.status,
		}))
	}

	return (dashboard.value?.running_tasks ?? []).map(mapDashboardTask)
})

const recentBackupItems = computed(() =>
	(dashboard.value?.recent_backups ?? []).map((backup) => ({
		id: backup.id,
		title: `${backup.instance_name} · ${formatBackupType(backup.backup_type)}`,
		description: `状态 ${formatStatusLabel(backup.status)}，目标 ${backup.storage_target_id}`,
		timestamp: formatDateTime(backup.finished_at ?? backup.started_at),
		tone: statusTone(backup.status),
	})),
)

const storageRows = computed(() =>
	(dashboard.value?.storage_overview ?? []).map((item) => ({
		id: item.storage_target_id,
		name: item.storage_target_name,
		type: item.storage_target_type,
		available: formatBytes(item.available_bytes),
		backupCount: item.backup_count,
		lastBackupAt: formatDateTime(item.last_backup_at),
	})),
)

async function loadDashboard(): Promise<void> {
	errorMessage.value = ""
	isLoading.value = true

	try {
		const [summary, status] = await Promise.all([getDashboard(), getSystemStatus()])
		dashboard.value = summary
		systemStatus.value = status
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载仪表盘失败。"
	} finally {
		isLoading.value = false
	}
}

async function handleCancel(taskId: string): Promise<void> {
	isCancelling.value = taskId
	try {
		await cancelTask(taskId)
		await loadDashboard()
		realtime.connect()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "取消任务失败。"
	} finally {
		isCancelling.value = null
	}
}

onMounted(() => {
	void loadDashboard()
	realtime.connect()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">DASHBOARD</p>
				<h1 class="page-header__title">系统概览</h1>
				<p class="page-header__subtitle">监控备份健康、运行任务与容量风险。</p>
			</div>
			<div class="page-header__actions">
				<AppButton variant="secondary" @click="loadDashboard">刷新概览</AppButton>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="仪表盘加载失败" tone="danger" :description="errorMessage" />

		<section class="page-kpi-grid" aria-label="关键指标">
			<AppCard title="实例总数" compact>
				<p class="page-kpi__value">{{ dashboard?.instance_count ?? 0 }}</p>
			</AppCard>
			<AppCard title="今日备份" compact>
				<p class="page-kpi__value">{{ dashboard?.today_backup_count ?? 0 }}</p>
			</AppCard>
			<AppCard title="成功" compact>
				<p class="page-kpi__value">{{ dashboard?.success_count ?? 0 }}</p>
			</AppCard>
			<AppCard title="失败" compact tone="danger">
				<p class="page-kpi__value">{{ dashboard?.failed_count ?? 0 }}</p>
			</AppCard>
		</section>

		<section class="page-summary-grid">
			<AppCard title="运行中任务" description="连接 WebSocket 后，进度、速率和 ETA 会自动刷新。">
				<div v-if="taskRows.length > 0" class="page-stack">
					<article v-for="task in taskRows" :key="task.taskId" class="dashboard-task">
						<div class="dashboard-task__header">
							<div class="dashboard-task__heading">
								<p class="page-section__title">任务 {{ task.taskId }}</p>
								<p class="page-muted">实例 {{ task.instanceId }} · 存储 {{ task.storageTargetId ?? "—" }}</p>
							</div>
							<AppTag :tone="statusTone(task.status)">{{ formatStatusLabel(task.status) }}</AppTag>
						</div>
						<AppProgress
							:percentage="task.percentage"
							:speed-text="task.speedText"
							:eta-text="task.etaText"
							tone="running"
							:aria-label="`任务 ${task.taskId} 进度`"
						>
							<template #label>任务进度</template>
						</AppProgress>
						<div class="page-action-row--wrap">
							<AppButton
								variant="ghost"
								size="sm"
								:loading="isCancelling === task.taskId"
								@click="handleCancel(task.taskId)"
							>
								取消任务
							</AppButton>
						</div>
					</article>
				</div>
				<AppEmpty v-else title="当前没有运行中任务" description="当备份或恢复执行时，这里会显示实时进度。" compact />
			</AppCard>

			<AppCard title="系统状态" description="来自 `/api/system/status` 的实时系统摘要。">
				<dl v-if="systemStatus" class="page-detail-list">
					<div>
						<dt>版本</dt>
						<dd>{{ systemStatus.version }}</dd>
					</div>
					<div>
						<dt>数据目录</dt>
						<dd class="page-mono">{{ systemStatus.data_dir }}</dd>
					</div>
					<div>
						<dt>已运行</dt>
						<dd>{{ systemStatus.uptime_seconds }} 秒</dd>
					</div>
					<div>
						<dt>总容量</dt>
						<dd>{{ formatBytes(systemStatus.disk_total_bytes) }}</dd>
					</div>
					<div>
						<dt>剩余容量</dt>
						<dd>{{ formatBytes(systemStatus.disk_free_bytes) }}</dd>
					</div>
				</dl>
				<AppEmpty v-else-if="!isLoading" title="系统状态不可用" compact />
			</AppCard>
		</section>

		<section class="page-two-column">
			<AppCard title="最近备份" description="按最近完成时间倒序显示。">
				<AppTimeline v-if="recentBackupItems.length > 0" :items="recentBackupItems" compact />
				<AppEmpty v-else-if="!isLoading" title="暂无备份记录" compact />
			</AppCard>

			<AppCard title="存储空间概览" description="汇总每个存储目标的已用情况与最新备份时间。">
				<AppTable
					:rows="storageRows"
					:columns="[
						{ key: 'name', label: '名称' },
						{ key: 'type', label: '类型' },
						{ key: 'available', label: '剩余空间' },
						{ key: 'backupCount', label: '备份数' },
						{ key: 'lastBackupAt', label: '最近备份' },
					]"
					dense
				/>
			</AppCard>
		</section>
	</section>
</template>

<style scoped>
.dashboard-task,
.dashboard-task__heading {
	display: grid;
	gap: var(--space-3);
}

.dashboard-task {
	padding: var(--space-4);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-elevated) 92%, var(--surface-panel-solid));
}

.dashboard-task__header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: var(--space-3);
	flex-wrap: wrap;
}
</style>