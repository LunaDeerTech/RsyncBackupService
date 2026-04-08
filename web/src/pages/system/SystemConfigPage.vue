<script setup lang="ts">
import { ref } from 'vue'
import AppTabs from '../../components/AppTabs.vue'
import type { TabItem } from '../../components/AppTabs.vue'
import RemoteConfigPanel from './RemoteConfigPanel.vue'
import UserManagementPage from './UserManagementPage.vue'
import SmtpConfigPanel from './SmtpConfigPanel.vue'

const tabs: TabItem[] = [
  { key: 'remotes', label: '远程配置' },
  { key: 'users', label: '用户管理' },
  { key: 'smtp', label: 'SMTP 配置' },
]

const activeTab = ref('remotes')
</script>

<template>
  <div class="system-config-page">
    <AppTabs :tabs="tabs" v-model:active-key="activeTab">
      <template #tab-remotes>
        <RemoteConfigPanel />
      </template>
      <template #tab-users>
        <UserManagementPage />
      </template>
      <template #tab-smtp>
        <SmtpConfigPanel />
      </template>
    </AppTabs>
  </div>
</template>

<style scoped>
.system-config-page {
  display: flex;
  flex-direction: column;
}
</style>
