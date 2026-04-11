export interface Policy {
  id: number
  instance_id: number
  name: string
  type: 'rolling' | 'cold'
  target_id: number
  schedule_type: 'interval' | 'cron'
  schedule_value: string
  enabled: boolean
  compression: boolean
  encryption: boolean
  split_enabled: boolean
  split_size_mb?: number
  retry_enabled: boolean
  retry_max_retries: number
  retention_type: 'time' | 'count'
  retention_value: number
  created_at: string
  updated_at: string
  last_execution_time?: string
  last_execution_status?: string
  latest_backup_id?: number
}

export interface CreatePolicyRequest {
  name: string
  type: 'rolling' | 'cold'
  target_id: number
  schedule_type: 'interval' | 'cron'
  schedule_value: string
  enabled: boolean
  compression: boolean
  encryption: boolean
  encryption_key?: string
  split_enabled: boolean
  split_size_mb?: number
  retry_enabled: boolean
  retry_max_retries: number
  retention_type: 'time' | 'count'
  retention_value: number
}

export interface UpdatePolicyRequest extends CreatePolicyRequest {}
