<script setup lang="ts">
import { Inbox } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    message?: string
    icon?: typeof Inbox
  }>(),
  {
    message: '暂无数据',
  },
)
</script>

<template>
  <div class="app-empty">
    <component :is="icon ?? Inbox" :size="40" class="app-empty__icon" />
    <p class="app-empty__message">{{ message }}</p>
  </div>
</template>

<style scoped>
.app-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
  gap: 12px;
}
.app-empty__icon {
  color: var(--text-muted);
  opacity: 0.6;
}
.app-empty__message {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
}
</style>
