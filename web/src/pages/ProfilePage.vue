<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useAuthStore } from '../stores/auth'
import AppCard from '../components/AppCard.vue'
import NotificationSubscriptions from '../components/NotificationSubscriptions.vue'
import { UserCircle } from 'lucide-vue-next'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
</script>

<template>
  <div class="profile-page">
    <!-- User info card -->
    <AppCard title="账户信息">
      <div class="profile-page__user">
        <div class="profile-page__avatar">
          <UserCircle :size="48" />
        </div>
        <div class="profile-page__details">
          <p class="profile-page__name">{{ user?.name ?? '-' }}</p>
          <p class="profile-page__email">{{ user?.email ?? '-' }}</p>
          <p class="profile-page__role">
            {{ user?.role === 'admin' ? '管理员' : '普通用户' }}
          </p>
        </div>
      </div>
    </AppCard>

    <!-- Notification subscriptions -->
    <AppCard title="通知订阅">
      <NotificationSubscriptions />
    </AppCard>
  </div>
</template>

<style scoped>
.profile-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
  max-width: 640px;
}

.profile-page__user {
  display: flex;
  align-items: center;
  gap: 16px;
}

.profile-page__avatar {
  color: var(--text-muted);
  flex-shrink: 0;
}

.profile-page__details {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.profile-page__name {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}

.profile-page__email {
  margin: 0;
  font-size: 14px;
  color: var(--text-secondary);
}

.profile-page__role {
  margin: 0;
  font-size: 13px;
  color: var(--text-muted);
}
</style>
