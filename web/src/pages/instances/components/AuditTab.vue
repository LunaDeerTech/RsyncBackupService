<script setup lang="ts">
import { ref } from 'vue'
import { listInstanceAuditLogs } from '../../../api/audit'
import type { AuditLog, AuditLogParams } from '../../../api/audit'
import { useToastStore } from '../../../stores/toast'
import { getActionLabel, actionOptions, formatAuditDetail, getActionBadgeVariant } from '../../../utils/audit'
import type { TableColumn } from '../../../components/AppTable.vue'
import AppTable from '../../../components/AppTable.vue'
import AppSelect from '../../../components/AppSelect.vue'
import AppBadge from '../../../components/AppBadge.vue'
import AppPagination from '../../../components/AppPagination.vue'

const props = defineProps<{
  instanceId: number
}>()

const toast = useToastStore()

const auditLogs = ref<AuditLog[]>([])
const auditLoading = ref(false)
const auditPage = ref(1)
const auditPageSize = ref(20)
const auditTotal = ref(0)
const auditStartDate = ref('')
const auditEndDate = ref('')
const auditAction = ref<string | number>('')

const auditColumns: TableColumn[] = [
  { key: 'created_at', title: '时间' },
  { key: 'action', title: '操作类型' },
  { key: 'user', title: '操作人' },
  { key: 'detail', title: '详情' },
]

async function fetchAuditLogs() {
  auditLoading.value = true
  try {
    const params: AuditLogParams = {
      page: auditPage.value,
      page_size: auditPageSize.value,
    }
    if (auditStartDate.value) params.start_date = auditStartDate.value
    if (auditEndDate.value) params.end_date = auditEndDate.value
    if (auditAction.value) params.action = String(auditAction.value)
    const res = await listInstanceAuditLogs(props.instanceId, params)
    auditLogs.value = res.items ?? []
    auditTotal.value = res.total
  } catch {
    toast.error('加载审计日志失败')
  } finally {
    auditLoading.value = false
  }
}

function refresh() {
  fetchAuditLogs()
}

defineExpose({ refresh })
</script>

<template>
  <div class="tab-content">
    <!-- Filters -->
    <div class="audit-filters">
      <div class="audit-filter-item">
        <label class="audit-filter-label">开始日期</label>
        <input type="date" class="audit-date-input" v-model="auditStartDate"
          @change="auditPage = 1; fetchAuditLogs()" />
      </div>
      <div class="audit-filter-item">
        <label class="audit-filter-label">结束日期</label>
        <input type="date" class="audit-date-input" v-model="auditEndDate"
          @change="auditPage = 1; fetchAuditLogs()" />
      </div>
      <div class="audit-filter-item">
        <label class="audit-filter-label">操作类型</label>
        <AppSelect :model-value="auditAction" :options="actionOptions" placeholder="全部"
          @update:model-value="(v: string | number) => { auditAction = v; auditPage = 1; fetchAuditLogs() }" />
      </div>
    </div>

    <!-- Table -->
    <div class="tab-table">
      <AppTable :columns="auditColumns" :data="(auditLogs as unknown as Record<string, unknown>[])"
        :loading="auditLoading">
        <template #cell-created_at="{ row }">
          {{ new Date(row.created_at as string).toLocaleString('zh-CN') }}
        </template>
        <template #cell-action="{ row }">
          <AppBadge :variant="getActionBadgeVariant(row.action as string)">{{ getActionLabel(row.action as string) }}</AppBadge>
        </template>
        <template #cell-user="{ row }">
          <span>{{ row.user_name || '-' }}</span>
          <span v-if="row.user_email" class="audit-email">{{ row.user_email }}</span>
        </template>
        <template #cell-detail="{ row }">
          <span class="audit-detail">{{ formatAuditDetail(row.action as string, row.detail as Record<string,
              any>) }}</span>
        </template>
      </AppTable>
    </div>

    <!-- Pagination -->
    <AppPagination v-if="auditTotal > 0" :page="auditPage" :page-size="auditPageSize" :total="auditTotal"
      @update:page="(p: number) => { auditPage = p; fetchAuditLogs() }"
      @update:page-size="(s: number) => { auditPageSize = s; auditPage = 1; fetchAuditLogs() }" />
  </div>
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

/* Audit tab */
.audit-filters {
  display: flex;
  align-items: flex-end;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 16px;
}

.audit-filter-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 160px;
}

.audit-filter-label {
  font-size: 13px;
  color: var(--text-secondary);
  font-weight: 500;
}

.audit-date-input {
  padding: 8px 12px;
  font-size: 14px;
  line-height: 20px;
  color: var(--text-primary);
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  outline: none;
  transition: border-color 0.15s;
  color-scheme: light;
}

:root[data-theme="dark"] .audit-date-input {
  color-scheme: dark;
}

.audit-date-input:focus {
  border-color: var(--primary-500);
}

.audit-email {
  display: block;
  font-size: 12px;
  color: var(--text-muted);
}

.audit-detail {
  font-size: 13px;
  color: var(--text-secondary);
}
</style>
