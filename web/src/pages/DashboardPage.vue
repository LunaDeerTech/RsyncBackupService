<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import {
  Play,
  AlertTriangle,
  ShieldAlert,
  Shield,
  HardDrive,
  Server,
  Settings,
  Clock,
  ChevronRight,
} from 'lucide-vue-next'
import AppCard from '../components/AppCard.vue'
import AppBadge from '../components/AppBadge.vue'
import AppEmpty from '../components/AppEmpty.vue'
import {
  getOverview,
  getRisks,
  getTrends,
  getFocusInstances,
  getUpcomingTasks,
  type DashboardOverview,
  type DashboardTrends,
  type DashboardRiskEvent,
  type FocusInstance,
  type UpcomingTask,
} from '../api/dashboard'
import { formatRelativeTime } from '../utils/time'
import { getDRLevelColor, getDRLevelLabel, getDRLevelBadgeVariant } from '../utils/disaster-recovery'

const router = useRouter()

const overview = ref<DashboardOverview | null>(null)
const trends = ref<DashboardTrends | null>(null)
const risks = ref<DashboardRiskEvent[]>([])
const risksTotal = ref(0)
const focusInstances = ref<FocusInstance[]>([])
const upcomingTasks = ref<UpcomingTask[]>([])
const loading = ref(true)

let refreshTimer: ReturnType<typeof setInterval> | null = null

async function loadData() {
  try {
    const [ovRes, trRes, riRes, fiRes, utRes] = await Promise.all([
      getOverview(),
      getTrends(),
      getRisks({ page: 1, page_size: 10 }),
      getFocusInstances(),
      getUpcomingTasks(),
    ])
    overview.value = ovRes
    trends.value = trRes
    risks.value = riRes.items
    risksTotal.value = riRes.total
    focusInstances.value = fiRes
    upcomingTasks.value = utRes.items ?? []
  } catch {
    // Silent fail on refresh
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
  refreshTimer = setInterval(loadData, 30000)
})

onUnmounted(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})

// Trend chart helpers
const maxTrendValue = computed(() => {
  if (!trends.value?.backup_results?.length) return 1
  return Math.max(
    ...trends.value.backup_results.map((d) => d.success + d.failed),
    1,
  )
})

function formatTrendDate(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

// Instance health distribution
const healthTotal = computed(() => {
  if (!trends.value?.instance_health) return 0
  const h = trends.value.instance_health
  return h.safe + h.caution + h.risk + h.danger
})

function healthPercent(count: number): number {
  return healthTotal.value > 0 ? (count / healthTotal.value) * 100 : 0
}

// Upcoming task time
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

function severityVariant(severity: string): 'error' | 'warning' | 'info' | 'default' {
  switch (severity) {
    case 'critical': return 'error'
    case 'high': return 'error'
    case 'medium': return 'warning'
    case 'low': return 'info'
    default: return 'default'
  }
}

function sourceLabel(source: string): string {
  switch (source) {
    case 'backup_failure': return '备份失败'
    case 'target_unreachable': return '目标不可达'
    case 'low_dr_score': return '容灾率低'
    case 'no_recent_backup': return '长期未备份'
    default: return source
  }
}

function backupStatusVariant(status: string): 'success' | 'error' | 'warning' | 'default' {
  switch (status) {
    case 'success': return 'success'
    case 'failed': return 'error'
    case 'running': return 'warning'
    default: return 'default'
  }
}

function backupStatusLabel(status: string): string {
  switch (status) {
    case 'success': return '成功'
    case 'failed': return '失败'
    case 'running': return '运行中'
    default: return status || '无'
  }
}

function taskTypeLabel(type: string): string {
  switch (type) {
    case 'rolling': return '滚动备份'
    case 'cold': return '冷备份'
    default: return type
  }
}

const quickLinks = [
  { label: '实例列表', icon: Server, path: '/instances' },
  { label: '备份目标', icon: HardDrive, path: '/targets' },
  { label: '系统配置', icon: Settings, path: '/system' },
  { label: '风险事件', icon: ShieldAlert, path: '/system/risks' },
]
</script>

<template>
  <div class="dashboard">
    <!-- Loading skeleton -->
    <div v-if="loading" class="dashboard__loading">
      <div class="skeleton-card" v-for="i in 5" :key="i" />
    </div>

    <template v-else>
      <!-- 1. Overview cards -->
      <section class="dashboard__overview">
        <button
          class="overview-card"
          @click="router.push('/instances')"
        >
          <div class="overview-card__icon overview-card__icon--primary">
            <Play :size="20" />
          </div>
          <div class="overview-card__content">
            <span class="overview-card__value">{{ overview?.running_tasks ?? 0 }}</span>
            <span class="overview-card__label">运行中任务</span>
          </div>
          <span v-if="overview?.queued_tasks" class="overview-card__sub">+{{ overview.queued_tasks }} 排队</span>
        </button>

        <button
          class="overview-card"
          @click="router.push('/instances')"
        >
          <div class="overview-card__icon overview-card__icon--warning">
            <AlertTriangle :size="20" />
          </div>
          <div class="overview-card__content">
            <span class="overview-card__value">{{ overview?.abnormal_instances ?? 0 }}</span>
            <span class="overview-card__label">异常实例</span>
          </div>
        </button>

        <button
          class="overview-card"
          @click="router.push('/system/risks')"
        >
          <div class="overview-card__icon overview-card__icon--error">
            <ShieldAlert :size="20" />
          </div>
          <div class="overview-card__content">
            <span class="overview-card__value">{{ overview?.unresolved_risks ?? 0 }}</span>
            <span class="overview-card__label">待处理风险</span>
          </div>
        </button>

        <div class="overview-card overview-card--static">
          <div class="overview-card__icon" :style="{ background: `color-mix(in srgb, ${getDRLevelColor(overview?.system_dr_level ?? '')} 15%, transparent)`, color: getDRLevelColor(overview?.system_dr_level ?? '') }">
            <Shield :size="20" />
          </div>
          <div class="overview-card__content">
            <span class="overview-card__value">{{ overview ? Math.round(overview.system_dr_score) : '-' }}</span>
            <span class="overview-card__label">系统容灾率</span>
          </div>
          <AppBadge v-if="overview?.system_dr_level" :variant="getDRLevelBadgeVariant(overview.system_dr_level)">
            {{ getDRLevelLabel(overview.system_dr_level) }}
          </AppBadge>
        </div>

        <button
          class="overview-card"
          @click="router.push('/targets')"
        >
          <div class="overview-card__icon overview-card__icon--info">
            <HardDrive :size="20" />
          </div>
          <div class="overview-card__content">
            <span class="overview-card__label">目标健康度</span>
          </div>
          <div v-if="overview" class="target-health-badges">
            <span class="health-dot health-dot--success" />
            <span class="health-dot-label">{{ overview.target_health_summary.healthy }}</span>
            <span class="health-dot health-dot--warning" />
            <span class="health-dot-label">{{ overview.target_health_summary.degraded }}</span>
            <span class="health-dot health-dot--error" />
            <span class="health-dot-label">{{ overview.target_health_summary.unreachable }}</span>
          </div>
        </button>
      </section>

      <!-- 2. Middle section: left = tasks + risks, right = trends -->
      <section class="dashboard__middle">
        <div class="dashboard__left">
          <!-- Current tasks -->
          <AppCard title="当前任务">
            <div v-if="(overview?.running_tasks ?? 0) + (overview?.queued_tasks ?? 0) === 0" class="py-4">
              <AppEmpty message="暂无运行中任务" />
            </div>
            <div v-else class="task-summary">
              <div class="task-summary__row">
                <span class="task-summary__label">运行中</span>
                <span class="task-summary__count">{{ overview?.running_tasks ?? 0 }}</span>
              </div>
              <div class="task-summary__row">
                <span class="task-summary__label">排队中</span>
                <span class="task-summary__count">{{ overview?.queued_tasks ?? 0 }}</span>
              </div>
            </div>
          </AppCard>

          <!-- Risks -->
          <AppCard title="风险提醒">
            <div v-if="risks.length === 0" class="py-4">
              <AppEmpty message="暂无未解决风险" />
            </div>
            <div v-else class="risk-list">
              <div v-for="risk in risks" :key="risk.id" class="risk-item">
                <AppBadge :variant="severityVariant(risk.severity)">{{ risk.severity }}</AppBadge>
                <span class="risk-item__source">{{ sourceLabel(risk.source) }}</span>
                <span class="risk-item__target">{{ risk.instance_name || risk.target_name }}</span>
                <span class="risk-item__msg">{{ risk.message }}</span>
                <span class="risk-item__time">{{ formatRelativeTime(risk.created_at) }}</span>
              </div>
              <router-link
                v-if="risksTotal > 10"
                to="/system/risks"
                class="risk-list__more"
              >
                查看全部 ({{ risksTotal }})
                <ChevronRight :size="14" />
              </router-link>
            </div>
          </AppCard>
        </div>

        <div class="dashboard__right">
          <!-- Backup trend chart -->
          <AppCard title="备份结果趋势（近 7 天）">
            <div v-if="!trends?.backup_results?.length" class="py-4">
              <AppEmpty message="暂无备份数据" />
            </div>
            <div v-else class="trend-chart">
              <div class="trend-chart__bars">
                <div
                  v-for="day in trends!.backup_results"
                  :key="day.date"
                  class="trend-chart__col"
                >
                  <div class="trend-chart__bar-group">
                    <div
                      class="trend-chart__bar trend-chart__bar--success"
                      :style="{ height: `${(day.success / maxTrendValue) * 100}%` }"
                      :title="`成功: ${day.success}`"
                    >
                      <span v-if="day.success > 0" class="trend-chart__bar-label">{{ day.success }}</span>
                    </div>
                    <div
                      class="trend-chart__bar trend-chart__bar--error"
                      :style="{ height: `${(day.failed / maxTrendValue) * 100}%` }"
                      :title="`失败: ${day.failed}`"
                    >
                      <span v-if="day.failed > 0" class="trend-chart__bar-label">{{ day.failed }}</span>
                    </div>
                  </div>
                  <span class="trend-chart__date">{{ formatTrendDate(day.date) }}</span>
                </div>
              </div>
              <div class="trend-chart__legend">
                <span class="trend-chart__legend-item"><span class="dot dot--success" /> 成功</span>
                <span class="trend-chart__legend-item"><span class="dot dot--error" /> 失败</span>
              </div>
            </div>
          </AppCard>

          <!-- Instance health distribution -->
          <AppCard title="实例健康分布">
            <div v-if="healthTotal === 0" class="py-4">
              <AppEmpty message="暂无实例数据" />
            </div>
            <div v-else class="health-dist">
              <div class="health-dist__bar">
                <div class="health-dist__seg health-dist__seg--safe" :style="{ width: healthPercent(trends!.instance_health.safe) + '%' }" />
                <div class="health-dist__seg health-dist__seg--caution" :style="{ width: healthPercent(trends!.instance_health.caution) + '%' }" />
                <div class="health-dist__seg health-dist__seg--risk" :style="{ width: healthPercent(trends!.instance_health.risk) + '%' }" />
                <div class="health-dist__seg health-dist__seg--danger" :style="{ width: healthPercent(trends!.instance_health.danger) + '%' }" />
              </div>
              <div class="health-dist__legend">
                <span class="health-dist__item"><span class="dot dot--safe" /> 安全 {{ trends!.instance_health.safe }}</span>
                <span class="health-dist__item"><span class="dot dot--caution" /> 注意 {{ trends!.instance_health.caution }}</span>
                <span class="health-dist__item"><span class="dot dot--risk" /> 风险 {{ trends!.instance_health.risk }}</span>
                <span class="health-dist__item"><span class="dot dot--danger" /> 危险 {{ trends!.instance_health.danger }}</span>
              </div>
            </div>
          </AppCard>

          <!-- Upcoming tasks -->
          <AppCard title="即将执行的任务">
            <div v-if="upcomingTasks.length === 0" class="py-4">
              <AppEmpty message="暂无计划任务" />
            </div>
            <div v-else class="upcoming-list">
              <div v-for="task in upcomingTasks" :key="task.policy_id" class="upcoming-item">
                <div class="upcoming-item__info">
                  <span class="upcoming-item__name">{{ task.instance_name }}</span>
                  <span class="upcoming-item__policy">{{ task.policy_name }}</span>
                </div>
                <AppBadge :variant="task.type === 'cold' ? 'info' : 'default'">{{ taskTypeLabel(task.type) }}</AppBadge>
                <span class="upcoming-item__time">
                  <Clock :size="12" />
                  {{ formatFutureTime(task.next_run_at) }}
                </span>
              </div>
            </div>
          </AppCard>
        </div>
      </section>

      <!-- 3. Focus instances -->
      <section v-if="focusInstances.length > 0" class="dashboard__focus">
        <h3 class="dashboard__section-title">重点关注实例</h3>
        <div class="focus-cards">
          <button
            v-for="inst in focusInstances"
            :key="inst.id"
            class="focus-card"
            @click="router.push(`/instances/${inst.id}`)"
          >
            <div class="focus-card__header">
              <span class="focus-card__name">{{ inst.name }}</span>
              <AppBadge :variant="getDRLevelBadgeVariant(inst.dr_level)">{{ getDRLevelLabel(inst.dr_level) }}</AppBadge>
            </div>
            <div class="focus-card__score" :style="{ color: getDRLevelColor(inst.dr_level) }">
              {{ Math.round(inst.dr_score) }}
              <span class="focus-card__score-unit">分</span>
            </div>
            <div class="focus-card__meta">
              <span>风险 {{ inst.unresolved_risks }}</span>
              <span>·</span>
              <AppBadge :variant="backupStatusVariant(inst.last_backup_status)">
                {{ backupStatusLabel(inst.last_backup_status) }}
              </AppBadge>
              <span v-if="inst.last_backup_time" class="focus-card__time">{{ formatRelativeTime(inst.last_backup_time) }}</span>
            </div>
          </button>
        </div>
      </section>

      <!-- 4. Quick links -->
      <section class="dashboard__quick">
        <h3 class="dashboard__section-title">快捷入口</h3>
        <div class="quick-links">
          <button
            v-for="link in quickLinks"
            :key="link.path"
            class="quick-link"
            @click="router.push(link.path)"
          >
            <component :is="link.icon" :size="20" />
            <span>{{ link.label }}</span>
          </button>
        </div>
      </section>
    </template>
  </div>
</template>

<style scoped>
.dashboard {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

/* Loading skeleton */
.dashboard__loading {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
}
.skeleton-card {
  height: 96px;
  border-radius: var(--radius-lg);
  background: var(--surface-raised);
  border: 1px solid var(--border-subtle);
  animation: pulse 1.5s ease-in-out infinite;
}
@keyframes pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.5; }
}

/* Overview cards */
.dashboard__overview {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 16px;
}
@media (max-width: 1023px) {
  .dashboard__overview {
    grid-template-columns: repeat(3, 1fr);
  }
}
@media (max-width: 767px) {
  .dashboard__overview {
    grid-template-columns: repeat(2, 1fr);
  }
  .dashboard__loading {
    grid-template-columns: repeat(2, 1fr);
  }
}

.overview-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  text-align: left;
  transition: box-shadow var(--transition-fast), border-color var(--transition-fast);
}
.overview-card--static {
  cursor: default;
}
.overview-card:not(.overview-card--static):hover {
  box-shadow: var(--shadow-md);
  border-color: color-mix(in srgb, var(--primary-500) 30%, var(--border-default));
}

.overview-card__icon {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
}
.overview-card__icon--primary {
  background: color-mix(in srgb, var(--primary-500) 15%, transparent);
  color: var(--primary-500);
}
.overview-card__icon--warning {
  background: color-mix(in srgb, var(--warning-500) 15%, transparent);
  color: var(--warning-500);
}
.overview-card__icon--error {
  background: color-mix(in srgb, var(--error-500) 15%, transparent);
  color: var(--error-500);
}
.overview-card__icon--info {
  background: color-mix(in srgb, var(--info-500) 15%, transparent);
  color: var(--info-500);
}

.overview-card__content {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.overview-card__value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
  color: var(--text-primary);
}
.overview-card__label {
  font-size: 13px;
  color: var(--text-secondary);
}
.overview-card__sub {
  font-size: 12px;
  color: var(--text-muted);
}

.target-health-badges {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}
.health-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.health-dot--success { background: var(--success-500); }
.health-dot--warning { background: var(--warning-500); }
.health-dot--error { background: var(--error-500); }
.health-dot-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  margin-right: 4px;
}

/* Middle section */
.dashboard__middle {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 24px;
}
@media (max-width: 1023px) {
  .dashboard__middle {
    grid-template-columns: 1fr;
  }
}
.dashboard__left,
.dashboard__right {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

/* Task summary */
.task-summary {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.task-summary__row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
}
.task-summary__row:last-child { border-bottom: none; }
.task-summary__label {
  font-size: 14px;
  color: var(--text-secondary);
}
.task-summary__count {
  font-size: 18px;
  font-weight: 700;
  color: var(--text-primary);
}

/* Risk list */
.risk-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.risk-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 13px;
  flex-wrap: wrap;
}
.risk-item:last-child { border-bottom: none; }
.risk-item__source {
  color: var(--text-secondary);
  white-space: nowrap;
}
.risk-item__target {
  color: var(--text-primary);
  font-weight: 500;
  white-space: nowrap;
}
.risk-item__msg {
  color: var(--text-secondary);
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.risk-item__time {
  color: var(--text-muted);
  white-space: nowrap;
  margin-left: auto;
}
.risk-list__more {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding-top: 8px;
  font-size: 13px;
  color: var(--primary-500);
  text-decoration: none;
  transition: color var(--transition-fast);
}
.risk-list__more:hover {
  color: var(--primary-600);
}

/* Trend chart */
.trend-chart {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.trend-chart__bars {
  display: flex;
  align-items: flex-end;
  gap: 8px;
  height: 160px;
}
.trend-chart__col {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  height: 100%;
}
.trend-chart__bar-group {
  flex: 1;
  display: flex;
  align-items: flex-end;
  gap: 3px;
  width: 100%;
}
.trend-chart__bar {
  flex: 1;
  min-height: 2px;
  border-radius: 3px 3px 0 0;
  transition: height var(--transition-normal);
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: center;
}
.trend-chart__bar--success { background: var(--success-500); }
.trend-chart__bar--error { background: var(--error-500); }
.trend-chart__bar-label {
  font-size: 10px;
  font-weight: 600;
  color: var(--text-primary);
  position: absolute;
  top: -16px;
}
.trend-chart__date {
  font-size: 11px;
  color: var(--text-muted);
  white-space: nowrap;
}
.trend-chart__legend {
  display: flex;
  gap: 16px;
  justify-content: center;
}
.trend-chart__legend-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

/* Dots */
.dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  border-radius: 50%;
}
.dot--success { background: var(--success-500); }
.dot--error { background: var(--error-500); }
.dot--safe { background: var(--success-500); }
.dot--caution { background: var(--warning-500); }
.dot--risk { background: var(--error-500); }
.dot--danger { background: var(--error-600, var(--error-500)); }

/* Health distribution */
.health-dist {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.health-dist__bar {
  display: flex;
  height: 20px;
  border-radius: 10px;
  overflow: hidden;
  background: var(--surface-sunken);
}
.health-dist__seg {
  transition: width var(--transition-normal);
}
.health-dist__seg--safe { background: var(--success-500); }
.health-dist__seg--caution { background: var(--warning-500); }
.health-dist__seg--risk { background: var(--error-500); }
.health-dist__seg--danger { background: var(--error-600, var(--error-500)); }
.health-dist__legend {
  display: flex;
  gap: 16px;
  flex-wrap: wrap;
}
.health-dist__item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: var(--text-secondary);
}

/* Upcoming list */
.upcoming-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.upcoming-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-subtle);
  font-size: 13px;
}
.upcoming-item:last-child { border-bottom: none; }
.upcoming-item__info {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
}
.upcoming-item__name {
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.upcoming-item__policy {
  font-size: 12px;
  color: var(--text-muted);
}
.upcoming-item__time {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  color: var(--text-muted);
  white-space: nowrap;
}

/* Section titles */
.dashboard__section-title {
  margin: 0 0 12px;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

/* Focus cards */
.dashboard__focus {
  display: flex;
  flex-direction: column;
}
.focus-cards {
  display: flex;
  gap: 16px;
  overflow-x: auto;
  padding-bottom: 4px;
}
.focus-card {
  flex: 0 0 220px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 16px;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  text-align: left;
  transition: box-shadow var(--transition-fast), border-color var(--transition-fast);
}
.focus-card:hover {
  box-shadow: var(--shadow-md);
  border-color: color-mix(in srgb, var(--primary-500) 30%, var(--border-default));
}
.focus-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}
.focus-card__name {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.focus-card__score {
  font-size: 32px;
  font-weight: 700;
  line-height: 1.2;
}
.focus-card__score-unit {
  font-size: 14px;
  font-weight: 400;
  opacity: 0.7;
}
.focus-card__meta {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-secondary);
  flex-wrap: wrap;
}
.focus-card__time {
  color: var(--text-muted);
}

/* Quick links */
.dashboard__quick {
  display: flex;
  flex-direction: column;
}
.quick-links {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 12px;
}
@media (max-width: 767px) {
  .quick-links {
    grid-template-columns: repeat(2, 1fr);
  }
}
.quick-link {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 14px 16px;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-sm);
  cursor: pointer;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  transition: box-shadow var(--transition-fast), border-color var(--transition-fast), color var(--transition-fast);
}
.quick-link:hover {
  box-shadow: var(--shadow-md);
  border-color: color-mix(in srgb, var(--primary-500) 30%, var(--border-default));
  color: var(--primary-500);
}

/* Utilities */
.py-4 { padding-top: 16px; padding-bottom: 16px; }
</style>