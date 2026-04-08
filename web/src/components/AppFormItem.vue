<script setup lang="ts">
withDefaults(
  defineProps<{
    label?: string
    required?: boolean
    error?: string
  }>(),
  {
    label: '',
    required: false,
    error: '',
  },
)
</script>

<template>
  <div class="app-form-item">
    <label v-if="label || $slots.label" class="app-form-item__label">
      <slot name="label">
        {{ label }}
      </slot>
      <span v-if="required" class="app-form-item__required">*</span>
    </label>
    <slot />
    <p v-if="error" class="app-form-item__error">{{ error }}</p>
  </div>
</template>

<style scoped>
.app-form-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.app-form-item__label {
  font-size: 13px;
  font-weight: 500;
  color: var(--text-secondary);
}
.app-form-item__required {
  color: var(--error-500);
  margin-left: 2px;
}
.app-form-item__error {
  margin: 0;
  font-size: 12px;
  color: var(--error-500);
}
</style>
