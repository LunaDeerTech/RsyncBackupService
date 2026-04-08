export interface Instance {
  id: number
  name: string
  source_type: 'local' | 'ssh'
  source_path: string
  exclude_patterns?: string[]
  remote_config_id?: number
  status: 'idle' | 'running'
  created_at: string
  updated_at: string
}

export interface InstanceListItem extends Instance {
  last_backup_status?: 'success' | 'failed'
  last_backup_time?: string
  backup_count: number
  dr_score?: number
  dr_level?: string
}

export interface DisasterRecoveryScore {
  total: number
  level: 'safe' | 'caution' | 'risk' | 'danger'
  freshness: number
  recovery_points: number
  redundancy: number
  stability: number
  deductions: string[]
  calculated_at: string
}

export interface CreateInstanceRequest {
  name: string
  source_type: 'local' | 'ssh'
  source_path: string
  exclude_patterns?: string[]
  remote_config_id?: number
}

export interface UpdateInstanceRequest {
  name: string
  source_type: 'local' | 'ssh'
  source_path: string
  exclude_patterns?: string[]
  remote_config_id?: number
}

export interface Backup {
  id: number
  instance_id: number
  policy_id: number
  type: 'rolling' | 'cold'
  status: string
  snapshot_path: string
  backup_size_bytes: number
  actual_size_bytes: number
  started_at?: string
  completed_at?: string
  duration_seconds: number
  error_message: string
  rsync_stats: string
  created_at: string
}

export interface BackupTrendPoint {
  date: string
  count: number
  success_count: number
  failure_count: number
}

export interface InstanceStats {
  backup_count: number
  success_backup_count: number
  failure_backup_count: number
  total_backup_size_bytes: number
  policy_count: number
  last_backup?: Backup
  recent_trend: BackupTrendPoint[]
}

export interface InstanceDetail {
  instance: Instance
  stats: InstanceStats
}

export interface PermissionItem {
  user_id: number
  permission: string
}
