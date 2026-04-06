import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { ThemeMode } from '../types/theme'
import { getStoredTheme, setStoredTheme } from '../utils/storage'

function resolveSystemTheme(): ThemeMode {
  if (typeof window === 'undefined') {
    return 'light'
  }

  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

function applyTheme(theme: ThemeMode) {
  if (typeof document === 'undefined') {
    return
  }

  document.documentElement.dataset.theme = theme
  document.documentElement.style.colorScheme = theme
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<ThemeMode>('light')

  function setTheme(nextTheme: ThemeMode) {
    theme.value = nextTheme
    setStoredTheme(nextTheme)
    applyTheme(nextTheme)
  }

  function initializeTheme() {
    const preferredTheme = getStoredTheme() ?? resolveSystemTheme()
    setTheme(preferredTheme)
  }

  function toggleTheme() {
    setTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  return {
    theme,
    initializeTheme,
    setTheme,
    toggleTheme,
  }
})