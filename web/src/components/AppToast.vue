<script setup lang="ts">
import { CheckCircle, XCircle, AlertTriangle, Info, X } from 'lucide-vue-next'
import { useToastStore } from '../stores/toast'

const toastStore = useToastStore()

const iconMap = {
  success: CheckCircle,
  error: XCircle,
  warning: AlertTriangle,
  info: Info,
}
</script>

<template>
  <Teleport to="body">
    <div class="app-toast-container">
      <TransitionGroup name="toast">
        <div
          v-for="item in toastStore.items"
          :key="item.id"
          class="app-toast"
          :class="`app-toast--${item.type}`"
        >
          <component :is="iconMap[item.type]" :size="18" class="app-toast__icon" />
          <span class="app-toast__message">{{ item.message }}</span>
          <button type="button" class="app-toast__close" @click="toastStore.remove(item.id)">
            <X :size="14" />
          </button>
        </div>
      </TransitionGroup>
    </div>
  </Teleport>
</template>

<style scoped>
.app-toast-container {
  position: fixed;
  top: 16px;
  right: 16px;
  z-index: 2000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  pointer-events: none;
  max-width: 380px;
}
.app-toast {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 14px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border-default);
  background: var(--surface-raised);
  box-shadow: var(--shadow-md);
  color: var(--text-primary);
  font-size: 14px;
  pointer-events: auto;
}
.app-toast__icon { flex-shrink: 0; }
.app-toast--success .app-toast__icon { color: var(--success-500); }
.app-toast--error .app-toast__icon { color: var(--error-500); }
.app-toast--warning .app-toast__icon { color: var(--warning-500); }
.app-toast--info .app-toast__icon { color: var(--primary-500); }
.app-toast__message { flex: 1; line-height: 1.4; }
.app-toast__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  flex-shrink: 0;
  transition: all var(--transition-fast);
}
.app-toast__close:hover {
  background: var(--surface-sunken);
  color: var(--text-primary);
}

/* Transitions */
.toast-enter-active { transition: all 250ms ease; }
.toast-leave-active { transition: all 200ms ease; }
.toast-enter-from { opacity: 0; transform: translateX(30px); }
.toast-leave-to { opacity: 0; transform: translateX(30px); }
.toast-move { transition: transform 250ms ease; }
</style>
