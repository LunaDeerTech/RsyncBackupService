<script setup lang="ts">
import ThemeToggle from '../components/ThemeToggle.vue'

interface Props {
  eyebrow: string
  title: string
  description: string
}

defineProps<Props>()
</script>

<template>
  <div class="relative min-h-screen overflow-hidden bg-surface-canvas text-content-primary">
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute left-[10%] top-[-8rem] h-72 w-72 rounded-full bg-primary-300/35 blur-3xl"></div>
      <div class="absolute right-[8%] top-[14%] h-80 w-80 rounded-full bg-emerald-300/20 blur-3xl"></div>
      <div class="absolute bottom-[-12rem] left-1/2 h-[26rem] w-[26rem] -translate-x-1/2 rounded-full bg-sky-300/20 blur-3xl"></div>
    </div>

    <div class="relative mx-auto flex min-h-screen w-full max-w-6xl flex-col px-4 py-6 sm:px-6 lg:px-8">
      <header class="flex items-center justify-center py-4 sm:justify-between">
        <RouterLink to="/" class="inline-flex items-center gap-3 text-content-primary transition hover:text-primary-600">
          <img src="/brand/logo-final.svg" alt="Rsync Backup Service" class="h-11 w-11 rounded-2xl shadow-glow" />
          <div>
            <p class="text-xs font-semibold uppercase tracking-[0.36em] text-content-muted">Secure Access</p>
            <p class="text-lg font-semibold">Rsync Backup Service</p>
          </div>
        </RouterLink>
      </header>

      <main class="flex flex-1 items-center justify-center py-8">
        <section class="w-full max-w-md overflow-hidden rounded-[32px] border border-outline bg-surface-base/94 shadow-panel backdrop-blur">
          <div class="border-b border-outline-subtle bg-surface-raised/80 px-6 py-7 text-center sm:px-8">
            <p class="text-xs font-semibold uppercase tracking-[0.36em] text-content-muted">{{ eyebrow }}</p>
            <h1 class="mt-4 text-3xl font-semibold tracking-tight text-content-primary">{{ title }}</h1>
            <p class="mt-3 text-sm leading-7 text-content-secondary">{{ description }}</p>
          </div>

          <div class="px-6 py-7 sm:px-8">
            <slot />
          </div>
        </section>
      </main>

      <footer class="flex justify-center pb-2 pt-4">
        <ThemeToggle />
      </footer>
    </div>
  </div>
</template>