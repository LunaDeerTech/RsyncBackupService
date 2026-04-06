import axios, { AxiosError, type AxiosRequestConfig } from 'axios'
import type { ApiResponse } from './types'
import { clearAccessToken, getAccessToken } from '../utils/storage'

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

const httpClient = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

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
  (error: AxiosError<ApiResponse<unknown>>) => {
    const status = error.response?.status

    if (status === 401) {
      clearAccessToken()
      if (typeof window !== 'undefined') {
        window.location.replace('/login')
      }
    }

    const payload = error.response?.data
    if (payload && typeof payload.code === 'number' && payload.code !== 0) {
      return Promise.reject(
        new ApiBusinessError(payload.message || 'Request failed.', payload.code, status),
      )
    }

    const message = error.code === 'ERR_NETWORK'
      ? 'Network error. Please confirm the backend service is reachable.'
      : error.message || 'Request failed due to an unexpected network error.'

    return Promise.reject(new ApiNetworkError(message, status))
  },
)

export const apiClient = {
  get<T>(url: string, config?: AxiosRequestConfig) {
    return httpClient.get<ApiResponse<T>, T>(url, config)
  },
  post<T>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return httpClient.post<ApiResponse<T>, T>(url, data, config)
  },
  put<T>(url: string, data?: unknown, config?: AxiosRequestConfig) {
    return httpClient.put<ApiResponse<T>, T>(url, data, config)
  },
  delete<T>(url: string, config?: AxiosRequestConfig) {
    return httpClient.delete<ApiResponse<T>, T>(url, config)
  },
}