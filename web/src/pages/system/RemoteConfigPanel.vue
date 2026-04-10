<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { listRemotes, createRemote, updateRemote, deleteRemote, testRemoteConnection } from '../../api/remotes'
import { useListViewPreferenceStore, type ListViewMode, SHARED_LIST_VIEW_PREFERENCE_KEY } from '../../stores/list-view-preference'
import { useToastStore } from '../../stores/toast'
import { useConfirm } from '../../composables/useConfirm'
import { ApiBusinessError } from '../../api/client'
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
import ListViewToggle from '../../components/ListViewToggle.vue'
import AppConfirm from '../../components/AppConfirm.vue'
import { Plus, Pencil, Plug, Trash2 } from 'lucide-vue-next'

const toast = useToastStore()
const { confirm } = useConfirm()
const listViewPreferenceStore = useListViewPreferenceStore()

// ── List state ──
const remotes = ref<RemoteConfig[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const inferredViewMode: ListViewMode = typeof window !== 'undefined' && window.innerWidth < 768 ? 'card' : 'list'

listViewPreferenceStore.initializeViewMode(SHARED_LIST_VIEW_PREFERENCE_KEY, inferredViewMode)

const viewMode = computed({
  get: (): ListViewMode => listViewPreferenceStore.getViewMode(SHARED_LIST_VIEW_PREFERENCE_KEY) ?? inferredViewMode,
  set: (mode: ListViewMode) => listViewPreferenceStore.setViewMode(SHARED_LIST_VIEW_PREFERENCE_KEY, mode),
})

// ── Modal state ──
const modalVisible = ref(false)
const isEditing = ref(false)
const editingId = ref<number | null>(null)
const submitting = ref(false)

const form = reactive({
  name: '',
  type: 'ssh' as 'ssh' | 'openlist' | 'cloud',
  host: '',
  port: 22,
  username: '',
  password: '',
})
const privateKeyFile = ref<File | null>(null)

const errors = reactive({
  name: '',
  host: '',
  port: '',
  username: '',
  password: '',
  privateKey: '',
})

// ── Test connection state ──
const testingIds = ref<Set<number>>(new Set())

// ── Columns ──
const columns: TableColumn[] = [
  { key: 'name', title: '名称', sortable: true },
  { key: 'type', title: '类型' },
  { key: 'host', title: '主机' },
  { key: 'port', title: '端口' },
  { key: 'username', title: '用户名' },
  { key: 'created_at', title: '创建时间', sortable: true },
  { key: 'actions', title: '操作', width: '200px' },
]

const typeOptions = [
  { label: 'SSH', value: 'ssh' },
  { label: 'OpenList', value: 'openlist' },
  { label: '更多云存储（即将支持）', value: 'cloud' },
]

const tableData = computed(() =>
  remotes.value.map((r) => ({
    ...r,
    type_label: r.type === 'ssh' ? 'SSH' : r.type === 'openlist' ? 'OpenList' : '更多云存储',
    created_at_display: new Date(r.created_at).toLocaleString(),
  })),
)

// ── Fetch list ──
async function fetchList() {
  loading.value = true
  try {
    const res = await listRemotes({ page: page.value, page_size: pageSize.value })
    remotes.value = res.items ?? []
    total.value = res.total
  } catch (e) {
    toast.error('加载远程配置失败')
  } finally {
    loading.value = false
  }
}

onMounted(fetchList)

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
  form.type = 'ssh'
  form.host = ''
  form.port = 22
  form.username = ''
  form.password = ''
  privateKeyFile.value = null
  errors.name = ''
  errors.host = ''
  errors.port = ''
  errors.username = ''
  errors.password = ''
  errors.privateKey = ''
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
  form.type = row.type as 'ssh' | 'openlist' | 'cloud'
  form.host = (row.host as string) ?? ''
  form.port = (row.port as number) ?? 22
  form.username = (row.username as string) ?? ''
  modalVisible.value = true
}

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  privateKeyFile.value = target.files?.[0] ?? null
  errors.privateKey = ''
}

function validateForm(): boolean {
  let valid = true
  errors.name = ''
  errors.host = ''
  errors.port = ''
  errors.username = ''
  errors.password = ''
  errors.privateKey = ''

  if (!form.name.trim()) {
    errors.name = '名称不能为空'
    valid = false
  }

  if (form.type === 'ssh') {
    if (!form.host.trim()) {
      errors.host = '主机地址不能为空'
      valid = false
    }
    if (!form.port || form.port < 1 || form.port > 65535) {
      errors.port = '端口范围 1-65535'
      valid = false
    }
    if (!form.username.trim()) {
      errors.username = '用户名不能为空'
      valid = false
    }
    if (!isEditing.value && !privateKeyFile.value) {
      errors.privateKey = '私钥文件不能为空'
      valid = false
    }
  }

  if (form.type === 'openlist') {
    if (!form.host.trim()) {
      errors.host = 'OpenList 地址不能为空'
      valid = false
    }
    if (!form.username.trim()) {
      errors.username = '用户名不能为空'
      valid = false
    }
    if (!isEditing.value && !form.password.trim()) {
      errors.password = '密码不能为空'
      valid = false
    }
  }

  return valid
}

async function handleSubmit() {
  if (!validateForm()) return

  submitting.value = true
  try {
    const fd = new FormData()
    fd.append('name', form.name.trim())
    fd.append('type', form.type)
    if (form.type === 'ssh') {
      fd.append('host', form.host.trim())
      fd.append('port', String(form.port))
      fd.append('username', form.username.trim())
      if (privateKeyFile.value) {
        fd.append('private_key', privateKeyFile.value)
      }
    } else if (form.type === 'openlist') {
      fd.append('host', form.host.trim())
      fd.append('username', form.username.trim())
      if (form.password.trim()) {
        fd.append('password', form.password.trim())
      }
    }

    if (isEditing.value && editingId.value !== null) {
      await updateRemote(editingId.value, fd)
      toast.success('远程配置已更新')
    } else {
      await createRemote(fd)
      toast.success('远程配置已创建')
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

// ── Connection test ──
async function handleTest(row: Record<string, unknown>) {
  const id = row.id as number
  testingIds.value.add(id)
  try {
    const res = await testRemoteConnection(id)
    toast.success(res.message || '连接测试成功')
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message || '连接测试失败')
    } else {
      toast.error('连接测试失败')
    }
  } finally {
    testingIds.value.delete(id)
  }
}

// ── Delete ──
async function handleDelete(row: Record<string, unknown>) {
  const ok = await confirm({
    title: '删除远程配置',
    message: `确定要删除「${row.name}」吗？此操作不可撤销。`,
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return

  try {
    await deleteRemote(row.id as number)
    toast.success('远程配置已删除')
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
  <div class="remote-config-page">
    <!-- Header -->
    <div class="remote-config-page__header">
      <div class="remote-config-page__header-actions">
        <ListViewToggle v-model="viewMode" />
        <AppButton variant="primary" size="sm" @click="openCreateModal">
          <Plus :size="16" style="margin-right: 4px" />
          新增远程配置
        </AppButton>
      </div>
    </div>

    <!-- Table -->
    <div v-if="viewMode === 'list'" class="remote-config-page__table">
      <AppTable :columns="columns" :data="tableData" :loading="loading">
        <template #cell-type="{ row }">
          {{ row.type_label }}
        </template>
        <template #cell-created_at="{ row }">
          {{ row.created_at_display }}
        </template>
        <template #cell-actions="{ row }">
          <div class="remote-config-page__actions">
            <AppButton variant="ghost" size="sm" @click="openEditModal(row)">
              <Pencil :size="14" />
            </AppButton>
            <AppButton
              variant="ghost"
              size="sm"
              :disabled="row.type === 'cloud'"
              :loading="testingIds.has(row.id as number)"
              @click="handleTest(row)"
            >
              <Plug :size="14" />
            </AppButton>
            <AppButton variant="ghost" size="sm" @click="handleDelete(row)">
              <Trash2 :size="14" class="text-error" />
            </AppButton>
          </div>
        </template>
      </AppTable>
    </div>

    <div v-else class="remote-config-card-grid">
      <div v-if="loading" class="remote-config-card-grid__loading">加载中…</div>
      <template v-else-if="tableData.length > 0">
        <div v-for="remote in tableData" :key="remote.id" class="remote-config-card">
          <div class="remote-config-card__header">
            <span class="remote-config-card__name">{{ remote.name }}</span>
            <span class="remote-config-card__type">{{ remote.type_label }}</span>
          </div>

          <div class="remote-config-card__body">
            <div class="remote-config-card__field">
              <span class="remote-config-card__label">主机</span>
              <span class="remote-config-card__value">{{ remote.host || '—' }}</span>
            </div>
            <div class="remote-config-card__field">
              <span class="remote-config-card__label">端口</span>
              <span class="remote-config-card__value">{{ remote.port || '—' }}</span>
            </div>
            <div class="remote-config-card__field">
              <span class="remote-config-card__label">用户名</span>
              <span class="remote-config-card__value">{{ remote.username || '—' }}</span>
            </div>
            <div class="remote-config-card__field">
              <span class="remote-config-card__label">创建时间</span>
              <span class="remote-config-card__value">{{ remote.created_at_display }}</span>
            </div>
          </div>

          <div class="remote-config-card__footer">
            <div class="remote-config-page__actions">
              <AppButton variant="ghost" size="sm" @click="openEditModal(remote as unknown as Record<string, unknown>)">
                <Pencil :size="14" />
              </AppButton>
              <AppButton
                variant="ghost"
                size="sm"
                :disabled="remote.type === 'cloud'"
                :loading="testingIds.has(remote.id)"
                @click="handleTest(remote as unknown as Record<string, unknown>)"
              >
                <Plug :size="14" />
              </AppButton>
              <AppButton variant="ghost" size="sm" @click="handleDelete(remote as unknown as Record<string, unknown>)">
                <Trash2 :size="14" class="text-error" />
              </AppButton>
            </div>
          </div>
        </div>
      </template>
      <div v-else class="remote-config-card-grid__empty">暂无远程配置</div>
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
    <AppModal v-model:visible="modalVisible" :title="isEditing ? '编辑远程配置' : '新增远程配置'" width="520px">
      <form @submit.prevent="handleSubmit">
        <AppFormGroup>
          <AppFormItem label="名称" :required="true" :error="errors.name">
            <AppInput v-model="form.name" placeholder="例如：生产服务器" />
          </AppFormItem>

          <AppFormItem label="类型" :required="true">
            <AppSelect
              v-model="form.type"
              :options="typeOptions.map(o => ({
                ...o,
                label: o.value === 'cloud' ? o.label : o.label,
              }))"
              :disabled="isEditing"
            />
            <p v-if="form.type === 'cloud'" class="form-hint">更多云存储类型后续补充，当前已支持 OpenList。</p>
          </AppFormItem>

          <template v-if="form.type === 'ssh'">
            <AppFormItem label="主机地址" :required="true" :error="errors.host">
              <AppInput v-model="form.host" placeholder="例如：192.168.1.100" />
            </AppFormItem>

            <AppFormItem label="端口" :required="true" :error="errors.port">
              <AppInput v-model="form.port" type="number" placeholder="22" />
            </AppFormItem>

            <AppFormItem label="用户名" :required="true" :error="errors.username">
              <AppInput v-model="form.username" placeholder="例如：root" />
            </AppFormItem>

            <AppFormItem
              :label="isEditing ? '私钥文件（留空则不更新）' : '私钥文件'"
              :required="!isEditing"
              :error="errors.privateKey"
            >
              <input
                type="file"
                class="file-input"
                accept=".pem,.key,.pub,.ppk,*"
                @change="onFileChange"
              />
            </AppFormItem>
          </template>

          <template v-else-if="form.type === 'openlist'">
            <AppFormItem label="OpenList 地址" :required="true" :error="errors.host">
              <AppInput v-model="form.host" placeholder="例如：https://openlist.example.com" />
            </AppFormItem>

            <AppFormItem label="用户名" :required="true" :error="errors.username">
              <AppInput v-model="form.username" placeholder="例如：admin" />
            </AppFormItem>

            <AppFormItem
              :label="isEditing ? '密码（留空则不更新）' : '密码'"
              :required="!isEditing"
              :error="errors.password"
            >
              <AppInput v-model="form.password" type="password" placeholder="输入 OpenList 登录密码" />
            </AppFormItem>
          </template>
        </AppFormGroup>
      </form>

      <template #footer>
        <div class="modal-footer">
          <AppButton variant="outline" size="md" @click="modalVisible = false">取消</AppButton>
          <AppButton
            variant="primary"
            size="md"
            :loading="submitting"
            :disabled="form.type === 'cloud'"
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
.remote-config-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}
.remote-config-page__header {
  display: flex;
  align-items: center;
  justify-content: flex-end;
}
.remote-config-page__header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}
.remote-config-page__table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}
.remote-config-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}
.remote-config-card-grid__loading,
.remote-config-card-grid__empty {
  grid-column: 1 / -1;
  text-align: center;
  padding: 40px 0;
  color: var(--text-muted);
}
.remote-config-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  background: var(--surface-raised);
}
.remote-config-card__header,
.remote-config-card__footer {
  display: flex;
  align-items: center;
  gap: 12px;
}
.remote-config-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
.remote-config-card__footer {
  justify-content: flex-end;
}
.remote-config-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}
.remote-config-card__type {
  font-size: 12px;
  color: var(--primary-600);
  background: var(--primary-50);
  padding: 4px 8px;
  border-radius: 999px;
}
.remote-config-card__body {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}
.remote-config-card__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}
.remote-config-card__label {
  font-size: 12px;
  color: var(--text-muted);
}
.remote-config-card__value {
  font-size: 13px;
  color: var(--text-primary);
  overflow-wrap: anywhere;
}
.remote-config-page__actions {
  display: flex;
  gap: 4px;
}
.text-error {
  color: var(--error-500);
}
.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}
.file-input {
  width: 100%;
  padding: 8px 12px;
  font-size: 14px;
  color: var(--text-primary);
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  cursor: pointer;
}
.file-input::file-selector-button {
  padding: 4px 12px;
  margin-right: 12px;
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary);
  background: var(--surface-sunken);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  cursor: pointer;
  transition: background var(--transition-fast);
}
.file-input::file-selector-button:hover {
  background: var(--surface-overlay);
}
.form-hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

@media (max-width: 767px) {
  .remote-config-card-grid {
    grid-template-columns: 1fr;
  }

  .remote-config-card__body {
    grid-template-columns: 1fr;
  }
}
</style>
