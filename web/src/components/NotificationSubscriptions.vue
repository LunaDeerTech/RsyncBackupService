<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { getMySubscriptions, updateMySubscriptions } from '../api/notifications'
import type { SubscriptionItem } from '../api/notifications'
import { useToastStore } from '../stores/toast'
import { ApiBusinessError } from '../api/client'
import AppSwitch from './AppSwitch.vue'
import AppEmpty from './AppEmpty.vue'
import { Bell } from 'lucide-vue-next'

const toast = useToastStore()

const subscriptions = ref<SubscriptionItem[]>([])
const loading = ref(false)
const savingIds = ref<Set<number>>(new Set())
const enabledCount = computed(() => subscriptions.value.filter((item) => item.enabled).length)

function setSaving(instanceID: number, saving: boolean) {
  const next = new Set(savingIds.value)
  if (saving) {
    next.add(instanceID)
  } else {
    next.delete(instanceID)
  }
  savingIds.value = next
}

async function fetchSubscriptions() {
  loading.value = true
  try {
    const res = await getMySubscriptions()
    subscriptions.value = res.subscriptions ?? []
  } catch {
    toast.error('加载通知订阅失败')
  } finally {
    loading.value = false
  }
}

onMounted(fetchSubscriptions)

async function handleToggle(item: SubscriptionItem, enabled: boolean) {
  setSaving(item.instance_id, true)
  try {
    const res = await updateMySubscriptions([{ instance_id: item.instance_id, enabled }])
    subscriptions.value = res.subscriptions ?? []
  } catch (e) {
    if (e instanceof ApiBusinessError) {
      toast.error(e.message)
    } else {
      toast.error('更新订阅失败')
    }
  } finally {
    setSaving(item.instance_id, false)
  }
}
</script>

<template>
  <div class="notification-subs">
    <div v-if="!loading && subscriptions.length > 0" class="notification-subs__summary">
      <div>
        <p class="notification-subs__summary-label">订阅概览</p>
        <p class="notification-subs__summary-value">已开启 {{ enabledCount }} / {{ subscriptions.length }} 个实例通知</p>
      </div>
      <span class="notification-subs__summary-hint">支持同时订阅多个实例</span>
    </div>

    <div v-if="loading" class="notification-subs__loading">
      <p class="notification-subs__loading-text">加载中…</p>
    </div>

    <AppEmpty
      v-else-if="subscriptions.length === 0"
      message="暂无可订阅的实例"
      :icon="Bell"
    />

    <div v-else class="notification-subs__list">
      <div
        v-for="item in subscriptions"
        :key="item.instance_id"
        class="notification-subs__item"
        :class="{ 'notification-subs__item--enabled': item.enabled }"
      >
        <div class="notification-subs__info">
          <span class="notification-subs__name">{{ item.instance_name }}</span>
          <span class="notification-subs__status">{{ item.enabled ? '风险邮件已开启' : '风险邮件已关闭' }}</span>
        </div>
        <AppSwitch
          :model-value="item.enabled"
          :disabled="savingIds.has(item.instance_id)"
          @update:model-value="handleToggle(item, $event)"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.notification-subs {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.notification-subs__summary {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 16px;
  border-radius: var(--radius-md);
  border: 1px solid color-mix(in srgb, var(--primary-500) 16%, var(--border-subtle));
  background: linear-gradient(135deg, color-mix(in srgb, var(--primary-500) 10%, white) 0%, transparent 70%);
}

.notification-subs__summary-label {
  margin: 0 0 4px;
  font-size: 12px;
  color: var(--text-muted);
}

.notification-subs__summary-value {
  margin: 0;
  font-size: 15px;
  font-weight: 600;
  color: var(--text-primary);
}

.notification-subs__summary-hint {
  font-size: 12px;
  color: var(--text-secondary);
  white-space: nowrap;
}

.notification-subs__loading {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 48px 24px;
}

.notification-subs__loading-text {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
}

.notification-subs__list {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 12px;
}

.notification-subs__item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 14px 16px;
  border: 1px solid var(--border-subtle);
  border-radius: var(--radius-md);
  background: var(--surface-sunken);
  transition:
    border-color var(--transition-fast),
    box-shadow var(--transition-fast),
    transform var(--transition-fast);
}

.notification-subs__item--enabled {
  border-color: color-mix(in srgb, var(--primary-500) 28%, var(--border-subtle));
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--primary-500) 14%, transparent);
}

.notification-subs__info {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.notification-subs__name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
}

.notification-subs__status {
  font-size: 12px;
  color: var(--text-secondary);
}

@media (hover: hover) and (pointer: fine) {
  .notification-subs__item:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--primary-500) 22%, var(--border-subtle));
    box-shadow: var(--shadow-sm);
  }
}

@media (max-width: 640px) {
  .notification-subs__summary {
    flex-direction: column;
    align-items: flex-start;
  }

  .notification-subs__summary-hint {
    white-space: normal;
  }
}
</style>
