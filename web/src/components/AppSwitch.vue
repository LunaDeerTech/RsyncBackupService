<script setup lang="ts">
withDefaults(
  defineProps<{
    modelValue?: boolean
    disabled?: boolean
  }>(),
  {
    modelValue: false,
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()
</script>

<template>
  <button
    type="button"
    role="switch"
    :aria-checked="modelValue"
    class="app-switch"
    :class="{ 'app-switch--on': modelValue }"
    :disabled="disabled"
    @click="emit('update:modelValue', !modelValue)"
  >
    <span class="app-switch__thumb" />
  </button>
</template>

<style scoped>
.app-switch {
  position: relative;
  display: inline-flex;
  align-items: center;
  width: 40px;
  height: 22px;
  border-radius: 11px;
  border: 1px solid var(--border-default);
  background: var(--surface-sunken);
  cursor: pointer;
  transition: background var(--transition-fast), border-color var(--transition-fast);
  padding: 0;
  flex-shrink: 0;
  vertical-align: middle;
}
.app-switch:focus-visible {
  outline: none;
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-500) 40%, transparent);
}
.app-switch:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.app-switch--on {
  background: var(--primary-500);
  border-color: var(--primary-600);
}
.app-switch__thumb {
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  transition: transform var(--transition-fast);
  box-shadow: 0 1px 2px rgba(0, 0, 0, 0.15);
}
.app-switch--on .app-switch__thumb {
  transform: translateX(18px);
}
</style>
