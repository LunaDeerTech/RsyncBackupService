<script setup lang="ts">
import { computed } from 'vue'
import { storeToRefs } from 'pinia'
import { Sun, Moon } from 'lucide-vue-next'
import { useThemeStore } from '../stores/theme'

const props = withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })

const themeStore = useThemeStore()
const { theme } = storeToRefs(themeStore)

const nextLabel = computed(() => theme.value === 'dark' ? 'Switch to Light' : 'Switch to Dark')
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center justify-center rounded-lg border border-outline text-sm font-medium text-content-primary transition hover:border-primary-500 hover:text-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40"
    :class="props.compact ? 'h-9 w-9' : 'gap-2 px-3 py-2'"
    :title="nextLabel"
    @click="themeStore.toggleTheme"
  >
    <Sun v-if="theme === 'dark'" :size="16" />
    <Moon v-else :size="16" />
    <span v-if="!props.compact">{{ nextLabel }}</span>
  </button>
</template>