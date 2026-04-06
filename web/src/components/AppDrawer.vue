<script setup lang="ts">
import { watch, onMounted, onBeforeUnmount } from 'vue'
import { X } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    visible: boolean
    title?: string
    side?: 'left' | 'right'
    width?: string
  }>(),
  {
    title: '',
    side: 'right',
    width: '400px',
  },
)

const emit = defineEmits<{
  'update:visible': [value: boolean]
}>()

function close() {
  emit('update:visible', false)
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
    <Transition name="drawer-fade">
      <div v-if="visible" class="app-drawer-overlay" @click.self="close">
        <Transition :name="side === 'right' ? 'drawer-slide-right' : 'drawer-slide-left'">
          <div
            v-if="visible"
            class="app-drawer"
            :class="`app-drawer--${side}`"
            :style="{ width }"
          >
            <div class="app-drawer__header">
              <h3 class="app-drawer__title">{{ title }}</h3>
              <button type="button" class="app-drawer__close" @click="close">
                <X :size="18" />
              </button>
            </div>
            <div class="app-drawer__body">
              <slot />
            </div>
          </div>
        </Transition>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.app-drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 1000;
  background: rgba(0, 0, 0, 0.4);
  backdrop-filter: blur(4px);
}
.app-drawer {
  position: fixed;
  top: 0;
  bottom: 0;
  max-width: 100vw;
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  box-shadow: var(--shadow-lg);
  display: flex;
  flex-direction: column;
}
.app-drawer--right { right: 0; }
.app-drawer--left { left: 0; }
.app-drawer__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  flex-shrink: 0;
}
.app-drawer__title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
  color: var(--text-primary);
}
.app-drawer__close {
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
.app-drawer__close:hover {
  background: var(--surface-sunken);
  color: var(--text-primary);
}
.app-drawer__body {
  flex: 1;
  overflow-y: auto;
  padding: 20px;
}

/* Overlay fade */
.drawer-fade-enter-active { transition: opacity 200ms ease; }
.drawer-fade-leave-active { transition: opacity 150ms ease; }
.drawer-fade-enter-from, .drawer-fade-leave-to { opacity: 0; }

/* Slide right */
.drawer-slide-right-enter-active { transition: transform 250ms ease; }
.drawer-slide-right-leave-active { transition: transform 200ms ease; }
.drawer-slide-right-enter-from { transform: translateX(100%); }
.drawer-slide-right-leave-to { transform: translateX(100%); }

/* Slide left */
.drawer-slide-left-enter-active { transition: transform 250ms ease; }
.drawer-slide-left-leave-active { transition: transform 200ms ease; }
.drawer-slide-left-enter-from { transform: translateX(-100%); }
.drawer-slide-left-leave-to { transform: translateX(-100%); }
</style>
