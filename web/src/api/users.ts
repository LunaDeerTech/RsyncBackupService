import { apiClient } from './client'
import type { PaginatedData } from './types'
import type { User } from '../types/auth'

export function listUsers(params?: { page?: number; page_size?: number }) {
  return apiClient.get<PaginatedData<User>>('/users', { params })
}

export function createUser(data: { email: string; name?: string; role: string }) {
  return apiClient.post<User>('/users', data)
}

export function updateUser(id: number, data: { name?: string; role?: string }) {
  return apiClient.put<User>(`/users/${id}`, data)
}

export function deleteUser(id: number) {
  return apiClient.delete<void>(`/users/${id}`)
}

export function updateCurrentUserProfile(data: { name: string }) {
  return apiClient.put<User>('/users/me/profile', data)
}

export function updateCurrentUserPassword(data: { old_password: string; new_password: string }) {
  return apiClient.put<{ message: string }>('/users/me/password', data)
}
