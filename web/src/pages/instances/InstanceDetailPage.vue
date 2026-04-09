<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getInstance, getInstanceStats, getDisasterRecovery, updateInstance, updateInstancePermissions, listInstancePermissions } from '../../api/instances'
import { listPolicies, createPolicy, updatePolicy, deletePolicy, triggerPolicy } from '../../api/policies'
import { listBackups, restoreBackup, downloadBackup } from '../../api/backups'
import type { RestoreRequest } from '../../api/backups'
import { listInstanceAuditLogs } from '../../api/audit'
import { getUpcomingTasks as fetchUpcomingTasksAPI, type UpcomingTask } from '../../api/dashboard'
import { listTasks, type TaskItem } from '../../api/tasks'
import { useTaskStore } from '../../stores/task'
import { useElapsedTime } from '../../composables/useElapsedTime'
import { useCountUp } from '../../composables/useCountUp'
import AppProgress from '../../components/AppProgress.vue'
import type { AuditLog, AuditLogParams } from '../../api/audit'
import { listTargets } from '../../api/targets'
import { listRemotes } from '../../api/remotes'
import { listUsers } from '../../api/users'
import { useAuthStore } from '../../stores/auth'
import { useToastStore } from '../../stores/toast'
import { useConfirm } from '../../composables/useConfirm'
import { ApiBusinessError } from '../../api/client'
import { formatBytes } from '../../utils/format'
import { EXCLUDE_PATTERN_HELP_EXAMPLES, excludePatternsToText, normalizeExcludePatternsInput } from '../../utils/exclude-patterns'
import { formatRelativeTime } from '../../utils/time'
import { formatScheduleValue } from '../../utils/schedule'
import { getActionLabel, actionOptions, formatAuditDetail, getActionBadgeVariant } from '../../utils/audit'
import type { Instance, InstanceStats, Backup, UpdateInstanceRequest, PermissionItem, DisasterRecoveryScore } from '../../types/instance'
import type { Policy, CreatePolicyRequest, UpdatePolicyRequest } from '../../types/policy'
import type { BackupTarget } from '../../types/target'
import type { RemoteConfig } from '../../types/remote'
import type { User } from '../../types/auth'
import type { TableColumn } from '../../components/AppTable.vue'
import AppTabs from '../../components/AppTabs.vue'
import AppTable from '../../components/AppTable.vue'
import AppModal from '../../components/AppModal.vue'
import AppFormGroup from '../../components/AppFormGroup.vue'
import AppFormItem from '../../components/AppFormItem.vue'
import AppInput from '../../components/AppInput.vue'
import AppSelect from '../../components/AppSelect.vue'
import AppButton from '../../components/AppButton.vue'
import AppBadge from '../../components/AppBadge.vue'
import AppCard from '../../components/AppCard.vue'
import AppSwitch from '../../components/AppSwitch.vue'
import AppEmpty from '../../components/AppEmpty.vue'
import AppConfirm from '../../components/AppConfirm.vue'
import AppPagination from '../../components/AppPagination.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import { getDRLevelLabel, getDRLevelBadgeVariant, getDRLevelRingColor } from '../../utils/disaster-recovery'
import {
  taskStatusMap, instanceStatusMap, backupTypeMap,
  getStatusConfig,
} from '../../utils/status-config'
import {
  ArrowLeft, Play, Plus, Pencil, Trash2, Save,
  Database, CheckCircle, HardDrive, Download, RotateCcw,
  AlertTriangle, Clock, XCircle, CircleHelp,
} from 'lucide-vue-next'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const toast = useToastStore()
const { confirm } = useConfirm()
const instanceId = computed(() => Number(route.params.id))

// ── Tabs ──
const tabs = computed(() => {
  const items = [
    { key: 'overview', label: '概览' },
    { key: 'policies', label: '策略' },
    { key: 'backups', label: '备份' },
    { key: 'audit', label: '审计' },
  ]
  if (authStore.isAdmin) {
    items.push({ key: 'settings', label: '设置' })
  }
  return items
})
const activeTab = ref('overview')

// ── Instance data ──
const instance = ref<Instance | null>(null)
const stats = ref<InstanceStats | null>(null)
const drScore = ref<DisasterRecoveryScore | null>(null)
const viewerPermission = ref<string | undefined>(undefined)
const canDownload = computed(() => authStore.isAdmin || viewerPermission.value === 'readdownload')
const pageLoading = ref(false)

// ── Policy data ──
const policies = ref<Policy[]>([])
const policyLoading = ref(false)
const policyModalVisible = ref(false)
const policyEditing = ref(false)
const policyEditingId = ref<number | null>(null)
const policySubmitting = ref(false)
const targets = ref<BackupTarget[]>([])

const intervalUnitOptions = [
  { label: '秒', value: 'seconds' },
  { label: '分钟', value: 'minutes' },
  { label: '小时', value: 'hours' },
  { label: '天', value: 'days' },
]

const scheduleOptions = [
  { label: '固定间隔', value: 'interval' },
  { label: '每天凌晨2点', value: '0 2 * * *' },
  { label: '每天凌晨4点', value: '0 4 * * *' },
  { label: '每周一凌晨3点', value: '0 3 * * 1' },
  { label: '每月1号凌晨2点', value: '0 2 1 * *' },
  { label: '自定义 Cron', value: 'cron_custom' },
]

const policyForm = reactive({
  name: '',
  type: 'rolling' as 'rolling' | 'cold',
  target_id: undefined as number | undefined,
  schedule_mode: 'interval' as string,
  schedule_input: '',
  interval_value: undefined as number | undefined,
  interval_unit: 'hours' as string,
  enabled: true,
  compression: false,
  encryption: false,
  encryption_key: '',
  split_enabled: false,
  split_size_mb: undefined as number | undefined,
  retention_type: 'count' as 'time' | 'count',
  retention_value: 7,
})

const policyErrors = reactive({
  name: '',
  target_id: '',
  schedule_input: '',
  encryption_key: '',
  split_size_mb: '',
  retention_value: '',
})

// ── Backup data (full list) ──
const backups = ref<Backup[]>([])
const backupLoading = ref(false)
const backupPage = ref(1)
const backupTotal = ref(0)
const backupPageSize = 20
const backupDetailTarget = ref<Backup | null>(null)
const backupDetailVisible = ref(false)

// ── Restore modal ──
const restoreModalVisible = ref(false)
const restoreSubmitting = ref(false)
const restoreBackupTarget = ref<Record<string, unknown> | null>(null)
const restoreError = ref('')
const restoreForm = reactive({
  restore_type: 'source' as 'source' | 'custom',
  target_path: '',
  instance_name: '',
  password: '',
  encryption_key: '',
})
const restoreFormErrors = reactive({
  target_path: '',
  instance_name: '',
  password: '',
  encryption_key: '',
})

// ── Download state ──
const downloadingBackupId = ref<number | null>(null)

// ── Audit ──
const auditLogs = ref<AuditLog[]>([])
const auditLoading = ref(false)
const auditPage = ref(1)
const auditPageSize = ref(20)
const auditTotal = ref(0)
const auditStartDate = ref('')
const auditEndDate = ref('')
const auditAction = ref<string | number>('')

const auditColumns: TableColumn[] = [
  { key: 'created_at', title: '时间' },
  { key: 'action', title: '操作类型' },
  { key: 'user', title: '操作人' },
  { key: 'detail', title: '详情' },
]

// ── Settings ──
const settingsForm = reactive({
  name: '',
  source_type: 'local' as 'local' | 'ssh',
  source_path: '',
  exclude_patterns_text: '',
  remote_config_id: undefined as number | undefined,
})
const settingsErrors = reactive({ name: '', source_path: '', remote_config_id: '' })
const settingsSubmitting = ref(false)
const remotes = ref<RemoteConfig[]>([])
const users = ref<User[]>([])
// ── Permission state ──
interface PermUserEntry { user_id: number; name: string; email: string; permission: string }
const permEntries = ref<PermUserEntry[]>([])
const permissionSaving = ref(false)
const addPermVisible = ref(false)
const addPermSearch = ref('')
const addPermSelected = ref<number | null>(null)
const addPermLevel = ref<string>('readonly')
const addPermSaving = ref(false)
const filteredAddPermUsers = computed(() => {
  const existingIds = new Set(permEntries.value.map(e => e.user_id))
  const available = users.value.filter(u => !existingIds.has(u.id))
  const q = addPermSearch.value.trim().toLowerCase()
  if (!q) return available
  return available.filter(u => u.name.toLowerCase().includes(q) || u.email.toLowerCase().includes(q))
})

// ── Computed: policy form helpers ──
const policyTypeOptions = [
  { label: '滚动备份', value: 'rolling' },
  { label: '冷备份', value: 'cold' },
]

// scheduleOptions defined above with intervalUnitOptions

const retentionTypeOptions = [
  { label: '按时间（天）', value: 'time' },
  { label: '按数量（条）', value: 'count' },
]

const filteredTargetOptions = computed(() => {
  return targets.value
    .filter((t) => t.backup_type === policyForm.type)
    .map((t) => ({ label: t.name, value: t.id }))
})

const sourceTypeOptions = [
  { label: '本地', value: 'local' },
  { label: 'SSH', value: 'ssh' },
]

const remoteOptions = computed(() =>
  remotes.value.map((r) => ({ label: r.name, value: r.id })),
)

const policyTypeLabel: Record<string, string> = { rolling: '滚动', cold: '冷备' }

const policyColumns: TableColumn[] = [
  { key: 'name', title: '名称' },
  { key: 'type', title: '类型' },
  { key: 'target_name', title: '目标' },
  { key: 'schedule', title: '调度' },
  { key: 'enabled', title: '启用' },
  { key: 'last_execution', title: '上次执行' },
  { key: 'actions', title: '操作', width: '140px' },
]

// backupStatusVariant/backupStatusLabel removed – using StatusBadge with taskStatusMap

const backupColumns: TableColumn[] = [
  { key: 'started_at', title: '备份时间' },
  { key: 'completed_at', title: '完成时间' },
  { key: 'type', title: '类型' },
  { key: 'backup_size_bytes', title: '备份大小' },
  { key: 'actual_size_bytes', title: '数据原始大小' },
  { key: 'duration_seconds', title: '持续时间' },
  { key: 'actions', title: '操作', width: '200px' },
]

// backupStatusLabel removed – using StatusBadge now

const excludePatternHelpText = EXCLUDE_PATTERN_HELP_EXAMPLES.join('\n')

// ── Task data (for overview) ──
const taskStore = useTaskStore()
const instanceTasks = ref<TaskItem[]>([])
const instanceUpcoming = ref<UpcomingTask[]>([])
const taskWatcherStoppers = ref<(() => void)[]>([])

// Leading running task for elapsed-time display
const leadingTask = computed(() => instanceTasks.value.find(t => t.status === 'running') ?? null)
const leadingTaskStartTime = computed(() => leadingTask.value?.started_at ?? null)
const elapsedTime = useElapsedTime(leadingTaskStartTime)

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
        // Refresh related data
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

// ── Fetch core data ──
async function fetchInstance() {
  pageLoading.value = true
  try {
    const res = await getInstance(instanceId.value)
    instance.value = res.instance
    stats.value = res.stats
    viewerPermission.value = res.permission
  } catch {
    toast.error('加载实例详情失败')
    router.push('/instances')
  } finally {
    pageLoading.value = false
  }
}

async function fetchStats() {
  try {
    stats.value = await getInstanceStats(instanceId.value)
  } catch {
    // silent
  }
}

async function fetchDR() {
  try {
    drScore.value = await getDisasterRecovery(instanceId.value)
  } catch {
    // silent
  }
}

async function fetchPolicies() {
  policyLoading.value = true
  try {
    const res = await listPolicies(instanceId.value)
    policies.value = Array.isArray(res) ? res : (res.items ?? [])
  } catch {
    toast.error('加载策略列表失败')
  } finally {
    policyLoading.value = false
  }
}

async function fetchTargets() {
  try {
    const res = await listTargets({ page: 1, page_size: 200 })
    targets.value = res.items ?? []
  } catch {
    // silent
  }
}

async function fetchRemotes() {
  try {
    const res = await listRemotes({ page: 1, page_size: 100 })
    remotes.value = res.items ?? []
  } catch {
    // silent
  }
}

async function fetchUsers() {
  try {
    const res = await listUsers({ page: 1, page_size: 200 })
    users.value = (res.items ?? []).filter((u: User) => u.role === 'viewer')
  } catch {
    // silent
  }
}

async function fetchPermissions() {
  try {
    const res = await listInstancePermissions(instanceId.value)
    const perms = res.permissions ?? []
    const userMap = new Map(users.value.map(u => [u.id, u]))
    permEntries.value = perms
      .filter(p => userMap.has(p.user_id))
      .map(p => {
        const u = userMap.get(p.user_id)!
        return { user_id: p.user_id, name: u.name, email: u.email, permission: p.permission }
      })
  } catch {
    // silent
  }
}

async function fetchAuditLogs() {
  auditLoading.value = true
  try {
    const params: AuditLogParams = {
      page: auditPage.value,
      page_size: auditPageSize.value,
    }
    if (auditStartDate.value) params.start_date = auditStartDate.value
    if (auditEndDate.value) params.end_date = auditEndDate.value
    if (auditAction.value) params.action = String(auditAction.value)
    const res = await listInstanceAuditLogs(instanceId.value, params)
    auditLogs.value = res.items ?? []
    auditTotal.value = res.total
  } catch {
    toast.error('加载审计日志失败')
  } finally {
    auditLoading.value = false
  }
}

async function fetchInstanceTasks() {
  try {
    const res = await listTasks()
    instanceTasks.value = (res.items ?? []).filter(t => t.instance_id === instanceId.value)
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
    const res = await fetchUpcomingTasksAPI()
    instanceUpcoming.value = (res.items ?? []).filter(t => t.instance_id === instanceId.value)
  } catch {
    // silent
  }
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

// taskStatusLabel removed – using StatusBadge now

onMounted(async () => {
  await fetchInstance()
  fetchPolicies()
  fetchDR()
  fetchInstanceTasks()
  fetchInstanceUpcoming()
  if (authStore.isAdmin) {
    fetchTargets()
    fetchRemotes()
    fetchUsers().then(() => fetchPermissions())
  }
})

onUnmounted(() => {
  stopTaskWatchers()
})

// ── Watch tab changes ──
watch(activeTab, (tab) => {
  if (tab === 'overview') { fetchStats(); fetchDR(); fetchInstanceTasks(); fetchInstanceUpcoming() }
  if (tab === 'policies') fetchPolicies()
  if (tab === 'backups') { fetchBackups(); fetchPolicies() }
  if (tab === 'audit') fetchAuditLogs()
  if (tab === 'settings' && instance.value) {
    settingsForm.name = instance.value.name
    settingsForm.source_type = instance.value.source_type
    settingsForm.source_path = instance.value.source_path
    settingsForm.exclude_patterns_text = excludePatternsToText(instance.value.exclude_patterns)
    settingsForm.remote_config_id = instance.value.remote_config_id
  }
})

// ── Overview computed ──
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
const animSuccessCount = useCountUp(computed(() => stats.value?.success_backup_count ?? 0))
const animSuccessRate = useCountUp(computed(() => successRate.value ?? 0))
const animDrTotal = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.total) : 0))
const animDrFreshness = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.freshness) : 0))
const animDrRecovery = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.recovery_points) : 0))
const animDrRedundancy = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.redundancy) : 0))
const animDrStability = useCountUp(computed(() => drScore.value ? Math.round(drScore.value.stability) : 0))
const animTotalSizeBytes = useCountUp(computed(() => stats.value?.total_backup_size_bytes ?? 0), 800)

const sourceTypeLabel: Record<string, string> = { local: '本地', ssh: 'SSH' }

// ── Helper: target name lookup ──
function getTargetName(targetId: number): string {
  const t = targets.value.find((t) => t.id === targetId)
  return t?.name ?? `#${targetId}`
}

// ── Policy CRUD ──
function resetPolicyForm() {
  policyForm.name = ''
  policyForm.type = 'rolling'
  policyForm.target_id = undefined
  policyForm.schedule_mode = 'interval'
  policyForm.schedule_input = ''
  policyForm.interval_value = undefined
  policyForm.interval_unit = 'hours'
  policyForm.enabled = true
  policyForm.compression = false
  policyForm.encryption = false
  policyForm.encryption_key = ''
  policyForm.split_enabled = false
  policyForm.split_size_mb = undefined
  policyForm.retention_type = 'count'
  policyForm.retention_value = 7
  Object.keys(policyErrors).forEach((k) => (policyErrors as Record<string, string>)[k] = '')
}

function openCreatePolicy() {
  resetPolicyForm()
  policyEditing.value = false
  policyEditingId.value = null
  policyModalVisible.value = true
}

function openEditPolicy(row: Record<string, unknown>) {
  resetPolicyForm()
  policyEditing.value = true
  policyEditingId.value = row.id as number
  policyForm.name = row.name as string
  policyForm.type = row.type as 'rolling' | 'cold'
  policyForm.target_id = row.target_id as number
  const scheduleType = row.schedule_type as string
  const scheduleValue = row.schedule_value as string
  if (scheduleType === 'interval') {
    policyForm.schedule_mode = 'interval'
    const secs = parseInt(scheduleValue, 10)
    if (!isNaN(secs) && secs > 0) {
      if (secs >= 86400 && secs % 86400 === 0) {
        policyForm.interval_value = secs / 86400
        policyForm.interval_unit = 'days'
      } else if (secs >= 3600 && secs % 3600 === 0) {
        policyForm.interval_value = secs / 3600
        policyForm.interval_unit = 'hours'
      } else if (secs >= 60 && secs % 60 === 0) {
        policyForm.interval_value = secs / 60
        policyForm.interval_unit = 'minutes'
      } else {
        policyForm.interval_value = secs
        policyForm.interval_unit = 'seconds'
      }
    }
  } else {
    const preset = scheduleOptions.find(p => p.value !== 'interval' && p.value !== 'cron_custom' && p.value === scheduleValue)
    if (preset) {
      policyForm.schedule_mode = preset.value
    } else {
      policyForm.schedule_mode = 'cron_custom'
      policyForm.schedule_input = scheduleValue
    }
  }
  policyForm.enabled = row.enabled as boolean
  policyForm.compression = row.compression as boolean
  policyForm.encryption = row.encryption as boolean
  policyForm.encryption_key = ''
  policyForm.split_enabled = row.split_enabled as boolean
  policyForm.split_size_mb = row.split_size_mb as number | undefined
  policyForm.retention_type = row.retention_type as 'time' | 'count'
  policyForm.retention_value = row.retention_value as number
  policyModalVisible.value = true
}

// Reset target when policy type changes
watch(() => policyForm.type, () => {
  if (!policyEditing.value) {
    policyForm.target_id = undefined
  }
  if (policyForm.type === 'rolling') {
    policyForm.compression = false
    policyForm.encryption = false
    policyForm.encryption_key = ''
    policyForm.split_enabled = false
    policyForm.split_size_mb = undefined
  }
})

function validatePolicyForm(): boolean {
  let valid = true
  Object.keys(policyErrors).forEach((k) => (policyErrors as Record<string, string>)[k] = '')

  if (!policyForm.name.trim()) {
    policyErrors.name = '名称不能为空'
    valid = false
  }
  if (!policyForm.target_id) {
    policyErrors.target_id = '请选择目标'
    valid = false
  }
  if (policyForm.schedule_mode === 'interval') {
    if (!policyForm.interval_value || policyForm.interval_value <= 0) {
      policyErrors.schedule_input = '请输入有效的间隔值'
      valid = false
    }
  } else if (policyForm.schedule_mode === 'cron_custom') {
    if (!policyForm.schedule_input.trim()) {
      policyErrors.schedule_input = '请输入 Cron 表达式'
      valid = false
    }
  }
  if (policyForm.encryption && !policyEditing.value && !policyForm.encryption_key.trim()) {
    policyErrors.encryption_key = '请输入加密密钥'
    valid = false
  }
  if (policyForm.split_enabled && (!policyForm.split_size_mb || policyForm.split_size_mb <= 0)) {
    policyErrors.split_size_mb = '请输入有效的分卷大小'
    valid = false
  }
  if (!policyForm.retention_value || policyForm.retention_value <= 0) {
    policyErrors.retention_value = '保留值必须大于 0'
    valid = false
  }
  return valid
}

async function handlePolicySubmit() {
  if (!validatePolicyForm()) return

  policySubmitting.value = true
  try {
    let scheduleType: 'interval' | 'cron'
    let scheduleValue: string
    if (policyForm.schedule_mode === 'interval') {
      scheduleType = 'interval'
      const unitMultiplier: Record<string, number> = { seconds: 1, minutes: 60, hours: 3600, days: 86400 }
      const multiplier = unitMultiplier[policyForm.interval_unit] ?? 1
      scheduleValue = String((policyForm.interval_value ?? 0) * multiplier)
    } else if (policyForm.schedule_mode === 'cron_custom') {
      scheduleType = 'cron'
      scheduleValue = policyForm.schedule_input.trim()
    } else {
      scheduleType = 'cron'
      scheduleValue = policyForm.schedule_mode
    }

    const data: CreatePolicyRequest = {
      name: policyForm.name.trim(),
      type: policyForm.type,
      target_id: policyForm.target_id!,
      schedule_type: scheduleType,
      schedule_value: scheduleValue,
      enabled: policyForm.enabled,
      compression: policyForm.compression,
      encryption: policyForm.encryption,
      encryption_key: policyForm.encryption ? policyForm.encryption_key : undefined,
      split_enabled: policyForm.split_enabled,
      split_size_mb: policyForm.split_enabled ? policyForm.split_size_mb : undefined,
      retention_type: policyForm.retention_type,
      retention_value: policyForm.retention_value,
    }

    if (policyEditing.value && policyEditingId.value !== null) {
      await updatePolicy(instanceId.value, policyEditingId.value, data as UpdatePolicyRequest)
      toast.success('策略已更新')
    } else {
      await createPolicy(instanceId.value, data)
      toast.success('策略已创建')
    }
    policyModalVisible.value = false
    await fetchPolicies()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('操作失败')
    }
  } finally {
    policySubmitting.value = false
  }
}

async function handleDeletePolicy(row: Record<string, unknown>) {
  const ok = await confirm({
    title: '删除策略',
    message: `确定要删除「${row.name}」策略吗？此操作不可撤销。`,
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return
  try {
    await deletePolicy(instanceId.value, row.id as number)
    toast.success('策略已删除')
    await fetchPolicies()
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('删除失败')
  }
}

async function handleTriggerPolicy(row: Record<string, unknown>) {
  const ok = await confirm({
    title: '手动触发',
    message: `确定要手动触发「${row.name}」策略吗？`,
    confirmText: '触发',
  })
  if (!ok) return
  try {
    await triggerPolicy(instanceId.value, row.id as number)
    toast.success('任务已创建')
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('触发失败')
  }
}

async function handleTogglePolicy(row: Record<string, unknown>, enabled: boolean) {
  try {
    const data: UpdatePolicyRequest = {
      name: row.name as string,
      type: row.type as 'rolling' | 'cold',
      target_id: row.target_id as number,
      schedule_type: row.schedule_type as 'interval' | 'cron',
      schedule_value: row.schedule_value as string,
      enabled,
      compression: row.compression as boolean,
      encryption: row.encryption as boolean,
      split_enabled: row.split_enabled as boolean,
      split_size_mb: row.split_size_mb as number | undefined,
      retention_type: row.retention_type as 'time' | 'count',
      retention_value: row.retention_value as number,
    }
    await updatePolicy(instanceId.value, row.id as number, data)
    await fetchPolicies()
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('更新失败')
  }
}

// ── Fetch backups ──
async function fetchBackups() {
  backupLoading.value = true
  try {
    const res = await listBackups(instanceId.value, { page: backupPage.value, page_size: backupPageSize })
    backups.value = res.items ?? []
    backupTotal.value = res.total ?? 0
  } catch {
    toast.error('加载备份列表失败')
  } finally {
    backupLoading.value = false
  }
}

// ── Backup detail modal ──
function openBackupDetail(row: Record<string, unknown>) {
  backupDetailTarget.value = row as unknown as Backup
  backupDetailVisible.value = true
}

// ── Check if backup is encrypted cold ──
function isEncryptedCold(row: Record<string, unknown>): boolean {
  if (row.type !== 'cold') return false
  const policy = policies.value.find((p) => p.id === (row.policy_id as number))
  return !!policy?.encryption
}

// ── Restore ──
function openRestoreModal(row: Record<string, unknown>) {
  restoreBackupTarget.value = row
  restoreForm.restore_type = 'source'
  restoreForm.target_path = ''
  restoreForm.instance_name = ''
  restoreForm.password = ''
  restoreForm.encryption_key = ''
  restoreError.value = ''
  Object.keys(restoreFormErrors).forEach((k) => (restoreFormErrors as Record<string, string>)[k] = '')
  restoreModalVisible.value = true
}

const restoreSubmitDisabled = computed(() => {
  if (!instance.value) return true
  return restoreForm.instance_name !== instance.value.name
})

function validateRestoreForm(): boolean {
  let valid = true
  Object.keys(restoreFormErrors).forEach((k) => (restoreFormErrors as Record<string, string>)[k] = '')

  if (restoreForm.restore_type === 'custom' && !restoreForm.target_path.trim()) {
    restoreFormErrors.target_path = '目标路径不能为空'
    valid = false
  }
  if (!restoreForm.instance_name.trim()) {
    restoreFormErrors.instance_name = '请输入实例名称'
    valid = false
  }
  if (!restoreForm.password.trim()) {
    restoreFormErrors.password = '请输入密码'
    valid = false
  }
  if (restoreBackupTarget.value && isEncryptedCold(restoreBackupTarget.value) && !restoreForm.encryption_key.trim()) {
    restoreFormErrors.encryption_key = '加密备份需要提供密钥'
    valid = false
  }
  return valid
}

async function handleRestoreSubmit() {
  if (!validateRestoreForm()) return
  if (restoreSubmitDisabled.value) return
  if (!restoreBackupTarget.value) return

  restoreSubmitting.value = true
  restoreError.value = ''
  try {
    const data: RestoreRequest = {
      restore_type: restoreForm.restore_type,
      instance_name: restoreForm.instance_name,
      password: restoreForm.password,
    }
    if (restoreForm.restore_type === 'custom') {
      data.target_path = restoreForm.target_path.trim()
    }
    if (isEncryptedCold(restoreBackupTarget.value)) {
      data.encryption_key = restoreForm.encryption_key
    }
    await restoreBackup(instanceId.value, restoreBackupTarget.value.id as number, data)
    toast.success('恢复任务已创建')
    restoreModalVisible.value = false
    activeTab.value = 'overview'
    fetchStats()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      restoreError.value = e.message
    } else {
      restoreError.value = '恢复操作失败'
    }
  } finally {
    restoreSubmitting.value = false
  }
}

// ── Download ──
async function handleDownload(row: Record<string, unknown>) {
  downloadingBackupId.value = row.id as number
  try {
    const res = await downloadBackup(instanceId.value, row.id as number)
    const url = res.url
    const a = document.createElement('a')
    a.href = url
    a.style.display = 'none'
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('获取下载链接失败')
  } finally {
    downloadingBackupId.value = null
  }
}

// ── Settings ──
function validateSettings(): boolean {
  let valid = true
  settingsErrors.name = ''
  settingsErrors.source_path = ''
  settingsErrors.remote_config_id = ''

  if (!settingsForm.name.trim()) {
    settingsErrors.name = '名称不能为空'
    valid = false
  }
  if (!settingsForm.source_path.trim()) {
    settingsErrors.source_path = '路径不能为空'
    valid = false
  }
  if (settingsForm.source_type === 'ssh' && !settingsForm.remote_config_id) {
    settingsErrors.remote_config_id = '请选择远程配置'
    valid = false
  }
  return valid
}

async function handleSaveSettings() {
  if (!validateSettings()) return

  settingsSubmitting.value = true
  try {
    const excludePatterns = normalizeExcludePatternsInput(settingsForm.exclude_patterns_text)
    const data: UpdateInstanceRequest = {
      name: settingsForm.name.trim(),
      source_type: settingsForm.source_type,
      source_path: settingsForm.source_path.trim(),
      exclude_patterns: excludePatterns.length > 0 ? excludePatterns : undefined,
      remote_config_id: settingsForm.source_type === 'ssh' ? settingsForm.remote_config_id : undefined,
    }
    const updated = await updateInstance(instanceId.value, data)
    instance.value = updated
    settingsForm.exclude_patterns_text = excludePatternsToText(updated.exclude_patterns)
    toast.success('实例信息已更新')
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('保存失败')
  } finally {
    settingsSubmitting.value = false
  }
}

async function savePermissions() {
  permissionSaving.value = true
  try {
    const permissions: PermissionItem[] = permEntries.value.map(e => ({ user_id: e.user_id, permission: e.permission }))
    await updateInstancePermissions(instanceId.value, permissions)
    toast.success('权限已更新')
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('保存失败')
  } finally {
    permissionSaving.value = false
  }
}

async function handleAddPermission() {
  if (!addPermSelected.value) return
  addPermSaving.value = true
  try {
    const user = users.value.find(u => u.id === addPermSelected.value)
    if (!user) return
    permEntries.value.push({ user_id: user.id, name: user.name, email: user.email, permission: addPermLevel.value })
    await savePermissions()
    addPermVisible.value = false
    addPermSearch.value = ''
    addPermSelected.value = null
    addPermLevel.value = 'readonly'
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('添加失败')
  } finally {
    addPermSaving.value = false
  }
}

async function handleRemovePermission(userId: number) {
  permEntries.value = permEntries.value.filter(e => e.user_id !== userId)
  await savePermissions()
}

async function handleChangePermission(userId: number, permission: string) {
  const entry = permEntries.value.find(e => e.user_id === userId)
  if (entry) {
    entry.permission = permission
    await savePermissions()
  }
}

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${h} 小时 ${m} 分`
}

const permissionOptions = [
  { label: '只读', value: 'readonly' },
  { label: '只读+下载', value: 'readdownload' },
]

</script>

<template>
  <div class="instance-detail-page">
    <!-- Header -->
    <div class="instance-detail-page__header">
      <AppButton variant="ghost" size="sm" @click="router.push('/instances')">
        <ArrowLeft :size="16" style="margin-right: 4px" />
        返回
      </AppButton>
      <h2 v-if="instance" class="instance-detail-page__title">{{ instance.name }}</h2>
    </div>

    <!-- Loading state -->
    <div v-if="pageLoading" class="instance-detail-page__loading">加载中…</div>

    <!-- Content -->
    <template v-if="!pageLoading && instance">
      <AppTabs :tabs="tabs" :active-key="activeTab" @update:active-key="activeTab = $event">
        <!-- ═══ Overview Tab ═══ -->
        <template #tab-overview>
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
                  <button class="stat-card stat-card--clickable" @click="activeTab = 'backups'">
                    <div class="stat-card__content">
                      <span class="stat-card__value">{{ animSuccessCount }}</span>
                      <span class="stat-card__label">可用备份</span>
                      <span class="stat-card__sub">共 {{ stats?.backup_count ?? 0 }} 次备份</span>
                    </div>
                    <Database :size="22" class="stat-icon stat-icon--primary" />
                  </button>
                </AppCard>
                <AppCard>
                  <button class="stat-card stat-card--clickable" @click="activeTab = 'audit'">
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

        <!-- ═══ Policies Tab ═══ -->
        <template #tab-policies>
          <div class="tab-content">
            <div class="tab-header" v-if="authStore.isAdmin">
              <AppButton variant="primary" size="sm" @click="openCreatePolicy">
                <Plus :size="16" style="margin-right: 4px" />
                新增策略
              </AppButton>
            </div>

            <div class="tab-table">
              <AppTable :columns="policyColumns" :data="policies" :loading="policyLoading">
                <template #cell-type="{ row }">
                  <StatusBadge :config="getStatusConfig(backupTypeMap, row.type as string)" />
                </template>

                <template #cell-target_name="{ row }">
                  {{ getTargetName(row.target_id as number) }}
                </template>

                <template #cell-schedule="{ row }">
                  {{ formatScheduleValue(row.schedule_type as string, row.schedule_value as string) }}
                </template>

                <template #cell-enabled="{ row }">
                  <AppSwitch :model-value="row.enabled as boolean" :disabled="!authStore.isAdmin"
                    @update:model-value="handleTogglePolicy(row, $event)" />
                </template>

                <template #cell-last_execution="{ row }">
                  <template v-if="row.last_execution_time">
                    <StatusBadge :config="getStatusConfig(taskStatusMap, row.last_execution_status as string)" />
                    <span class="last-exec-time">{{ formatRelativeTime(row.last_execution_time as string) }}</span>
                  </template>
                  <span v-else class="text-muted">未执行</span>
                </template>

                <template #cell-actions="{ row }">
                  <div class="actions-cell" v-if="authStore.isAdmin">
                    <AppButton variant="ghost" size="sm" @click="openEditPolicy(row)">
                      <Pencil :size="14" />
                    </AppButton>
                    <AppButton variant="ghost" size="sm" @click="handleTriggerPolicy(row)">
                      <Play :size="14" />
                    </AppButton>
                    <AppButton variant="ghost" size="sm" @click="handleDeletePolicy(row)">
                      <Trash2 :size="14" class="text-error" />
                    </AppButton>
                  </div>
                </template>
              </AppTable>
            </div>
          </div>
        </template>

        <!-- ═══ Backups Tab ═══ -->
        <template #tab-backups>
          <div class="tab-content">
            <div class="tab-table">
              <AppTable :columns="backupColumns" :data="(backups as unknown as Record<string, unknown>[])"
                :loading="backupLoading">
                <template #cell-started_at="{ row }">
                  {{ row.started_at ? formatRelativeTime(row.started_at as string) : '--' }}
                </template>

                <template #cell-completed_at="{ row }">
                  {{ row.completed_at ? formatRelativeTime(row.completed_at as string) : '--' }}
                </template>

                <template #cell-type="{ row }">
                  <StatusBadge :config="getStatusConfig(backupTypeMap, row.type as string)" />
                </template>

                <template #cell-backup_size_bytes="{ row }">
                  {{ formatBytes(row.backup_size_bytes as number) }}
                </template>

                <template #cell-actual_size_bytes="{ row }">
                  {{ formatBytes(row.actual_size_bytes as number) }}
                </template>

                <template #cell-duration_seconds="{ row }">
                  {{ formatDuration(row.duration_seconds as number) }}
                </template>

                <template #cell-actions="{ row }">
                  <div class="actions-cell">
                    <AppButton variant="ghost" size="sm" @click="openBackupDetail(row)">
                      详情
                    </AppButton>
                    <AppButton v-if="authStore.isAdmin" variant="ghost" size="sm" @click="openRestoreModal(row)">
                      <RotateCcw :size="14" style="margin-right: 2px" />
                      恢复
                    </AppButton>
                    <AppButton v-if="row.type === 'cold' && canDownload" variant="ghost" size="sm"
                      :loading="downloadingBackupId === (row.id as number)" @click="handleDownload(row)">
                      <Download :size="14" style="margin-right: 2px" />
                      下载
                    </AppButton>
                  </div>
                </template>
              </AppTable>
            </div>

            <!-- Pagination -->
            <div v-if="backupTotal > backupPageSize" class="backup-pagination">
              <AppButton variant="outline" size="sm" :disabled="backupPage <= 1" @click="backupPage--; fetchBackups()">
                上一页
              </AppButton>
              <span class="text-muted">第 {{ backupPage }} 页 / 共 {{ Math.ceil(backupTotal / backupPageSize) }} 页</span>
              <AppButton variant="outline" size="sm" :disabled="backupPage >= Math.ceil(backupTotal / backupPageSize)"
                @click="backupPage++; fetchBackups()">
                下一页
              </AppButton>
            </div>
          </div>
        </template>

        <!-- ═══ Audit Tab ═══ -->
        <template #tab-audit>
          <div class="tab-content">
            <!-- Filters -->
            <div class="audit-filters">
              <div class="audit-filter-item">
                <label class="audit-filter-label">开始日期</label>
                <input type="date" class="audit-date-input" v-model="auditStartDate"
                  @change="auditPage = 1; fetchAuditLogs()" />
              </div>
              <div class="audit-filter-item">
                <label class="audit-filter-label">结束日期</label>
                <input type="date" class="audit-date-input" v-model="auditEndDate"
                  @change="auditPage = 1; fetchAuditLogs()" />
              </div>
              <div class="audit-filter-item">
                <label class="audit-filter-label">操作类型</label>
                <AppSelect :model-value="auditAction" :options="actionOptions" placeholder="全部"
                  @update:model-value="(v: string | number) => { auditAction = v; auditPage = 1; fetchAuditLogs() }" />
              </div>
            </div>

            <!-- Table -->
            <div class="tab-table">
              <AppTable :columns="auditColumns" :data="(auditLogs as unknown as Record<string, unknown>[])"
                :loading="auditLoading">
                <template #cell-created_at="{ row }">
                  {{ new Date(row.created_at as string).toLocaleString('zh-CN') }}
                </template>
                <template #cell-action="{ row }">
                  <AppBadge :variant="getActionBadgeVariant(row.action as string)">{{ getActionLabel(row.action as string) }}</AppBadge>
                </template>
                <template #cell-user="{ row }">
                  <span>{{ row.user_name || '-' }}</span>
                  <span v-if="row.user_email" class="audit-email">{{ row.user_email }}</span>
                </template>
                <template #cell-detail="{ row }">
                  <span class="audit-detail">{{ formatAuditDetail(row.action as string, row.detail as Record<string,
                      any>) }}</span>
                </template>
              </AppTable>
            </div>

            <!-- Pagination -->
            <AppPagination v-if="auditTotal > 0" :page="auditPage" :page-size="auditPageSize" :total="auditTotal"
              @update:page="(p: number) => { auditPage = p; fetchAuditLogs() }"
              @update:page-size="(s: number) => { auditPageSize = s; auditPage = 1; fetchAuditLogs() }" />
          </div>
        </template>

        <!-- ═══ Settings Tab ═══ -->
        <template #tab-settings v-if="authStore.isAdmin">
          <div class="tab-content settings-layout">
            <!-- Instance info form -->
            <AppCard title="基础信息">
              <form @submit.prevent="handleSaveSettings">
                <AppFormGroup>
                  <AppFormItem label="实例名称" :required="true" :error="settingsErrors.name">
                    <AppInput v-model="settingsForm.name" />
                  </AppFormItem>
                  <AppFormItem label="数据源类型" :required="true">
                    <AppSelect v-model="settingsForm.source_type" :options="sourceTypeOptions" />
                  </AppFormItem>
                  <AppFormItem label="数据源路径" :required="true" :error="settingsErrors.source_path">
                    <AppInput v-model="settingsForm.source_path" />
                  </AppFormItem>
                  <AppFormItem>
                    <template #label>
                      <span class="exclude-field-label">
                        <span>排除文件</span>
                        <span class="exclude-help" :title="excludePatternHelpText" aria-label="排除规则示例">
                          <CircleHelp :size="14" />
                        </span>
                      </span>
                    </template>
                    <textarea
                      v-model="settingsForm.exclude_patterns_text"
                      class="instance-textarea"
                      rows="5"
                      placeholder="每行一条规则，例如：&#10;*.log&#10;node_modules/&#10;cache/**"
                    />
                  </AppFormItem>
                  <AppFormItem v-if="settingsForm.source_type === 'ssh'" label="关联远程配置" :required="true"
                    :error="settingsErrors.remote_config_id">
                    <AppSelect v-model="settingsForm.remote_config_id" :options="remoteOptions" placeholder="请选择远程配置" />
                  </AppFormItem>
                </AppFormGroup>

                <div class="settings-actions">
                  <AppButton variant="primary" size="md" :loading="settingsSubmitting" @click="handleSaveSettings">
                    <Save :size="16" style="margin-right: 4px" />
                    保存
                  </AppButton>
                </div>
              </form>
            </AppCard>

            <!-- Permissions -->
            <AppCard title="访问权限">
              <div class="permission-toolbar">
                <AppButton variant="outline" size="sm" @click="addPermVisible = true">
                  <Plus :size="14" style="margin-right: 4px" />
                  添加用户
                </AppButton>
                <span v-if="permEntries.length > 0" class="permission-toolbar__count">{{ permEntries.length }} 位已授权用户</span>
              </div>
              <template v-if="permEntries.length > 0">
                <div class="permission-table">
                  <div class="permission-table__header">
                    <span class="permission-table__col--user">用户</span>
                    <span class="permission-table__col--perm">权限</span>
                    <span class="permission-table__col--action">操作</span>
                  </div>
                  <div class="permission-table__body">
                    <div v-for="entry in permEntries" :key="entry.user_id" class="permission-table__row">
                      <div class="permission-table__col--user">
                        <span class="permission-user__name">{{ entry.name }}</span>
                        <span class="permission-user__email">{{ entry.email }}</span>
                      </div>
                      <div class="permission-table__col--perm">
                        <AppSelect :model-value="entry.permission" :options="permissionOptions"
                          @update:model-value="handleChangePermission(entry.user_id, $event as string)" />
                      </div>
                      <div class="permission-table__col--action">
                        <AppButton variant="ghost" size="sm" @click="handleRemovePermission(entry.user_id)">
                          <Trash2 :size="14" />
                        </AppButton>
                      </div>
                    </div>
                  </div>
                </div>
              </template>
              <AppEmpty v-else message="暂无已授权用户，点击上方按钮添加" />
            </AppCard>
          </div>
        </template>
      </AppTabs>
    </template>

    <!-- Add Permission Modal -->
    <AppModal v-model:visible="addPermVisible" title="添加用户权限" width="480px">
      <AppFormGroup>
        <AppFormItem label="搜索用户">
          <AppInput v-model="addPermSearch" placeholder="输入用户名或邮箱搜索…" />
        </AppFormItem>
        <AppFormItem v-if="addPermSearch.trim()" label="选择用户" :required="true">
          <div class="add-perm-user-list">
            <div v-for="u in filteredAddPermUsers" :key="u.id"
              class="add-perm-user-item" :class="{ 'add-perm-user-item--selected': addPermSelected === u.id }"
              @click="addPermSelected = u.id">
              <div class="add-perm-user-info">
                <span class="permission-user__name">{{ u.name }}</span>
                <span class="permission-user__email">{{ u.email }}</span>
              </div>
              <CheckCircle v-if="addPermSelected === u.id" :size="16" class="add-perm-check" />
            </div>
            <AppEmpty v-if="filteredAddPermUsers.length === 0" message="无匹配的 viewer 用户" />
          </div>
        </AppFormItem>
        <AppFormItem label="权限级别" :required="true">
          <AppSelect v-model="addPermLevel" :options="permissionOptions" />
        </AppFormItem>
      </AppFormGroup>
      <template #footer>
        <AppButton variant="outline" size="md" @click="addPermVisible = false">取消</AppButton>
        <AppButton variant="primary" size="md" :loading="addPermSaving" :disabled="!addPermSelected" @click="handleAddPermission">确认添加</AppButton>
      </template>
    </AppModal>

    <!-- Policy Create/Edit Modal -->
    <AppModal v-model:visible="policyModalVisible" :title="policyEditing ? '编辑策略' : '新增策略'" width="560px">
      <form @submit.prevent="handlePolicySubmit">
        <AppFormGroup>
          <AppFormItem label="策略名称" :required="true" :error="policyErrors.name">
            <AppInput v-model="policyForm.name" placeholder="例如：每日滚动备份" />
          </AppFormItem>

          <AppFormItem label="类型" :required="true">
            <AppSelect v-model="policyForm.type" :options="policyTypeOptions" :disabled="policyEditing" />
          </AppFormItem>

          <AppFormItem label="目标" :required="true" :error="policyErrors.target_id">
            <AppSelect v-model="policyForm.target_id" :options="filteredTargetOptions" placeholder="请选择备份目标" />
          </AppFormItem>

          <AppFormItem label="调度周期" :required="true">
            <AppSelect v-model="policyForm.schedule_mode" :options="scheduleOptions" />
          </AppFormItem>

          <!-- Interval input: number + unit -->
          <AppFormItem v-if="policyForm.schedule_mode === 'interval'" label="执行间隔" :required="true"
            :error="policyErrors.schedule_input">
            <div class="schedule-interval-row">
              <AppInput v-model="policyForm.interval_value" type="number" placeholder="数值" />
              <AppSelect v-model="policyForm.interval_unit" :options="intervalUnitOptions" />
            </div>
          </AppFormItem>

          <!-- Custom cron input -->
          <AppFormItem v-if="policyForm.schedule_mode === 'cron_custom'" label="Cron 表达式" :required="true"
            :error="policyErrors.schedule_input">
            <AppInput v-model="policyForm.schedule_input" placeholder="分 时 日 月 周，如 0 2 * * *" />
          </AppFormItem>

          <AppFormItem label="保留策略" :required="true">
            <AppSelect v-model="policyForm.retention_type" :options="retentionTypeOptions" />
          </AppFormItem>

          <AppFormItem :label="policyForm.retention_type === 'time' ? '保留天数' : '保留条数'" :required="true"
            :error="policyErrors.retention_value">
            <AppInput v-model="policyForm.retention_value" type="number"
              :placeholder="policyForm.retention_type === 'time' ? '天数' : '条数'" />
          </AppFormItem>

          <AppFormItem label="启用">
            <AppSwitch v-model="policyForm.enabled" />
          </AppFormItem>

          <!-- Cold-only options -->
          <template v-if="policyForm.type === 'cold'">
            <div class="form-divider">冷备份选项</div>

            <AppFormItem label="压缩">
              <AppSwitch v-model="policyForm.compression" />
            </AppFormItem>

            <AppFormItem label="加密">
              <AppSwitch v-model="policyForm.encryption" />
            </AppFormItem>

            <AppFormItem v-if="policyForm.encryption" label="加密密钥" :required="!policyEditing"
              :error="policyErrors.encryption_key">
              <AppInput v-model="policyForm.encryption_key" type="password"
                :placeholder="policyEditing ? '留空保持不变' : '请输入加密密钥'" />
            </AppFormItem>

            <AppFormItem label="分卷">
              <AppSwitch v-model="policyForm.split_enabled" />
            </AppFormItem>

            <AppFormItem v-if="policyForm.split_enabled" label="分卷大小 (MB)" :required="true"
              :error="policyErrors.split_size_mb">
              <AppInput v-model="policyForm.split_size_mb" type="number" placeholder="如 1024" />
            </AppFormItem>
          </template>
        </AppFormGroup>
      </form>

      <template #footer>
        <div class="modal-footer">
          <AppButton variant="outline" size="md" @click="policyModalVisible = false">取消</AppButton>
          <AppButton variant="primary" size="md" :loading="policySubmitting" @click="handlePolicySubmit">
            {{ policyEditing ? '保存' : '创建' }}
          </AppButton>
        </div>
      </template>
    </AppModal>

    <!-- Backup Detail Modal -->
    <AppModal v-model:visible="backupDetailVisible" title="备份详情" width="560px">
      <template v-if="backupDetailTarget">
        <div class="backup-detail__grid">
          <div class="backup-detail__item">
            <span class="backup-detail__label">类型</span>
            <span class="backup-detail__value">
              <StatusBadge :config="getStatusConfig(backupTypeMap, backupDetailTarget.type)" />
            </span>
          </div>
          <div class="backup-detail__item">
            <span class="backup-detail__label">状态</span>
            <span class="backup-detail__value">
              <StatusBadge :config="getStatusConfig(taskStatusMap, backupDetailTarget.status)" />
            </span>
          </div>
          <div class="backup-detail__item">
            <span class="backup-detail__label">开始时间</span>
            <span class="backup-detail__value">{{ backupDetailTarget.started_at ?? '--' }}</span>
          </div>
          <div class="backup-detail__item">
            <span class="backup-detail__label">完成时间</span>
            <span class="backup-detail__value">{{ backupDetailTarget.completed_at ?? '--' }}</span>
          </div>
          <div class="backup-detail__item">
            <span class="backup-detail__label">备份大小</span>
            <span class="backup-detail__value">{{ formatBytes(backupDetailTarget.backup_size_bytes) }}</span>
          </div>
          <div class="backup-detail__item">
            <span class="backup-detail__label">数据原始大小</span>
            <span class="backup-detail__value">{{ formatBytes(backupDetailTarget.actual_size_bytes) }}</span>
          </div>
          <div class="backup-detail__item backup-detail__item--wide">
            <span class="backup-detail__label">快照路径</span>
            <span class="backup-detail__value">{{ backupDetailTarget.snapshot_path || '--' }}</span>
          </div>
          <div v-if="backupDetailTarget.rsync_stats" class="backup-detail__item backup-detail__item--wide">
            <span class="backup-detail__label">Rsync 统计</span>
            <pre class="backup-detail__pre">{{ backupDetailTarget.rsync_stats }}</pre>
          </div>
          <div v-if="backupDetailTarget.error_message" class="backup-detail__item backup-detail__item--wide">
            <span class="backup-detail__label">失败原因</span>
            <span class="backup-detail__value backup-detail__value--error">{{ backupDetailTarget.error_message }}</span>
          </div>
        </div>
      </template>
      <template #footer>
        <div class="modal-footer">
          <AppButton variant="outline" size="md" @click="backupDetailVisible = false">关闭</AppButton>
        </div>
      </template>
    </AppModal>

    <!-- Restore Confirm Modal -->
    <AppModal v-model:visible="restoreModalVisible" title="恢复备份" width="520px">
      <form @submit.prevent="handleRestoreSubmit">
        <AppFormGroup>
          <!-- Restore type -->
          <AppFormItem label="恢复类型" :required="true">
            <div class="restore-type-group">
              <label class="restore-type-option">
                <input type="radio" v-model="restoreForm.restore_type" value="source" />
                <span>恢复到原始位置</span>
              </label>
              <label class="restore-type-option">
                <input type="radio" v-model="restoreForm.restore_type" value="custom" />
                <span>恢复到指定位置</span>
              </label>
            </div>
          </AppFormItem>

          <!-- Warning for source restore -->
          <div v-if="restoreForm.restore_type === 'source'" class="restore-warning">
            ⚠ 将覆盖源路径的现有数据
          </div>

          <!-- Target path -->
          <AppFormItem v-if="restoreForm.restore_type === 'custom'" label="目标路径" :required="true"
            :error="restoreFormErrors.target_path">
            <AppInput v-model="restoreForm.target_path" placeholder="如 /data/restore/" />
          </AppFormItem>

          <!-- Encryption key (for encrypted cold backups) -->
          <AppFormItem v-if="restoreBackupTarget && isEncryptedCold(restoreBackupTarget)" label="加密密钥" :required="true"
            :error="restoreFormErrors.encryption_key">
            <AppInput v-model="restoreForm.encryption_key" type="password" placeholder="输入备份加密时使用的密钥" />
          </AppFormItem>

          <!-- Danger confirmation area -->
          <div class="restore-danger-zone">
            <p class="restore-danger-zone__hint">
              请输入实例名称 <code>{{ instance?.name }}</code> 和您的账号密码以确认恢复操作
            </p>
            <AppFormItem label="实例名称" :required="true" :error="restoreFormErrors.instance_name">
              <AppInput v-model="restoreForm.instance_name" :placeholder="`请输入：${instance?.name}`" />
            </AppFormItem>
            <AppFormItem label="当前密码" :required="true" :error="restoreFormErrors.password">
              <AppInput v-model="restoreForm.password" type="password" placeholder="输入您的登录密码" />
            </AppFormItem>
          </div>

          <!-- Error message -->
          <div v-if="restoreError" class="restore-error">{{ restoreError }}</div>
        </AppFormGroup>
      </form>

      <template #footer>
        <div class="modal-footer">
          <AppButton variant="outline" size="md" @click="restoreModalVisible = false">取消</AppButton>
          <AppButton variant="danger" size="md" :loading="restoreSubmitting" :disabled="restoreSubmitDisabled"
            @click="handleRestoreSubmit">
            确认恢复
          </AppButton>
        </div>
      </template>
    </AppModal>

    <AppConfirm />
  </div>
</template>

<style scoped>
.instance-detail-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.instance-detail-page__header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.instance-detail-page__title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
}

.instance-detail-page__loading {
  text-align: center;
  padding: 60px 0;
  color: var(--text-muted);
}

.tab-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-top: 16px;
}

.settings-layout {
  display: grid;
  grid-template-columns: 1fr;
  gap: 20px;
  padding-top: 16px;
}

@media (min-width: 960px) {
  .settings-layout {
    grid-template-columns: 1fr 1fr;
    align-items: start;
  }
}

.tab-header {
  display: flex;
  justify-content: flex-end;
}

.tab-table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
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

.exclude-field-label {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.exclude-help {
  display: inline-flex;
  align-items: center;
  color: var(--text-muted);
  cursor: help;
}

.instance-textarea {
  width: 100%;
  min-height: 116px;
  padding: 10px 12px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--surface-base);
  color: var(--text-primary);
  font: inherit;
  line-height: 1.5;
  resize: vertical;
}

.instance-textarea:focus {
  outline: none;
  border-color: var(--primary-500);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-500) 18%, transparent);
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

.stat-card__sub {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 4px;
}

.stat-card__value--sm {
  font-size: 18px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 12px;
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

.stat-card__label {
  font-size: 13px;
  color: var(--text-muted);
  margin-top: 2px;
}

/* Mini table */
.mini-table {
  overflow-x: auto;
}

.mini-table table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.mini-table th {
  text-align: left;
  padding: 8px 12px;
  color: var(--text-muted);
  font-weight: 500;
  border-bottom: 1px solid var(--border-default);
}

.mini-table td {
  padding: 8px 12px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-default);
}

.mini-table tr:last-child td {
  border-bottom: none;
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

/* Disaster Recovery ring & sub-scores */

/* Actions */
.actions-cell {
  display: flex;
  gap: 4px;
}

.last-exec-time {
  font-size: 12px;
  color: var(--text-muted);
  margin-left: 6px;
}

/* Settings */
.settings-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.permission-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.permission-toolbar__count {
  font-size: 13px;
  color: var(--text-muted);
  white-space: nowrap;
}

.permission-table {
  border: 1px solid var(--border-default);
  border-radius: 8px;
  overflow: hidden;
}

.permission-table__header {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  background: var(--bg-subtle);
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  border-bottom: 1px solid var(--border-default);
}

.permission-table__body {
  max-height: 360px;
  overflow-y: auto;
}

.permission-table__row {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  border-bottom: 1px solid var(--border-default);
}

.permission-table__row:last-child {
  border-bottom: none;
}

.permission-table__col--user {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}

.permission-table__col--perm {
  flex-shrink: 0;
  width: 160px;
}

.permission-table__col--action {
  flex-shrink: 0;
  width: 48px;
  display: flex;
  justify-content: center;
}

.permission-user__name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.permission-user__email {
  font-size: 12px;
  color: var(--text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.add-perm-user-list {
  max-height: 200px;
  overflow-y: auto;
  border: 1px solid var(--border-default);
  border-radius: 8px;
}

.add-perm-user-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 12px;
  cursor: pointer;
  border-bottom: 1px solid var(--border-default);
  transition: background 0.15s;
}

.add-perm-user-item:last-child {
  border-bottom: none;
}

.add-perm-user-item:hover {
  background: var(--bg-subtle);
}

.add-perm-user-item--selected {
  background: var(--primary-50);
  border-color: var(--primary-200);
}

.add-perm-user-info {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
}

.add-perm-check {
  flex-shrink: 0;
  color: var(--primary-500);
}

/* Common */
.text-muted {
  color: var(--text-muted);
  font-size: 13px;
}

.text-error {
  color: var(--error-500);
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.form-divider {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  padding: 8px 0 4px;
  border-top: 1px solid var(--border-default);
  margin-top: 4px;
}

.schedule-interval-row {
  display: flex;
  gap: 8px;
}

.schedule-interval-row> :first-child {
  flex: 1;
}

.schedule-interval-row> :last-child {
  width: 100px;
  flex-shrink: 0;
}

/* Backup detail */
.backup-detail__grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px 24px;
}

.backup-detail__item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.backup-detail__item--wide {
  grid-column: 1 / -1;
}

.backup-detail__label {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.backup-detail__value {
  font-size: 13px;
  color: var(--text-primary);
  word-break: break-all;
}

.backup-detail__value--error {
  color: var(--error-500);
}

.backup-detail__pre {
  font-size: 12px;
  font-family: monospace;
  color: var(--text-secondary);
  background: var(--surface-default);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  padding: 8px 12px;
  margin: 0;
  white-space: pre-wrap;
  word-break: break-all;
  overflow-x: auto;
}

.backup-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 12px 0;
}

/* Restore modal */
.restore-type-group {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.restore-type-option {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 14px;
  color: var(--text-primary);
  cursor: pointer;
}

.restore-type-option input[type="radio"] {
  accent-color: var(--primary-500);
}

.restore-warning {
  background: color-mix(in srgb, var(--warning-500) 10%, transparent);
  color: var(--warning-500);
  border: 1px solid color-mix(in srgb, var(--warning-500) 25%, transparent);
  border-radius: var(--radius-md);
  padding: 8px 12px;
  font-size: 13px;
}

.restore-danger-zone {
  background: color-mix(in srgb, var(--error-500) 6%, transparent);
  border: 1px solid color-mix(in srgb, var(--error-500) 20%, transparent);
  border-radius: var(--radius-md);
  padding: 16px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.restore-danger-zone__hint {
  font-size: 13px;
  color: var(--text-secondary);
  margin: 0;
  line-height: 1.5;
}

.restore-danger-zone__hint code {
  font-weight: 600;
  color: var(--text-primary);
  background: var(--surface-sunken);
  padding: 1px 4px;
  border-radius: var(--radius-sm);
}

.restore-error {
  color: var(--error-500);
  font-size: 13px;
  background: color-mix(in srgb, var(--error-500) 8%, transparent);
  border-radius: var(--radius-md);
  padding: 8px 12px;
}

/* Audit tab */
.audit-filters {
  display: flex;
  align-items: flex-end;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.audit-filter-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 160px;
}

.audit-filter-label {
  font-size: 13px;
  color: var(--text-secondary);
  font-weight: 500;
}

.audit-date-input {
  padding: 8px 12px;
  font-size: 14px;
  line-height: 20px;
  color: var(--text-primary);
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  outline: none;
  transition: border-color 0.15s;
  color-scheme: light;
}

:root[data-theme="dark"] .audit-date-input {
  color-scheme: dark;
}

.audit-date-input:focus {
  border-color: var(--primary-500);
}

.audit-email {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
}

.audit-detail {
  font-size: 13px;
  color: var(--text-secondary);
}

/* Stat card clickable */
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

/* Overview tasks row */
.overview-tasks-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
  align-items: start;
}

/* Prevent the upcoming card from stretching with the current-tasks card */
.overview-tasks-row__upcoming {
  align-self: start;
}

@media (max-width: 767px) {
  .overview-tasks-row {
    grid-template-columns: 1fr;
  }
}

.instance-task-list,
.instance-upcoming-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
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

.py-4 {
  padding-top: 16px;
  padding-bottom: 16px;
}

</style>
