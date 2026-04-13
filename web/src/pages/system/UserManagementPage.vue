<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { listUsers, createUser, updateUser, deleteUser, resetUserPassword } from '../../api/users'
import { getRegistrationStatus, updateRegistrationStatus } from '../../api/system'
import { useAuthStore } from '../../stores/auth'
import { useListViewPreferenceStore, type ListViewMode, SHARED_LIST_VIEW_PREFERENCE_KEY } from '../../stores/list-view-preference'
import { useToastStore } from '../../stores/toast'
import { useConfirm } from '../../composables/useConfirm'
import { ApiBusinessError } from '../../api/client'
import { formatRelativeTime } from '../../utils/time'
import type { User } from '../../types/auth'
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
import AppBadge from '../../components/AppBadge.vue'
import AppSwitch from '../../components/AppSwitch.vue'
import AppConfirm from '../../components/AppConfirm.vue'
import { Plus, Pencil, RefreshCw, Trash2 } from 'lucide-vue-next'

const authStore = useAuthStore()
const listViewPreferenceStore = useListViewPreferenceStore()
const toast = useToastStore()
const { confirm } = useConfirm()

// ── List state ──
const users = ref<User[]>([])
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

// ── Registration toggle ──
const registrationEnabled = ref(false)
const registrationLoading = ref(false)
const resettingUserId = ref<number | null>(null)

// ── Modal state ──
const modalVisible = ref(false)
const submitting = ref(false)
const editingUser = ref<User | null>(null)

const form = reactive({
  email: '',
  name: '',
  role: 'viewer' as string,
})

const errors = reactive({
  email: '',
  name: '',
  role: '',
})

const isEditing = computed(() => editingUser.value !== null)

const roleOptions = [
  { label: '管理员', value: 'admin' },
  { label: '普通用户', value: 'viewer' },
]

const columns: TableColumn[] = [
  { key: 'email', title: '邮箱' },
  { key: 'name', title: '名称' },
  { key: 'role', title: '角色', width: '100px' },
  { key: 'created_at', title: '创建时间' },
  { key: 'actions', title: '操作', width: '168px' },
]

// ── Fetch ──
async function fetchList() {
  loading.value = true
  try {
    const res = await listUsers({ page: page.value, page_size: pageSize.value })
    users.value = res.items ?? []
    total.value = res.total
  } catch {
    toast.error('加载用户列表失败')
  } finally {
    loading.value = false
  }
}

async function fetchRegistrationStatus() {
  try {
    const res = await getRegistrationStatus()
    registrationEnabled.value = res.enabled
  } catch {
    // silent
  }
}

onMounted(() => {
  fetchList()
  fetchRegistrationStatus()
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

// ── Registration toggle ──
async function handleRegistrationToggle(enabled: boolean) {
  registrationLoading.value = true
  try {
    const res = await updateRegistrationStatus(enabled)
    registrationEnabled.value = res.enabled
    toast.success(enabled ? '已开启用户注册' : '已关闭用户注册')
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('更新注册状态失败')
    }
  } finally {
    registrationLoading.value = false
  }
}

// ── Form ──
function resetForm() {
  form.email = ''
  form.name = ''
  form.role = 'viewer'
  errors.email = ''
  errors.name = ''
  errors.role = ''
  editingUser.value = null
}

function openCreateModal() {
  resetForm()
  modalVisible.value = true
}

function openEditModal(row: Record<string, unknown>) {
  resetForm()
  editingUser.value = row as unknown as User
  form.name = (row.name as string) ?? ''
  form.role = (row.role as string) ?? 'viewer'
  modalVisible.value = true
}

function validateForm(): boolean {
  let valid = true
  errors.email = ''
  errors.name = ''
  errors.role = ''

  if (!isEditing.value) {
    if (!form.email.trim()) {
      errors.email = '邮箱不能为空'
      valid = false
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(form.email.trim())) {
      errors.email = '请输入有效的邮箱地址'
      valid = false
    }
  }

  if (!form.role) {
    errors.role = '请选择角色'
    valid = false
  }

  return valid
}

async function handleSubmit() {
  if (!validateForm()) return

  submitting.value = true
  try {
    if (isEditing.value) {
      await updateUser(editingUser.value!.id, {
        name: form.name.trim() || undefined,
        role: form.role,
      })
      toast.success('用户已更新')
    } else {
      await createUser({
        email: form.email.trim(),
        name: form.name.trim() || undefined,
        role: form.role,
      })
      toast.success('用户已创建，新密码已发送至邮箱；若 SMTP 未配置或发送失败，则密码已输出到服务器日志')
    }
    modalVisible.value = false
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error(isEditing.value ? '更新失败' : '创建失败')
    }
  } finally {
    submitting.value = false
  }
}

// ── Delete ──
async function handleDelete(row: Record<string, unknown>) {
  if ((row.id as number) === authStore.user?.id) {
    toast.error('不能删除自己')
    return
  }

  const ok = await confirm({
    title: '删除用户',
    message: `确定要删除「${row.email}」吗？将同步清理该用户所有权限和订阅配置，此操作不可撤销。`,
    confirmText: '删除',
    danger: true,
  })
  if (!ok) return

  try {
    await deleteUser(row.id as number)
    toast.success('用户已删除')
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('删除失败')
    }
  }
}

async function handleResetPassword(row: Record<string, unknown>) {
  const userId = row.id as number
  const ok = await confirm({
    title: '重置密码',
    message: `确定要为「${row.email}」重置登录密码吗？系统会重新生成随机密码，并尝试通过 SMTP 发送；若 SMTP 未配置或发送失败，则密码会输出到服务器日志。`,
    confirmText: '确认重置',
    danger: true,
  })
  if (!ok) return

  resettingUserId.value = userId
  try {
    await resetUserPassword(userId)
    toast.success('密码已重置，新密码已发送至邮箱；若 SMTP 未配置或发送失败，则密码已输出到服务器日志')
    await fetchList()
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('重置密码失败')
    }
  } finally {
    resettingUserId.value = null
  }
}

function roleVariant(role: string): 'default' | 'info' {
  return role === 'admin' ? 'info' : 'default'
}

function roleLabel(role: string): string {
  return role === 'admin' ? '管理员' : '普通用户'
}

function isSelf(row: Record<string, unknown>): boolean {
  return (row.id as number) === authStore.user?.id
}

function isResetting(row: Record<string, unknown>): boolean {
	return resettingUserId.value === (row.id as number)
}
</script>

<template>
  <div class="user-mgmt">
    <!-- Header -->
    <div class="user-mgmt__header">
      <div class="user-mgmt__header-left">
        <h2 class="user-mgmt__title">用户管理</h2>
      </div>
      <div class="user-mgmt__header-right">
        <div class="user-mgmt__reg-toggle">
          <span class="user-mgmt__reg-label">允许新用户注册</span>
          <AppSwitch
            :model-value="registrationEnabled"
            :disabled="registrationLoading"
            @update:model-value="handleRegistrationToggle"
          />
        </div>
        <ListViewToggle v-model="viewMode" />
        <AppButton variant="primary" size="sm" @click="openCreateModal">
          <Plus :size="16" style="margin-right: 4px" />
          新增用户
        </AppButton>
      </div>
    </div>

    <!-- Table -->
    <div v-if="viewMode === 'list'" class="user-mgmt__table">
      <AppTable :columns="columns" :data="users as unknown as Record<string, unknown>[]" :loading="loading">
        <template #cell-role="{ row }">
          <AppBadge :variant="roleVariant(row.role as string)">
            {{ roleLabel(row.role as string) }}
          </AppBadge>
        </template>

        <template #cell-created_at="{ row }">
          <span v-if="row.created_at">{{ formatRelativeTime(row.created_at as string) }}</span>
          <span v-else class="text-muted">—</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="user-mgmt__actions">
            <AppButton variant="ghost" size="sm" :loading="isResetting(row)" :disabled="isResetting(row)" title="重置密码" @click="handleResetPassword(row)">
              <RefreshCw v-if="!isResetting(row)" :size="14" />
            </AppButton>
            <AppButton variant="ghost" size="sm" @click="openEditModal(row)">
              <Pencil :size="14" />
            </AppButton>
            <AppButton
              variant="ghost"
              size="sm"
              :disabled="isSelf(row)"
              @click="handleDelete(row)"
            >
              <Trash2 :size="14" class="text-error" />
            </AppButton>
          </div>
        </template>
      </AppTable>
    </div>

    <div v-else class="user-mgmt-card-grid">
      <div v-if="loading" class="user-mgmt-card-grid__loading">加载中…</div>
      <template v-else-if="users.length > 0">
        <div v-for="user in users" :key="user.id" class="user-mgmt-card">
          <div class="user-mgmt-card__header">
            <div class="user-mgmt-card__identity">
              <span class="user-mgmt-card__name">{{ user.name || '未设置名称' }}</span>
              <span class="user-mgmt-card__email">{{ user.email }}</span>
            </div>
            <AppBadge :variant="roleVariant(user.role)">
              {{ roleLabel(user.role) }}
            </AppBadge>
          </div>

          <div class="user-mgmt-card__body">
            <div class="user-mgmt-card__field">
              <span class="user-mgmt-card__label">创建时间</span>
              <span class="user-mgmt-card__value">{{ user.created_at ? formatRelativeTime(user.created_at) : '—' }}</span>
            </div>
            <div class="user-mgmt-card__field">
              <span class="user-mgmt-card__label">身份说明</span>
              <span class="user-mgmt-card__value">{{ isSelf(user as unknown as Record<string, unknown>) ? '当前登录用户' : '普通成员记录' }}</span>
            </div>
          </div>

          <div class="user-mgmt-card__footer">
            <div class="user-mgmt__actions">
              <AppButton
                variant="ghost"
                size="sm"
                :loading="isResetting(user as unknown as Record<string, unknown>)"
                :disabled="isResetting(user as unknown as Record<string, unknown>)"
                title="重置密码"
                @click="handleResetPassword(user as unknown as Record<string, unknown>)"
              >
                <RefreshCw v-if="!isResetting(user as unknown as Record<string, unknown>)" :size="14" />
              </AppButton>
              <AppButton variant="ghost" size="sm" @click="openEditModal(user as unknown as Record<string, unknown>)">
                <Pencil :size="14" />
              </AppButton>
              <AppButton
                variant="ghost"
                size="sm"
                :disabled="isSelf(user as unknown as Record<string, unknown>)"
                @click="handleDelete(user as unknown as Record<string, unknown>)"
              >
                <Trash2 :size="14" class="text-error" />
              </AppButton>
            </div>
          </div>
        </div>
      </template>
      <div v-else class="user-mgmt-card-grid__empty">暂无用户</div>
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

    <!-- Create/Edit Modal -->
    <AppModal v-model:visible="modalVisible" :title="isEditing ? '编辑用户' : '新增用户'" width="480px">
      <form @submit.prevent="handleSubmit">
        <AppFormGroup>
          <AppFormItem v-if="!isEditing" label="邮箱" :required="true" :error="errors.email">
            <AppInput v-model="form.email" placeholder="user@example.com" />
          </AppFormItem>

          <AppFormItem label="名称" :error="errors.name">
            <AppInput v-model="form.name" placeholder="用户名称（可选）" />
          </AppFormItem>

          <AppFormItem label="角色" :required="true" :error="errors.role">
            <AppSelect
              v-model="form.role"
              :options="roleOptions"
              :disabled="isEditing && isSelf(editingUser as unknown as Record<string, unknown>)"
            />
            <p v-if="isEditing && isSelf(editingUser as unknown as Record<string, unknown>)" class="user-mgmt__hint">
              不能修改自己的角色
            </p>
          </AppFormItem>
        </AppFormGroup>
      </form>

      <template #footer>
        <div class="modal-footer">
          <AppButton variant="outline" size="md" @click="modalVisible = false">取消</AppButton>
          <AppButton variant="primary" size="md" :loading="submitting" @click="handleSubmit">
            {{ isEditing ? '保存' : '创建' }}
          </AppButton>
        </div>
      </template>
    </AppModal>

    <AppConfirm />
  </div>
</template>

<style scoped>
.user-mgmt {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.user-mgmt__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 12px;
}

.user-mgmt__header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-mgmt__header-right {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-mgmt__title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.user-mgmt__reg-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
}

.user-mgmt__reg-label {
  font-size: 13px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.user-mgmt__table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.user-mgmt-card-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 16px;
}

.user-mgmt-card-grid__loading,
.user-mgmt-card-grid__empty {
  grid-column: 1 / -1;
  text-align: center;
  padding: 40px 0;
  color: var(--text-muted);
}

.user-mgmt-card {
  display: flex;
  flex-direction: column;
  gap: 14px;
  padding: 16px;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  background: var(--surface-raised);
}

.user-mgmt-card__header,
.user-mgmt-card__footer {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-mgmt-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.user-mgmt-card__footer {
  justify-content: flex-end;
}

.user-mgmt-card__identity,
.user-mgmt-card__body {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-mgmt-card__name {
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.user-mgmt-card__email {
  font-size: 13px;
  color: var(--text-secondary);
  overflow-wrap: anywhere;
}

.user-mgmt-card__body {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 12px;
}

.user-mgmt-card__field {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.user-mgmt-card__label {
  font-size: 12px;
  color: var(--text-muted);
}

.user-mgmt-card__value {
  font-size: 13px;
  color: var(--text-primary);
}

.user-mgmt__actions {
  display: flex;
  gap: 4px;
}

.user-mgmt__hint {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--text-muted);
}

.text-muted {
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

@media (max-width: 767px) {
  .user-mgmt-card-grid {
    grid-template-columns: 1fr;
  }

  .user-mgmt-card__body {
    grid-template-columns: 1fr;
  }
}
</style>
