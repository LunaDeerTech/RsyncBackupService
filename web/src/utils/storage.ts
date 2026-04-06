import type { ThemeMode } from '../types/theme'

const THEME_STORAGE_KEY = 'rbs-theme'
const ACCESS_TOKEN_STORAGE_KEY = 'rbs-access-token'

function canUseStorage() {
  return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined'
}

export function getStoredTheme(): ThemeMode | null {
  if (!canUseStorage()) {
    return null
  }

  const value = window.localStorage.getItem(THEME_STORAGE_KEY)
  return value === 'light' || value === 'dark' ? value : null
}

export function setStoredTheme(theme: ThemeMode) {
  if (!canUseStorage()) {
    return
  }

  window.localStorage.setItem(THEME_STORAGE_KEY, theme)
}

export function getAccessToken() {
  if (!canUseStorage()) {
    return ''
  }

  return window.localStorage.getItem(ACCESS_TOKEN_STORAGE_KEY) ?? ''
}

export function clearAccessToken() {
  if (!canUseStorage()) {
    return
  }

  window.localStorage.removeItem(ACCESS_TOKEN_STORAGE_KEY)
}