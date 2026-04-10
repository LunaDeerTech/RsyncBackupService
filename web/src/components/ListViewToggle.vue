<script setup lang="ts">
import { LayoutGrid, List } from 'lucide-vue-next'
import type { ListViewMode } from '../stores/list-view-preference'

const model = defineModel<ListViewMode>({ required: true })

function selectViewMode(mode: ListViewMode) {
  model.value = mode
}
</script>

<template>
  <div class="list-view-toggle" role="group" aria-label="切换展示形式">
    <button
      type="button"
      class="list-view-toggle__btn"
      :class="{ 'list-view-toggle__btn--active': model === 'list' }"
      :aria-pressed="model === 'list'"
      title="列表视图"
      @click="selectViewMode('list')"
    >
      <List :size="16" />
    </button>
    <button
      type="button"
      class="list-view-toggle__btn"
      :class="{ 'list-view-toggle__btn--active': model === 'card' }"
      :aria-pressed="model === 'card'"
      title="卡片视图"
      @click="selectViewMode('card')"
    >
      <LayoutGrid :size="16" />
    </button>
  </div>
</template>

<style scoped>
.list-view-toggle {
  display: inline-flex;
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  overflow: hidden;
}

.list-view-toggle__btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  background: var(--surface-base);
  color: var(--text-muted);
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}

.list-view-toggle__btn:hover {
  background: var(--surface-sunken);
  color: var(--text-primary);
}

.list-view-toggle__btn--active {
  background: var(--primary-50);
  color: var(--primary-600);
}

.list-view-toggle__btn + .list-view-toggle__btn {
  border-left: 1px solid var(--border-default);
}
</style>