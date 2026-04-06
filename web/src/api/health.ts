import { apiClient } from './client'

export interface HealthResponse {
  status: string
}

export function getHealth() {
  return apiClient.get<HealthResponse>('/health')
}