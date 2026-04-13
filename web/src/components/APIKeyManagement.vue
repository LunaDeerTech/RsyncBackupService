<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ApiBusinessError } from '../api/client'
import { createAPIKey, deleteAPIKey, listAPIKeys, type APIKeyCreateResponse, type APIKeyItem } from '../api/apikeys'
import { useConfirm } from '../composables/useConfirm'
import { useToastStore } from '../stores/toast'
import AppButton from './AppButton.vue'
import AppEmpty from './AppEmpty.vue'
import AppFormGroup from './AppFormGroup.vue'
import AppFormItem from './AppFormItem.vue'
import AppInput from './AppInput.vue'
import AppModal from './AppModal.vue'

const toast = useToastStore()
const { confirm } = useConfirm()

const form = reactive({
  name: '',
})

const formError = ref('')
const loading = ref(false)
const creating = ref(false)
const deletingId = ref<number | null>(null)
const items = ref<APIKeyItem[]>([])
const latestCreated = ref<APIKeyCreateResponse | null>(null)
const swaggerDocPath = '/api/v2/openapi.json'
const createModalVisible = ref(false)
const createdModalVisible = ref(false)

function validateName() {
  const name = form.name.trim()
  if (!name) {
    formError.value = '请输入 key 名称'
    return false
  }
  if (name.length > 64) {
    formError.value = '名称长度不能超过 64 个字符'
    return false
  }

  formError.value = ''
  return true
}

function openCreateModal() {
  formError.value = ''
  form.name = ''
  createModalVisible.value = true
}

function closeCreateModal() {
  createModalVisible.value = false
}

function closeCreatedModal() {
  createdModalVisible.value = false
}

function formatDateTime(value?: string) {
  if (!value) {
    return '从未使用'
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }

  return date.toLocaleString('zh-CN', { hour12: false })
}

async function loadItems() {
  loading.value = true
  try {
    const response = await listAPIKeys()
    items.value = response.items
  } catch (error) {
    if (error instanceof ApiBusinessError) {
      toast.error(error.message)
    } else {
      toast.error('加载 key 列表失败')
    }
  } finally {
    loading.value = false
  }
}

async function copyText(value: string, successMessage: string) {
  if (typeof navigator !== 'undefined' && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      toast.success(successMessage)
      return
    } catch {
      // Fall through to legacy copy path.
    }
  }

  try {
    const textarea = document.createElement('textarea')
    textarea.value = value
    textarea.setAttribute('readonly', 'true')
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    textarea.style.pointerEvents = 'none'
    document.body.appendChild(textarea)
    textarea.select()
    textarea.setSelectionRange(0, textarea.value.length)
    const copied = document.execCommand('copy')
    document.body.removeChild(textarea)

    if (!copied) {
      throw new Error('copy command failed')
    }

    toast.success(successMessage)
  } catch {
    toast.error('复制失败，请手动复制')
  }
}

function getSwaggerDocURL() {
  if (typeof window === 'undefined') {
    return swaggerDocPath
  }

  return new URL(swaggerDocPath, window.location.origin).toString()
}

async function handleCreate() {
  if (!validateName()) {
    return
  }

  creating.value = true
  try {
    const response = await createAPIKey({ name: form.name.trim() })
    latestCreated.value = response
    createModalVisible.value = false
    createdModalVisible.value = true
    items.value = [response.api_key, ...items.value.filter((item) => item.id !== response.api_key.id)]
    form.name = ''
    formError.value = ''
    toast.success('API key 已创建')
  } catch (error) {
    if (error instanceof ApiBusinessError) {
      toast.error(error.message)
    } else {
      toast.error('创建 API key 失败')
    }
  } finally {
    creating.value = false
  }
}

async function handleDelete(item: APIKeyItem) {
  const confirmed = await confirm({
    title: '删除 Key',
    message: `确认删除 key “${item.name}” 吗？删除后使用该 key 的请求将立即失效。`,
    confirmText: '删除',
    cancelText: '取消',
    danger: true,
  })
  if (!confirmed) {
    return
  }

  deletingId.value = item.id
  try {
    await deleteAPIKey(item.id)
    items.value = items.value.filter((current) => current.id !== item.id)
    if (latestCreated.value?.api_key.id === item.id) {
      latestCreated.value = null
    }
    toast.success('API key 已删除')
  } catch (error) {
    if (error instanceof ApiBusinessError) {
      toast.error(error.message)
    } else {
      toast.error('删除 API key 失败')
    }
  } finally {
    deletingId.value = null
  }
}

onMounted(() => {
  void loadItems()
})
</script>

<template>
  <div class="api-key-management">
    <div class="api-key-management__intro">
      <p class="api-key-management__desc">
        为自动化脚本创建专用访问 key。明文 key 只会在创建成功后展示一次，请立即复制并妥善保存。
      </p>
      <div class="api-key-management__doc-row">
        <p class="api-key-management__doc">
          Swagger 文档 JSON: <span>{{ swaggerDocPath }}</span>
        </p>
        <AppButton size="sm" variant="ghost" @click="copyText(getSwaggerDocURL(), 'Swagger 文档地址已复制')">
          复制链接
        </AppButton>
      </div>
    </div>

    <div class="api-key-management__list">
      <div class="api-key-management__list-header">
        <h4 class="api-key-management__list-title">已有 Key</h4>
        <div class="api-key-management__actions">
          <AppButton size="sm" variant="ghost" :loading="loading" @click="loadItems">
            刷新
          </AppButton>
          <AppButton size="sm" variant="primary" @click="openCreateModal">
            创建 key
          </AppButton>
        </div>
      </div>

      <div v-if="!loading && items.length === 0" class="api-key-management__empty">
        <AppEmpty message="还没有创建任何 API key" />
      </div>

      <div v-else class="api-key-management__items">
        <article v-for="item in items" :key="item.id" class="api-key-management__item">
          <div class="api-key-management__item-main">
            <div>
              <h5 class="api-key-management__item-name">{{ item.name }}</h5>
              <p class="api-key-management__item-prefix">前缀：{{ item.key_prefix }}...</p>
            </div>
            <AppButton
              size="sm"
              variant="danger"
              :loading="deletingId === item.id"
              @click="handleDelete(item)"
            >
              删除
            </AppButton>
          </div>

          <dl class="api-key-management__meta">
            <div>
              <dt>创建时间</dt>
              <dd>{{ formatDateTime(item.created_at) }}</dd>
            </div>
            <div>
              <dt>最近使用</dt>
              <dd>{{ formatDateTime(item.last_used_at) }}</dd>
            </div>
          </dl>
        </article>
      </div>
    </div>

    <AppModal
      :visible="createModalVisible"
      title="创建 Key"
      width="520px"
      :close-on-overlay="false"
      @update:visible="(visible: boolean) => { if (!visible) closeCreateModal() }"
    >
      <AppFormGroup>
        <AppFormItem label="Key 名称" :required="true" :error="formError">
          <AppInput v-model="form.name" placeholder="例如：备份巡检脚本" />
        </AppFormItem>
      </AppFormGroup>
      <p class="api-key-management__modal-hint">
        创建后会立即生成完整 key，并只在随后弹出的提示框中展示一次。
      </p>
      <template #footer>
        <AppButton variant="outline" size="md" @click="closeCreateModal">
          取消
        </AppButton>
        <AppButton variant="primary" size="md" :loading="creating" @click="handleCreate">
          创建 key
        </AppButton>
      </template>
    </AppModal>

    <AppModal
      :visible="createdModalVisible && !!latestCreated"
      title="Key 已生成"
      width="640px"
      :close-on-overlay="false"
      @update:visible="(visible: boolean) => { if (!visible) closeCreatedModal() }"
    >
      <div v-if="latestCreated" class="api-key-management__created">
        <div class="api-key-management__created-header">
          <div>
            <p class="api-key-management__created-title">请立即复制并保存完整 key</p>
            <p class="api-key-management__created-subtitle">关闭当前窗口后将无法再次查看这段明文 key</p>
          </div>
          <AppButton size="sm" variant="outline" @click="copyText(latestCreated.key, '完整 key 已复制')">
            复制完整 key
          </AppButton>
        </div>
        <pre class="api-key-management__secret">{{ latestCreated.key }}</pre>
      </div>
      <template #footer>
        <AppButton variant="outline" size="md" @click="closeCreatedModal">
          关闭
        </AppButton>
        <AppButton
          v-if="latestCreated"
          variant="primary"
          size="md"
          @click="copyText(latestCreated.key, '完整 key 已复制')"
        >
          复制完整 key
        </AppButton>
      </template>
    </AppModal>
  </div>
</template>

<style scoped>
.api-key-management {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.api-key-management__intro {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.api-key-management__doc-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.api-key-management__desc,
.api-key-management__doc,
.api-key-management__created-subtitle,
.api-key-management__item-prefix {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}

.api-key-management__doc span {
  font-family: 'JetBrains Mono', 'Fira Code', monospace;
  font-size: 12px;
  color: var(--text-secondary);
}

.api-key-management__created {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--surface-sunken) 78%, transparent);
}

.api-key-management__created-header,
.api-key-management__item-main,
.api-key-management__list-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.api-key-management__actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}

.api-key-management__created-title,
.api-key-management__list-title,
.api-key-management__item-name {
  margin: 0;
  color: var(--text-primary);
}

.api-key-management__created-title {
  font-size: 14px;
  font-weight: 700;
}

.api-key-management__list-title {
  font-size: 15px;
}

.api-key-management__item-name {
  font-size: 14px;
  font-weight: 600;
}

.api-key-management__secret {
  margin: 0;
  padding: 12px 14px;
  overflow-x: auto;
  border-radius: var(--radius-md);
  background: #0f172a;
  color: #d7f9ff;
  font-size: 12px;
  line-height: 1.6;
}

.api-key-management__list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.api-key-management__modal-hint {
  margin: 4px 0 0;
  font-size: 13px;
  line-height: 1.6;
  color: var(--text-muted);
}

.api-key-management__items {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.api-key-management__item {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px 16px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-subtle);
  background: color-mix(in srgb, var(--surface-sunken) 68%, transparent);
}

.api-key-management__meta {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  margin: 0;
}

.api-key-management__meta div {
  min-width: 0;
}

.api-key-management__meta dt {
  margin: 0 0 4px;
  font-size: 12px;
  color: var(--text-muted);
}

.api-key-management__meta dd {
  margin: 0;
  font-size: 13px;
  color: var(--text-secondary);
  word-break: break-word;
}

.api-key-management__empty {
  border: 1px dashed var(--border-default);
  border-radius: var(--radius-md);
}

@media (max-width: 640px) {
  .api-key-management__doc-row,
  .api-key-management__created-header,
  .api-key-management__item-main,
  .api-key-management__list-header {
    flex-direction: column;
    align-items: stretch;
  }

  .api-key-management__actions {
    display: grid;
    grid-template-columns: 1fr 1fr;
  }

  .api-key-management__meta {
    grid-template-columns: 1fr;
  }
}
</style>