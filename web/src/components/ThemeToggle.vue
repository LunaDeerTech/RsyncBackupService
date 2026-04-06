<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { useThemeStore } from '../stores/theme'

const themeStore = useThemeStore()
const { theme } = storeToRefs(themeStore)

const nextLabel = computed(() => theme.value === 'dark' ? 'Switch to Light' : 'Switch to Dark')
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center gap-3 rounded-full border border-outline bg-surface-base px-4 py-2 text-sm font-medium text-content-primary transition hover:border-primary-500 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
    @click="themeStore.toggleTheme"
  >
    <span class="inline-flex h-2.5 w-2.5 rounded-full bg-primary-500 shadow-[0_0_18px_var(--primary-500)]"></span>
    <span>{{ nextLabel }}</span>
  </button>
</template>