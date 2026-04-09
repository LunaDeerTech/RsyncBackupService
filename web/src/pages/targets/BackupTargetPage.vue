<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { listTargets, createTarget, updateTarget, deleteTarget, healthCheck } from '../../api/targets'
import { listRemotes } from '../../api/remotes'
import { useToastStore } from '../../stores/toast'
import { useConfirm } from '../../composables/useConfirm'
import { ApiBusinessError } from '../../api/client'
import { formatBytes } from '../../utils/format'
import type { BackupTarget, CreateTargetRequest, UpdateTargetRequest } from '../../types/target'
import type { RemoteConfig } from '../../types/remote'
import type { TableColumn } from '../../components/AppTable.vue'
import AppTable from '../../components/AppTable.vue'
import AppPagination from '../../components/AppPagination.vue'
import AppModal from '../../components/AppModal.vue'
import AppFormGroup from '../../components/AppFormGroup.vue'
import AppFormItem from '../../components/AppFormItem.vue'
import AppInput from '../../components/AppInput.vue'
import AppSelect from '../../components/AppSelect.vue'
import AppButton from '../../components/AppButton.vue'
import AppProgress from '../../components/AppProgress.vue'
import AppConfirm from '../../components/AppConfirm.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import { Plus, Pencil, HeartPulse, Trash2 } from 'lucide-vue-next'
import {
  healthStatusMap, backupTypeMap,
  getStatusConfig,
} from '../../utils/status-config'

const toast = useToastStore()
const { confirm } = useConfirm()

// ── List state ──
const targets = ref<BackupTarget[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

// ── Modal state ──
const modalVisible = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)

const form = reactive({
  name: '',
  backup_type: 'rolling' as 'rolling' | 'cold',
  storage_type: 'local' as 'local' | 'ssh' | 'cloud',
  storage_path: '',
  remote_config_id: undefined as number | undefined,
})

const errors = reactive({
  name: '',
  storage_path: '',
  remote_config_id: '',
})

// ── Remote configs for select ──
const remotes = ref<RemoteConfig[]>([])

// ── Health check loading state ──
const checkingIds = ref<Set<number>>(new Set())

// ── Columns ──
const columns: TableColumn[] = [
  { key: 'name', title: '名称', sortable: true },
  { key: 'backup_type', title: '备份类型' },
  { key: 'storage_type', title: '存储类型' },
  { key: 'storage_path', title: '存储路径' },
  { key: 'capacity', title: '容量使用', width: '180px' },
  { key: 'health_status', title: '健康状态' },
  { key: 'last_health_check', title: '上次检查' },
  { key: 'actions', title: '操作', width: '160px' },
]

const backupTypeOptions = [
  { label: '滚动备份', value: 'rolling' },
  { label: '冷备份', value: 'cold' },
]

const storageTypeOptions = computed(() => {
  const options = [
    { label: '本地存储', value: 'local' },
    { label: 'SSH 远程', value: 'ssh' },
  ]
  if (form.backup_type === 'cold') {
    options.push({ label: '云存储（即将支持）', value: 'cloud' })
  }
  return options
})

const remoteOptions = computed(() =>
  remotes.value.map((r) => ({ label: r.name, value: r.id })),
)

const storageTypeLabel: Record<string, string> = {
  local: '本地',
  ssh: 'SSH',
  cloud: '云存储',
}

// healthStatusVariant / healthStatusLabel removed – using StatusBadge

function capacityPercent(row: BackupTarget): number | null {
  if (row.total_capacity_bytes == null || row.used_capacity_bytes == null || row.total_capacity_bytes === 0) {
    return null
  }
  return Math.round((row.used_capacity_bytes / row.total_capacity_bytes) * 100)
}

function capacityVariant(percent: number | null): 'primary' | 'success' | 'warning' | 'error' {
  if (percent == null) return 'primary'
  if (percent > 95) return 'error'
  if (percent > 80) return 'warning'
  return 'primary'
}

function formatTime(dateStr?: string): string {
  if (!dateStr) return '—'
  return new Date(dateStr).toLocaleString()
}

// ── Watchers ──
watch(() => form.backup_type, () => {
  if (isEditing.value) return
  // Reset storage_type if not valid for new backup_type
  if (form.backup_type === 'rolling' && form.storage_type === 'cloud') {
    form.storage_type = 'local'
  }
})

watch(() => form.storage_type, () => {
  if (form.storage_type !== 'ssh') {
    form.remote_config_id = undefined
  }
})

// ── Fetch list ──
async function fetchList() {
  loading.value = true
  try {
    const res = await listTargets({ page: page.value, page_size: pageSize.value })
    targets.value = res.items ?? []
    total.value = res.total
  } catch {
    toast.error('加载备份目标失败')
  } finally {
    loading.value = false
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

onMounted(() => {
  fetchList()
  fetchRemotes()
})

function onPageChange(p: number) {
  page.value = p
  fetchList()
}

function onPageSizeChange(ps: number) {
  pageSize.value = ps
  page.value = 1
  fetchList()
}

// ── Form helpers ──
function resetForm() {
  form.name = ''
  form.backup_type = 'rolling'
  form.storage_type = 'local'
  form.storage_path = ''
  form.remote_config_id = undefined
  errors.name = ''
  errors.storage_path = ''
  errors.remote_config_id = ''
}

function openCreateModal() {
  resetForm()
  isEditing.value = false
  editingId.value = null
  modalVisible.value = true
}

function openEditModal(row: Record<string, unknown>) {
  resetForm()
  isEditing.value = true
  editingId.value = row.id as number
  form.name = row.name as string
  form.backup_type = row.backup_type as 'rolling' | 'cold'
  form.storage_type = row.storage_type as 'local' | 'ssh' | 'cloud'
  form.storage_path = row.storage_path as string
  form.remote_config_id = row.remote_config_id as number | undefined
  modalVisible.value = true
}

function validateForm(): boolean {
  let valid = true
  errors.name = ''
  errors.storage_path = ''
  errors.remote_config_id = ''

  if (!form.name.trim()) {
    errors.name = '名称不能为空'
    valid = false
  }
  if (!form.storage_path.trim()) {
    errors.storage_path = '存储路径不能为空'
    valid = false
  }
  if (form.storage_type === 'ssh' && !form.remote_config_id) {
    errors.remote_config_id = '请选择关联远程配置'
    valid = false
  }
  return valid
}

async function handleSubmit() {
  if (!validateForm()) return

  submitting.value = true
  try {
    const data = {
      name: form.name.trim(),
      backup_type: form.backup_type,
      storage_type: form.storage_type,
      storage_path: form.storage_path.trim(),
      remote_config_id: form.storage_type === 'ssh' ? form.remote_config_id : undefined,
    }

    if (isEditing.value && editingId.value !== null) {
      await updateTarget(editingId.value, data as UpdateTargetRequest)
      toast.success('备份目标已更新')
    } else {
      await createTarget(data as CreateTargetRequest)
      toast.success('备份目标已创建')
    }

    modalVisible.value = false
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('操作失败')
    }
  } finally {
    submitting.value = false
  }
}

// ── Health check ──
async function handleHealthCheck(row: Record<string, unknown>) {
  const id = row.id as number
  checkingIds.value.add(id)
  try {
    const updated = await healthCheck(id)
    // Update the row in-place
    const idx = targets.value.findIndex((t) => t.id === id)
    if (idx !== -1) {
      targets.value[idx] = updated
    }
    const status = getStatusConfig(healthStatusMap, updated.health_status).label || updated.health_status
    toast.success(`健康检查完成：${status}`)
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message || '健康检查失败')
    } else {
      toast.error('健康检查失败')
    }
  } finally {
    checkingIds.value.delete(id)
  }
}

// ── Delete ──
async function handleDelete(row: Record<string, unknown>) {
  const ok = await confirm({
    title: '删除备份目标',
    message: `确定要删除「${row.name}」吗？此操作不可撤销。`,
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return

  try {
    await deleteTarget(row.id as number)
    toast.success('备份目标已删除')
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('删除失败')
    }
  }
}
</script>

<template>
  <div class="backup-target-page">
    <!-- Header -->
    <div class="backup-target-page__header">
      <AppButton variant="primary" size="sm" @click="openCreateModal">
        <Plus :size="16" style="margin-right: 4px" />
        新增备份目标
      </AppButton>
    </div>

    <!-- Table -->
    <div class="backup-target-page__table">
      <AppTable :columns="columns" :data="targets" :loading="loading">
        <template #cell-backup_type="{ row }">
          <StatusBadge :config="getStatusConfig(backupTypeMap, row.backup_type as string)" />
        </template>

        <template #cell-storage_type="{ row }">
          {{ storageTypeLabel[row.storage_type as string] ?? row.storage_type }}
        </template>

        <template #cell-capacity="{ row }">
          <div class="capacity-cell">
            <template v-if="capacityPercent(row as unknown as BackupTarget) != null">
              <AppProgress
                :value="capacityPercent(row as unknown as BackupTarget)!"
                :variant="capacityVariant(capacityPercent(row as unknown as BackupTarget))"
                size="sm"
              />
              <span class="capacity-text">
                {{ formatBytes((row as Record<string, unknown>).used_capacity_bytes as number) }}
                /
                {{ formatBytes((row as Record<string, unknown>).total_capacity_bytes as number) }}
              </span>
            </template>
            <span v-else class="capacity-na">未检测</span>
          </div>
        </template>

        <template #cell-health_status="{ row }">
          <StatusBadge :config="getStatusConfig(healthStatusMap, row.health_status as string)" />
        </template>

        <template #cell-last_health_check="{ row }">
          {{ formatTime(row.last_health_check as string | undefined) }}
        </template>

        <template #cell-actions="{ row }">
          <div class="backup-target-page__actions">
            <AppButton variant="ghost" size="sm" @click="openEditModal(row)">
              <Pencil :size="14" />
            </AppButton>
            <AppButton
              variant="ghost"
              size="sm"
              :loading="checkingIds.has(row.id as number)"
              @click="handleHealthCheck(row)"
            >
              <HeartPulse :size="14" />
            </AppButton>
            <AppButton variant="ghost" size="sm" @click="handleDelete(row)">
              <Trash2 :size="14" class="text-error" />
            </AppButton>
          </div>
        </template>
      </AppTable>
    </div>

    <!-- Pagination -->
    <AppPagination
      v-if="total > 0"
      :page="page"
      :page-size="pageSize"
      :total="total"
      @update:page="onPageChange"
      @update:page-size="onPageSizeChange"
    />

    <!-- Create / Edit Modal -->
    <AppModal v-model:visible="modalVisible" :title="isEditing ? '编辑备份目标' : '新增备份目标'" width="520px">
      <form @submit.prevent="handleSubmit">
        <AppFormGroup>
          <AppFormItem label="名称" :required="true" :error="errors.name">
            <AppInput v-model="form.name" placeholder="例如：主备份存储" />
          </AppFormItem>

          <AppFormItem label="备份类型" :required="true">
            <AppSelect
              v-model="form.backup_type"
              :options="backupTypeOptions"
              :disabled="isEditing"
            />
          </AppFormItem>

          <AppFormItem label="存储类型" :required="true">
            <AppSelect
              v-model="form.storage_type"
              :options="storageTypeOptions"
              :disabled="isEditing || form.storage_type === 'cloud'"
            />
            <p v-if="form.storage_type === 'cloud'" class="form-hint">云存储类型即将支持。</p>
          </AppFormItem>

          <AppFormItem label="存储路径" :required="true" :error="errors.storage_path">
            <AppInput v-model="form.storage_path" placeholder="例如：/data/backups" />
            <p class="form-hint">路径必须已存在且可写，系统不会自动创建目录。</p>
          </AppFormItem>

          <AppFormItem
            v-if="form.storage_type === 'ssh'"
            label="关联远程配置"
            :required="true"
            :error="errors.remote_config_id"
          >
            <AppSelect
              v-model="form.remote_config_id"
              :options="remoteOptions"
              placeholder="请选择远程配置"
            />
          </AppFormItem>
        </AppFormGroup>
      </form>

      <template #footer>
        <div class="modal-footer">
          <AppButton variant="outline" size="md" @click="modalVisible = false">取消</AppButton>
          <AppButton
            variant="primary"
            size="md"
            :loading="submitting"
            :disabled="form.storage_type === 'cloud'"
            @click="handleSubmit"
          >
            {{ isEditing ? '保存' : '创建' }}
          </AppButton>
        </div>
      </template>
    </AppModal>

    <!-- Confirm dialog -->
    <AppConfirm />
  </div>
</template>

<style scoped>
.backup-target-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.backup-target-page__header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}
.backup-target-page__table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.backup-target-page__actions {
  display: flex;
  gap: 4px;
}
.capacity-cell {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 120px;
}
.capacity-text {
  font-size: 12px;
  color: var(--text-muted);
}
.capacity-na {
  font-size: 13px;
  color: var(--text-muted);
}
.text-error {
  color: var(--error-500);
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.form-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}
</style>
