<script setup lang="ts">
/**
 * StatusBadge – renders a badge with an icon for any status value.
 *
 * Usage:
 *   <StatusBadge :config="getStatusConfig(taskStatusMap, 'running')" />
 *   <StatusBadge :config="getStatusConfig(healthStatusMap, status)" size="sm" />
 */
import {
  Loader, Clock, CircleCheck, CircleX, Ban, Minus,
  HeartPulse, AlertTriangle, Unplug,
  ShieldAlert, Info, RefreshCw, Snowflake, Circle,
} from 'lucide-vue-next'
import type { StatusConfig } from '../utils/status-config'
import type { Component } from 'vue'

withDefaults(
  defineProps<{
    config: StatusConfig
    size?: 'sm' | 'md'
  }>(),
  { size: 'md' },
)

const iconMap: Record<string, Component> = {
  Loader,
  Clock,
  CircleCheck,
  CircleX,
  Ban,
  Minus,
  HeartPulse,
  AlertTriangle,
  Unplug,
  ShieldAlert,
  Info,
  RefreshCw,
  Snowflake,
  Circle,
}
</script>

<template>
  <span
    class="status-badge"
    :class="[`status-badge--${config.variant}`, size === 'sm' && 'status-badge--sm']"
  >
    <component
      :is="iconMap[config.icon] ?? iconMap.Circle"
      :size="size === 'sm' ? 11 : 13"
      class="status-badge__icon"
      :class="{ 'status-badge__icon--spin': config.animated }"
    />
    <span class="status-badge__label">{{ config.label }}</span>
  </span>
</template>

<style scoped>
.status-badge {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  font-size: 12px;
  font-weight: 600;
  line-height: 18px;
  border-radius: 9999px;
  white-space: nowrap;
}

.status-badge--sm {
  padding: 1px 6px;
  font-size: 11px;
  line-height: 16px;
}

.status-badge--default {
  background: color-mix(in srgb, var(--text-secondary) 12%, transparent);
  color: var(--text-primary);
}
.status-badge--success {
  background: color-mix(in srgb, var(--success-500) 15%, transparent);
  color: var(--success-500);
}
.status-badge--warning {
  background: color-mix(in srgb, #d97706 12%, transparent);
  color: #b45309;
}
[data-theme='dark'] .status-badge--warning {
  background: color-mix(in srgb, var(--warning-500) 15%, transparent);
  color: var(--warning-500);
}
.status-badge--error {
  background: color-mix(in srgb, var(--error-500) 15%, transparent);
  color: var(--error-500);
}
.status-badge--info {
  background: color-mix(in srgb, var(--primary-500) 15%, transparent);
  color: var(--primary-500);
}
.status-badge--rolling {
  background: color-mix(in srgb, #3b82f6 15%, transparent);
  color: #3b82f6;
}
.status-badge--cold {
  background: color-mix(in srgb, #8b5cf6 15%, transparent);
  color: #8b5cf6;
}

/* Icon */
.status-badge__icon {
  flex-shrink: 0;
}

.status-badge__icon--spin {
  animation: badge-spin 1.2s linear infinite;
}

@keyframes badge-spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
