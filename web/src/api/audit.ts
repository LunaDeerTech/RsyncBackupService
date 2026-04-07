import { apiClient } from './client'
import type { PaginatedData } from './types'

export interface AuditLog {
  id: number
  instance_id?: number
  user_id?: number
  user_name: string
  user_email: string
  action: string
  detail: Record<string, any>
  created_at: string
}

export interface AuditLogParams {
  page?: number
  page_size?: number
  start_date?: string
  end_date?: string
  action?: string
}

export function listAuditLogs(params?: AuditLogParams) {
  return apiClient.get<PaginatedData<AuditLog>>('/audit-logs', { params })
}

export function listInstanceAuditLogs(instanceId: number, params?: AuditLogParams) {
  return apiClient.get<PaginatedData<AuditLog>>(`/instances/${instanceId}/audit-logs`, { params })
}
