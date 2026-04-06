import { ref } from 'vue'
import { getHealth, type HealthResponse } from '../api/health'

export function useHealthCheck() {
  const loading = ref(false)
  const error = ref('')
  const response = ref<HealthResponse | null>(null)
  const checkedAt = ref('')

  async function runHealthCheck() {
    loading.value = true
    error.value = ''

    try {
      response.value = await getHealth()
      checkedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    } catch (reason) {
      response.value = null
      error.value = reason instanceof Error ? reason.message : 'Unknown request error.'
    } finally {
      loading.value = false
    }
  }

  return {
    loading,
    error,
    response,
    checkedAt,
    runHealthCheck,
  }
}