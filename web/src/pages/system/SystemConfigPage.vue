<script setup lang="ts">
import { ref } from 'vue'
import AppTabs from '../../components/AppTabs.vue'
import type { TabItem } from '../../components/AppTabs.vue'
import RemoteConfigPanel from './RemoteConfigPanel.vue'

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
        <div class="coming-soon">
          <p class="coming-soon__text">用户管理功能即将上线</p>
        </div>
      </template>
      <template #tab-smtp>
        <div class="coming-soon">
          <p class="coming-soon__text">SMTP 配置功能即将上线</p>
        </div>
      </template>
    </AppTabs>
  </div>
</template>

<style scoped>
.system-config-page {
  display: flex;
  flex-direction: column;
}
.coming-soon {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 64px 0;
}
.coming-soon__text {
  margin: 0;
  font-size: 14px;
  color: var(--text-muted);
}
</style>
