<script setup lang="ts">
import { ref, computed, type VNode } from 'vue'
import { ChevronUp, ChevronDown } from 'lucide-vue-next'
import AppEmpty from './AppEmpty.vue'
import { Loader2 } from 'lucide-vue-next'

export interface TableColumn {
  key: string
  title: string
  width?: string
  sortable?: boolean
  render?: (value: unknown, row: Record<string, unknown>) => VNode | string
}

const props = withDefaults(
  defineProps<{
    columns: TableColumn[]
    data: Record<string, unknown>[]
    loading?: boolean
  }>(),
  {
    loading: false,
  },
)

const sortKey = ref('')
const sortDir = ref<'asc' | 'desc'>('asc')

function toggleSort(key: string) {
  if (sortKey.value === key) {
    sortDir.value = sortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortDir.value = 'asc'
  }
}

const sortedData = computed(() => {
  if (!sortKey.value) return props.data
  const col = props.columns.find((c) => c.key === sortKey.value)
  if (!col?.sortable) return props.data
  return [...props.data].sort((a, b) => {
    const va = a[sortKey.value]
    const vb = b[sortKey.value]
    if (va == null && vb == null) return 0
    if (va == null) return 1
    if (vb == null) return -1
    const cmp = String(va).localeCompare(String(vb), undefined, { numeric: true })
    return sortDir.value === 'asc' ? cmp : -cmp
  })
})

function getCellValue(row: Record<string, unknown>, key: string): unknown {
  return row[key]
}
</script>

<template>
  <div class="app-table-container">
    <div v-if="loading" class="app-table__loading">
      <Loader2 :size="24" class="animate-spin" />
    </div>
    <table v-else-if="sortedData.length > 0" class="app-table">
      <thead>
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            class="app-table__th"
            :style="col.width ? { width: col.width } : undefined"
            :class="{ 'app-table__th--sortable': col.sortable }"
            @click="col.sortable && toggleSort(col.key)"
          >
            <span class="app-table__th-content">
              {{ col.title }}
              <span v-if="col.sortable" class="app-table__sort-icon">
                <ChevronUp
                  :size="14"
                  :class="{ 'app-table__sort--active': sortKey === col.key && sortDir === 'asc' }"
                />
                <ChevronDown
                  :size="14"
                  :class="{ 'app-table__sort--active': sortKey === col.key && sortDir === 'desc' }"
                />
              </span>
            </span>
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, idx) in sortedData" :key="idx" class="app-table__tr">
          <td v-for="col in columns" :key="col.key" class="app-table__td">
            <slot :name="`cell-${col.key}`" :row="row" :value="getCellValue(row, col.key)">
              {{ getCellValue(row, col.key) ?? '' }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
    <AppEmpty v-else />
  </div>
</template>

<style scoped>
.app-table-container {
  width: 100%;
  overflow-x: auto;
}
.app-table__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 0;
  color: var(--text-muted);
}
.animate-spin {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
.app-table {
  width: 100%;
  border-collapse: collapse;
}
.app-table__th {
  text-align: left;
  padding: 10px 12px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  border-bottom: 1px solid var(--border-default);
  white-space: nowrap;
}
.app-table__th--sortable {
  cursor: pointer;
  user-select: none;
}
.app-table__th--sortable:hover {
  color: var(--text-primary);
}
.app-table__th-content {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.app-table__sort-icon {
  display: inline-flex;
  flex-direction: column;
  gap: -2px;
  color: var(--text-muted);
  opacity: 0.5;
}
.app-table__sort--active {
  color: var(--primary-500);
  opacity: 1 !important;
}
.app-table__tr {
  transition: background var(--transition-fast);
}
.app-table__tr:hover {
  background: var(--surface-sunken);
}
.app-table__td {
  padding: 10px 12px;
  font-size: 14px;
  color: var(--text-primary);
  border-bottom: 1px solid var(--border-subtle);
}
</style>
