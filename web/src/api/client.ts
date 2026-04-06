import axios, { AxiosError, AxiosHeaders, type AxiosRequestConfig } from 'axios'
import type { ApiResponse } from './types'
import { clearAuthTokens, getAccessToken, getRefreshToken, setAccessToken } from '../utils/storage'

export class ApiBusinessError extends Error {
  code: number
  status?: number

  constructor(message: string, code: number, status?: number) {
    super(message)
    this.name = 'ApiBusinessError'
    this.code = code
    this.status = status
  }
}

export class ApiNetworkError extends Error {
  status?: number

  constructor(message: string, status?: number) {
    super(message)
    this.name = 'ApiNetworkError'
    this.status = status
  }
}

export interface ApiRequestConfig extends AxiosRequestConfig {
  _retry?: boolean
  skipAuthRefresh?: boolean
}

type RefreshTokenResponse = {
  access_token: string
}

const httpClient = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

const rawClient = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

let refreshRequest: Promise<string> | null = null

function isAuthRoute(url?: string) {
  return typeof url === 'string' && /^\/auth\/(login|register|refresh)$/.test(url)
}

function redirectToLogin() {
  if (typeof window === 'undefined') {
    return
  }

  if (window.location.pathname !== '/login') {
    window.location.replace('/login')
  }
}

function normalizeAxiosError(error: unknown) {
  if (error instanceof ApiBusinessError || error instanceof ApiNetworkError) {
    return error
  }

  if (axios.isAxiosError<ApiResponse<unknown>>(error)) {
    const status = error.response?.status
    const payload = error.response?.data

    if (payload && typeof payload.code === 'number' && payload.code !== 0) {
      return new ApiBusinessError(payload.message || 'Request failed.', payload.code, status)
    }

    const message = error.code === 'ERR_NETWORK'
      ? 'Network error. Please confirm the backend service is reachable.'
      : error.message || 'Request failed due to an unexpected network error.'

    return new ApiNetworkError(message, status)
  }

  return new ApiNetworkError('Request failed due to an unexpected network error.')
}

async function refreshAccessTokenWithLock() {
  const storedRefreshToken = getRefreshToken()
  if (!storedRefreshToken) {
    clearAuthTokens()
    redirectToLogin()
    throw new ApiBusinessError('Authentication expired. Please sign in again.', 40101, 401)
  }

  if (!refreshRequest) {
    refreshRequest = rawClient
      .post<ApiResponse<RefreshTokenResponse>>('/auth/refresh', {
        refresh_token: storedRefreshToken,
      })
      .then((response) => {
        const payload = response.data

        if (!payload || typeof payload.code !== 'number') {
          throw new ApiNetworkError('Unexpected response format received from server.', response.status)
        }

        if (payload.code !== 0) {
          throw new ApiBusinessError(payload.message || 'Request failed.', payload.code, response.status)
        }

        const nextAccessToken = payload.data?.access_token
        if (!nextAccessToken) {
          throw new ApiNetworkError('Unexpected response format received from server.', response.status)
        }

        setAccessToken(nextAccessToken)
        return nextAccessToken
      })
      .catch((error: unknown) => {
        const normalizedError = normalizeAxiosError(error)
        clearAuthTokens()
        redirectToLogin()
        throw normalizedError
      })
      .finally(() => {
        refreshRequest = null
      })
  }

  return refreshRequest
}

httpClient.interceptors.request.use((config) => {
  const token = getAccessToken()
  if (token) {
    config.headers.set('Authorization', `Bearer ${token}`)
  }
  return config
})

httpClient.interceptors.response.use(
  (response) => {
    const payload = response.data as ApiResponse<unknown>

    if (!payload || typeof payload.code !== 'number') {
      throw new ApiNetworkError('Unexpected response format received from server.', response.status)
    }

    if (payload.code !== 0) {
      throw new ApiBusinessError(payload.message || 'Request failed.', payload.code, response.status)
    }

    return payload.data as never
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    const status = error.response?.status
    const requestConfig = error.config as ApiRequestConfig | undefined

    if (
      status === 401
      && requestConfig
      && !requestConfig._retry
      && !requestConfig.skipAuthRefresh
      && !isAuthRoute(requestConfig.url)
    ) {
      requestConfig._retry = true

      try {
        const nextAccessToken = await refreshAccessTokenWithLock()
        const headers = requestConfig.headers instanceof AxiosHeaders
          ? requestConfig.headers.toJSON()
          : { ...(requestConfig.headers ?? {}) }
        headers.Authorization = `Bearer ${nextAccessToken}`
        requestConfig.headers = headers
        return httpClient.request(requestConfig)
      } catch (refreshError) {
        return Promise.reject(refreshError)
      }
    }

    if (status === 401 && requestConfig?.url === '/auth/refresh') {
      clearAuthTokens()
    }

    return Promise.reject(normalizeAxiosError(error))
  },
)

export const apiClient = {
  get<T>(url: string, config?: ApiRequestConfig) {
    return httpClient.get<ApiResponse<T>, T>(url, config)
  },
  post<T>(url: string, data?: unknown, config?: ApiRequestConfig) {
    return httpClient.post<ApiResponse<T>, T>(url, data, config)
  },
  put<T>(url: string, data?: unknown, config?: ApiRequestConfig) {
    return httpClient.put<ApiResponse<T>, T>(url, data, config)
  },
  delete<T>(url: string, config?: ApiRequestConfig) {
    return httpClient.delete<ApiResponse<T>, T>(url, config)
  },
}