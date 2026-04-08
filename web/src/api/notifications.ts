import { apiClient } from './client'

export interface SubscriptionItem {
  id: number
  user_id: number
  instance_id: number
  instance_name: string
  enabled: boolean
  created_at: string
}

export function getMySubscriptions() {
  return apiClient.get<{ subscriptions: SubscriptionItem[] }>('/users/me/subscriptions')
}

export function updateMySubscriptions(subscriptions: { instance_id: number; enabled: boolean }[]) {
  return apiClient.put<{ subscriptions: SubscriptionItem[] }>('/users/me/subscriptions', { subscriptions })
}
