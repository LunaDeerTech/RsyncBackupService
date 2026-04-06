import { ref } from 'vue'

export interface ConfirmOptions {
  title?: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
}

interface ConfirmState extends ConfirmOptions {
  resolve: (value: boolean) => void
}

const state = ref<ConfirmState | null>(null)

export function useConfirm() {
  function confirm(options: ConfirmOptions): Promise<boolean> {
    return new Promise((resolve) => {
      state.value = { ...options, resolve }
    })
  }

  function handleConfirm() {
    state.value?.resolve(true)
    state.value = null
  }

  function handleCancel() {
    state.value?.resolve(false)
    state.value = null
  }

  return { state, confirm, handleConfirm, handleCancel }
}
