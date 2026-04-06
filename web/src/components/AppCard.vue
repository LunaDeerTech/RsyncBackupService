<script setup lang="ts">
withDefaults(
  defineProps<{
    title?: string
    padding?: string
  }>(),
  {
    title: '',
    padding: '20px',
  },
)
</script>

<template>
  <div class="app-card" :style="{ '--card-padding': padding }">
    <div v-if="$slots.header || title" class="app-card__header">
      <slot name="header">
        <h3 class="app-card__title">{{ title }}</h3>
      </slot>
    </div>
    <div class="app-card__body">
      <slot />
    </div>
  </div>
</template>

<style scoped>
.app-card {
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-lg);
}
.app-card__header {
  padding: 16px var(--card-padding);
  border-bottom: 1px solid var(--border-subtle);
}
.app-card__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}
.app-card__body {
  padding: var(--card-padding);
}
</style>
