<script setup lang="ts">
withDefaults(
  defineProps<{
    variant?: 'success' | 'warning' | 'error' | 'info' | 'default'
  }>(),
  {
    variant: 'default',
  },
)
</script>

<template>
  <span class="app-badge" :class="`app-badge--${variant}`">
    <slot />
  </span>
</template>

<style scoped>
.app-badge {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
  border-radius: 9999px;
  white-space: nowrap;
}
.app-badge--default {
  background: var(--surface-sunken);
  color: var(--text-secondary);
}
.app-badge--success {
  background: color-mix(in srgb, var(--success-500) 15%, transparent);
  color: var(--success-500);
}
.app-badge--warning {
  background: color-mix(in srgb, var(--warning-500) 15%, transparent);
  color: var(--warning-500);
}
.app-badge--error {
  background: color-mix(in srgb, var(--error-500) 15%, transparent);
  color: var(--error-500);
}
.app-badge--info {
  background: color-mix(in srgb, var(--primary-500) 15%, transparent);
  color: var(--primary-500);
}
</style>
