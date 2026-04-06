<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    type?: 'text' | 'password' | 'number' | 'email'
    modelValue?: string | number
    placeholder?: string
    disabled?: boolean
    error?: string
  }>(),
  {
    type: 'text',
    modelValue: '',
    placeholder: '',
    disabled: false,
    error: '',
  },
)

const emit = defineEmits<{
  'update:modelValue': [value: string | number]
}>()

function onInput(e: Event) {
  const target = e.target as HTMLInputElement
  emit('update:modelValue', props.type === 'number' ? Number(target.value) : target.value)
}
</script>

<template>
  <div class="app-input-wrapper">
    <input
      class="app-input"
      :class="{ 'app-input--error': !!error }"
      :type="type"
      :value="modelValue"
      :placeholder="placeholder"
      :disabled="disabled"
      @input="onInput"
    />
    <p v-if="error" class="app-input__error">{{ error }}</p>
  </div>
</template>

<style scoped>
.app-input-wrapper {
  display: flex;
  flex-direction: column;
}
.app-input {
  width: 100%;
  padding: 8px 12px;
  font-size: 14px;
  line-height: 20px;
  color: var(--text-primary);
  background: var(--surface-raised);
  border: 1px solid var(--border-default);
  border-radius: var(--radius-md);
  transition: border-color var(--transition-fast), box-shadow var(--transition-fast);
  outline: none;
}
.app-input::placeholder {
  color: var(--text-muted);
}
.app-input:focus {
  border-color: var(--border-focus);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--primary-500) 25%, transparent);
}
.app-input:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}
.app-input--error {
  border-color: var(--error-500);
}
.app-input--error:focus {
  border-color: var(--error-500);
  box-shadow: 0 0 0 3px color-mix(in srgb, var(--error-500) 25%, transparent);
}
.app-input__error {
  margin: 4px 0 0;
  font-size: 12px;
  color: var(--error-500);
}
</style>
