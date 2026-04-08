<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import {
  LayoutDashboard,
  Server,
  HardDrive,
  Settings,
  ShieldAlert,
  UserCircle,
  LogOut,
  Menu,
  X,
  Activity,
} from 'lucide-vue-next'
import ThemeToggle from '../components/ThemeToggle.vue'
import AppBadge from '../components/AppBadge.vue'
import AppProgress from '../components/AppProgress.vue'
import { useAuthStore } from '../stores/auth'
import { useTaskStore } from '../stores/task'

const authStore = useAuthStore()
const { isAdmin, user, isAuthenticated } = storeToRefs(authStore)
const taskStore = useTaskStore()
const route = useRoute()
const router = useRouter()

const drawerOpen = ref(false)
const taskPanelOpen = ref(false)

// Start/stop task polling based on auth
watch(isAuthenticated, (val) => {
  if (val) taskStore.startPolling()
  else taskStore.stopPolling()
}, { immediate: true })

onUnmounted(() => {
  taskStore.stopPolling()
})

function toggleTaskPanel() {
  taskPanelOpen.value = !taskPanelOpen.value
}

function closeTaskPanel() {
  taskPanelOpen.value = false
}

function taskTypeLabel(type: string): string {
  switch (type) {
    case 'rolling': return '滚动备份'
    case 'cold': return '冷备份'
    case 'restore': return '恢复'
    default: return type
  }
}

interface NavItem {
  label: string
  path: string
  icon: typeof LayoutDashboard
  adminOnly: boolean
}

const navItems = computed<NavItem[]>(() => {
  const items: NavItem[] = [
    { label: '仪表盘', path: '/dashboard', icon: LayoutDashboard, adminOnly: true },
    { label: '实例列表', path: '/instances', icon: Server, adminOnly: false },
    { label: '备份目标', path: '/targets', icon: HardDrive, adminOnly: true },
    { label: '系统配置', path: '/system', icon: Settings, adminOnly: true },
    { label: '风险事件', path: '/system/risks', icon: ShieldAlert, adminOnly: true },
  ]
  return items.filter((item) => !item.adminOnly || isAdmin.value)
})

function isActive(path: string): boolean {
  if (path === '/system') {
    return route.path === '/system'
  }
  return route.path === path || route.path.startsWith(path + '/')
}

function navigateTo(path: string) {
  router.push(path)
  drawerOpen.value = false
}

async function handleLogout() {
  taskStore.stopPolling()
  authStore.logout()
  drawerOpen.value = false
  await router.replace('/login')
}

// Close drawer on route change
watch(() => route.path, () => {
  drawerOpen.value = false
  taskPanelOpen.value = false
})
</script>

<template>
  <div class="min-h-screen bg-surface-canvas text-content-primary">
    <!-- Mobile top bar -->
    <header class="fixed inset-x-0 top-0 z-40 flex h-14 items-center gap-3 border-b border-outline bg-surface-base/95 px-4 backdrop-blur lg:hidden">
      <button
        type="button"
        class="inline-flex h-9 w-9 items-center justify-center rounded-md text-content-secondary transition hover:bg-surface-sunken hover:text-content-primary"
        @click="drawerOpen = !drawerOpen"
      >
        <Menu v-if="!drawerOpen" :size="20" />
        <X v-else :size="20" />
      </button>
      <img src="/brand/logo-final.svg" alt="Rsync Backup Service" class="h-8 w-8 rounded-lg" />
      <span class="text-sm font-semibold text-content-primary">Rsync Backup Service</span>
      <div class="ml-auto flex items-center gap-2">
        <button
          v-if="taskStore.runningCount > 0"
          type="button"
          class="task-indicator"
          @click="toggleTaskPanel"
        >
          <Activity :size="16" />
          <span class="task-indicator__count">{{ taskStore.runningCount }}</span>
        </button>
        <ThemeToggle compact />
      </div>
    </header>

    <!-- Mobile drawer overlay -->
    <Transition name="fade">
      <div
        v-if="drawerOpen"
        class="fixed inset-0 z-40 bg-black/40 backdrop-blur-sm lg:hidden"
        @click="drawerOpen = false"
      />
    </Transition>

    <!-- Sidebar / Drawer -->
    <aside
      class="fixed inset-y-0 left-0 z-50 flex w-60 flex-col border-r border-outline bg-surface-base transition-transform duration-300 lg:z-30 lg:translate-x-0"
      :class="drawerOpen ? 'translate-x-0' : '-translate-x-full'"
    >
      <!-- Sidebar header -->
      <div class="flex h-14 items-center gap-3 border-b border-outline px-5">
        <img src="/brand/logo-final.svg" alt="Rsync Backup Service" class="h-8 w-8 rounded-lg" />
        <span class="text-sm font-semibold text-content-primary">Rsync Backup Service</span>
      </div>

      <!-- Navigation -->
      <nav class="flex-1 overflow-y-auto px-3 py-4">
        <ul class="space-y-1">
          <li v-for="item in navItems" :key="item.path">
            <button
              type="button"
              class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition"
              :class="isActive(item.path)
                ? 'bg-primary-500/10 text-primary-600'
                : 'text-content-secondary hover:bg-surface-sunken hover:text-content-primary'"
              @click="navigateTo(item.path)"
            >
              <component :is="item.icon" :size="18" class="shrink-0" />
              <span>{{ item.label }}</span>
            </button>
          </li>
        </ul>
      </nav>

      <!-- Sidebar footer -->
      <div class="border-t border-outline px-3 py-3 space-y-1">
        <button
          type="button"
          class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-content-secondary transition hover:bg-surface-sunken hover:text-content-primary"
          @click="navigateTo('/profile')"
        >
          <UserCircle :size="18" class="shrink-0" />
          <span class="truncate">{{ user?.name ?? '个人中心' }}</span>
        </button>
        <button
          type="button"
          class="flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium text-content-secondary transition hover:bg-surface-sunken hover:text-error-500"
          @click="handleLogout"
        >
          <LogOut :size="18" class="shrink-0" />
          <span>登出</span>
        </button>
      </div>
    </aside>

    <!-- Main content area -->
    <div class="lg:pl-60">
      <!-- Desktop top bar -->
      <header class="sticky top-0 z-20 hidden h-14 items-center justify-between border-b border-outline bg-surface-base/95 px-6 backdrop-blur lg:flex">
        <h1 class="text-lg font-semibold text-content-primary">
          {{ route.meta.title ?? '' }}
        </h1>
        <div class="flex items-center gap-3">
          <div class="task-indicator-wrapper">
            <button
              type="button"
              class="task-indicator"
              :class="{ 'task-indicator--active': taskStore.runningCount > 0 }"
              @click="toggleTaskPanel"
            >
              <Activity :size="16" />
              <span v-if="taskStore.runningCount > 0" class="task-indicator__count">{{ taskStore.runningCount }}</span>
            </button>
            <!-- Task dropdown panel -->
            <Transition name="fade">
              <div v-if="taskPanelOpen" class="task-panel" @click.stop>
                <div class="task-panel__header">
                  <span class="task-panel__title">运行中任务</span>
                  <button type="button" class="task-panel__close" @click="closeTaskPanel">
                    <X :size="14" />
                  </button>
                </div>
                <div v-if="taskStore.activeTasks.length === 0" class="task-panel__empty">
                  暂无运行中任务
                </div>
                <div v-else class="task-panel__list">
                  <div v-for="t in taskStore.activeTasks" :key="t.id" class="task-panel__item">
                    <div class="task-panel__item-header">
                      <AppBadge :variant="t.status === 'running' ? 'info' : 'warning'" class="task-panel__badge">
                        {{ t.status === 'running' ? '运行中' : '排队中' }}
                      </AppBadge>
                      <span class="task-panel__item-name">{{ t.instance_name }}</span>
                      <span class="task-panel__item-type">{{ taskTypeLabel(t.type) }}</span>
                    </div>
                    <div class="task-panel__item-progress">
                      <AppProgress :value="t.progress" size="sm" />
                      <span class="task-panel__item-percent">{{ t.progress }}%</span>
                    </div>
                  </div>
                </div>
              </div>
            </Transition>
          </div>
          <ThemeToggle />
        </div>
      </header>

      <!-- Page content -->
      <main class="min-h-[calc(100vh-3.5rem)] px-4 py-6 pt-20 sm:px-6 lg:px-8 lg:pt-6">
        <router-view />
      </main>
    </div>
  </div>
</template>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity var(--transition-normal);
}
.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Task indicator */
.task-indicator {
  position: relative;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 6px 10px;
  border-radius: var(--radius-md, 8px);
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary, #6b7280);
  background: transparent;
  border: none;
  cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.task-indicator:hover {
  background: var(--surface-sunken, #f3f4f6);
  color: var(--text-primary, #111827);
}
.task-indicator--active {
  color: var(--primary-500, #3b82f6);
}
.task-indicator__count {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: 9999px;
  background: var(--primary-500, #3b82f6);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  line-height: 1;
}

/* Task panel dropdown */
.task-indicator-wrapper {
  position: relative;
}
.task-panel {
  position: absolute;
  top: calc(100% + 8px);
  right: 0;
  width: 360px;
  max-height: 400px;
  overflow-y: auto;
  background: var(--surface-base, #fff);
  border: 1px solid var(--border-default, #e5e7eb);
  border-radius: var(--radius-lg, 12px);
  box-shadow: 0 10px 25px -5px rgba(0,0,0,.1), 0 8px 10px -6px rgba(0,0,0,.1);
  z-index: 100;
}
.task-panel__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle, #f3f4f6);
}
.task-panel__title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary, #111827);
}
.task-panel__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  background: transparent;
  color: var(--text-muted, #9ca3af);
  border-radius: var(--radius-sm, 4px);
  cursor: pointer;
  transition: background 0.15s;
}
.task-panel__close:hover {
  background: var(--surface-sunken, #f3f4f6);
}
.task-panel__empty {
  padding: 24px 16px;
  text-align: center;
  font-size: 13px;
  color: var(--text-muted, #9ca3af);
}
.task-panel__list {
  padding: 8px;
}
.task-panel__item {
  padding: 10px 8px;
  border-radius: var(--radius-md, 8px);
  transition: background 0.15s;
}
.task-panel__item:hover {
  background: var(--surface-sunken, #f3f4f6);
}
.task-panel__item + .task-panel__item {
  border-top: 1px solid var(--border-subtle, #f3f4f6);
}
.task-panel__item-header {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-bottom: 6px;
}
.task-panel__item-name {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-primary, #111827);
}
.task-panel__item-type {
  font-size: 12px;
  color: var(--text-muted, #9ca3af);
}
.task-panel__item-progress {
  display: flex;
  align-items: center;
  gap: 8px;
}
.task-panel__item-progress .app-progress {
  flex: 1;
}
.task-panel__item-percent {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-primary, #111827);
  min-width: 32px;
  text-align: right;
}
</style>