import { defineStore } from 'pinia'
import { ref } from 'vue'

export interface ToastItem {
  id: number
  type: 'success' | 'error' | 'warning' | 'info'
  message: string
}

let nextId = 0

export const useToastStore = defineStore('toast', () => {
  const items = ref<ToastItem[]>([])

  function add(type: ToastItem['type'], message: string, duration = 4000) {
    const id = ++nextId
    items.value.push({ id, type, message })
    setTimeout(() => remove(id), duration)
  }

  function remove(id: number) {
    items.value = items.value.filter((t) => t.id !== id)
  }

  function success(message: string) { add('success', message) }
  function error(message: string) { add('error', message) }
  function warning(message: string) { add('warning', message) }
  function info(message: string) { add('info', message) }

  return { items, success, error, warning, info, remove }
})
