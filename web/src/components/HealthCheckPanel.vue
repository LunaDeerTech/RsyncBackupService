<script setup lang="ts">
import { computed } from 'vue'
import { useHealthCheck } from '../composables/useHealthCheck'

const { loading, error, response, checkedAt, runHealthCheck } = useHealthCheck()

const statusTone = computed(() => {
  if (error.value) {
    return 'text-error-500'
  }

  if (response.value?.status === 'healthy') {
    return 'text-success-500'
  }

  return 'text-content-secondary'
})
</script>

<template>
  <section class="rounded-[28px] border border-outline bg-surface-base/90 p-6 shadow-panel backdrop-blur md:p-7">
    <div class="flex flex-col gap-5 lg:flex-row lg:items-end lg:justify-between">
      <div class="space-y-3">
        <p class="text-xs font-semibold uppercase tracking-[0.32em] text-content-muted">API handshake</p>
        <div class="space-y-2">
          <h2 class="text-2xl font-semibold text-content-primary">Health endpoint validation</h2>
          <p class="max-w-2xl text-sm leading-6 text-content-secondary">
            This panel calls <span class="font-mono text-content-primary">/api/v1/health</span> through the Vite proxy so the frontend can verify the backend contract early.
          </p>
        </div>
      </div>

      <button
        type="button"
        class="inline-flex items-center justify-center rounded-full bg-primary-500 px-5 py-3 text-sm font-semibold text-slate-950 transition hover:bg-primary-600 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary-500/40 disabled:cursor-not-allowed disabled:opacity-60"
        :disabled="loading"
        @click="runHealthCheck"
      >
        {{ loading ? 'Checking...' : 'Run Health Check' }}
      </button>
    </div>

    <div class="mt-6 grid gap-4 md:grid-cols-[1.2fr_0.8fr]">
      <div class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
        <p class="text-xs font-semibold uppercase tracking-[0.28em] text-content-muted">Latest response</p>
        <div class="mt-4 flex items-center justify-between gap-4">
          <div>
            <p class="text-sm text-content-secondary">Service state</p>
            <p class="mt-2 text-3xl font-semibold" :class="statusTone">
              {{ response?.status ?? (error ? 'Unavailable' : 'Pending') }}
            </p>
          </div>
          <div class="rounded-2xl border border-outline bg-surface-overlay px-4 py-3 text-right">
            <p class="text-xs uppercase tracking-[0.2em] text-content-muted">Checked at</p>
            <p class="mt-2 font-mono text-sm text-content-primary">{{ checkedAt || '--:--:--' }}</p>
          </div>
        </div>
      </div>

      <div class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
        <p class="text-xs font-semibold uppercase tracking-[0.28em] text-content-muted">Payload</p>
        <pre class="mt-4 overflow-x-auto rounded-2xl bg-slate-950/90 p-4 font-mono text-sm leading-6 text-slate-100">{{ error || JSON.stringify(response ?? { status: 'waiting' }, null, 2) }}</pre>
      </div>
    </div>
  </section>
</template>