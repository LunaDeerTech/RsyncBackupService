import { apiClient } from './client'
import type { PaginatedData } from './types'
import type { RemoteConfig } from '../types/remote'

export function listRemotes(params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<RemoteConfig>>('/remotes', { params })
}

export function createRemote(formData: FormData) {
  return apiClient.post<RemoteConfig>('/remotes', formData)
}

export function updateRemote(id: number, formData: FormData) {
  return apiClient.put<RemoteConfig>(`/remotes/${id}`, formData)
}

export function deleteRemote(id: number) {
  return apiClient.delete<void>(`/remotes/${id}`)
}

export function testRemoteConnection(id: number) {
  return apiClient.post<{ message: string }>(`/remotes/${id}/test`)
}
