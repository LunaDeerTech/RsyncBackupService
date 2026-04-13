<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getInstance } from '../../api/instances'
import { useAuthStore } from '../../stores/auth'
import { useToastStore } from '../../stores/toast'
import type { Instance } from '../../types/instance'
import AppTabs from '../../components/AppTabs.vue'
import AppButton from '../../components/AppButton.vue'
import AppConfirm from '../../components/AppConfirm.vue'
import { ArrowLeft } from 'lucide-vue-next'
import OverviewTab from './components/OverviewTab.vue'
import PoliciesTab from './components/PoliciesTab.vue'
import BackupsTab from './components/BackupsTab.vue'
import AuditTab from './components/AuditTab.vue'
import SettingsTab from './components/SettingsTab.vue'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const toast = useToastStore()
const instanceId = computed(() => Number(route.params.id))

// ── Tabs ──
const tabs = computed(() => {
  const items = [
    { key: 'overview', label: '概览' },
    { key: 'policies', label: '策略' },
    { key: 'backups', label: '备份' },
    { key: 'audit', label: '审计' },
  ]
  if (authStore.isAdmin) {
    items.push({ key: 'settings', label: '设置' })
  }
  return items
})
const activeTab = ref('overview')

// ── Instance data ──
const instance = ref<Instance | null>(null)
const viewerPermission = ref<string | undefined>(undefined)
const canDownload = computed(() => authStore.isAdmin || viewerPermission.value === 'readdownload')
const pageLoading = ref(false)

// ── Tab refs ──
const overviewRef = ref<InstanceType<typeof OverviewTab>>()
const policiesRef = ref<InstanceType<typeof PoliciesTab>>()
const backupsRef = ref<InstanceType<typeof BackupsTab>>()
const auditRef = ref<InstanceType<typeof AuditTab>>()
const settingsRef = ref<InstanceType<typeof SettingsTab>>()

// ── Fetch core data ──
async function fetchInstance() {
  pageLoading.value = true
  try {
    const res = await getInstance(instanceId.value)
    instance.value = res.instance
    viewerPermission.value = res.permission
  } catch {
    toast.error('加载实例详情失败')
    router.push('/instances')
  } finally {
    pageLoading.value = false
  }
}

onMounted(async () => {
  await fetchInstance()
})

// ── Watch tab changes ──
watch(activeTab, (tab) => {
  if (tab === 'overview') overviewRef.value?.refresh()
  if (tab === 'policies') policiesRef.value?.refresh()
  if (tab === 'backups') backupsRef.value?.refresh()
  if (tab === 'audit') auditRef.value?.refresh()
  if (tab === 'settings') settingsRef.value?.refresh()
})

function handleChangeTab(tab: string) {
  activeTab.value = tab
}

function handleInstanceUpdated(updated: Instance) {
  instance.value = updated
}

function handleInstanceDeleted() {
  router.push({ name: 'instances' })
}
</script>

<template>
  <div class="instance-detail-page">
    <!-- Header -->
    <div class="instance-detail-page__header">
      <AppButton variant="ghost" size="sm" @click="router.push('/instances')">
        <ArrowLeft :size="16" style="margin-right: 4px" />
        返回
      </AppButton>
      <h2 v-if="instance" class="instance-detail-page__title">{{ instance.name }}</h2>
    </div>

    <!-- Loading state -->
    <div v-if="pageLoading" class="instance-detail-page__loading">加载中…</div>

    <!-- Content -->
    <template v-if="!pageLoading && instance">
      <AppTabs :tabs="tabs" :active-key="activeTab" @update:active-key="activeTab = $event">
        <template #tab-overview>
          <OverviewTab
            ref="overviewRef"
            :instance-id="instanceId"
            :instance="instance"
            @change-tab="handleChangeTab"
          />
        </template>

        <template #tab-policies>
          <PoliciesTab
            ref="policiesRef"
            :instance-id="instanceId"
          />
        </template>

        <template #tab-backups>
          <BackupsTab
            ref="backupsRef"
            :instance-id="instanceId"
            :instance="instance"
            :can-download="canDownload"
            @change-tab="handleChangeTab"
          />
        </template>

        <template #tab-audit>
          <AuditTab
            ref="auditRef"
            :instance-id="instanceId"
          />
        </template>

        <template #tab-settings v-if="authStore.isAdmin">
          <SettingsTab
            ref="settingsRef"
            :instance-id="instanceId"
            :instance="instance"
            @instance-updated="handleInstanceUpdated"
            @instance-deleted="handleInstanceDeleted"
          />
        </template>
      </AppTabs>
    </template>

    <AppConfirm />
  </div>
</template>

<style scoped>
.instance-detail-page {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.instance-detail-page__header {
  display: flex;
  align-items: center;
  gap: 12px;
}

.instance-detail-page__title {
  font-size: 20px;
  font-weight: 600;
  color: var(--text-primary);
}

.instance-detail-page__loading {
  text-align: center;
  padding: 60px 0;
  color: var(--text-muted);
}
</style>
