import { apiClient } from './client'
import type { PaginatedData } from './types'

export interface RiskEvent {
  id: number
  instance_id?: number
  instance_name?: string
  target_id?: number
  target_name?: string
  severity: 'info' | 'warning' | 'critical'
  source: string
  message: string
  resolved: boolean
  created_at: string
  resolved_at?: string
}

export interface RiskEventParams {
  page?: number
  page_size?: number
  severity?: string
  source?: string
  resolved?: boolean
}

export function listRiskEvents(params?: RiskEventParams) {
  return apiClient.get<PaginatedData<RiskEvent>>('/dashboard/risks', { params })
}
