<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { listRiskEvents, type RiskEvent } from '../../api/risks'
import { useToastStore } from '../../stores/toast'
import { formatRelativeTime } from '../../utils/time'
import type { TableColumn } from '../../components/AppTable.vue'
import AppTable from '../../components/AppTable.vue'
import AppPagination from '../../components/AppPagination.vue'
import AppSelect from '../../components/AppSelect.vue'
import AppBadge from '../../components/AppBadge.vue'

const router = useRouter()
const toast = useToastStore()

// ── Filters ──
const severityFilter = ref('')
const sourceFilter = ref('')
const resolvedFilter = ref('false')

// ── List state ──
const risks = ref<RiskEvent[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)

const severityOptions = [
  { label: '全部等级', value: '' },
  { label: 'Info', value: 'info' },
  { label: 'Warning', value: 'warning' },
  { label: 'Critical', value: 'critical' },
]

const resolvedOptions = [
  { label: '全部状态', value: '' },
  { label: '未解决', value: 'false' },
  { label: '已解决', value: 'true' },
]

const riskSourceLabels: Record<string, string> = {
  'backup_failed': '备份失败',
  'backup_failure': '备份失败',
  'backup_overdue': '备份超期',
  'cold_backup_missing': '缺少冷备份',
  'target_unreachable': '目标不可达',
  'target_capacity_low': '目标容量不足',
  'restore_failed': '恢复失败',
  'credential_error': '凭证错误',
  'low_dr_score': '容灾率低',
  'no_recent_backup': '长期未备份',
}

const sourceOptions = [
  { label: '全部来源', value: '' },
  ...Object.entries(riskSourceLabels).map(([value, label]) => ({ label, value })),
]

function sourceLabel(source: string): string {
  return riskSourceLabels[source] ?? source
}

function severityVariant(severity: string): 'info' | 'warning' | 'error' | 'default' {
  switch (severity) {
    case 'critical': return 'error'
    case 'warning': return 'warning'
    case 'info': return 'info'
    default: return 'default'
  }
}

function severityLabel(severity: string): string {
  switch (severity) {
    case 'critical': return 'Critical'
    case 'warning': return 'Warning'
    case 'info': return 'Info'
    default: return severity
  }
}

const columns: TableColumn[] = [
  { key: 'severity', title: '严重等级', width: '100px' },
  { key: 'source', title: '来源类型', width: '120px' },
  { key: 'related', title: '关联对象' },
  { key: 'message', title: '描述' },
  { key: 'status', title: '状态', width: '140px' },
  { key: 'created_at', title: '创建时间', width: '140px' },
]

// ── Fetch ──
async function fetchList() {
  loading.value = true
  try {
    const params: Record<string, unknown> = {
      page: page.value,
      page_size: pageSize.value,
    }
    if (severityFilter.value) params.severity = severityFilter.value
    if (sourceFilter.value) params.source = sourceFilter.value
    if (resolvedFilter.value !== '') params.resolved = resolvedFilter.value === 'true'

    const res = await listRiskEvents(params as any)
    risks.value = res.items ?? []
    total.value = res.total
  } catch {
    toast.error('加载风险事件失败')
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

function onFilterChange() {
  page.value = 1
  fetchList()
}

function goToInstance(id: number) {
  router.push(`/instances/${id}`)
}
</script>

<template>
  <div class="risk-events-page">
    <!-- Header -->
    <div class="risk-events-page__header">
      <h2 class="risk-events-page__title">风险事件</h2>
    </div>

    <!-- Filters -->
    <div class="risk-events-page__filters">
      <div class="risk-events-page__filter">
        <AppSelect
          v-model="severityFilter"
          :options="severityOptions"
          placeholder="严重等级"
          @update:model-value="onFilterChange"
        />
      </div>
      <div class="risk-events-page__filter">
        <AppSelect
          v-model="sourceFilter"
          :options="sourceOptions"
          placeholder="来源类型"
          @update:model-value="onFilterChange"
        />
      </div>
      <div class="risk-events-page__filter">
        <AppSelect
          v-model="resolvedFilter"
          :options="resolvedOptions"
          placeholder="状态"
          @update:model-value="onFilterChange"
        />
      </div>
    </div>

    <!-- Table -->
    <div class="risk-events-page__table">
      <AppTable :columns="columns" :data="risks as unknown as Record<string, unknown>[]" :loading="loading">
        <template #cell-severity="{ row }">
          <AppBadge :variant="severityVariant(row.severity as string)">
            {{ severityLabel(row.severity as string) }}
          </AppBadge>
        </template>

        <template #cell-source="{ row }">
          {{ sourceLabel(row.source as string) }}
        </template>

        <template #cell-related="{ row }">
          <span v-if="row.instance_id" class="risk-events-page__link" @click="goToInstance(row.instance_id as number)">
            {{ row.instance_name }}
          </span>
          <span v-if="row.instance_id && row.target_name"> / </span>
          <span v-if="row.target_name" class="text-secondary">{{ row.target_name }}</span>
          <span v-if="!row.instance_id && !row.target_name" class="text-muted">—</span>
        </template>

        <template #cell-message="{ row }">
          <span class="risk-events-page__message">{{ row.message }}</span>
        </template>

        <template #cell-status="{ row }">
          <AppBadge v-if="row.resolved" variant="success">已解决</AppBadge>
          <AppBadge v-else variant="warning">未解决</AppBadge>
          <div v-if="row.resolved && row.resolved_at" class="risk-events-page__resolved-at">
            {{ formatRelativeTime(row.resolved_at as string) }}
          </div>
        </template>

        <template #cell-created_at="{ row }">
          <span v-if="row.created_at">{{ formatRelativeTime(row.created_at as string) }}</span>
          <span v-else class="text-muted">—</span>
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
  </div>
</template>

<style scoped>
.risk-events-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.risk-events-page__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.risk-events-page__title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
  margin: 0;
}

.risk-events-page__filters {
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.risk-events-page__filter {
  width: 180px;
}

.risk-events-page__table {
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
  overflow: hidden;
}

.risk-events-page__link {
  color: var(--primary-600);
  cursor: pointer;
  font-weight: 500;
}

.risk-events-page__link:hover {
  text-decoration: underline;
}

.risk-events-page__message {
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  font-size: 13px;
}

.risk-events-page__resolved-at {
  font-size: 11px;
  color: var(--text-muted);
  margin-top: 2px;
}

.text-secondary {
  color: var(--text-secondary);
}

.text-muted {
  color: var(--text-muted);
}
</style>
