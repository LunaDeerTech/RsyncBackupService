<script setup lang="ts">
import { Loader2 } from 'lucide-vue-next'

const props = withDefaults(
  defineProps<{
    variant?: 'primary' | 'outline' | 'danger' | 'ghost'
    size?: 'sm' | 'md' | 'lg'
    disabled?: boolean
    loading?: boolean
  }>(),
  {
    variant: 'primary',
    size: 'md',
    disabled: false,
    loading: false,
  },
)

const emit = defineEmits<{ click: [e: MouseEvent] }>()

function handleClick(e: MouseEvent) {
  if (!props.disabled && !props.loading) {
    emit('click', e)
  }
}
</script>

<template>
  <button
    type="button"
    class="btn"
    :class="[`btn--${variant}`, `btn--${size}`]"
    :disabled="disabled || loading"
    @click="handleClick"
  >
    <Loader2 v-if="loading" class="btn__spinner" :size="size === 'sm' ? 14 : 16" />
    <slot />
  </button>
</template>

<style scoped>
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  font-weight: 600;
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  cursor: pointer;
  transition: all var(--transition-fast);
  white-space: nowrap;
  user-select: none;
}
.btn:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-500) 40%, transparent);
}
.btn:disabled {
  cursor: not-allowed;
  opacity: 0.55;
}

/* Sizes */
.btn--sm { padding: 6px 14px; font-size: 13px; line-height: 20px; }
.btn--md { padding: 8px 20px; font-size: 14px; line-height: 20px; }
.btn--lg { padding: 10px 28px; font-size: 15px; line-height: 24px; }

/* Primary */
.btn--primary {
  background: linear-gradient(135deg, var(--primary-500), var(--primary-600));
  color: #0a1628;
  box-shadow: 0 1px 3px color-mix(in srgb, var(--primary-500) 30%, transparent);
}
.btn--primary:hover:not(:disabled) {
  background: linear-gradient(135deg, var(--primary-600), var(--primary-500));
  box-shadow: 0 2px 8px color-mix(in srgb, var(--primary-500) 40%, transparent);
}

/* Outline */
.btn--outline {
  border-color: color-mix(in srgb, var(--primary-500) 50%, var(--border-default));
  background: color-mix(in srgb, var(--primary-500) 6%, var(--surface-raised));
  color: var(--primary-500);
  box-shadow: var(--shadow-sm);
}
.btn--outline:hover:not(:disabled) {
  border-color: var(--primary-500);
  color: var(--primary-600);
  background: color-mix(in srgb, var(--primary-500) 12%, var(--surface-raised));
  box-shadow: 0 2px 6px color-mix(in srgb, var(--primary-500) 20%, transparent);
}

/* Danger */
.btn--danger {
  background: var(--error-500);
  color: #fff;
  box-shadow: 0 1px 3px color-mix(in srgb, var(--error-500) 30%, transparent);
}
.btn--danger:hover:not(:disabled) {
  filter: brightness(1.1);
  box-shadow: 0 2px 8px color-mix(in srgb, var(--error-500) 40%, transparent);
}

/* Ghost */
.btn--ghost {
  background: transparent;
  color: var(--text-secondary);
}
.btn--ghost:hover:not(:disabled) {
  background: var(--surface-sunken);
  color: var(--text-primary);
}

/* Spinner animation */
.btn__spinner {
  animation: spin 0.8s linear infinite;
}
@keyframes spin {
  to { transform: rotate(360deg); }
}
</style>
