<script setup lang="ts">
import { computed } from 'vue'
import { ChevronLeft, ChevronRight } from 'lucide-vue-next'
import AppSelect from './AppSelect.vue'

const props = defineProps<{
  page: number
  pageSize: number
  total: number
}>()

const emit = defineEmits<{
  'update:page': [value: number]
  'update:pageSize': [value: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const visiblePages = computed(() => {
  const total = totalPages.value
  const current = props.page
  const pages: (number | '...')[] = []

  if (total <= 7) {
    for (let i = 1; i <= total; i++) pages.push(i)
  } else {
    pages.push(1)
    if (current > 3) pages.push('...')
    const start = Math.max(2, current - 1)
    const end = Math.min(total - 1, current + 1)
    for (let i = start; i <= end; i++) pages.push(i)
    if (current < total - 2) pages.push('...')
    pages.push(total)
  }
  return pages
})

const pageSizeOptions = [
  { label: '10 条/页', value: 10 },
  { label: '20 条/页', value: 20 },
  { label: '50 条/页', value: 50 },
]

function goTo(p: number) {
  if (p >= 1 && p <= totalPages.value && p !== props.page) {
    emit('update:page', p)
  }
}

function onPageSizeChange(v: string | number) {
  emit('update:pageSize', Number(v))
  emit('update:page', 1)
}
</script>

<template>
  <div class="app-pagination">
    <span class="app-pagination__total">共 {{ total }} 条</span>
    <div class="app-pagination__pages">
      <button
        type="button"
        class="app-pagination__btn"
        :disabled="page <= 1"
        @click="goTo(page - 1)"
      >
        <ChevronLeft :size="16" />
      </button>
      <template v-for="(p, idx) in visiblePages" :key="idx">
        <span v-if="p === '...'" class="app-pagination__ellipsis">…</span>
        <button
          v-else
          type="button"
          class="app-pagination__btn"
          :class="{ 'app-pagination__btn--active': p === page }"
          @click="goTo(p)"
        >
          {{ p }}
        </button>
      </template>
      <button
        type="button"
        class="app-pagination__btn"
        :disabled="page >= totalPages"
        @click="goTo(page + 1)"
      >
        <ChevronRight :size="16" />
      </button>
    </div>
    <div class="app-pagination__size">
      <AppSelect
        :model-value="pageSize"
        :options="pageSizeOptions"
        @update:model-value="onPageSizeChange"
      />
    </div>
  </div>
</template>

<style scoped>
.app-pagination {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-wrap: wrap;
}
.app-pagination__total {
  font-size: 13px;
  color: var(--text-muted);
  white-space: nowrap;
}
.app-pagination__pages {
  display: flex;
  align-items: center;
  gap: 4px;
}
.app-pagination__btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 32px;
  padding: 0 6px;
  font-size: 13px;
  font-weight: 500;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-sm);
  background: var(--surface-raised);
  color: var(--text-primary);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.app-pagination__btn:hover:not(:disabled) {
  border-color: var(--primary-500);
  color: var(--primary-500);
}
.app-pagination__btn--active {
  background: var(--primary-500);
  border-color: var(--primary-500);
  color: #0a1628;
}
.app-pagination__btn--active:hover {
  background: var(--primary-600) !important;
  border-color: var(--primary-600) !important;
  color: #0a1628 !important;
}
.app-pagination__btn:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
.app-pagination__ellipsis {
  padding: 0 4px;
  color: var(--text-muted);
  font-size: 13px;
}
.app-pagination__size {
  width: 120px;
}
.app-pagination__size :deep(.app-select__trigger) {
  padding: 4px 10px;
  font-size: 13px;
}
</style>
