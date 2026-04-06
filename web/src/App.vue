<template>
  <RouterView />
  <AppToast />
  <AppConfirm />
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { RouterView } from 'vue-router'
import { useAuthStore } from './stores/auth'
import AppToast from './components/AppToast.vue'
import AppConfirm from './components/AppConfirm.vue'

const authStore = useAuthStore()

onMounted(() => {
  void authStore.ensureInitialized()
})
</script>
