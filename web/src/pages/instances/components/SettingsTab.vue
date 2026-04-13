<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { updateInstance, updateInstancePermissions, listInstancePermissions, deleteInstance } from '../../../api/instances'
import { listRemotes } from '../../../api/remotes'
import { listUsers } from '../../../api/users'
import { useAuthStore } from '../../../stores/auth'
import { useToastStore } from '../../../stores/toast'
import { ApiBusinessError } from '../../../api/client'
import { EXCLUDE_PATTERN_HELP_EXAMPLES, excludePatternsToText, normalizeExcludePatternsInput } from '../../../utils/exclude-patterns'
import type { Instance, UpdateInstanceRequest, PermissionItem } from '../../../types/instance'
import type { RemoteConfig } from '../../../types/remote'
import type { User } from '../../../types/auth'
import AppCard from '../../../components/AppCard.vue'
import AppModal from '../../../components/AppModal.vue'
import AppFormGroup from '../../../components/AppFormGroup.vue'
import AppFormItem from '../../../components/AppFormItem.vue'
import AppInput from '../../../components/AppInput.vue'
import AppSelect from '../../../components/AppSelect.vue'
import AppButton from '../../../components/AppButton.vue'
import AppEmpty from '../../../components/AppEmpty.vue'
import {
  Plus, Trash2, Save,
  AlertTriangle, Clock, CircleHelp, CheckCircle,
} from 'lucide-vue-next'

const props = defineProps<{
  instanceId: number
  instance: Instance
}>()

const emit = defineEmits<{
  'instance-updated': [instance: Instance]
  'instance-deleted': []
}>()

const authStore = useAuthStore()
const toast = useToastStore()

// ── Settings form ──
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

const sourceTypeOptions = [
  { label: '本地', value: 'local' },
  { label: 'SSH', value: 'ssh' },
]

const remoteOptions = computed(() =>
  remotes.value.map((r) => ({ label: r.name, value: r.id })),
)

const excludePatternHelpText = EXCLUDE_PATTERN_HELP_EXAMPLES.join('\n')

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

const permissionOptions = [
  { label: '只读', value: 'readonly' },
  { label: '只读+下载', value: 'readdownload' },
]

// ── Delete instance state ──
const deleteModalVisible = ref(false)
const deleteSubmitting = ref(false)
const deleteForm = reactive({ instance_name: '', password: '' })
const deleteFormErrors = reactive({ instance_name: '', password: '' })

// ── Settings methods ──
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
    const updated = await updateInstance(props.instanceId, data)
    emit('instance-updated', updated)
    settingsForm.exclude_patterns_text = excludePatternsToText(updated.exclude_patterns)
    toast.success('实例信息已更新')
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('保存失败')
  } finally {
    settingsSubmitting.value = false
  }
}

// ── Permission methods ──
async function savePermissions() {
  permissionSaving.value = true
  try {
    const permissions: PermissionItem[] = permEntries.value.map(e => ({ user_id: e.user_id, permission: e.permission }))
    await updateInstancePermissions(props.instanceId, permissions)
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

// ── Delete instance methods ──
function openDeleteModal() {
  deleteForm.instance_name = ''
  deleteForm.password = ''
  deleteFormErrors.instance_name = ''
  deleteFormErrors.password = ''
  deleteModalVisible.value = true
}

function validateDeleteForm(): boolean {
  let valid = true
  deleteFormErrors.instance_name = ''
  deleteFormErrors.password = ''
  if (!deleteForm.instance_name.trim()) {
    deleteFormErrors.instance_name = '请输入实例名称'
    valid = false
  } else if (props.instance && deleteForm.instance_name.trim() !== props.instance.name) {
    deleteFormErrors.instance_name = '实例名称不匹配'
    valid = false
  }
  if (!deleteForm.password.trim()) {
    deleteFormErrors.password = '请输入当前密码'
    valid = false
  }
  return valid
}

async function handleDeleteInstance() {
  if (!validateDeleteForm()) return
  deleteSubmitting.value = true
  try {
    await deleteInstance(props.instanceId, {
      instance_name: deleteForm.instance_name.trim(),
      password: deleteForm.password.trim(),
    })
    deleteModalVisible.value = false
    toast.success('实例已删除')
    emit('instance-deleted')
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      if (e.message.includes('password')) {
        deleteFormErrors.password = '密码错误'
      } else if (e.message.includes('instance_name')) {
        deleteFormErrors.instance_name = '实例名称不匹配'
      } else {
        toast.error(e.message)
      }
    } else {
      toast.error('删除实例失败')
    }
  } finally {
    deleteSubmitting.value = false
  }
}

// ── Fetch ──
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
    const res = await listInstancePermissions(props.instanceId)
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

function refresh() {
  settingsForm.name = props.instance.name
  settingsForm.source_type = props.instance.source_type
  settingsForm.source_path = props.instance.source_path
  settingsForm.exclude_patterns_text = excludePatternsToText(props.instance.exclude_patterns)
  settingsForm.remote_config_id = props.instance.remote_config_id
}

onMounted(() => {
  if (authStore.isAdmin) {
    fetchRemotes()
    fetchUsers().then(() => fetchPermissions())
  }
})

defineExpose({ refresh })
</script>

<template>
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

    <AppCard title="危险操作">
      <div class="danger-zone">
        <div class="danger-zone__panel">
          <div class="danger-zone__info">
            <div class="danger-zone__heading">
              <AlertTriangle :size="16" class="danger-zone__icon" />
              <span class="danger-zone__label">删除实例</span>
            </div>
            <span class="danger-zone__desc">删除后所有关联的策略、备份记录和审计日志将被永久移除，此操作不可撤销。</span>
          </div>
          <AppButton variant="danger" size="md" @click="openDeleteModal" :disabled="instance?.status !== 'idle'">
            <Trash2 :size="16" style="margin-right: 4px" />
            删除实例
          </AppButton>
        </div>
        <p v-if="instance?.status !== 'idle'" class="danger-zone__hint">
          <Clock :size="14" /> 仅空闲状态的实例可以删除
        </p>
      </div>
    </AppCard>
  </div>

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

  <!-- Delete Instance Modal -->
  <AppModal v-model:visible="deleteModalVisible" title="删除实例" width="480px">
    <div class="delete-modal__warning">
      <AlertTriangle :size="20" />
      <span>此操作将永久删除实例 <strong>{{ instance?.name }}</strong> 及其所有关联数据，不可撤销。</span>
    </div>
    <AppFormGroup>
      <AppFormItem label="实例名称确认" :required="true" :error="deleteFormErrors.instance_name">
        <AppInput v-model="deleteForm.instance_name" :placeholder="`请输入 ${instance?.name ?? ''} 以确认`" />
      </AppFormItem>
      <AppFormItem label="当前密码" :required="true" :error="deleteFormErrors.password">
        <AppInput v-model="deleteForm.password" type="password" placeholder="请输入你的登录密码" />
      </AppFormItem>
    </AppFormGroup>
    <template #footer>
      <AppButton variant="outline" size="md" @click="deleteModalVisible = false">取消</AppButton>
      <AppButton variant="danger" size="md" :loading="deleteSubmitting" @click="handleDeleteInstance">
        确认删除
      </AppButton>
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

.settings-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
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

/* Permissions */
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

/* Danger Zone */
.danger-zone {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.danger-zone__panel {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px 20px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-subtle);
}

.danger-zone__info {
  display: flex;
  flex-direction: column;
  gap: 6px;
  min-width: 0;
}

.danger-zone__heading {
  display: flex;
  align-items: center;
  gap: 8px;
}

.danger-zone__icon {
  color: var(--color-error);
  flex-shrink: 0;
}

.danger-zone__label {
  font-weight: 600;
  font-size: var(--font-size-md);
  color: var(--text-primary);
}

.danger-zone__desc {
  font-size: var(--font-size-sm);
  color: var(--text-secondary);
  line-height: 1.5;
}

.danger-zone__hint {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: var(--font-size-sm);
  color: var(--text-muted);
}

@media (max-width: 768px) {
  .danger-zone__panel {
    flex-direction: column;
    align-items: stretch;
  }
}

/* Delete Modal */
.delete-modal__warning {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  padding: 12px 16px;
  margin-bottom: 16px;
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--color-error) 8%, transparent);
  color: var(--color-error);
  font-size: var(--font-size-sm);
  line-height: 1.5;
}

.delete-modal__warning svg {
  flex-shrink: 0;
  margin-top: 2px;
}
</style>
