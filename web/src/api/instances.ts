import { apiClient } from './client'
import type { PaginatedData } from './types'
import type {
  Instance,
  InstanceListItem,
  InstanceDetail,
  InstanceStats,
  CreateInstanceRequest,
  UpdateInstanceRequest,
  PermissionItem,
} from '../types/instance'

export function listInstances(params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<InstanceListItem>>('/instances', { params })
}

export function createInstance(data: CreateInstanceRequest) {
  return apiClient.post<Instance>('/instances', data)
}

export function getInstance(id: number) {
  return apiClient.get<InstanceDetail>(`/instances/${id}`)
}

export function getInstanceStats(id: number) {
  return apiClient.get<InstanceStats>(`/instances/${id}/stats`)
}

export function updateInstance(id: number, data: UpdateInstanceRequest) {
  return apiClient.put<Instance>(`/instances/${id}`, data)
}

export function deleteInstance(id: number) {
  return apiClient.delete<void>(`/instances/${id}`)
}

export function updateInstancePermissions(id: number, permissions: PermissionItem[]) {
  return apiClient.put<void>(`/instances/${id}/permissions`, { permissions })
}
