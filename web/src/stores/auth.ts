import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import { getMe, login as loginRequest, refreshToken as refreshTokenRequest, register as registerRequest } from '../api/auth'
import type { User } from '../types/auth'
import { clearAuthTokens, getAccessToken, getRefreshToken, setAccessToken, setAuthTokens } from '../utils/storage'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref<string | null>(getAccessToken())
  const refreshToken = ref<string | null>(getRefreshToken())
  const user = ref<User | null>(null)
  const isInitialized = ref(false)
  let initializePromise: Promise<void> | null = null

  const isAuthenticated = computed(() => Boolean(accessToken.value && user.value))
  const isAdmin = computed(() => user.value?.role === 'admin')
  const defaultRoute = computed(() => isAdmin.value ? '/dashboard' : '/instances')

  function applySession(accessTokenValue: string | null, refreshTokenValue: string | null) {
    accessToken.value = accessTokenValue
    refreshToken.value = refreshTokenValue
    setAuthTokens(accessTokenValue, refreshTokenValue)
  }

  function syncTokensFromStorage() {
    accessToken.value = getAccessToken()
    refreshToken.value = getRefreshToken()
  }

  async function login(email: string, password: string) {
    const response = await loginRequest(email, password)
    applySession(response.access_token, response.refresh_token)
    user.value = response.user
    await fetchCurrentUser()
  }

  async function register(email: string) {
    await registerRequest(email)
  }

  function logout() {
    applySession(null, null)
    user.value = null
  }

  async function refreshAccessToken() {
    const currentRefreshToken = refreshToken.value ?? getRefreshToken()
    if (!currentRefreshToken) {
      throw new Error('Refresh token is not available.')
    }

    const response = await refreshTokenRequest(currentRefreshToken)
    accessToken.value = response.access_token
    setAccessToken(response.access_token)
  }

  async function fetchCurrentUser() {
    user.value = await getMe()
  }

  async function initialize() {
    syncTokensFromStorage()

    if (!accessToken.value && !refreshToken.value) {
      user.value = null
      return
    }

    try {
      if (!accessToken.value && refreshToken.value) {
        await refreshAccessToken()
      }

      if (accessToken.value) {
        await fetchCurrentUser()
        return
      }
    } catch {
      clearAuthTokens()
      accessToken.value = null
      refreshToken.value = null
      user.value = null
      return
    }

    clearAuthTokens()
    accessToken.value = null
    refreshToken.value = null
    user.value = null
  }

  async function ensureInitialized() {
    if (isInitialized.value) {
      return
    }

    if (!initializePromise) {
      initializePromise = initialize()
        .finally(() => {
          isInitialized.value = true
          initializePromise = null
        })
    }

    return initializePromise
  }

  return {
    accessToken,
    refreshToken,
    user,
    isInitialized,
    isAuthenticated,
    isAdmin,
    defaultRoute,
    ensureInitialized,
    fetchCurrentUser,
    login,
    logout,
    refreshAccessToken,
    register,
  }
})