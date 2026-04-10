import { apiClient } from './client'
import type { PaginatedData } from './types'

export interface DashboardTargetHealthSummary {
  healthy: number
  degraded: number
  unreachable: number
}

export interface DashboardOverview {
  running_tasks: number
  queued_tasks: number
  abnormal_instances: number
  unresolved_risks: number
  system_dr_score: number
  system_dr_level: string
  target_health_summary: DashboardTargetHealthSummary
  total_instances: number
  total_backups: number
}

export interface DashboardInstanceHealth {
  safe: number
  caution: number
  risk: number
  danger: number
}

export interface DailyBackupResult {
  date: string
  success: number
  failed: number
}

export interface DashboardTrends {
  backup_results: DailyBackupResult[]
  instance_health: DashboardInstanceHealth
}

export interface FocusInstance {
  id: number
  name: string
  dr_score: number
  dr_level: string
  unresolved_risks: number
  last_backup_time: string | null
  last_backup_status: string
}

export interface DashboardRiskEvent {
  id: number
  instance_id?: number
  instance_name: string
  target_id?: number
  target_name: string
  severity: string
  source: string
  message: string
  resolved: boolean
  created_at: string
  resolved_at?: string
}

export interface UpcomingTask {
  policy_id: number
  policy_name: string
  instance_id: number
  instance_name: string
  type: string
  next_run_at: string
}

export function getOverview() {
  return apiClient.get<DashboardOverview>('/dashboard/overview')
}

export function getRisks(params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<DashboardRiskEvent>>('/dashboard/risks', { params })
}

export function getTrends() {
  return apiClient.get<DashboardTrends>('/dashboard/trends')
}

export function getFocusInstances() {
  return apiClient.get<FocusInstance[]>('/dashboard/focus-instances')
}

export function getUpcomingTasks(params?: { within_hours?: number }) {
  return apiClient.get<{ items: UpcomingTask[] }>('/dashboard/upcoming-tasks', { params })
}
