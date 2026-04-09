<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { listInstances, createInstance, deleteInstance } from '../../api/instances'
import { listRemotes } from '../../api/remotes'
import { useAuthStore } from '../../stores/auth'
import { useToastStore } from '../../stores/toast'
import { useConfirm } from '../../composables/useConfirm'
import { ApiBusinessError } from '../../api/client'
import { formatRelativeTime } from '../../utils/time'
import { EXCLUDE_PATTERN_HELP_EXAMPLES, normalizeExcludePatternsInput } from '../../utils/exclude-patterns'
import type { InstanceListItem, CreateInstanceRequest } from '../../types/instance'
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
import AppConfirm from '../../components/AppConfirm.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import { getDRLevelColor } from '../../utils/disaster-recovery'
import {
  taskStatusMap, instanceStatusMap,
  getStatusConfig,
} from '../../utils/status-config'
import { Plus, Eye, Trash2, CircleHelp } from 'lucide-vue-next'

const router = useRouter()
const authStore = useAuthStore()
const toast = useToastStore()
const { confirm } = useConfirm()

// ── List state ──
const instances = ref<InstanceListItem[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

// ── Modal state ──
const modalVisible = ref(false)
const submitting = ref(false)

const form = reactive({
  name: '',
  source_type: 'local' as 'local' | 'ssh',
  source_path: '',
  exclude_patterns_text: '',
  remote_config_id: undefined as number | undefined,
})

const errors = reactive({
  name: '',
  source_path: '',
  remote_config_id: '',
})

// ── Remote configs ──
const remotes = ref<RemoteConfig[]>([])

// ── Columns ──
const columns = computed<TableColumn[]>(() => {
  const cols: TableColumn[] = [
    { key: 'name', title: '名称' },
    { key: 'source', title: '数据源' },
    { key: 'status', title: '状态' },
    { key: 'dr_score', title: '容灾率', width: '100px' },
    { key: 'last_backup_status', title: '上次备份结果' },
    { key: 'last_backup_time', title: '上次备份时间' },
    { key: 'actions', title: '操作', width: '120px' },
  ]
  return cols
})

const sourceTypeLabel: Record<string, string> = {
  local: '本地',
  ssh: 'SSH',
}

const sourceTypeOptions = [
  { label: '本地', value: 'local' },
  { label: 'SSH', value: 'ssh' },
]

const remoteOptions = computed(() =>
  remotes.value.map((r) => ({ label: r.name, value: r.id })),
)

// statusVariant / statusLabel / backupStatusVariant / backupStatusLabel removed – using StatusBadge

const excludePatternHelpText = EXCLUDE_PATTERN_HELP_EXAMPLES.join('\n')

// ── Fetch ──
async function fetchList() {
  loading.value = true
  try {
    const res = await listInstances({ page: page.value, page_size: pageSize.value })
    instances.value = res.items ?? []
    total.value = res.total
  } catch {
    toast.error('加载实例列表失败')
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
  if (authStore.isAdmin) {
    fetchRemotes()
  }
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

// ── Form ──
function resetForm() {
  form.name = ''
  form.source_type = 'local'
  form.source_path = ''
  form.exclude_patterns_text = ''
  form.remote_config_id = undefined
  errors.name = ''
  errors.source_path = ''
  errors.remote_config_id = ''
}

function openCreateModal() {
  resetForm()
  fetchRemotes()
  modalVisible.value = true
}

function validateForm(): boolean {
  let valid = true
  errors.name = ''
  errors.source_path = ''
  errors.remote_config_id = ''

  if (!form.name.trim()) {
    errors.name = '名称不能为空'
    valid = false
  }
  if (!form.source_path.trim()) {
    errors.source_path = '数据源路径不能为空'
    valid = false
  }
  if (form.source_type === 'ssh' && !form.remote_config_id) {
    errors.remote_config_id = '请选择关联远程配置'
    valid = false
  }
  return valid
}

async function handleSubmit() {
  if (!validateForm()) return

  submitting.value = true
  try {
    const excludePatterns = normalizeExcludePatternsInput(form.exclude_patterns_text)
    const data: CreateInstanceRequest = {
      name: form.name.trim(),
      source_type: form.source_type,
      source_path: form.source_path.trim(),
      exclude_patterns: excludePatterns.length > 0 ? excludePatterns : undefined,
      remote_config_id: form.source_type === 'ssh' ? form.remote_config_id : undefined,
    }

    await createInstance(data)
    toast.success('实例已创建')
    modalVisible.value = false
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('创建失败')
    }
  } finally {
    submitting.value = false
  }
}

// ── Delete ──
async function handleDelete(row: Record<string, unknown>) {
  const ok = await confirm({
    title: '删除实例',
    message: `确定要删除「${row.name}」吗？将删除所有关联的策略、备份和任务，此操作不可撤销。`,
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return

  try {
    await deleteInstance(row.id as number)
    toast.success('实例已删除')
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('删除失败')
    }
  }
}

function goToDetail(row: Record<string, unknown>) {
  router.push(`/instances/${row.id}`)
}
</script>

<template>
  <div class="instance-list-page">
    <!-- Header -->
    <div class="instance-list-page__header">
      <h2 class="instance-list-page__title">备份实例</h2>
      <AppButton v-if="authStore.isAdmin" variant="primary" size="sm" @click="openCreateModal">
        <Plus :size="16" style="margin-right: 4px" />
        新增实例
      </AppButton>
    </div>

    <!-- Table -->
    <div class="instance-list-page__table">
      <AppTable :columns="columns" :data="instances" :loading="loading">
        <template #cell-name="{ row }">
          <a class="instance-name-link" @click.prevent="goToDetail(row)">{{ row.name }}</a>
        </template>

        <template #cell-source="{ row }">
          {{ sourceTypeLabel[row.source_type as string] ?? row.source_type }}: {{ row.source_path }}
        </template>

        <template #cell-status="{ row }">
          <StatusBadge :config="getStatusConfig(instanceStatusMap, row.status as string)" />
        </template>

        <template #cell-dr_score="{ row }">
          <span v-if="row.dr_score != null" class="dr-score-cell">
            <span class="dr-score-dot" :style="{ background: getDRLevelColor(row.dr_level as string) }" />
            <span>{{ Math.round(row.dr_score as number) }}</span>
          </span>
          <span v-else class="text-muted">—</span>
        </template>

        <template #cell-last_backup_status="{ row }">
          <StatusBadge
            v-if="row.last_backup_status"
            :config="getStatusConfig(taskStatusMap, row.last_backup_status as string)"
          />
          <span v-else class="text-muted">无记录</span>
        </template>

        <template #cell-last_backup_time="{ row }">
          <span v-if="row.last_backup_time">{{ formatRelativeTime(row.last_backup_time as string) }}</span>
          <span v-else class="text-muted">—</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="instance-list-page__actions">
            <AppButton variant="ghost" size="sm" @click="goToDetail(row)">
              <Eye :size="14" />
            </AppButton>
            <AppButton
              v-if="authStore.isAdmin"
              variant="ghost"
              size="sm"
              :disabled="row.status === 'running'"
              @click="handleDelete(row)"
            >
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

    <!-- Create Modal -->
    <AppModal v-model:visible="modalVisible" title="新增实例" width="520px">
      <form @submit.prevent="handleSubmit">
        <AppFormGroup>
          <AppFormItem label="实例名称" :required="true" :error="errors.name">
            <AppInput v-model="form.name" placeholder="例如：我的应用数据" />
          </AppFormItem>

          <AppFormItem label="数据源类型" :required="true">
            <AppSelect v-model="form.source_type" :options="sourceTypeOptions" />
          </AppFormItem>

          <AppFormItem label="数据源路径" :required="true" :error="errors.source_path">
            <AppInput v-model="form.source_path" placeholder="例如：/data/myapp" />
          </AppFormItem>

          <AppFormItem :error="''">
            <template #label>
              <span class="exclude-field-label">
                <span>排除文件</span>
                <span class="exclude-help" :title="excludePatternHelpText" aria-label="排除规则示例">
                  <CircleHelp :size="14" />
                </span>
              </span>
            </template>
            <textarea
              v-model="form.exclude_patterns_text"
              class="instance-textarea"
              rows="5"
              placeholder="每行一条规则，例如：&#10;*.log&#10;node_modules/&#10;cache/**"
            />
          </AppFormItem>

          <AppFormItem
            v-if="form.source_type === 'ssh'"
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
          <AppButton variant="primary" size="md" :loading="submitting" @click="handleSubmit">
            创建
          </AppButton>
        </div>
      </template>
    </AppModal>

    <AppConfirm />
  </div>
</template>

<style scoped>
.instance-list-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.instance-list-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.instance-list-page__title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
}
.instance-list-page__table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.instance-list-page__actions {
  display: flex;
  gap: 4px;
}
.instance-name-link {
  color: var(--primary-600);
  cursor: pointer;
  font-weight: 500;
  text-decoration: none;
}
.instance-name-link:hover {
  text-decoration: underline;
}
.text-muted {
  color: var(--text-muted);
  font-size: 13px;
}
.text-error {
  color: var(--error-500);
}
.dr-score-cell {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: 500;
}
.dr-score-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  flex-shrink: 0;
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
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
</style>
