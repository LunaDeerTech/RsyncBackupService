<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { listUsers, createUser, updateUser, deleteUser } from '../../api/users'
import { getRegistrationStatus, updateRegistrationStatus } from '../../api/system'
import { useAuthStore } from '../../stores/auth'
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
import AppBadge from '../../components/AppBadge.vue'
import AppSwitch from '../../components/AppSwitch.vue'
import AppConfirm from '../../components/AppConfirm.vue'
import { Plus, Pencil, Trash2 } from 'lucide-vue-next'

const authStore = useAuthStore()
const toast = useToastStore()
const { confirm } = useConfirm()

// ── List state ──
const users = ref<User[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

// ── Registration toggle ──
const registrationEnabled = ref(false)
const registrationLoading = ref(false)

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
  { key: 'actions', title: '操作', width: '120px' },
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
      toast.success('用户已创建，密码已发送至邮箱（SMTP 未配置时密码已输出到服务器日志）')
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

function roleVariant(role: string): 'default' | 'info' {
  return role === 'admin' ? 'info' : 'default'
}

function roleLabel(role: string): string {
  return role === 'admin' ? '管理员' : '普通用户'
}

function isSelf(row: Record<string, unknown>): boolean {
  return (row.id as number) === authStore.user?.id
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
        <AppButton variant="primary" size="sm" @click="openCreateModal">
          <Plus :size="16" style="margin-right: 4px" />
          新增用户
        </AppButton>
      </div>
    </div>

    <!-- Table -->
    <div class="user-mgmt__table">
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
</style>
