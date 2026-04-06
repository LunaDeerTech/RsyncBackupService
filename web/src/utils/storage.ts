import type { ThemeMode } from '../types/theme'

const THEME_STORAGE_KEY = 'rbs-theme'
const ACCESS_TOKEN_STORAGE_KEY = 'rbs-access-token'
const REFRESH_TOKEN_STORAGE_KEY = 'rbs-refresh-token'

function canUseStorage() {
  return typeof window !== 'undefined' && typeof window.localStorage !== 'undefined'
}

function getStoredValue(key: string) {
  if (!canUseStorage()) {
    return null
  }

  const value = window.localStorage.getItem(key)
  return value && value.trim() !== '' ? value : null
}

function setStoredValue(key: string, value: string | null) {
  if (!canUseStorage()) {
    return
  }

  if (!value) {
    window.localStorage.removeItem(key)
    return
  }

  window.localStorage.setItem(key, value)
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
  return getStoredValue(ACCESS_TOKEN_STORAGE_KEY)
}

export function setAccessToken(token: string | null) {
  setStoredValue(ACCESS_TOKEN_STORAGE_KEY, token)
}

export function getRefreshToken() {
  return getStoredValue(REFRESH_TOKEN_STORAGE_KEY)
}

export function setRefreshToken(token: string | null) {
  setStoredValue(REFRESH_TOKEN_STORAGE_KEY, token)
}

export function setAuthTokens(accessToken: string | null, refreshToken: string | null) {
  setAccessToken(accessToken)
  setRefreshToken(refreshToken)
}

export function clearAuthTokens() {
  setAuthTokens(null, null)
}