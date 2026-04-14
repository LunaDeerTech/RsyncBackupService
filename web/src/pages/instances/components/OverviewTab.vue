<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { getInstanceStats, getDisasterRecovery } from '../../../api/instances'
import { getUpcomingTasks as fetchUpcomingTasksAPI, type UpcomingTask } from '../../../api/dashboard'
import { listTasks, type TaskItem } from '../../../api/tasks'
import { useTaskStore } from '../../../stores/task'
import { useAuthStore } from '../../../stores/auth'
import { useConfirm } from '../../../composables/useConfirm'
import { useElapsedTime } from '../../../composables/useElapsedTime'
import { useCountUp } from '../../../composables/useCountUp'
import { formatBytes } from '../../../utils/format'
import { formatRelativeTime } from '../../../utils/time'
import { getDRLevelLabel, getDRLevelBadgeVariant, getDRLevelRingColor } from '../../../utils/disaster-recovery'
import {
  taskStatusMap, instanceStatusMap, backupTypeMap,
  getStatusConfig,
} from '../../../utils/status-config'
import type { Instance, InstanceStats, DisasterRecoveryScore } from '../../../types/instance'
import AppCard from '../../../components/AppCard.vue'
import AppBadge from '../../../components/AppBadge.vue'
import AppEmpty from '../../../components/AppEmpty.vue'
import AppButton from '../../../components/AppButton.vue'
import AppProgress from '../../../components/AppProgress.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import {
  Database, CheckCircle, HardDrive, Clock,
  AlertTriangle, XCircle,
} from 'lucide-vue-next'

const props = defineProps<{
  instanceId: number
  instance: Instance
}>()

const emit = defineEmits<{
  'change-tab': [tab: string]
}>()

const authStore = useAuthStore()
const taskStore = useTaskStore()
const { confirm } = useConfirm()

const stats = ref<InstanceStats | null>(null)
const drScore = ref<DisasterRecoveryScore | null>(null)

// ── Task data ──
const instanceTasks = ref<TaskItem[]>([])
const instanceUpcoming = ref<UpcomingTask[]>([])
const taskWatcherStoppers = ref<(() => void)[]>([])

// Leading running task for elapsed-time display
const leadingTask = computed(() => instanceTasks.value.find(t => t.status === 'running') ?? null)
const leadingTaskStartTime = computed(() => leadingTask.value?.started_at ?? null)
const elapsedTime = useElapsedTime(leadingTaskStartTime)

// ── Computed ──
const successRate = computed(() => {
  if (!stats.value || stats.value.backup_count === 0) return null
  return Math.round((stats.value.success_backup_count / stats.value.backup_count) * 100)
})

const successRateColor = computed(() => {
  if (successRate.value === null) return 'var(--text-muted)'
  if (successRate.value >= 90) return 'var(--success-500)'
  if (successRate.value >= 70) return 'var(--warning-500)'
  return 'var(--error-500)'
})

// ── Countup animations ──
const animAvailableBackupCount = useCountUp(computed(() => stats.value?.available_backup_count ?? 0))
const animSuccessRate = useCountUp(computed(() => successRate.value ?? 0))
const animDrTotal = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.total) : 0))
const animDrFreshness = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.freshness) : 0))
const animDrRecovery = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.recovery_points) : 0))
const animDrRedundancy = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.redundancy) : 0))
const animDrStability = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.stability) : 0))
const animTotalSizeBytes = useCountUp(computed(() => stats.value?.total_backup_size_bytes ?? 0), 800)
const animTotalDiskBytes = useCountUp(computed(() => stats.value?.total_backup_disk_bytes ?? 0), 800)

const sourceTypeLabel: Record<string, string> = { local: '本地', ssh: 'SSH' }
const policyTypeLabel: Record<string, string> = { rolling: '滚动', cold: '冷备' }

// ── Format helpers ──
function formatEstimatedRemaining(task: TaskItem): string {
  if (!task.started_at || task.progress == null || task.progress <= 0) return '--'
  if (task.progress >= 100) return '即将完成'
  const elapsedMs = Date.now() - new Date(task.started_at).getTime()
  if (elapsedMs <= 0) return '--'
  const totalEstMs = elapsedMs / (task.progress / 100)
  const remainMs = totalEstMs - elapsedMs
  if (remainMs <= 0) return '即将完成'
  const m = Math.floor(remainMs / 60000)
  const h = Math.floor(m / 60)
  if (h > 0) return `约 ${h} 小时 ${m % 60} 分钟`
  if (m > 0) return `约 ${m} 分钟`
  return '不到 1 分钟'
}

function formatEstimatedEnd(task: TaskItem): string {
  if (!task.started_at || task.progress == null || task.progress <= 0) return '--'
  if (task.progress >= 100) return '即将完成'
  const elapsedMs = Date.now() - new Date(task.started_at).getTime()
  if (elapsedMs <= 0) return '--'
  const totalEstMs = elapsedMs / (task.progress / 100)
  const etaDate = new Date(new Date(task.started_at).getTime() + totalEstMs)
  return etaDate.toLocaleString('zh-CN')
}

function formatDateTime(dateStr: string | undefined): string {
  if (!dateStr) return '--'
  return new Date(dateStr).toLocaleString('zh-CN')
}

function formatFutureTime(dateStr: string): string {
  const target = new Date(dateStr)
  const now = new Date()
  const diffMs = target.getTime() - now.getTime()
  if (diffMs <= 0) return '即将执行'
  const minutes = Math.floor(diffMs / 60000)
  const hours = Math.floor(minutes / 60)
  if (hours > 0) return `${hours} 小时 ${minutes % 60} 分钟后`
  return `${minutes} 分钟后`
}

function taskTypeLabel(type: string): string {
  return type === 'rolling' ? '滚动备份' : type === 'cold' ? '冷备份' : type
}

// ── Task watchers ──
function startTaskWatchers() {
  stopTaskWatchers()
  const running = instanceTasks.value.filter(t => t.status === 'running' || t.status === 'queued')
  for (const t of running) {
    const stop = taskStore.watchTask(t.id, (updated) => {
      const idx = instanceTasks.value.findIndex(x => x.id === updated.id)
      if (idx >= 0) {
        instanceTasks.value[idx] = updated
      }
      if (updated.status === 'success' || updated.status === 'failed' || updated.status === 'cancelled') {
        fetchStats()
        fetchDR()
        fetchInstanceTasks()
      }
    })
    taskWatcherStoppers.value.push(stop)
  }
}

function stopTaskWatchers() {
  for (const stop of taskWatcherStoppers.value) stop()
  taskWatcherStoppers.value = []
}

// ── Fetch functions ──
async function fetchStats() {
  try {
    stats.value = await getInstanceStats(props.instanceId)
  } catch {
    // silent
  }
}

async function fetchDR() {
  try {
    drScore.value = await getDisasterRecovery(props.instanceId)
  } catch {
    // silent
  }
}

async function fetchInstanceTasks() {
  try {
    const res = await listTasks()
    instanceTasks.value = (res.items ?? []).filter(t => t.instance_id === props.instanceId)
    startTaskWatchers()
  } catch {
    // silent
  }
}

async function handleCancelTask(taskId: number) {
  const yes = await confirm({ title: '取消任务', message: '确定要取消该任务吗？此操作不可撤销。', confirmText: '取消任务', danger: true })
  if (!yes) return
  await taskStore.doCancelTask(taskId)
  fetchInstanceTasks()
}

async function fetchInstanceUpcoming() {
  try {
    const res = await fetchUpcomingTasksAPI({ within_hours: 168 })
    instanceUpcoming.value = (res.items ?? []).filter(t => t.instance_id === props.instanceId).slice(0, 3)
  } catch {
    // silent
  }
}

function refresh() {
  fetchStats()
  fetchDR()
  fetchInstanceTasks()
  fetchInstanceUpcoming()
}

onMounted(() => {
  refresh()
})

onUnmounted(() => {
  stopTaskWatchers()
})

defineExpose({ refresh })
</script>

<template>
  <div class="tab-content">
    <!-- Row 1: Combined info + DR card & Stats 2×2 -->
    <div class="overview-top-row">
      <!-- Combined Info + DR Card -->
      <AppCard>
        <div class="hero-card">
          <div class="hero-card__top">
            <div class="hero-card__info">
              <div class="overview-info__item">
                <span class="overview-info__label">数据源</span>
                <span class="overview-info__value">{{ sourceTypeLabel[instance.source_type] ?? instance.source_type }}: {{ instance.source_path }}</span>
              </div>
              <div class="overview-info__item">
                <span class="overview-info__label">状态</span>
                <span>
                  <StatusBadge :config="getStatusConfig(instanceStatusMap, instance.status)" />
                </span>
              </div>
              <div v-if="drScore && drScore.deductions && drScore.deductions.length > 0" class="hero-card__deductions">
                <span class="overview-info__label">容灾扣分项</span>
                <div class="hero-deduction-list">
                  <div v-for="(d, i) in drScore.deductions" :key="i" class="hero-deduction-item">
                    <AlertTriangle :size="12" class="hero-deduction-item__icon" />
                    <span>{{ d }}</span>
                  </div>
                </div>
              </div>
            </div>
            <div v-if="drScore" class="hero-card__ring">
              <span class="hero-card__ring-label">容灾评分</span>
              <div class="dr-ring"
                :style="{ '--dr-ring-color': getDRLevelRingColor(drScore.level), '--dr-ring-pct': Math.round(drScore.total) }">
                <svg viewBox="0 0 36 36" class="dr-ring__svg">
                  <path class="dr-ring__bg"
                    d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
                  <path class="dr-ring__fg" :stroke-dasharray="`${animDrTotal}, 100`"
                    d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831" />
                </svg>
                <span class="dr-ring__value">{{ animDrTotal }}</span>
              </div>
              <AppBadge :variant="getDRLevelBadgeVariant(drScore.level)">
                {{ getDRLevelLabel(drScore.level) }}
              </AppBadge>
            </div>
          </div>

          <!-- Sub-scores -->
          <div v-if="drScore" class="dr-sub-scores">
            <div class="dr-sub-score">
              <div class="dr-sub-score__header">
                <span class="dr-sub-score__name">备份新鲜度</span>
                <span class="dr-sub-score__value">{{ animDrFreshness }}</span>
              </div>
              <div class="dr-sub-score__bar">
                <div class="dr-sub-score__fill" :style="{ width: animDrFreshness + '%' }" />
              </div>
            </div>
            <div class="dr-sub-score">
              <div class="dr-sub-score__header">
                <span class="dr-sub-score__name">恢复点可用性</span>
                <span class="dr-sub-score__value">{{ animDrRecovery }}</span>
              </div>
              <div class="dr-sub-score__bar">
                <div class="dr-sub-score__fill" :style="{ width: animDrRecovery + '%' }" />
              </div>
            </div>
            <div class="dr-sub-score">
              <div class="dr-sub-score__header">
                <span class="dr-sub-score__name">冗余与隔离度</span>
                <span class="dr-sub-score__value">{{ animDrRedundancy }}</span>
              </div>
              <div class="dr-sub-score__bar">
                <div class="dr-sub-score__fill" :style="{ width: animDrRedundancy + '%' }" />
              </div>
            </div>
            <div class="dr-sub-score">
              <div class="dr-sub-score__header">
                <span class="dr-sub-score__name">执行稳定性</span>
                <span class="dr-sub-score__value">{{ animDrStability }}</span>
              </div>
              <div class="dr-sub-score__bar">
                <div class="dr-sub-score__fill" :style="{ width: animDrStability + '%' }" />
              </div>
            </div>
          </div>
        </div>
      </AppCard>

      <!-- Stats cards 2×2 -->
      <div class="stats-grid-2x2">
        <AppCard>
          <button class="stat-card stat-card--clickable" @click="emit('change-tab', 'backups')">
            <div class="stat-card__content">
              <span class="stat-card__value">{{ animAvailableBackupCount }}</span>
              <span class="stat-card__label">可用备份</span>
              <span class="stat-card__sub">累计 {{ stats?.backup_count ?? 0 }} 次备份</span>
            </div>
            <Database :size="22" class="stat-icon stat-icon--primary" />
          </button>
        </AppCard>
        <AppCard>
          <button class="stat-card stat-card--clickable" @click="emit('change-tab', 'audit')">
            <div class="stat-card__content">
              <span class="stat-card__value" :style="{ color: successRateColor }">
                {{ successRate !== null ? animSuccessRate + '%' : '--' }}
              </span>
              <span class="stat-card__label">成功率</span>
              <span class="stat-card__sub">成功 {{ stats?.success_backup_count ?? 0 }} / 失败 {{ stats?.failure_backup_count ?? 0 }}</span>
            </div>
            <CheckCircle :size="22" class="stat-icon stat-icon--success" />
          </button>
        </AppCard>
        <AppCard>
          <div class="stat-card">
            <div class="stat-card__content">
              <span class="stat-card__value">{{ formatBytes(animTotalSizeBytes) }}</span>
              <span class="stat-card__label">总备份大小</span>
              <span class="stat-card__sub">实际占用 · {{ formatBytes(animTotalDiskBytes) }}</span>
            </div>
            <HardDrive :size="22" class="stat-icon stat-icon--info" />
          </div>
        </AppCard>
        <AppCard>
          <div class="stat-card">
            <div class="stat-card__content">
              <template v-if="stats?.last_backup">
                <span class="stat-card__value stat-card__value--sm">
                  {{ stats.last_backup.completed_at ? formatRelativeTime(stats.last_backup.completed_at) : '--' }}
                </span>
                <span class="stat-card__label">最近备份</span>
                <span class="stat-card__sub">
                  <StatusBadge :config="getStatusConfig(taskStatusMap, stats.last_backup.status)" size="sm" />
                  {{ policyTypeLabel[stats.last_backup.type] ?? stats.last_backup.type }}
                  · {{ formatBytes(stats.last_backup.backup_size_bytes) }}
                </span>
              </template>
              <template v-else>
                <span class="stat-card__value">--</span>
                <span class="stat-card__label">最近备份</span>
              </template>
            </div>
            <Clock :size="22" class="stat-icon stat-icon--muted" />
          </div>
        </AppCard>
      </div>
    </div>

    <!-- Row 2: Upcoming tasks & Current tasks -->
    <div class="overview-tasks-row">
      <!-- Upcoming tasks for this instance -->
      <AppCard title="即将执行的任务" class="overview-tasks-row__upcoming">
        <div v-if="instanceUpcoming.length === 0" class="py-4">
          <AppEmpty message="暂无计划任务" />
        </div>
        <div v-else class="instance-upcoming-list">
          <div v-for="task in instanceUpcoming" :key="task.policy_id" class="instance-upcoming-item">
            <div class="instance-upcoming-item__info">
              <span class="instance-upcoming-item__name">{{ task.policy_name }}</span>
              <StatusBadge :config="getStatusConfig(backupTypeMap, task.type)" />
            </div>
            <span class="instance-upcoming-item__time">
              <Clock :size="12" />
              {{ formatFutureTime(task.next_run_at) }}
            </span>
          </div>
        </div>
      </AppCard>

      <!-- Current tasks for this instance -->
      <AppCard title="当前任务">
        <div v-if="instanceTasks.length === 0" class="py-4">
          <AppEmpty message="暂无运行中任务" />
        </div>
        <div v-else class="task-progress-list">
          <div v-for="t in instanceTasks" :key="t.id" class="task-progress-card">
            <div class="task-progress-header">
              <div class="task-progress-header__left">
                <StatusBadge :config="getStatusConfig(taskStatusMap, t.status)" />
                <span class="task-progress-header__type">{{ taskTypeLabel(t.type) }}</span>
              </div>
              <AppButton
                v-if="authStore.isAdmin && (t.status === 'running' || t.status === 'queued')"
                variant="danger"
                size="sm"
                @click="handleCancelTask(t.id)"
              >
                <XCircle :size="14" />
                取消
              </AppButton>
            </div>
            <div class="task-progress-body">
              <div class="task-progress-bar-row">
                <AppProgress :value="t.progress" size="md" />
                <span class="task-progress-percent">{{ t.progress }}%</span>
              </div>
              <div v-if="t.current_step" class="task-progress-step">{{ t.current_step }}</div>
              <div class="task-progress-meta">
                <div class="task-progress-meta__item">
                  <span class="task-progress-meta__label">开始时间</span>
                  <span class="task-progress-meta__value">{{ formatDateTime(t.started_at) }}</span>
                </div>
                <div class="task-progress-meta__item">
                  <span class="task-progress-meta__label">已运行</span>
                  <span class="task-progress-meta__value task-progress-meta__value--mono">{{ t.status === 'running' && t.id === leadingTask?.id ? elapsedTime : '--' }}</span>
                </div>
                <div class="task-progress-meta__item">
                  <span class="task-progress-meta__label">预计完成</span>
                  <span class="task-progress-meta__value">{{ formatEstimatedEnd(t) }}</span>
                </div>
                <div class="task-progress-meta__item">
                  <span class="task-progress-meta__label">预计剩余</span>
                  <span class="task-progress-meta__value">{{ formatEstimatedRemaining(t) }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </AppCard>
    </div>
  </div>
</template>

<style scoped>
.tab-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-top: 16px;
}

/* Overview – Top row: hero card + stats 2×2 */
.overview-top-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

@media (max-width: 767px) {
  .overview-top-row {
    grid-template-columns: 1fr;
  }
}

/* Hero card = merged info + DR */
.hero-card {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.hero-card__top {
  display: flex;
  justify-content: space-between;
  gap: 20px;
}

.hero-card__info {
  display: flex;
  flex-direction: column;
  gap: 12px;
  flex: 1;
  min-width: 0;
  max-width: calc(100% - 110px);
}

.hero-card__ring {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.hero-card__ring-label {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.hero-card__deductions {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.hero-deduction-list {
  display: flex;
  flex-direction: column;
  gap: 3px;
}

.hero-deduction-item {
  display: flex;
  align-items: flex-start;
  gap: 4px;
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.4;
}

.hero-deduction-item__icon {
  flex-shrink: 0;
  color: var(--warning-500);
  margin-top: 1px;
}

.overview-info__item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.overview-info__label {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.overview-info__value {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}

/* Stats 2×2 grid */
.stats-grid-2x2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.stats-grid-2x2 .stat-card {
  justify-content: space-between;
  align-items: flex-start;
}

.stats-grid-2x2 .stat-icon {
  width: 28px;
  height: 28px;
  opacity: 0.7;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 12px;
}

.stat-card--clickable {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 12px;
  background: none;
  border: none;
  padding: 0;
  cursor: pointer;
  text-align: left;
}

.stat-card--clickable:hover .stat-card__label {
  color: var(--primary-500);
}

.stat-card__content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.stat-card__value {
  font-size: 26px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.3;
}

.stat-card__value--sm {
  font-size: 18px;
}

.stat-card__label {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 2px;
}

.stat-card__sub {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.stat-icon {
  flex-shrink: 0;
}

.stat-icon--primary {
  color: var(--primary-500);
}

.stat-icon--success {
  color: var(--success-500);
}

.stat-icon--info {
  color: var(--primary-600);
}

.stat-icon--muted {
  color: var(--text-muted);
}

/* Disaster Recovery ring & sub-scores */
.dr-ring {
  position: relative;
  width: 80px;
  height: 80px;
  flex-shrink: 0;
}

.dr-ring__svg {
  width: 100%;
  height: 100%;
  transform: rotate(-90deg);
}

.dr-ring__bg {
  fill: none;
  stroke: var(--border-default);
  stroke-width: 3;
}

.dr-ring__fg {
  fill: none;
  stroke: var(--dr-ring-color);
  stroke-width: 3;
  stroke-linecap: round;
  transition: stroke-dasharray 0.6s ease;
}

.dr-ring__value {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 20px;
  font-weight: 700;
  color: var(--text-primary);
}

.dr-sub-scores {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}

.dr-sub-score__header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}

.dr-sub-score__name {
  font-size: 13px;
  color: var(--text-secondary);
}

.dr-sub-score__value {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.dr-sub-score__bar {
  height: 6px;
  background: var(--surface-sunken);
  border-radius: 3px;
  overflow: hidden;
}

.dr-sub-score__fill {
  height: 100%;
  background: var(--primary-500);
  border-radius: 3px;
  transition: width 0.4s ease;
}

/* Overview tasks row */
.overview-tasks-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  align-items: start;
}

.overview-tasks-row__upcoming {
  align-self: start;
}

@media (max-width: 767px) {
  .overview-tasks-row {
    grid-template-columns: 1fr;
  }
}

.instance-upcoming-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.instance-upcoming-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 13px;
}

.instance-upcoming-item:last-child {
  border-bottom: none;
}

.instance-upcoming-item__info {
  flex: 1;
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.instance-upcoming-item__name {
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.instance-upcoming-item__time {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--text-muted);
  white-space: nowrap;
}

/* Task progress card styles */
.task-progress-list {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.task-progress-card {
  padding: 12px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
}

.task-progress-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: 10px;
}

.task-progress-header__left {
  display: flex;
  align-items: center;
  gap: 8px;
}

.task-progress-header__type {
  font-weight: 600;
  font-size: 14px;
  color: var(--text-primary);
}

.task-progress-body {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.task-progress-bar-row {
  display: flex;
  align-items: center;
  gap: 12px;
}

.task-progress-bar-row .app-progress {
  flex: 1;
}

.task-progress-percent {
  font-weight: 700;
  font-size: 14px;
  color: var(--text-primary);
  white-space: nowrap;
  min-width: 42px;
  text-align: right;
}

.task-progress-step {
  font-size: 13px;
  color: var(--text-secondary);
  padding: 6px 10px;
  background: var(--surface-sunken);
  border-radius: var(--radius-sm);
}

.task-progress-meta {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 8px 16px;
}

.task-progress-meta__item {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.task-progress-meta__label {
  font-size: 12px;
  color: var(--text-muted);
}

.task-progress-meta__value {
  font-size: 13px;
  color: var(--text-primary);
}

.task-progress-meta__value--mono {
  font-family: monospace;
  font-weight: 600;
}

.py-4 {
  padding-top: 16px;
  padding-bottom: 16px;
}
</style>
