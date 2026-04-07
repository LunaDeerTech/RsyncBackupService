import { apiClient } from './client'
import type { Policy, CreatePolicyRequest, UpdatePolicyRequest } from '../types/policy'

export function listPolicies(instanceId: number) {
  return apiClient.get<{ items: Policy[] }>(`/instances/${instanceId}/policies`)
}

export function createPolicy(instanceId: number, data: CreatePolicyRequest) {
  return apiClient.post<Policy>(`/instances/${instanceId}/policies`, data)
}

export function updatePolicy(instanceId: number, policyId: number, data: UpdatePolicyRequest) {
  return apiClient.put<Policy>(`/instances/${instanceId}/policies/${policyId}`, data)
}

export function deletePolicy(instanceId: number, policyId: number) {
  return apiClient.delete<void>(`/instances/${instanceId}/policies/${policyId}`)
}

export function triggerPolicy(instanceId: number, policyId: number) {
  return apiClient.post<void>(`/instances/${instanceId}/policies/${policyId}/trigger`)
}
