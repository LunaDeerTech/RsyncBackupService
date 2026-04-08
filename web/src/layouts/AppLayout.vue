<script setup lang="ts">
import { computed, ref, watch } from 'vue'
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
} from 'lucide-vue-next'
import ThemeToggle from '../components/ThemeToggle.vue'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const { isAdmin, user } = storeToRefs(authStore)
const route = useRoute()
const router = useRouter()

const drawerOpen = ref(false)

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
  authStore.logout()
  drawerOpen.value = false
  await router.replace('/login')
}

// Close drawer on route change
watch(() => route.path, () => {
  drawerOpen.value = false
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
      <span class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-[linear-gradient(135deg,var(--primary-500),var(--accent-mint-400))] font-mono text-xs font-bold text-slate-950">RBS</span>
      <span class="text-sm font-semibold text-content-primary">Rsync Backup Service</span>
      <div class="ml-auto">
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
        <span class="inline-flex h-8 w-8 items-center justify-center rounded-lg bg-[linear-gradient(135deg,var(--primary-500),var(--accent-mint-400))] font-mono text-xs font-bold text-slate-950">RBS</span>
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
        <ThemeToggle />
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
</style>