<script setup lang="ts">
import HealthCheckPanel from '../components/HealthCheckPanel.vue'
import AppLayout from '../layouts/AppLayout.vue'

const plannedRoutes = ['/login', '/register', '/dashboard', '/instances']
</script>

<template>
  <AppLayout>
    <div class="grid gap-6 xl:grid-cols-[1.2fr_0.8fr]">
      <section class="relative overflow-hidden rounded-[32px] border border-outline bg-surface-base/92 p-7 shadow-panel backdrop-blur md:p-8">
        <div class="absolute inset-x-6 top-0 h-px bg-[linear-gradient(90deg,transparent,var(--primary-500),transparent)]"></div>

        <div class="space-y-6">
          <div class="inline-flex items-center gap-3 rounded-full border border-outline-subtle bg-surface-raised px-4 py-2 text-xs font-semibold uppercase tracking-[0.28em] text-content-muted">
            <span class="inline-flex h-2 w-2 rounded-full bg-success-500"></span>
            scaffold ready
          </div>

          <div class="space-y-4">
            <p class="text-sm font-semibold uppercase tracking-[0.4em] text-content-muted">RBS - Coming Soon</p>
            <h2 class="max-w-3xl text-4xl font-semibold leading-tight text-content-primary md:text-5xl">
              A controlled launch surface for backup operations, recovery flows and runtime telemetry.
            </h2>
            <p class="max-w-2xl text-base leading-8 text-content-secondary">
              The frontend foundation is now structured for route expansion, design-token driven theming and backend integration without revisiting the app shell.
            </p>
          </div>

          <div class="grid gap-4 md:grid-cols-3">
            <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
              <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Stack</p>
              <p class="mt-4 text-lg font-semibold text-content-primary">Vue 3 + Vite + TypeScript</p>
              <p class="mt-2 text-sm leading-6 text-content-secondary">Project shell aligned for fast iteration and typed route/API growth.</p>
            </article>
            <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
              <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Tokens</p>
              <p class="mt-4 text-lg font-semibold text-content-primary">Light and dark theme variables</p>
              <p class="mt-2 text-sm leading-6 text-content-secondary">Theme state persists in local storage and updates the root data attribute.</p>
            </article>
            <article class="rounded-[24px] border border-outline-subtle bg-surface-raised p-5">
              <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">HTTP</p>
              <p class="mt-4 text-lg font-semibold text-content-primary">Axios contract wrapper</p>
              <p class="mt-2 text-sm leading-6 text-content-secondary">Authorization injection, response unwrapping and 401 handling are centralized.</p>
            </article>
          </div>
        </div>
      </section>

      <aside class="rounded-[32px] border border-outline bg-surface-base/92 p-7 shadow-panel backdrop-blur md:p-8">
        <p class="text-xs font-semibold uppercase tracking-[0.32em] text-content-muted">Reserved route map</p>
        <div class="mt-5 flex flex-wrap gap-3">
          <span
            v-for="route in plannedRoutes"
            :key="route"
            class="rounded-full border border-outline-subtle bg-surface-raised px-4 py-2 font-mono text-sm text-content-secondary"
          >
            {{ route }}
          </span>
        </div>

        <div class="mt-8 rounded-[24px] border border-outline-subtle bg-[linear-gradient(135deg,rgba(99,217,255,0.14),rgba(126,242,212,0.08))] p-5">
          <p class="text-xs font-semibold uppercase tracking-[0.24em] text-content-muted">Next expansion points</p>
          <ul class="mt-4 space-y-3 text-sm leading-6 text-content-secondary">
            <li>Authentication pages and route guards</li>
            <li>Dashboard metrics and system health cards</li>
            <li>Instance list, detail and execution workflows</li>
          </ul>
        </div>
      </aside>
    </div>

    <div class="mt-6">
      <HealthCheckPanel />
    </div>
  </AppLayout>
</template>