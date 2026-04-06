import { apiClient } from './client'
import type { PaginatedData } from './types'
import type { BackupTarget, CreateTargetRequest, UpdateTargetRequest } from '../types/target'

export function listTargets(params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<BackupTarget>>('/targets', { params })
}

export function createTarget(data: CreateTargetRequest) {
  return apiClient.post<BackupTarget>('/targets', data)
}

export function updateTarget(id: number, data: UpdateTargetRequest) {
  return apiClient.put<BackupTarget>(`/targets/${id}`, data)
}

export function deleteTarget(id: number) {
  return apiClient.delete<void>(`/targets/${id}`)
}

export function healthCheck(id: number) {
  return apiClient.post<BackupTarget>(`/targets/${id}/health-check`)
}
