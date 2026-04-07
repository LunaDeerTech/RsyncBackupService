<script setup lang="ts">
export interface TabItem {
  key: string
  label: string
}

const props = defineProps<{
  tabs: TabItem[]
  activeKey: string
}>()

const emit = defineEmits<{
  'update:activeKey': [value: string]
}>()
</script>

<template>
  <div class="app-tabs">
    <div class="app-tabs__header" role="tablist">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        role="tab"
        class="app-tabs__tab"
        :class="{ 'app-tabs__tab--active': activeKey === tab.key }"
        :aria-selected="activeKey === tab.key"
        @click="emit('update:activeKey', tab.key)"
      >
        {{ tab.label }}
      </button>
    </div>
    <div class="app-tabs__content">
      <template v-for="tab in tabs" :key="tab.key">
        <div v-show="activeKey === tab.key" role="tabpanel">
          <slot :name="`tab-${tab.key}`" />
        </div>
      </template>
    </div>
  </div>
</template>

<style scoped>
.app-tabs__header {
  display: flex;
  gap: 0;
  border-bottom: 1px solid var(--border-default);
}
.app-tabs__tab {
  position: relative;
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text-secondary);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: color var(--transition-fast);
  white-space: nowrap;
}
.app-tabs__tab:hover {
  color: var(--text-primary);
}
.app-tabs__tab--active {
  color: var(--primary-500);
}
.app-tabs__tab--active::after {
  content: '';
  position: absolute;
  bottom: -1px;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--primary-500);
  border-radius: 1px 1px 0 0;
}
.app-tabs__content {
  padding-top: 16px;
}
</style>
