import { apiClient } from './client'
import type { PaginatedData } from './types'
import type { Backup } from '../types/instance'

export function listBackups(instanceId: number, params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<Backup>>(`/instances/${instanceId}/backups`, { params })
}

export function getBackup(instanceId: number, backupId: number) {
  return apiClient.get<Backup>(`/instances/${instanceId}/backups/${backupId}`)
}

export interface RestoreRequest {
  restore_type: 'source' | 'custom'
  target_path?: string
  remote_config_id?: number
  instance_name: string
  password: string
  encryption_key?: string
}

export interface BackupDownloadPart {
  index: number
  name: string
  url: string
  size_bytes?: number
}

export interface SingleBackupDownloadResponse {
  mode: 'single'
  url: string
  file_name?: string
}

export interface SplitBackupDownloadResponse {
  mode: 'split'
  file_name?: string
  parts: BackupDownloadPart[]
}

export type BackupDownloadResponse = SingleBackupDownloadResponse | SplitBackupDownloadResponse

export function restoreBackup(instanceId: number, backupId: number, data: RestoreRequest) {
  return apiClient.post<void>(`/instances/${instanceId}/backups/${backupId}/restore`, data)
}

export function downloadBackup(instanceId: number, backupId: number) {
  return apiClient.get<BackupDownloadResponse>(`/instances/${instanceId}/backups/${backupId}/download`)
}
