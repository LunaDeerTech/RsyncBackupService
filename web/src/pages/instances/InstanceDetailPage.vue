<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getInstance, getInstanceStats, updateInstance, updateInstancePermissions } from '../../api/instances'
import { listPolicies, createPolicy, updatePolicy, deletePolicy, triggerPolicy } from '../../api/policies'
import { listTargets } from '../../api/targets'
import { listRemotes } from '../../api/remotes'
import { listUsers } from '../../api/users'
import { useAuthStore } from '../../stores/auth'
import { useToastStore } from '../../stores/toast'
import { useConfirm } from '../../composables/useConfirm'
import { ApiBusinessError } from '../../api/client'
import { formatBytes } from '../../utils/format'
import { formatRelativeTime } from '../../utils/time'
import { formatScheduleValue, parseIntervalInput } from '../../utils/schedule'
import type { Instance, InstanceStats, Backup, UpdateInstanceRequest, PermissionItem } from '../../types/instance'
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
import {
  ArrowLeft, Play, Plus, Pencil, Trash2, Save,
  Database, CheckCircle, HardDrive, Shield,
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
const pageLoading = ref(false)

// ── Policy data ──
const policies = ref<Policy[]>([])
const policyLoading = ref(false)
const policyModalVisible = ref(false)
const policyEditing = ref(false)
const policyEditingId = ref<number | null>(null)
const policySubmitting = ref(false)
const targets = ref<BackupTarget[]>([])

const policyForm = reactive({
  name: '',
  type: 'rolling' as 'rolling' | 'cold',
  target_id: undefined as number | undefined,
  schedule_type: 'interval' as 'interval' | 'cron',
  schedule_input: '',
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

// ── Backup data (from stats for now) ──
const recentBackups = computed<Backup[]>(() => {
  if (!stats.value?.last_backup) return []
  return [stats.value.last_backup]
})

// ── Audit (placeholder) ──
const auditColumns: TableColumn[] = [
  { key: 'time', title: '时间' },
  { key: 'action', title: '操作类型' },
  { key: 'user', title: '操作人' },
  { key: 'detail', title: '详情' },
]

// ── Settings ──
const settingsForm = reactive({
  name: '',
  source_type: 'local' as 'local' | 'ssh',
  source_path: '',
  remote_config_id: undefined as number | undefined,
})
const settingsErrors = reactive({ name: '', source_path: '', remote_config_id: '' })
const settingsSubmitting = ref(false)
const remotes = ref<RemoteConfig[]>([])
const users = ref<User[]>([])
const permissionMap = ref<Record<number, string>>({})
const permissionSaving = ref(false)

// ── Computed: policy form helpers ──
const policyTypeOptions = [
  { label: '滚动备份', value: 'rolling' },
  { label: '冷备份', value: 'cold' },
]

const scheduleTypeOptions = [
  { label: '间隔', value: 'interval' },
  { label: 'Cron', value: 'cron' },
]

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

const backupColumns: TableColumn[] = [
  { key: 'completed_at', title: '完成时间' },
  { key: 'type', title: '类型' },
  { key: 'status', title: '状态' },
  { key: 'backup_size_bytes', title: '备份大小' },
  { key: 'actual_size_bytes', title: '数据原始大小' },
  { key: 'duration_seconds', title: '持续时间' },
  { key: 'actions', title: '操作', width: '140px' },
]

const backupStatusVariant: Record<string, 'success' | 'error' | 'info' | 'warning' | 'default'> = {
  success: 'success',
  failed: 'error',
  running: 'info',
  pending: 'warning',
}

const backupStatusLabel: Record<string, string> = {
  success: '成功',
  failed: '失败',
  running: '运行中',
  pending: '等待中',
}

// ── Fetch core data ──
async function fetchInstance() {
  pageLoading.value = true
  try {
    const res = await getInstance(instanceId.value)
    instance.value = res.instance
    stats.value = res.stats
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

onMounted(async () => {
  await fetchInstance()
  fetchPolicies()
  if (authStore.isAdmin) {
    fetchTargets()
    fetchRemotes()
    fetchUsers()
  }
})

// ── Watch tab changes ──
watch(activeTab, (tab) => {
  if (tab === 'overview') fetchStats()
  if (tab === 'policies') fetchPolicies()
  if (tab === 'settings' && instance.value) {
    settingsForm.name = instance.value.name
    settingsForm.source_type = instance.value.source_type
    settingsForm.source_path = instance.value.source_path
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
  policyForm.schedule_type = 'interval'
  policyForm.schedule_input = ''
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
  policyForm.schedule_type = row.schedule_type as 'interval' | 'cron'
  policyForm.schedule_input = row.schedule_value as string
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
  if (!policyForm.schedule_input.trim()) {
    policyErrors.schedule_input = '请输入调度值'
    valid = false
  } else if (policyForm.schedule_type === 'interval') {
    const seconds = parseIntervalInput(policyForm.schedule_input)
    if (isNaN(seconds) || seconds <= 0) {
      policyErrors.schedule_input = '请输入有效的间隔值，如"6小时"、"30分钟"、"3600"'
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
    let scheduleValue = policyForm.schedule_input.trim()
    if (policyForm.schedule_type === 'interval') {
      scheduleValue = String(parseIntervalInput(policyForm.schedule_input))
    }

    const data: CreatePolicyRequest = {
      name: policyForm.name.trim(),
      type: policyForm.type,
      target_id: policyForm.target_id!,
      schedule_type: policyForm.schedule_type,
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
    const data: UpdateInstanceRequest = {
      name: settingsForm.name.trim(),
      source_type: settingsForm.source_type,
      source_path: settingsForm.source_path.trim(),
      remote_config_id: settingsForm.source_type === 'ssh' ? settingsForm.remote_config_id : undefined,
    }
    const updated = await updateInstance(instanceId.value, data)
    instance.value = updated
    toast.success('实例信息已更新')
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('保存失败')
  } finally {
    settingsSubmitting.value = false
  }
}

async function handleSavePermissions() {
  permissionSaving.value = true
  try {
    const permissions: PermissionItem[] = []
    for (const [userId, perm] of Object.entries(permissionMap.value)) {
      if (perm === 'readonly') {
        permissions.push({ user_id: Number(userId), permission: 'readonly' })
      }
    }
    await updateInstancePermissions(instanceId.value, permissions)
    toast.success('权限已更新')
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('保存失败')
  } finally {
    permissionSaving.value = false
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
  { label: '无权限', value: 'none' },
  { label: '只读', value: 'readonly' },
]

const schedulePlaceholder = computed(() =>
  policyForm.schedule_type === 'interval'
    ? '如 6小时、30分钟 或秒数'
    : 'Cron 表达式，如 0 2 * * *',
)
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
            <!-- Info card -->
            <AppCard>
              <div class="overview-info">
                <div class="overview-info__item">
                  <span class="overview-info__label">实例名称</span>
                  <span class="overview-info__value">{{ instance.name }}</span>
                </div>
                <div class="overview-info__item">
                  <span class="overview-info__label">数据源</span>
                  <span class="overview-info__value">{{ sourceTypeLabel[instance.source_type] ?? instance.source_type }}: {{ instance.source_path }}</span>
                </div>
                <div class="overview-info__item">
                  <span class="overview-info__label">状态</span>
                  <AppBadge :variant="instance.status === 'running' ? 'info' : 'default'">
                    {{ instance.status === 'running' ? '运行中' : '空闲' }}
                  </AppBadge>
                </div>
              </div>
            </AppCard>

            <!-- Stats cards -->
            <div class="stats-grid">
              <AppCard>
                <div class="stat-card">
                  <Database :size="20" class="stat-icon stat-icon--primary" />
                  <div class="stat-card__content">
                    <span class="stat-card__value">{{ stats?.backup_count ?? 0 }}</span>
                    <span class="stat-card__label">备份总数</span>
                  </div>
                </div>
              </AppCard>
              <AppCard>
                <div class="stat-card">
                  <CheckCircle :size="20" class="stat-icon stat-icon--success" />
                  <div class="stat-card__content">
                    <span class="stat-card__value" :style="{ color: successRateColor }">
                      {{ successRate !== null ? successRate + '%' : '--' }}
                    </span>
                    <span class="stat-card__label">成功率</span>
                  </div>
                </div>
              </AppCard>
              <AppCard>
                <div class="stat-card">
                  <HardDrive :size="20" class="stat-icon stat-icon--info" />
                  <div class="stat-card__content">
                    <span class="stat-card__value">{{ formatBytes(stats?.total_backup_size_bytes) }}</span>
                    <span class="stat-card__label">总备份大小</span>
                  </div>
                </div>
              </AppCard>
              <AppCard>
                <div class="stat-card">
                  <Shield :size="20" class="stat-icon stat-icon--muted" />
                  <div class="stat-card__content">
                    <span class="stat-card__value">--</span>
                    <span class="stat-card__label">容灾率</span>
                  </div>
                </div>
              </AppCard>
            </div>

            <!-- Recent backups mini table -->
            <AppCard title="最近备份">
              <template v-if="recentBackups.length > 0">
                <div class="mini-table">
                  <table>
                    <thead>
                      <tr>
                        <th>时间</th>
                        <th>类型</th>
                        <th>状态</th>
                        <th>大小</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="b in recentBackups" :key="b.id">
                        <td>{{ b.completed_at ? formatRelativeTime(b.completed_at) : '--' }}</td>
                        <td>{{ policyTypeLabel[b.type] ?? b.type }}</td>
                        <td>
                          <AppBadge :variant="backupStatusVariant[b.status] ?? 'default'">
                            {{ backupStatusLabel[b.status] ?? b.status }}
                          </AppBadge>
                        </td>
                        <td>{{ formatBytes(b.backup_size_bytes) }}</td>
                      </tr>
                    </tbody>
                  </table>
                </div>
              </template>
              <AppEmpty v-else message="暂无备份记录" />
            </AppCard>
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
                  <span class="policy-type-badge" :class="`policy-type-badge--${row.type}`">
                    {{ policyTypeLabel[row.type as string] ?? row.type }}
                  </span>
                </template>

                <template #cell-target_name="{ row }">
                  {{ getTargetName(row.target_id as number) }}
                </template>

                <template #cell-schedule="{ row }">
                  {{ formatScheduleValue(row.schedule_type as string, row.schedule_value as string) }}
                </template>

                <template #cell-enabled="{ row }">
                  <AppSwitch
                    :model-value="row.enabled as boolean"
                    :disabled="!authStore.isAdmin"
                    @update:model-value="handleTogglePolicy(row, $event)"
                  />
                </template>

                <template #cell-last_execution="{ row }">
                  <template v-if="row.last_execution_time">
                    <AppBadge :variant="backupStatusVariant[row.last_execution_status as string] ?? 'default'">
                      {{ backupStatusLabel[row.last_execution_status as string] ?? row.last_execution_status }}
                    </AppBadge>
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
              <AppTable :columns="backupColumns" :data="(recentBackups as unknown as Record<string, unknown>[])" :loading="false">
                <template #cell-completed_at="{ row }">
                  {{ row.completed_at ? formatRelativeTime(row.completed_at as string) : '--' }}
                </template>

                <template #cell-type="{ row }">
                  <span class="policy-type-badge" :class="`policy-type-badge--${row.type}`">
                    {{ policyTypeLabel[row.type as string] ?? row.type }}
                  </span>
                </template>

                <template #cell-status="{ row }">
                  <AppBadge :variant="backupStatusVariant[row.status as string] ?? 'default'">
                    {{ backupStatusLabel[row.status as string] ?? row.status }}
                  </AppBadge>
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
                    <AppButton variant="ghost" size="sm" @click="toast.info('功能开发中')">
                      恢复
                    </AppButton>
                    <AppButton
                      v-if="row.type === 'cold'"
                      variant="ghost"
                      size="sm"
                      @click="toast.info('功能开发中')"
                    >
                      下载
                    </AppButton>
                  </div>
                </template>
              </AppTable>
            </div>
          </div>
        </template>

        <!-- ═══ Audit Tab ═══ -->
        <template #tab-audit>
          <div class="tab-content">
            <div class="tab-table">
              <AppTable :columns="auditColumns" :data="[]" :loading="false" />
            </div>
          </div>
        </template>

        <!-- ═══ Settings Tab ═══ -->
        <template #tab-settings v-if="authStore.isAdmin">
          <div class="tab-content">
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
                  <AppFormItem
                    v-if="settingsForm.source_type === 'ssh'"
                    label="关联远程配置"
                    :required="true"
                    :error="settingsErrors.remote_config_id"
                  >
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
              <template v-if="users.length > 0">
                <div class="permission-list">
                  <div v-for="u in users" :key="u.id" class="permission-row">
                    <div class="permission-user">
                      <span class="permission-user__name">{{ u.name }}</span>
                      <span class="permission-user__email">{{ u.email }}</span>
                    </div>
                    <AppSelect
                      :model-value="permissionMap[u.id] ?? 'none'"
                      :options="permissionOptions"
                      @update:model-value="permissionMap[u.id] = $event as string"
                    />
                  </div>
                </div>
                <div class="settings-actions">
                  <AppButton variant="primary" size="md" :loading="permissionSaving" @click="handleSavePermissions">
                    <Save :size="16" style="margin-right: 4px" />
                    保存权限
                  </AppButton>
                </div>
              </template>
              <AppEmpty v-else message="暂无 viewer 用户" />
            </AppCard>
          </div>
        </template>
      </AppTabs>
    </template>

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

          <AppFormItem label="调度类型" :required="true">
            <AppSelect v-model="policyForm.schedule_type" :options="scheduleTypeOptions" />
          </AppFormItem>

          <AppFormItem label="调度值" :required="true" :error="policyErrors.schedule_input">
            <AppInput
              v-model="policyForm.schedule_input"
              :placeholder="schedulePlaceholder"
            />
          </AppFormItem>

          <AppFormItem label="保留策略" :required="true">
            <AppSelect v-model="policyForm.retention_type" :options="retentionTypeOptions" />
          </AppFormItem>

          <AppFormItem :label="policyForm.retention_type === 'time' ? '保留天数' : '保留条数'" :required="true" :error="policyErrors.retention_value">
            <AppInput v-model="policyForm.retention_value" type="number" :placeholder="policyForm.retention_type === 'time' ? '天数' : '条数'" />
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

            <AppFormItem v-if="policyForm.encryption" label="加密密钥" :required="!policyEditing" :error="policyErrors.encryption_key">
              <AppInput v-model="policyForm.encryption_key" type="password" :placeholder="policyEditing ? '留空保持不变' : '请输入加密密钥'" />
            </AppFormItem>

            <AppFormItem label="分卷">
              <AppSwitch v-model="policyForm.split_enabled" />
            </AppFormItem>

            <AppFormItem v-if="policyForm.split_enabled" label="分卷大小 (MB)" :required="true" :error="policyErrors.split_size_mb">
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
.tab-header {
  display: flex;
  justify-content: flex-end;
}
.tab-table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

/* Overview */
.overview-info {
  display: flex;
  flex-wrap: wrap;
  gap: 24px;
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
.stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 16px;
}
.stat-card {
  display: flex;
  align-items: center;
  gap: 12px;
}
.stat-icon {
  flex-shrink: 0;
}
.stat-icon--primary { color: var(--primary-500); }
.stat-icon--success { color: var(--success-500); }
.stat-icon--info { color: var(--primary-600); }
.stat-icon--muted { color: var(--text-muted); }
.stat-card__content {
  display: flex;
  flex-direction: column;
}
.stat-card__value {
  font-size: 22px;
  font-weight: 700;
  color: var(--text-primary);
  line-height: 1.2;
}
.stat-card__label {
  font-size: 12px;
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

/* Policy type badge */
.policy-type-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
  border-radius: 9999px;
  white-space: nowrap;
}
.policy-type-badge--rolling {
  background: color-mix(in srgb, #3b82f6 15%, transparent);
  color: #3b82f6;
}
.policy-type-badge--cold {
  background: color-mix(in srgb, #8b5cf6 15%, transparent);
  color: #8b5cf6;
}

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
.permission-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}
.permission-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 8px 0;
  border-bottom: 1px solid var(--border-default);
}
.permission-row:last-child {
  border-bottom: none;
}
.permission-user {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.permission-user__name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
}
.permission-user__email {
  font-size: 12px;
  color: var(--text-muted);
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
</style>
