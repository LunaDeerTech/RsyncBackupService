<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
</script>

<template>
    <div class="grid gap-6 xl:grid-cols-[1.15fr_0.85fr]">
      <section class="relative overflow-hidden rounded-[32px] border border-outline bg-surface-base/92 p-7 shadow-panel backdrop-blur md:p-8">
        <div class="absolute inset-x-6 top-0 h-px bg-[linear-gradient(90deg,transparent,var(--primary-500),transparent)]"></div>
        <p class="text-xs font-semibold uppercase tracking-[0.34em] text-content-muted">Instance Workspace</p>
        <h2 class="mt-4 text-4xl font-semibold tracking-tight text-content-primary">实例视图已就绪</h2>
        <p class="mt-4 max-w-2xl text-base leading-8 text-content-secondary">
          {{ user?.name ?? '当前用户' }} 已通过认证。这里将承接实例列表、备份状态和恢复入口，也是 viewer 用户的默认落点。
        </p>

        <div class="mt-8 grid gap-4 md:grid-cols-2">
          <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Routing</p>
            <p class="mt-4 text-lg font-semibold text-content-primary">Protected by guard</p>
            <p class="mt-2 text-sm leading-6 text-content-secondary">Unauthenticated visits are redirected to the login page before render.</p>
          </article>
          <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Session</p>
            <p class="mt-4 text-lg font-semibold text-content-primary">Silent refresh enabled</p>
            <p class="mt-2 text-sm leading-6 text-content-secondary">Expired access tokens are refreshed transparently through the API client.</p>
          </article>
        </div>
      </section>

      <aside class="rounded-[32px] border border-outline bg-surface-base/92 p-7 shadow-panel backdrop-blur md:p-8">
        <p class="text-xs font-semibold uppercase tracking-[0.32em] text-content-muted">Ready for next phase</p>
        <ul class="mt-5 space-y-3 text-sm leading-6 text-content-secondary">
          <li>实例表格与筛选器</li>
          <li>健康检查与同步状态</li>
          <li>备份任务与恢复快捷入口</li>
        </ul>
      </aside>
    </div>
</template>