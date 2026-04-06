<script setup lang="ts">
import { useRoute } from 'vue-router'
import { Construction } from 'lucide-vue-next'

const route = useRoute()
</script>

<template>
  <div class="flex flex-col items-center justify-center py-20 text-center">
    <div class="mb-6 inline-flex h-16 w-16 items-center justify-center rounded-2xl bg-primary-500/10">
      <Construction :size="32" class="text-primary-500" />
    </div>
    <h2 class="text-2xl font-semibold text-content-primary">{{ (route.meta.title as string) ?? '页面' }}即将上线</h2>
    <p class="mt-3 max-w-md text-sm leading-6 text-content-secondary">
      该功能正在开发中，敬请期待。
    </p>
  </div>
</template>