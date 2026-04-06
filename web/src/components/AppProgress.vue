<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value?: number
    variant?: 'primary' | 'success' | 'warning' | 'error'
    size?: 'sm' | 'md'
  }>(),
  {
    value: 0,
    variant: 'primary',
    size: 'md',
  },
)

const clampedValue = computed(() => Math.max(0, Math.min(100, props.value)))

const barColor = computed(() => {
  const colors: Record<string, string> = {
    primary: 'var(--primary-500)',
    success: 'var(--success-500)',
    warning: 'var(--warning-500)',
    error: 'var(--error-500)',
  }
  return colors[props.variant]
})

const barGradient = computed(() => {
  if (props.variant === 'primary') {
    return `linear-gradient(90deg, var(--primary-500), var(--accent-mint-400))`
  }
  return barColor.value
})
</script>

<template>
  <div class="app-progress" :class="`app-progress--${size}`" role="progressbar" :aria-valuenow="clampedValue" aria-valuemin="0" aria-valuemax="100">
    <div class="app-progress__track">
      <div
        class="app-progress__bar"
        :style="{ width: `${clampedValue}%`, background: barGradient }"
      />
    </div>
  </div>
</template>

<style scoped>
.app-progress__track {
  width: 100%;
  background: var(--surface-sunken);
  border-radius: 9999px;
  overflow: hidden;
}
.app-progress--sm .app-progress__track { height: 4px; }
.app-progress--md .app-progress__track { height: 8px; }
.app-progress__bar {
  height: 100%;
  border-radius: 9999px;
  transition: width 0.4s ease;
}
</style>
