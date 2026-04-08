import { apiClient } from './client'

export interface TaskItem {
  id: number
  instance_id: number
  instance_name: string
  backup_id?: number
  type: string
  status: string
  progress: number
  current_step: string
  started_at?: string
  completed_at?: string
  estimated_end?: string
  error_message?: string
  created_at: string
}

export function listTasks() {
  return apiClient.get<{ items: TaskItem[] }>('/tasks')
}

export function getTask(taskId: number) {
  return apiClient.get<TaskItem>(`/tasks/${taskId}`)
}

export function cancelTask(taskId: number) {
  return apiClient.post<{ message: string; task: TaskItem }>(`/tasks/${taskId}/cancel`, {})
}
