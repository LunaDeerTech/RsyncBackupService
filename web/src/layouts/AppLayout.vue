<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'
import ThemeToggle from '../components/ThemeToggle.vue'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const { isAdmin, isAuthenticated, user } = storeToRefs(authStore)
const route = useRoute()
const router = useRouter()

const primaryNavigation = computed(() => {
  const items = [
    { label: 'Dashboard', path: '/dashboard', adminOnly: true },
    { label: 'Instances', path: '/instances', adminOnly: false },
  ]

  return items.filter((item) => !item.adminOnly || isAdmin.value)
})

async function handleLogout() {
  authStore.logout()
  await router.replace('/login')
}
</script>

<template>
  <div class="relative min-h-screen overflow-hidden bg-surface-canvas text-content-primary">
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute left-[8%] top-[-9rem] h-72 w-72 rounded-full bg-primary-300/30 blur-3xl"></div>
      <div class="absolute right-[4%] top-[18%] h-80 w-80 rounded-full bg-emerald-300/20 blur-3xl"></div>
      <div class="absolute bottom-[-12rem] left-1/2 h-96 w-96 -translate-x-1/2 rounded-full bg-sky-300/20 blur-3xl"></div>
    </div>

    <div class="relative mx-auto flex min-h-screen w-full max-w-7xl flex-col px-5 py-6 sm:px-6 lg:px-8">
      <header class="flex flex-col gap-5 rounded-[32px] border border-outline bg-surface-base/80 px-6 py-5 shadow-panel backdrop-blur lg:flex-row lg:items-center lg:justify-between">
        <div class="space-y-2">
          <div class="inline-flex items-center gap-3">
            <span class="inline-flex h-10 w-10 items-center justify-center rounded-2xl bg-[linear-gradient(135deg,var(--primary-500),#7EF2D4)] font-mono text-sm font-bold text-slate-950 shadow-glow">RBS</span>
            <div>
              <p class="text-xs font-semibold uppercase tracking-[0.36em] text-content-muted">Frontend bootstrap</p>
              <h1 class="text-xl font-semibold text-content-primary">Rsync Backup Service</h1>
            </div>
          </div>
          <p class="max-w-2xl text-sm leading-6 text-content-secondary">
            Vue 3, TypeScript, Tailwind, Pinia and Axios are wired up as the initial shell for the admin console.
          </p>
        </div>

        <div class="flex flex-col gap-3 lg:items-end">
          <nav v-if="isAuthenticated" class="flex flex-wrap justify-end gap-2">
            <RouterLink
              v-for="item in primaryNavigation"
              :key="item.path"
              :to="item.path"
              class="rounded-full border px-4 py-2 text-sm font-medium transition"
              :class="route.path === item.path
                ? 'border-primary-500 bg-primary-500/10 text-primary-600'
                : 'border-outline-subtle bg-surface-raised text-content-secondary hover:border-primary-500 hover:text-primary-600'"
            >
              {{ item.label }}
            </RouterLink>
          </nav>

          <div class="flex flex-wrap items-center justify-end gap-3">
            <div v-if="user" class="rounded-full border border-outline-subtle bg-surface-raised px-4 py-2 text-sm text-content-secondary">
              <span class="font-semibold text-content-primary">{{ user.name }}</span>
              <span class="mx-2 text-content-muted">/</span>
              <span class="uppercase tracking-[0.16em] text-content-muted">{{ user.role }}</span>
            </div>

            <button
              v-if="isAuthenticated"
              type="button"
              class="inline-flex items-center rounded-full border border-outline bg-surface-base px-4 py-2 text-sm font-medium text-content-primary transition hover:border-error-500 hover:text-error-500 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
              @click="handleLogout"
            >
              Sign Out
            </button>

            <div class="hidden rounded-full border border-outline-subtle bg-surface-raised px-4 py-2 text-xs font-medium uppercase tracking-[0.24em] text-content-muted sm:block">
              data-theme driven tokens
            </div>

            <ThemeToggle />
          </div>
        </div>
      </header>

      <main class="flex-1 py-8">
        <slot />
      </main>
    </div>
  </div>
</template>