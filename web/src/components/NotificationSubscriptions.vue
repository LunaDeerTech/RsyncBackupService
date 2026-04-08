<script setup lang="ts">
import { ref, onMounted } from 'vue'
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
  savingIds.value.add(item.instance_id)
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
    savingIds.value.delete(item.instance_id)
  }
}
</script>

<template>
  <div class="notification-subs">
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
      >
        <div class="notification-subs__info">
          <span class="notification-subs__name">{{ item.instance_name }}</span>
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
  display: flex;
  flex-direction: column;
}

.notification-subs__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-bottom: 1px solid var(--border-subtle);
}

.notification-subs__item:last-child {
  border-bottom: none;
}

.notification-subs__info {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.notification-subs__name {
  font-size: 14px;
  font-weight: 500;
  color: var(--text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
