<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useAuthStore } from '../stores/auth'

const authStore = useAuthStore()
const { user } = storeToRefs(authStore)
</script>

<template>
    <div class="grid gap-6 xl:grid-cols-[1.1fr_0.9fr]">
      <section class="relative overflow-hidden rounded-[32px] border border-outline bg-surface-base/92 p-7 shadow-panel backdrop-blur md:p-8">
        <div class="absolute inset-x-6 top-0 h-px bg-[linear-gradient(90deg,transparent,var(--primary-500),transparent)]"></div>
        <p class="text-xs font-semibold uppercase tracking-[0.34em] text-content-muted">Admin Workspace</p>
        <h2 class="mt-4 text-4xl font-semibold tracking-tight text-content-primary">欢迎回来，{{ user?.name ?? '管理员' }}</h2>
        <p class="mt-4 max-w-2xl text-base leading-8 text-content-secondary">
          认证链路已经接入，当前页面作为管理员落点，后续可直接承接系统总览、任务趋势与告警概览。
        </p>

        <div class="mt-8 grid gap-4 md:grid-cols-3">
          <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Auth</p>
            <p class="mt-4 text-lg font-semibold text-content-primary">Session restored</p>
            <p class="mt-2 text-sm leading-6 text-content-secondary">Access and refresh tokens persist across reloads.</p>
          </article>
          <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Guard</p>
            <p class="mt-4 text-lg font-semibold text-content-primary">Admin-only route</p>
            <p class="mt-2 text-sm leading-6 text-content-secondary">Viewer accounts are redirected to the instances workspace.</p>
          </article>
          <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Next</p>
            <p class="mt-4 text-lg font-semibold text-content-primary">Dashboard widgets</p>
            <p class="mt-2 text-sm leading-6 text-content-secondary">This surface is ready for metric cards and health summaries.</p>
          </article>
        </div>
      </section>

      <aside class="rounded-[32px] border border-outline bg-surface-base/92 p-7 shadow-panel backdrop-blur md:p-8">
        <p class="text-xs font-semibold uppercase tracking-[0.32em] text-content-muted">Signed-in context</p>
        <div class="mt-6 space-y-4">
          <div class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-sm text-content-muted">邮箱</p>
            <p class="mt-2 text-lg font-semibold text-content-primary">{{ user?.email ?? '-' }}</p>
          </div>
          <div class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
            <p class="text-sm text-content-muted">角色</p>
            <p class="mt-2 text-lg font-semibold uppercase tracking-[0.18em] text-content-primary">{{ user?.role ?? '-' }}</p>
          </div>
        </div>
      </aside>
    </div>
</template>