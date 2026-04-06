import { apiClient } from './client'
import type { LoginResponse, RefreshTokenResponse, RegistrationStatus, User } from '../types/auth'

export function login(email: string, password: string) {
  return apiClient.post<LoginResponse>('/auth/login', {
    email,
    password,
  })
}

export async function register(email: string) {
  await apiClient.post<{ message: string }>('/auth/register', { email })
}

export function refreshToken(refreshTokenValue: string) {
  return apiClient.post<RefreshTokenResponse>('/auth/refresh', {
    refresh_token: refreshTokenValue,
  })
}

export function getMe() {
  return apiClient.get<User>('/users/me')
}

export function getRegistrationStatus() {
  return apiClient.get<RegistrationStatus>('/system/registration')
}