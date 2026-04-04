<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"

import { ApiError } from "../../api/client"
import { listBackups } from "../../api/backups"
import { listRunningTasks } from "../../api/system"
import type { BackupRecord, InstanceDetail, RunningTaskStatus, StrategySummary } from "../../api/types"
import AppCard from "../../components/ui/AppCard.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppProgress from "../../components/ui/AppProgress.vue"
import AppTag from "../../components/ui/AppTag.vue"
import AppTimeline from "../../components/ui/AppTimeline.vue"
import {
	formatBackupType,
	formatBytes,
	formatDateTime,
	formatRemainingTime,
	formatSource,
	formatStatusLabel,
	statusTone,
} from "../../utils/formatters"

const props = defineProps<{
	instance: InstanceDetail
	strategies: StrategySummary[]
	canViewRunningTasks: boolean
	relayMode: boolean
	relayModeHint?: string
	relayModeTitle?: string
}>()

const recentBackups = ref<BackupRecord[]>([])
const runningTasks = ref<RunningTaskStatus[]>([])
const isLoading = ref(true)
const runningTasksMessage = ref("")
const now = ref(Date.now())

let clockTimer: ReturnType<typeof setInterval> | null = null

const activeStrategyCount = computed(() => props.strategies.filter((strategy) => strategy.enabled).length)
const totalBackupCount = computed(() => recentBackups.value.length)
const cumulativeBytes = computed(() =>
	recentBackups.value.reduce((sum, record) => sum + record.total_size, 0),
)
const successRate = computed(() => {
	const total = recentBackups.value.length

	if (total === 0) {
		return "—"
	}

	const successes = recentBackups.value.filter((record) => record.status === "success").length
    return `${Math.round((successes / total) * 100)}%`
})
const lastBackup = computed(() => recentBackups.value[0] ?? null)
const instanceTasks = computed(() =>
	runningTasks.value.filter((task) => task.instance_id === props.instance.id),
)
const timelineItems = computed(() =>
	recentBackups.value.slice(0, 10).map((record) => {
		const strategyName = props.strategies.find((strategy) => strategy.id === record.strategy_id)?.name ?? "手动"

		return {
			id: record.id,
			title: `${formatBackupType(record.backup_type)} · ${strategyName} → 目标 #${record.storage_target_id}`,
			description: record.error_message
				? `失败：${record.error_message}`
				: `${formatBytes(record.total_size)}，${record.files_transferred} 个文件`,
			timestamp: formatDateTime(record.finished_at ?? record.started_at),
			tone: statusTone(record.status),
		}
	}),
)
const scheduledItems = computed(() =>
	props.strategies
		.flatMap((strategy) =>
			(strategy.upcoming_runs ?? []).map((runAt, index) => ({
				id: `${strategy.id}-${index}-${runAt}`,
				title: `${strategy.name} · ${formatBackupType(strategy.backup_type)}`,
				description: `剩余 ${formatRemainingTime(runAt, now.value)}`,
				timestamp: formatDateTime(runAt),
				sortValue: Date.parse(runAt),
			})),
		)
		.filter((item) => !Number.isNaN(item.sortValue))
		.filter((item) => item.sortValue > now.value)
		.sort((left, right) => left.sortValue - right.sortValue)
		.slice(0, 5)
		.map(({ sortValue: _sortValue, ...item }) => item),
)

async function loadData(): Promise<void> {
	isLoading.value = true
	runningTasksMessage.value = ""

	const [backupsResult, tasksResult] = await Promise.allSettled([
		listBackups(props.instance.id),
		props.canViewRunningTasks ? listRunningTasks() : Promise.resolve(null),
	])

	if (backupsResult.status === "fulfilled") {
		recentBackups.value = backupsResult.value
	} else {
		recentBackups.value = []
	}

	if (!props.canViewRunningTasks) {
		runningTasks.value = []
		runningTasksMessage.value = "当前账户没有实时任务查看权限。"
	} else if (tasksResult.status === "fulfilled") {
		runningTasks.value = tasksResult.value ?? []
	} else {
		runningTasks.value = []
		runningTasksMessage.value = tasksResult.reason instanceof ApiError && tasksResult.reason.status === 403
			? "当前账户没有实时任务查看权限。"
			: "运行中任务暂时不可用，请稍后刷新。"
	}

	isLoading.value = false
}

watch(
	[() => props.instance.id, () => props.canViewRunningTasks],
	() => {
		void loadData()
	},
	{ immediate: true },
)

onMounted(() => {
	clockTimer = setInterval(() => {
		now.value = Date.now()
	}, 1000)
})

onBeforeUnmount(() => {
	if (clockTimer !== null) {
		clearInterval(clockTimer)
		clockTimer = null
	}
})
</script>

<template>
	<section class="page-view">
		<AppNotification
			v-if="relayMode"
			:title="relayModeTitle || '远程到远程中继'"
			tone="warning"
			:description="relayModeHint || '该实例至少有一个远程源与远程目标的组合。执行链路会落到本机缓存目录，请检查可用磁盘空间。'"
		/>

		<section class="page-kpi-grid" aria-label="实例关键指标">
			<AppCard title="活跃策略" compact>
				<p class="page-kpi__value">{{ activeStrategyCount }}</p>
			</AppCard>
			<AppCard title="备份总数" compact>
				<p class="page-kpi__value">{{ totalBackupCount }}</p>
			</AppCard>
			<AppCard title="累计容量" compact>
				<p class="page-kpi__value">{{ formatBytes(cumulativeBytes) }}</p>
			</AppCard>
			<AppCard title="成功率" compact>
				<p class="page-kpi__value">{{ successRate }}</p>
			</AppCard>
			<AppCard title="最近备份" compact>
				<div v-if="lastBackup" class="overview-tab__last-backup">
					<AppTag :tone="statusTone(lastBackup.status)">{{ formatStatusLabel(lastBackup.status) }}</AppTag>
					<span class="page-muted">{{ formatDateTime(lastBackup.finished_at ?? lastBackup.started_at) }}</span>
				</div>
				<p v-else class="page-kpi__value">—</p>
			</AppCard>
		</section>

		<section class="page-two-column">
			<AppCard title="源配置" description="类型、路径、连接参数和排除规则。">
				<dl class="page-detail-list">
					<div>
						<dt>源类型</dt>
						<dd>{{ instance.source_type === "remote" ? "远程主机" : "本地路径" }}</dd>
					</div>
					<div>
						<dt>源位置</dt>
						<dd>{{ formatSource(instance.source_type, instance.source_path, instance.source_host) }}</dd>
					</div>
					<div v-if="instance.source_type === 'remote'">
						<dt>连接用户</dt>
						<dd>{{ instance.source_user || "—" }}</dd>
					</div>
					<div v-if="instance.source_type === 'remote'">
						<dt>端口</dt>
						<dd>{{ instance.source_port }}</dd>
					</div>
					<div>
						<dt>排除模式</dt>
						<dd>{{ instance.exclude_patterns.length > 0 ? instance.exclude_patterns.join("，") : "无" }}</dd>
					</div>
				</dl>
			</AppCard>

			<AppCard title="当前运行任务" description="该实例正在执行的备份或恢复任务。">
				<div v-if="instanceTasks.length > 0" class="page-stack">
					<article v-for="task in instanceTasks" :key="task.task_id" class="overview-tab__task">
						<div class="overview-tab__task-header">
							<p class="page-section__title">任务 {{ task.task_id }}</p>
							<AppTag :tone="statusTone(task.status)">{{ formatStatusLabel(task.status) }}</AppTag>
						</div>
						<AppProgress
							:percentage="task.percentage"
							:speed-text="task.speed_text"
							:eta-text="task.remaining_text"
							tone="running"
							:aria-label="`任务 ${task.task_id} 进度`"
						>
							<template #label>进度</template>
						</AppProgress>
					</article>
				</div>
				<AppEmpty v-else-if="runningTasksMessage !== ''" title="无法显示运行中任务" :description="runningTasksMessage" compact />
				<AppEmpty v-else title="没有运行中任务" description="当备份或恢复执行时，这里会显示实时进度。" compact />
			</AppCard>
		</section>

		<section class="page-two-column overview-tab__activity-row">
			<AppCard title="计划备份" description="按时间顺序展示未来 5 次计划中的备份启动。">
				<AppTimeline v-if="scheduledItems.length > 0" :items="scheduledItems" compact />
				<AppEmpty
					v-else
					title="暂无计划中的备份"
					description="启用滚动备份策略后，这里会显示接下来 5 次计划运行。"
					compact
				/>
			</AppCard>

			<AppCard title="最近活动" description="最近 10 条备份记录，按完成时间倒序。">
				<AppTimeline v-if="timelineItems.length > 0" :items="timelineItems" compact />
				<AppEmpty v-else-if="!isLoading" title="暂无备份记录" compact />
			</AppCard>
		</section>
	</section>
</template>

<style scoped>
.overview-tab__last-backup {
	display: flex;
	align-items: center;
	gap: var(--space-2);
	flex-wrap: wrap;
}

.overview-tab__task {
	display: grid;
	gap: var(--space-3);
	padding: var(--space-4);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-elevated) 92%, var(--surface-panel-solid));
}

.overview-tab__task-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: var(--space-3);
}

.overview-tab__activity-row {
	align-items: start;
}
</style>