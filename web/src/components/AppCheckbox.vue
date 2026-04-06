<script setup lang="ts">
import { Check } from 'lucide-vue-next'

withDefaults(
  defineProps<{
    modelValue?: boolean
    label?: string
    disabled?: boolean
  }>(),
  {
    modelValue: false,
    label: '',
    disabled: false,
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()
</script>

<template>
  <label class="app-checkbox" :class="{ 'app-checkbox--disabled': disabled }">
    <span
      class="app-checkbox__box"
      :class="{ 'app-checkbox__box--checked': modelValue }"
      @click.prevent="!disabled && emit('update:modelValue', !modelValue)"
    >
      <Check v-if="modelValue" :size="14" class="app-checkbox__icon" />
    </span>
    <input
      type="checkbox"
      class="sr-only"
      :checked="modelValue"
      :disabled="disabled"
      @change="emit('update:modelValue', !modelValue)"
    />
    <span v-if="label" class="app-checkbox__label">{{ label }}</span>
  </label>
</template>

<style scoped>
.app-checkbox {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  user-select: none;
}
.app-checkbox--disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.app-checkbox__box {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  min-height: 18px;
  border: 1px solid var(--border-default);
  border-radius: 4px;
  background: var(--surface-raised);
  transition: all var(--transition-fast);
  flex-shrink: 0;
}
.app-checkbox__box--checked {
  background: var(--primary-500);
  border-color: var(--primary-600);
}
.app-checkbox__icon {
  color: #0a1628;
  display: block;
}
.app-checkbox__label {
  font-size: 14px;
  line-height: 20px;
  color: var(--text-primary);
}
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  border: 0;
}
</style>
