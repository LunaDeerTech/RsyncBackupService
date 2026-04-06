<script setup lang="ts">
import { watch, onMounted, onBeforeUnmount } from 'vue'
import { X } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    visible: boolean
    title?: string
    width?: string
    closeOnOverlay?: boolean
  }>(),
  {
    title: '',
    width: '480px',
    closeOnOverlay: true,
  },
)

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

function close() {
  emit('update:visible', false)
}

function onOverlayClick() {
  if (props.closeOnOverlay) close()
}

function onKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.visible) close()
}

onMounted(() => document.addEventListener('keydown', onKeyDown))
onBeforeUnmount(() => document.removeEventListener('keydown', onKeyDown))

watch(
  () => props.visible,
  (v) => {
    document.body.style.overflow = v ? 'hidden' : ''
  },
)
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="visible" class="app-modal-overlay" @click.self="onOverlayClick">
        <div class="app-modal" :style="{ maxWidth: width }" role="dialog" aria-modal="true">
          <div class="app-modal__header">
            <h3 class="app-modal__title">{{ title }}</h3>
            <button type="button" class="app-modal__close" @click="close">
              <X :size="18" />
            </button>
          </div>
          <div class="app-modal__body">
            <slot />
          </div>
          <div v-if="$slots.footer" class="app-modal__footer">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.app-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.45);
  backdrop-filter: blur(4px);
  padding: 16px;
}
.app-modal {
  width: 100%;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-xl);
  box-shadow: var(--shadow-lg);
  overflow: hidden;
}
.app-modal__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
}
.app-modal__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}
.app-modal__close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  transition: all var(--transition-fast);
}
.app-modal__close:hover {
  background: var(--surface-sunken);
  color: var(--text-primary);
}
.app-modal__body {
  padding: 20px;
}
.app-modal__footer {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  padding: 12px 20px;
  border-top: 1px solid var(--border-subtle);
}

/* Transitions */
.modal-enter-active { transition: opacity 200ms ease, transform 200ms ease; }
.modal-leave-active { transition: opacity 150ms ease, transform 150ms ease; }
.modal-enter-from { opacity: 0; }
.modal-enter-from .app-modal { transform: scale(0.95) translateY(8px); }
.modal-leave-to { opacity: 0; }
.modal-leave-to .app-modal { transform: scale(0.95) translateY(8px); }
/* Fix: transition on overlay, modal gets transform via nested selector */
.modal-enter-active .app-modal,
.modal-leave-active .app-modal {
  transition: transform 200ms ease;
}
</style>
