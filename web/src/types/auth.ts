export interface User {
  id: number
  email: string
  name: string
  role: 'admin' | 'viewer' | string
  created_at: string
  updated_at: string
}

export interface LoginResponse {
  access_token: string
  refresh_token: string
  user: User
}

export interface RefreshTokenResponse {
  access_token: string
}

export interface RegistrationStatus {
  enabled: boolean
}