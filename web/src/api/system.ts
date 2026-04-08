import { apiClient } from './client'

export interface SmtpConfig {
  host: string
  port: number
  username: string
  password: string
  from: string
  encryption: string
}

export interface RegistrationStatus {
  enabled: boolean
}

export function getSmtpConfig() {
  return apiClient.get<SmtpConfig>('/system/smtp')
}

export function updateSmtpConfig(data: SmtpConfig) {
  return apiClient.put<SmtpConfig>('/system/smtp', data)
}

export function testSmtp(to: string) {
  return apiClient.post<{ message: string }>('/system/smtp/test', { to }, { timeout: 30000 })
}

export function getRegistrationStatus() {
  return apiClient.get<RegistrationStatus>('/system/registration')
}

export function updateRegistrationStatus(enabled: boolean) {
  return apiClient.put<RegistrationStatus>('/system/registration', { enabled })
}
