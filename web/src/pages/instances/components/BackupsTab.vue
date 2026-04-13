<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import { listBackups, restoreBackup, downloadBackup } from '../../../api/backups'
import type { RestoreRequest, BackupDownloadPart } from '../../../api/backups'
import { listPolicies } from '../../../api/policies'
import { listRemotes } from '../../../api/remotes'
import { useAuthStore } from '../../../stores/auth'
import { useToastStore } from '../../../stores/toast'
import { ApiBusinessError } from '../../../api/client'
import { formatBytes } from '../../../utils/format'
import { formatRelativeTime } from '../../../utils/time'
import {
  taskStatusMap, backupTypeMap,
  getStatusConfig,
} from '../../../utils/status-config'
import type { Instance, Backup } from '../../../types/instance'
import type { Policy } from '../../../types/policy'
import type { RemoteConfig } from '../../../types/remote'
import type { TableColumn } from '../../../components/AppTable.vue'
import AppTable from '../../../components/AppTable.vue'
import AppModal from '../../../components/AppModal.vue'
import AppFormGroup from '../../../components/AppFormGroup.vue'
import AppFormItem from '../../../components/AppFormItem.vue'
import AppInput from '../../../components/AppInput.vue'
import AppSelect from '../../../components/AppSelect.vue'
import AppButton from '../../../components/AppButton.vue'
import StatusBadge from '../../../components/StatusBadge.vue'
import { Download, RotateCcw } from 'lucide-vue-next'

const props = defineProps<{
  instanceId: number
  instance: Instance
  canDownload: boolean
}>()

const emit = defineEmits<{
  'change-tab': [tab: string]
}>()

const authStore = useAuthStore()
const toast = useToastStore()

// ── Backup data ──
const backups = ref<Backup[]>([])
const backupLoading = ref(false)
const backupPage = ref(1)
const backupTotal = ref(0)
const backupPageSize = 20
const backupDetailTarget = ref<Backup | null>(null)
const backupDetailVisible = ref(false)

// ── Policies (for encryption check) ──
const policies = ref<Policy[]>([])

// ── Remotes (for restore modal) ──
const remotes = ref<RemoteConfig[]>([])

const sshRemoteOptions = computed(() =>
  remotes.value
    .filter((r) => r.type === 'ssh')
    .map((r) => ({ label: `${r.name} (${r.host})`, value: r.id })),
)

// ── Restore modal ──
const restoreModalVisible = ref(false)
const restoreSubmitting = ref(false)
const restoreBackupTarget = ref<Record<string, unknown> | null>(null)
const restoreError = ref('')
const restoreForm = reactive({
  restore_type: 'source' as 'source' | 'custom',
  target_path: '',
  target_location: 'local' as 'local' | 'remote',
  remote_config_id: undefined as number | undefined,
  instance_name: '',
  password: '',
  encryption_key: '',
})
const restoreFormErrors = reactive({
  target_path: '',
  remote_config_id: '',
  instance_name: '',
  password: '',
  encryption_key: '',
})

// ── Download state ──
const downloadingBackupId = ref<number | null>(null)
const splitDownloadModalVisible = ref(false)
const splitDownloadParts = ref<BackupDownloadPart[]>([])
const splitDownloadTitle = ref('')
const downloadingSplitAll = ref(false)
const downloadingSplitPartUrl = ref<string | null>(null)
const copyingSplitLinks = ref(false)

// ── Columns ──
const backupColumns: TableColumn[] = [
  { key: 'started_at', title: '备份时间' },
  { key: 'completed_at', title: '完成时间' },
  { key: 'type', title: '类型' },
  { key: 'backup_size_bytes', title: '备份大小' },
  { key: 'actual_size_bytes', title: '数据原始大小' },
  { key: 'duration_seconds', title: '持续时间' },
  { key: 'actions', title: '操作', width: '200px' },
]

// ── Helpers ──
function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.floor(seconds / 60)} 分 ${seconds % 60} 秒`
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  return `${h} 小时 ${m} 分`
}

function isEncryptedCold(row: Record<string, unknown>): boolean {
  if (row.type !== 'cold') return false
  const policy = policies.value.find((p) => p.id === (row.policy_id as number))
  return !!policy?.encryption
}

const restoreSubmitDisabled = computed(() => {
  if (!props.instance) return true
  return restoreForm.instance_name !== props.instance.name
})

// ── Backup detail modal ──
function openBackupDetail(row: Record<string, unknown>) {
  backupDetailTarget.value = row as unknown as Backup
  backupDetailVisible.value = true
}

// ── Restore ──
function openRestoreModal(row: Record<string, unknown>) {
  restoreBackupTarget.value = row
  restoreForm.restore_type = 'source'
  restoreForm.target_path = ''
  restoreForm.target_location = 'local'
  restoreForm.remote_config_id = undefined
  restoreForm.instance_name = ''
  restoreForm.password = ''
  restoreForm.encryption_key = ''
  restoreError.value = ''
  Object.keys(restoreFormErrors).forEach((k) => (restoreFormErrors as Record<string, string>)[k] = '')
  restoreModalVisible.value = true
  if (authStore.isAdmin && remotes.value.length === 0) {
    fetchRemotes()
  }
}

function validateRestoreForm(): boolean {
  let valid = true
  Object.keys(restoreFormErrors).forEach((k) => (restoreFormErrors as Record<string, string>)[k] = '')

  if (restoreForm.restore_type === 'custom' && !restoreForm.target_path.trim()) {
    restoreFormErrors.target_path = '目标路径不能为空'
    valid = false
  }
  if (restoreForm.restore_type === 'custom' && restoreForm.target_location === 'remote' && !restoreForm.remote_config_id) {
    restoreFormErrors.remote_config_id = '请选择远程配置'
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
      if (restoreForm.target_location === 'remote' && restoreForm.remote_config_id) {
        data.remote_config_id = restoreForm.remote_config_id
      }
    }
    if (isEncryptedCold(restoreBackupTarget.value)) {
      data.encryption_key = restoreForm.encryption_key
    }
    await restoreBackup(props.instanceId, restoreBackupTarget.value.id as number, data)
    toast.success('恢复任务已创建')
    restoreModalVisible.value = false
    emit('change-tab', 'overview')
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
    const res = await downloadBackup(props.instanceId, row.id as number)
    if (res.mode === 'split') {
      splitDownloadTitle.value = res.file_name || `备份 #${row.id as number}`
      splitDownloadParts.value = res.parts ?? []
      splitDownloadModalVisible.value = true
      if (splitDownloadParts.value.length === 0) {
        toast.warning('未检测到可下载的分卷文件')
      }
      return
    }
    triggerBrowserDownload(res.url)
  } catch (e) {
    if (e instanceof ApiBusinessError) toast.error(e.message)
    else toast.error('获取下载链接失败')
  } finally {
    downloadingBackupId.value = null
  }
}

function triggerBrowserDownload(url: string) {
  const a = document.createElement('a')
  a.href = url
  a.style.display = 'none'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
}

function closeSplitDownloadModal() {
  if (downloadingSplitAll.value) return
  splitDownloadModalVisible.value = false
  splitDownloadTitle.value = ''
  splitDownloadParts.value = []
  downloadingSplitPartUrl.value = null
  copyingSplitLinks.value = false
}

async function handleDownloadSplitPart(part: BackupDownloadPart) {
  downloadingSplitPartUrl.value = part.url
  try {
    triggerBrowserDownload(part.url)
  } finally {
    window.setTimeout(() => {
      if (downloadingSplitPartUrl.value === part.url) {
        downloadingSplitPartUrl.value = null
      }
    }, 300)
  }
}

async function handleDownloadAllSplitParts() {
  if (splitDownloadParts.value.length === 0) return
  downloadingSplitAll.value = true
  try {
    for (const part of splitDownloadParts.value) {
      downloadingSplitPartUrl.value = part.url
      triggerBrowserDownload(part.url)
      await new Promise((resolve) => window.setTimeout(resolve, 350))
    }
    toast.info('已按顺序触发全部分卷下载，请留意浏览器的多文件下载权限提示')
  } finally {
    downloadingSplitAll.value = false
    downloadingSplitPartUrl.value = null
  }
}

function resolveDownloadURL(url: string) {
  if (/^https?:\/\//.test(url)) {
    return url
  }
  return new URL(url, window.location.origin).toString()
}

async function copyTextToClipboard(text: string) {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', 'true')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.select()
  const succeeded = document.execCommand('copy')
  document.body.removeChild(textarea)

  if (!succeeded) {
    throw new Error('copy failed')
  }
}

async function handleCopyAllSplitLinks() {
  if (splitDownloadParts.value.length === 0) return
  copyingSplitLinks.value = true
  try {
    const payload = splitDownloadParts.value
      .map((part) => `${part.name}\n${resolveDownloadURL(part.url)}`)
      .join('\n\n')
    await copyTextToClipboard(payload)
    toast.success('全部分卷下载链接已复制')
  } catch {
    toast.error('复制下载链接失败')
  } finally {
    copyingSplitLinks.value = false
  }
}

// ── Fetch ──
async function fetchBackups() {
  backupLoading.value = true
  try {
    const res = await listBackups(props.instanceId, { page: backupPage.value, page_size: backupPageSize })
    backups.value = res.items ?? []
    backupTotal.value = res.total ?? 0
  } catch {
    toast.error('加载备份列表失败')
  } finally {
    backupLoading.value = false
  }
}

async function fetchPolicies() {
  try {
    const res = await listPolicies(props.instanceId)
    policies.value = Array.isArray(res) ? res : (res.items ?? [])
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

function refresh() {
  fetchBackups()
  fetchPolicies()
}

defineExpose({ refresh })
</script>

<template>
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

  <!-- Split Download Modal -->
  <AppModal v-model:visible="splitDownloadModalVisible" title="分卷下载" width="640px" :close-on-overlay="!downloadingSplitAll">
    <div class="split-download-modal">
      <div class="split-download-modal__hero">
        <div>
          <p class="split-download-modal__eyebrow">检测到分卷冷备份</p>
          <h4 class="split-download-modal__title">{{ splitDownloadTitle }}</h4>
        </div>
        <div class="split-download-modal__summary">
          <span class="split-download-modal__summary-value">{{ splitDownloadParts.length }}</span>
          <span class="split-download-modal__summary-label">个分卷</span>
        </div>
      </div>
      <p class="split-download-modal__hint">你可以逐个下载，也可以使用一键下载按顺序拉取全部分卷。</p>
      <div class="split-download-list">
        <div v-for="part in splitDownloadParts" :key="part.url" class="split-download-item">
          <div class="split-download-item__meta">
            <span class="split-download-item__index">第 {{ part.index }} 卷</span>
            <span class="split-download-item__name">{{ part.name }}</span>
          </div>
          <div class="split-download-item__actions">
            <span class="split-download-item__size">{{ formatBytes(part.size_bytes) }}</span>
            <AppButton
              variant="outline"
              size="sm"
              :loading="downloadingSplitPartUrl === part.url"
              :disabled="downloadingSplitAll"
              @click="handleDownloadSplitPart(part)"
            >
              下载此卷
            </AppButton>
          </div>
        </div>
      </div>
    </div>
    <template #footer>
      <div class="modal-footer split-download-modal__footer">
        <AppButton variant="outline" size="md" :disabled="downloadingSplitAll" @click="closeSplitDownloadModal">关闭</AppButton>
        <AppButton variant="outline" size="md" :loading="copyingSplitLinks" :disabled="downloadingSplitAll || splitDownloadParts.length === 0" @click="handleCopyAllSplitLinks">
          复制全部下载链接
        </AppButton>
        <AppButton variant="primary" size="md" :loading="downloadingSplitAll" :disabled="splitDownloadParts.length === 0" @click="handleDownloadAllSplitParts">
          一键下载全部
        </AppButton>
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

        <!-- Source restore: show original path -->
        <div v-if="restoreForm.restore_type === 'source'" class="restore-source-info">
          <div class="restore-source-info__path">
            <span class="restore-source-info__label">原始位置</span>
            <span class="restore-source-info__value">
              <template v-if="instance?.source_type === 'ssh'">[SSH] </template>
              {{ instance?.source_path }}
            </span>
          </div>
          <div class="restore-warning">
            ⚠ 将覆盖源路径的现有数据
          </div>
        </div>

        <!-- Custom restore: target location selector -->
        <template v-if="restoreForm.restore_type === 'custom'">
          <AppFormItem label="目标位置">
            <div class="restore-type-group">
              <label class="restore-type-option">
                <input type="radio" v-model="restoreForm.target_location" value="local" />
                <span>本机路径</span>
              </label>
              <label class="restore-type-option">
                <input type="radio" v-model="restoreForm.target_location" value="remote" />
                <span>远程主机 (SSH)</span>
              </label>
            </div>
          </AppFormItem>

          <AppFormItem v-if="restoreForm.target_location === 'remote'" label="远程配置" :required="true"
            :error="restoreFormErrors.remote_config_id">
            <AppSelect v-model="restoreForm.remote_config_id"
              :options="sshRemoteOptions"
              placeholder="请选择 SSH 远程配置" />
          </AppFormItem>

          <AppFormItem label="目标路径" :required="true" :error="restoreFormErrors.target_path">
            <AppInput v-model="restoreForm.target_path"
              :placeholder="restoreForm.target_location === 'remote' ? '远程主机上的路径，如 /data/restore/' : '如 /data/restore/'" />
          </AppFormItem>
        </template>

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
</template>

<style scoped>
.tab-content {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding-top: 16px;
}

.tab-table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.actions-cell {
  display: flex;
  gap: 4px;
}

.text-muted {
  color: var(--text-muted);
  font-size: 13px;
}

.modal-footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.backup-pagination {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 12px;
  padding: 12px 0;
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

/* Split download modal */
.split-download-modal {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.split-download-modal__hero {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-lg);
  background: linear-gradient(135deg, color-mix(in srgb, var(--brand-primary) 8%, var(--surface-raised)) 0%, var(--surface-raised) 100%);
}

.split-download-modal__eyebrow {
  margin: 0 0 6px;
  font-size: 12px;
  color: var(--text-muted);
}

.split-download-modal__title {
  margin: 0;
  font-size: 16px;
  color: var(--text-primary);
  word-break: break-all;
}

.split-download-modal__summary {
  display: flex;
  flex-direction: column;
  align-items: flex-end;
  min-width: 88px;
}

.split-download-modal__summary-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1;
  color: var(--text-primary);
}

.split-download-modal__summary-label {
  font-size: 12px;
  color: var(--text-secondary);
}

.split-download-modal__hint {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary);
}

.split-download-modal__footer {
  width: 100%;
}

.split-download-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.split-download-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-raised);
}

.split-download-item__meta {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.split-download-item__index {
  font-size: 12px;
  color: var(--text-muted);
}

.split-download-item__name {
  font-size: 14px;
  color: var(--text-primary);
  word-break: break-all;
}

.split-download-item__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;
}

.split-download-item__size {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
}

@media (max-width: 720px) {
  .split-download-modal__hero,
  .split-download-item {
    flex-direction: column;
    align-items: stretch;
  }

  .split-download-modal__summary {
    align-items: flex-start;
  }

  .split-download-item__actions {
    justify-content: space-between;
  }
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

.restore-source-info {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.restore-source-info__path {
  display: flex;
  flex-direction: column;
  gap: 4px;
  padding: 10px 12px;
  background: var(--surface-sunken);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
}

.restore-source-info__label {
  font-size: 12px;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.restore-source-info__value {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  word-break: break-all;
  font-family: monospace;
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
</style>
