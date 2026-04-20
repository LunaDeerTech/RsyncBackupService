<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { listPolicies, createPolicy, updatePolicy, deletePolicy, triggerPolicy } from '../../../api/policies'
import { listTargets } from '../../../api/targets'
import { listRemotes } from '../../../api/remotes'
import { useAuthStore } from '../../../stores/auth'
import { useToastStore } from '../../../stores/toast'
import { useConfirm } from '../../../composables/useConfirm'
import { useListViewPreferenceStore, type ListViewMode, SHARED_LIST_VIEW_PREFERENCE_KEY } from '../../../stores/list-view-preference'
import { ApiBusinessError } from '../../../api/client'
import { formatRelativeTime } from '../../../utils/time'
import { formatScheduleValue } from '../../../utils/schedule'
import {
  taskStatusMap, backupTypeMap,
  getStatusConfig,
} from '../../../utils/status-config'
import type { Policy, CreatePolicyRequest, UpdatePolicyRequest } from '../../../types/policy'
import type { BackupTarget } from '../../../types/target'
import type { RemoteConfig } from '../../../types/remote'
import type { TableColumn } from '../../../components/AppTable.vue'
import type { TabItem } from '../../../components/AppTabs.vue'
import AppTable from '../../../components/AppTable.vue'
import AppTabs from '../../../components/AppTabs.vue'
import AppModal from '../../../components/AppModal.vue'
import AppFormGroup from '../../../components/AppFormGroup.vue'
import AppFormItem from '../../../components/AppFormItem.vue'
import AppInput from '../../../components/AppInput.vue'
import AppSelect from '../../../components/AppSelect.vue'
import AppButton from '../../../components/AppButton.vue'
import AppSwitch from '../../../components/AppSwitch.vue'
import ListViewToggle from '../../../components/ListViewToggle.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { Play, Plus, Pencil, Trash2, GripVertical, X } from 'lucide-vue-next'

const props = defineProps<{
  instanceId: number
}>()

const authStore = useAuthStore()
const toast = useToastStore()
const { confirm } = useConfirm()
const listViewPreferenceStore = useListViewPreferenceStore()

// ── Policy data ──
const policies = ref<Policy[]>([])
const policyLoading = ref(false)
const policyModalVisible = ref(false)
const policyEditing = ref(false)
const policyEditingId = ref<number | null>(null)
const policySubmitting = ref(false)
const targets = ref<BackupTarget[]>([])
const remotes = ref<RemoteConfig[]>([])

// ── Modal tab state ──
const modalActiveTab = ref('basic')
const modalTabs: TabItem[] = [
  { key: 'basic', label: '基本' },
  { key: 'advanced', label: '高级' },
  { key: 'commands', label: '前后命令' },
]

// ── Command types ──
interface HookCommand {
  id: number
  location: string   // 'local' or remote config id as string
  command: string
}
let hookCommandIdCounter = 0
const preCommands = ref<HookCommand[]>([])
const postCommands = ref<HookCommand[]>([])
const dragState = ref<{ list: 'pre' | 'post'; index: number } | null>(null)
const inferredPolicyViewMode: ListViewMode = typeof window !== 'undefined' && window.innerWidth < 768 ? 'card' : 'list'
listViewPreferenceStore.initializeViewMode(SHARED_LIST_VIEW_PREFERENCE_KEY, inferredPolicyViewMode)

const policyViewMode = computed({
  get: (): ListViewMode => listViewPreferenceStore.getViewMode(SHARED_LIST_VIEW_PREFERENCE_KEY) ?? inferredPolicyViewMode,
  set: (mode: ListViewMode) => listViewPreferenceStore.setViewMode(SHARED_LIST_VIEW_PREFERENCE_KEY, mode),
})

// ── Options ──
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

const policyTypeOptions = [
  { label: '滚动备份', value: 'rolling' },
  { label: '冷备份', value: 'cold' },
]

const retentionTypeOptions = [
  { label: '按时间（天）', value: 'time' },
  { label: '按数量（条）', value: 'count' },
]

const commandLocationOptions = computed(() => {
  const opts: { label: string; value: string }[] = [
    { label: '本地', value: 'local' },
  ]
  for (const r of remotes.value) {
    if (r.type === 'ssh') {
      opts.push({ label: r.name, value: String(r.id) })
    }
  }
  return opts
})

// ── Form ──
const policyForm = reactive({
  name: '',
  type: 'rolling' as 'rolling' | 'cold',
  target_id: undefined as number | undefined,
  schedule_mode: 'interval' as string,
  schedule_input: '',
  interval_value: undefined as number | undefined,
  interval_unit: 'hours' as string,
  bandwidth_limit_kb: -1,
  enabled: true,
  compression: false,
  encryption: false,
  encryption_key: '',
  split_enabled: false,
  split_size_mb: undefined as number | undefined,
  retry_enabled: true,
  retry_max_retries: 3,
  retention_type: 'count' as 'time' | 'count',
  retention_value: 7,
})

const policyErrors = reactive({
  name: '',
  target_id: '',
  schedule_input: '',
  bandwidth_limit_kb: '',
  encryption_key: '',
  split_size_mb: '',
  retry_max_retries: '',
  retention_value: '',
})

// ── Computed ──
const filteredTargetOptions = computed(() => {
  return targets.value
    .filter((t) => t.backup_type === policyForm.type)
    .map((t) => ({ label: t.name, value: t.id }))
})

const policyColumns: TableColumn[] = [
  { key: 'name', title: '名称' },
  { key: 'type', title: '类型' },
  { key: 'target_name', title: '目标' },
  { key: 'schedule', title: '调度' },
  { key: 'enabled', title: '启用' },
  { key: 'last_execution', title: '上次执行' },
  { key: 'actions', title: '操作', width: '140px' },
]

// ── Helpers ──
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
  policyForm.bandwidth_limit_kb = -1
  policyForm.enabled = true
  policyForm.compression = false
  policyForm.encryption = false
  policyForm.encryption_key = ''
  policyForm.split_enabled = false
  policyForm.split_size_mb = undefined
  policyForm.retry_enabled = true
  policyForm.retry_max_retries = 3
  policyForm.retention_type = 'count'
  policyForm.retention_value = 7
  Object.keys(policyErrors).forEach((k) => (policyErrors as Record<string, string>)[k] = '')
  preCommands.value = []
  postCommands.value = []
  modalActiveTab.value = 'basic'
}

// ── Command helpers ──
function addCommand(list: 'pre' | 'post') {
  const cmd: HookCommand = { id: ++hookCommandIdCounter, location: 'local', command: '' }
  if (list === 'pre') preCommands.value.push(cmd)
  else postCommands.value.push(cmd)
}

function removeCommand(list: 'pre' | 'post', index: number) {
  if (list === 'pre') preCommands.value.splice(index, 1)
  else postCommands.value.splice(index, 1)
}

function onDragStart(list: 'pre' | 'post', index: number, e: DragEvent) {
  dragState.value = { list, index }
  if (e.dataTransfer) {
    e.dataTransfer.effectAllowed = 'move'
    e.dataTransfer.setData('text/plain', '')
  }
}

function onDragOver(list: 'pre' | 'post', _index: number, e: DragEvent) {
  if (!dragState.value || dragState.value.list !== list) return
  e.preventDefault()
  if (e.dataTransfer) e.dataTransfer.dropEffect = 'move'
}

function onDrop(list: 'pre' | 'post', index: number, e: DragEvent) {
  e.preventDefault()
  if (!dragState.value || dragState.value.list !== list) return
  const fromIndex = dragState.value.index
  const arr = list === 'pre' ? preCommands.value : postCommands.value
  const [item] = arr.splice(fromIndex, 1)
  arr.splice(index, 0, item)
  dragState.value = null
}

function onDragEnd() {
  dragState.value = null
}

function openCreatePolicy() {
  resetPolicyForm()
  policyEditing.value = false
  policyEditingId.value = null
  policyModalVisible.value = true
  fetchRemotes()
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
  policyForm.bandwidth_limit_kb = row.bandwidth_limit_kb as number ?? -1
  policyForm.compression = row.compression as boolean
  policyForm.encryption = row.encryption as boolean
  policyForm.encryption_key = ''
  policyForm.split_enabled = row.split_enabled as boolean
  policyForm.split_size_mb = row.split_size_mb as number | undefined
  policyForm.retry_enabled = row.retry_enabled as boolean ?? true
  policyForm.retry_max_retries = row.retry_max_retries as number ?? 3
  policyForm.retention_type = row.retention_type as 'time' | 'count'
  policyForm.retention_value = row.retention_value as number
  // Load hook commands
  const rawPre = row.pre_commands as Array<{ location: string; command: string }> | undefined
  const rawPost = row.post_commands as Array<{ location: string; command: string }> | undefined
  if (rawPre && rawPre.length) {
    preCommands.value = rawPre.map(c => ({ id: ++hookCommandIdCounter, location: c.location, command: c.command }))
  }
  if (rawPost && rawPost.length) {
    postCommands.value = rawPost.map(c => ({ id: ++hookCommandIdCounter, location: c.location, command: c.command }))
  }
  policyModalVisible.value = true
  fetchRemotes()
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
  if (policyForm.bandwidth_limit_kb !== -1 && policyForm.bandwidth_limit_kb <= 0) {
    policyErrors.bandwidth_limit_kb = '限流值需为 -1 或正整数'
    valid = false
  }
  if (policyForm.split_enabled && (!policyForm.split_size_mb || policyForm.split_size_mb <= 0)) {
    policyErrors.split_size_mb = '请输入有效的分卷大小'
    valid = false
  }
  if (policyForm.retry_enabled && (policyForm.retry_max_retries < 1 || policyForm.retry_max_retries > 10)) {
    policyErrors.retry_max_retries = '重试次数需在 1-10 之间'
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
      bandwidth_limit_kb: policyForm.bandwidth_limit_kb,
      enabled: policyForm.enabled,
      compression: policyForm.compression,
      encryption: policyForm.encryption,
      encryption_key: policyForm.encryption ? policyForm.encryption_key : undefined,
      split_enabled: policyForm.split_enabled,
      split_size_mb: policyForm.split_enabled ? policyForm.split_size_mb : undefined,
      retry_enabled: policyForm.retry_enabled,
      retry_max_retries: policyForm.retry_enabled ? policyForm.retry_max_retries : 0,
      retention_type: policyForm.retention_type,
      retention_value: policyForm.retention_value,
      pre_commands: preCommands.value.map(c => ({ location: c.location, command: c.command })),
      post_commands: postCommands.value.map(c => ({ location: c.location, command: c.command })),
    }

    if (policyEditing.value && policyEditingId.value !== null) {
      await updatePolicy(props.instanceId, policyEditingId.value, data as UpdatePolicyRequest)
      toast.success('策略已更新')
    } else {
      await createPolicy(props.instanceId, data)
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
    await deletePolicy(props.instanceId, row.id as number)
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
    await triggerPolicy(props.instanceId, row.id as number)
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
      bandwidth_limit_kb: row.bandwidth_limit_kb as number ?? -1,
      enabled,
      compression: row.compression as boolean,
      encryption: row.encryption as boolean,
      split_enabled: row.split_enabled as boolean,
      split_size_mb: row.split_size_mb as number | undefined,
      retry_enabled: row.retry_enabled as boolean ?? true,
      retry_max_retries: row.retry_max_retries as number ?? 3,
      retention_type: row.retention_type as 'time' | 'count',
      retention_value: row.retention_value as number,
      pre_commands: (row.pre_commands as Array<{ location: string; command: string }>) ?? [],
      post_commands: (row.post_commands as Array<{ location: string; command: string }>) ?? [],
    }
    await updatePolicy(props.instanceId, row.id as number, data)
    await fetchPolicies()
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('更新失败')
  }
}

// ── Fetch ──
async function fetchPolicies() {
  policyLoading.value = true
  try {
    const res = await listPolicies(props.instanceId)
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
    const res = await listRemotes({ page: 1, page_size: 200 })
    remotes.value = res.items ?? []
  } catch {
    // silent
  }
}

function refresh() {
  fetchPolicies()
}

onMounted(() => {
  fetchPolicies()
  if (authStore.isAdmin) {
    fetchTargets()
  }
})

defineExpose({ refresh })
</script>

<template>
  <div class="tab-content">
    <div class="tab-header">
      <ListViewToggle v-model="policyViewMode" />
      <AppButton v-if="authStore.isAdmin" variant="primary" size="sm" @click="openCreatePolicy">
        <Plus :size="16" style="margin-right: 4px" />
        新增策略
      </AppButton>
    </div>

    <div v-if="policyViewMode === 'list'" class="tab-table">
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

    <div v-else class="policy-card-grid">
      <div v-if="policyLoading" class="policy-card-grid__loading">加载中…</div>
      <template v-else-if="policies.length > 0">
        <div v-for="policy in policies" :key="policy.id" class="policy-card">
          <div class="policy-card__header">
            <span class="policy-card__name">{{ policy.name }}</span>
            <AppSwitch
              :model-value="policy.enabled"
              :disabled="!authStore.isAdmin"
              @update:model-value="handleTogglePolicy(policy as unknown as Record<string, unknown>, $event)"
            />
          </div>

          <div class="policy-card__body">
            <div class="policy-card__meta-row">
              <div class="policy-card__field policy-card__field--half">
                <span class="policy-card__label">目标</span>
                <span class="policy-card__value">{{ getTargetName(policy.target_id) }}</span>
              </div>
              <div class="policy-card__field policy-card__field--half">
                <span class="policy-card__label">策略类型</span>
                <StatusBadge :config="getStatusConfig(backupTypeMap, policy.type)" />
              </div>
            </div>
            <div class="policy-card__meta-row">
              <div class="policy-card__field policy-card__field--half">
                <span class="policy-card__label">调度</span>
                <span class="policy-card__value">{{ formatScheduleValue(policy.schedule_type, policy.schedule_value) }}</span>
              </div>
              <div class="policy-card__field policy-card__field--half">
                <span class="policy-card__label">上次执行</span>
                <div v-if="policy.last_execution_time" class="policy-card__execution">
                  <StatusBadge :config="getStatusConfig(taskStatusMap, policy.last_execution_status as string)" />
                  <span class="policy-card__value">{{ formatRelativeTime(policy.last_execution_time) }}</span>
                </div>
                <span v-else class="text-muted">未执行</span>
              </div>
            </div>
          </div>

          <div v-if="authStore.isAdmin" class="policy-card__footer">
            <div class="actions-cell">
              <AppButton variant="ghost" size="sm" @click="openEditPolicy(policy as unknown as Record<string, unknown>)">
                <Pencil :size="14" />
              </AppButton>
              <AppButton variant="ghost" size="sm" @click="handleTriggerPolicy(policy as unknown as Record<string, unknown>)">
                <Play :size="14" />
              </AppButton>
              <AppButton variant="ghost" size="sm" @click="handleDeletePolicy(policy as unknown as Record<string, unknown>)">
                <Trash2 :size="14" class="text-error" />
              </AppButton>
            </div>
          </div>
        </div>
      </template>
      <div v-else class="policy-card-grid__empty">暂无策略</div>
    </div>
  </div>

  <!-- Policy Create/Edit Modal -->
  <AppModal v-model:visible="policyModalVisible" :title="policyEditing ? '编辑策略' : '新增策略'" width="720px">
    <form @submit.prevent="handlePolicySubmit">
      <AppTabs :tabs="modalTabs" :active-key="modalActiveTab" @update:active-key="modalActiveTab = $event">
        <!-- Tab 1: 基本 -->
        <template #tab-basic>
          <div class="policy-modal-grid">
            <div class="policy-modal-col">
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

                <AppFormItem v-if="policyForm.schedule_mode === 'interval'" label="执行间隔" :required="true"
                  :error="policyErrors.schedule_input">
                  <div class="schedule-interval-row">
                    <AppInput v-model="policyForm.interval_value" type="number" placeholder="数值" />
                    <AppSelect v-model="policyForm.interval_unit" :options="intervalUnitOptions" />
                  </div>
                </AppFormItem>

                <AppFormItem v-if="policyForm.schedule_mode === 'cron_custom'" label="Cron 表达式" :required="true"
                  :error="policyErrors.schedule_input">
                  <AppInput v-model="policyForm.schedule_input" placeholder="分 时 日 月 周，如 0 2 * * *" />
                </AppFormItem>
              </AppFormGroup>
            </div>

            <div class="policy-modal-col">
              <AppFormGroup>
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
              </AppFormGroup>
            </div>
          </div>
        </template>

        <!-- Tab 2: 高级 -->
        <template #tab-advanced>
          <div class="policy-modal-grid">
            <div class="policy-modal-col">
              <AppFormGroup>
                <AppFormItem label="失败自动重试">
                  <AppSwitch v-model="policyForm.retry_enabled" />
                </AppFormItem>

                <AppFormItem v-if="policyForm.retry_enabled" label="最大重试次数" :required="true"
                  :error="policyErrors.retry_max_retries">
                  <AppInput v-model="policyForm.retry_max_retries" type="number" placeholder="1-10，默认 3" />
                </AppFormItem>

                <p v-if="policyForm.retry_enabled" class="policy-modal-hint">
                  失败后依次等待 5s、10s、15s… 再自动重试，重试不阻塞其他任务。
                </p>

                <AppFormItem label="源端限流 (KB/s)" :error="policyErrors.bandwidth_limit_kb">
                  <AppInput v-model="policyForm.bandwidth_limit_kb" type="number" placeholder="-1 表示不限速" />
                </AppFormItem>

                <p class="policy-modal-hint">
                  仅在从源端 SSH 拉取到本机时生效，输入 -1 表示不限制。
                </p>
              </AppFormGroup>
            </div>

            <div class="policy-modal-col">
              <template v-if="policyForm.type === 'cold'">
                <div class="form-divider" style="margin-top: 0; border-top: none; padding-top: 0;">冷备份选项</div>
                <AppFormGroup>
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
                </AppFormGroup>
              </template>
              <div v-else class="policy-modal-empty-hint">
                当前为滚动备份模式，无额外高级选项。
              </div>
            </div>
          </div>
        </template>

        <!-- Tab 3: 前后命令 -->
        <template #tab-commands>
          <div class="hook-commands-section">
            <!-- Pre-backup commands -->
            <div class="hook-commands-block">
              <div class="hook-commands-label">备份前执行命令</div>
              <div class="hook-commands-list">
                <div
                  v-for="(cmd, idx) in preCommands"
                  :key="cmd.id"
                  class="hook-command-row"
                  :class="{ 'hook-command-row--dragging': dragState?.list === 'pre' && dragState.index === idx }"
                  draggable="true"
                  @dragstart="onDragStart('pre', idx, $event)"
                  @dragover="onDragOver('pre', idx, $event)"
                  @drop="onDrop('pre', idx, $event)"
                  @dragend="onDragEnd"
                >
                  <div class="hook-command-grip" title="拖拽排序">
                    <GripVertical :size="16" />
                  </div>
                  <div class="hook-command-location">
                    <AppSelect v-model="cmd.location" :options="commandLocationOptions" />
                  </div>
                  <div class="hook-command-input">
                    <AppInput v-model="cmd.command" placeholder="输入要执行的命令" />
                  </div>
                  <button type="button" class="hook-command-remove" @click="removeCommand('pre', idx)" title="删除">
                    <X :size="14" />
                  </button>
                </div>
                <button type="button" class="hook-command-add" @click="addCommand('pre')">
                  <Plus :size="14" />
                  <span>添加命令</span>
                </button>
              </div>
            </div>

            <!-- Post-backup commands -->
            <div class="hook-commands-block">
              <div class="hook-commands-label">备份后执行命令</div>
              <div class="hook-commands-list">
                <div
                  v-for="(cmd, idx) in postCommands"
                  :key="cmd.id"
                  class="hook-command-row"
                  :class="{ 'hook-command-row--dragging': dragState?.list === 'post' && dragState.index === idx }"
                  draggable="true"
                  @dragstart="onDragStart('post', idx, $event)"
                  @dragover="onDragOver('post', idx, $event)"
                  @drop="onDrop('post', idx, $event)"
                  @dragend="onDragEnd"
                >
                  <div class="hook-command-grip" title="拖拽排序">
                    <GripVertical :size="16" />
                  </div>
                  <div class="hook-command-location">
                    <AppSelect v-model="cmd.location" :options="commandLocationOptions" />
                  </div>
                  <div class="hook-command-input">
                    <AppInput v-model="cmd.command" placeholder="输入要执行的命令" />
                  </div>
                  <button type="button" class="hook-command-remove" @click="removeCommand('post', idx)" title="删除">
                    <X :size="14" />
                  </button>
                </div>
                <button type="button" class="hook-command-add" @click="addCommand('post')">
                  <Plus :size="14" />
                  <span>添加命令</span>
                </button>
              </div>
            </div>
          </div>
        </template>
      </AppTabs>
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
</template>

<style scoped>
.tab-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-top: 16px;
}

.tab-header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 12px;
  flex-wrap: wrap;
}

.tab-table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.policy-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.policy-card-grid__loading,
.policy-card-grid__empty {
  grid-column: 1 / -1;
  text-align: center;
  padding: 40px 0;
  color: var(--text-muted);
}

.policy-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  background: var(--surface-raised);
}

.policy-card__header,
.policy-card__footer {
  display: flex;
  align-items: center;
  gap: 12px;
}

.policy-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.policy-card__footer {
  justify-content: flex-end;
}

.policy-card__body {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.policy-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.policy-card__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.policy-card__field--half {
  flex: 1;
}

.policy-card__meta-row {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.policy-card__label {
  font-size: 12px;
  color: var(--text-muted);
}

.policy-card__value {
  font-size: 13px;
  color: var(--text-primary);
  line-height: 1.5;
  overflow-wrap: anywhere;
}

.policy-card__execution {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.actions-cell {
  display: flex;
  gap: 4px;
}

.last-exec-time {
  font-size: 12px;
  color: var(--text-muted);
  margin-left: 6px;
}

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

.schedule-interval-row > :first-child {
  flex: 1;
}

.schedule-interval-row > :last-child {
  width: 100px;
  flex-shrink: 0;
}

.policy-modal-hint {
  font-size: var(--font-size-xs, 12px);
  color: var(--text-muted);
  margin: -4px 0 8px;
  line-height: 1.5;
}

/* Policy modal two-column layout */
.policy-modal-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0 24px;
}

.policy-modal-col {
  min-width: 0;
}

.policy-modal-empty-hint {
  font-size: 13px;
  color: var(--text-muted);
  padding: 8px 0;
}

@media (max-width: 680px) {
  .policy-modal-grid {
    grid-template-columns: 1fr;
  }
}

/* ── Hook commands (Tab 3) ── */
.hook-commands-section {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.hook-commands-block {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.hook-commands-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
}

.hook-commands-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.hook-command-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  background: var(--surface-raised);
  transition: box-shadow var(--transition-fast), opacity var(--transition-fast);
}

.hook-command-row--dragging {
  opacity: 0.5;
}

.hook-command-row:hover {
  box-shadow: 0 1px 4px color-mix(in srgb, var(--text-primary) 8%, transparent);
}

.hook-command-grip {
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: grab;
  color: var(--text-muted);
  flex-shrink: 0;
  padding: 2px;
}

.hook-command-grip:active {
  cursor: grabbing;
}

.hook-command-location {
  flex-shrink: 0;
  width: 130px;
}

.hook-command-input {
  flex: 1;
  min-width: 0;
}

.hook-command-remove {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  flex-shrink: 0;
  transition: all var(--transition-fast);
}

.hook-command-remove:hover {
  background: color-mix(in srgb, var(--error-500) 12%, transparent);
  color: var(--error-500);
}

.hook-command-add {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 10px;
  border: 2px dashed var(--border-default);
  border-radius: var(--radius-md);
  background: transparent;
  color: var(--text-muted);
  font-size: 13px;
  cursor: pointer;
  transition: all var(--transition-fast);
}

.hook-command-add:hover {
  border-color: var(--primary-500);
  color: var(--primary-500);
  background: color-mix(in srgb, var(--primary-500) 5%, transparent);
}

@media (max-width: 767px) {
  .policy-card-grid {
    grid-template-columns: 1fr;
  }

  .policy-card__header {
    align-items: flex-start;
  }

  .policy-card__meta-row {
    flex-direction: column;
  }
}
</style>
