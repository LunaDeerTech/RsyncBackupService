import { apiClient } from './client'
import type { PaginatedData } from './types'
import type { Backup } from '../types/instance'

export function listBackups(instanceId: number, params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<Backup>>(`/instances/${instanceId}/backups`, { params })
}

export function getBackup(instanceId: number, backupId: number) {
  return apiClient.get<Backup>(`/instances/${instanceId}/backups/${backupId}`)
}
