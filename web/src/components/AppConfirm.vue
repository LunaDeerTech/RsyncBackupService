<script setup lang="ts">
import AppModal from './AppModal.vue'
import AppButton from './AppButton.vue'
import { useConfirm } from '../composables/useConfirm'

const { state, handleConfirm, handleCancel } = useConfirm()
</script>

<template>
  <AppModal
    :visible="!!state"
    :title="state?.title ?? '确认'"
    width="420px"
    :close-on-overlay="false"
    @update:visible="(v: boolean) => { if (!v) handleCancel() }"
  >
    <p class="app-confirm__message">{{ state?.message }}</p>
    <template #footer>
      <AppButton variant="outline" size="md" @click="handleCancel">
        {{ state?.cancelText ?? '取消' }}
      </AppButton>
      <AppButton
        :variant="state?.danger ? 'danger' : 'primary'"
        size="md"
        @click="handleConfirm"
      >
        {{ state?.confirmText ?? '确认' }}
      </AppButton>
    </template>
  </AppModal>
</template>

<style scoped>
.app-confirm__message {
  margin: 0;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text-secondary);
}
</style>
