export interface BackupTarget {
  id: number
  name: string
  backup_type: 'rolling' | 'cold'
  storage_type: 'local' | 'ssh' | 'cloud'
  storage_path: string
  remote_config_id?: number
  total_capacity_bytes?: number
  used_capacity_bytes?: number
  last_health_check?: string
  health_status: 'healthy' | 'degraded' | 'unreachable'
  health_message: string
  created_at: string
  updated_at: string
}

export interface CreateTargetRequest {
  name: string
  backup_type: 'rolling' | 'cold'
  storage_type: 'local' | 'ssh' | 'cloud'
  storage_path: string
  remote_config_id?: number
}

export interface UpdateTargetRequest {
  name: string
  backup_type: 'rolling' | 'cold'
  storage_type: 'local' | 'ssh' | 'cloud'
  storage_path: string
  remote_config_id?: number
}
