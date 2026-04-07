import { apiClient } from './client'
import type { User } from '../types/auth'

export function listUsers(params?: { page?: number; page_size?: number }) {
  return apiClient.get<{ items: User[]; total: number }>('/users', { params })
}
