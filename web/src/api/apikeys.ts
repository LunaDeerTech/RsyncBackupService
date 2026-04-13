import { apiClient } from './client'

export interface APIKeyItem {
  id: number
  name: string
  key_prefix: string
  last_used_at?: string
  created_at: string
}

export interface APIKeyListResponse {
  items: APIKeyItem[]
}

export interface APIKeyCreateResponse {
  api_key: APIKeyItem
  key: string
}

export function listAPIKeys() {
  return apiClient.get<APIKeyListResponse>('/users/me/api-keys')
}

export function createAPIKey(data: { name: string }) {
  return apiClient.post<APIKeyCreateResponse>('/users/me/api-keys', data)
}

export function deleteAPIKey(id: number) {
  return apiClient.delete<{ message: string }>(`/users/me/api-keys/${id}`)
}