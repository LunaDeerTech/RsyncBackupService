/**
 * Centralized status → badge configuration.
 * Every page that renders a status badge should import from here
 * so labels, variants, and icons stay consistent.
 */

/* ── Icon names (lucide-vue-next) ── */

export interface StatusConfig {
  label: string
  variant: 'success' | 'warning' | 'error' | 'info' | 'default' | 'rolling' | 'cold'
  /** lucide icon component name – resolved at render time by the StatusBadge component */
  icon: string
  /** If true the icon gets a CSS spin animation */
  animated?: boolean
}

// ─── Task / Backup execution status ───
export const taskStatusMap: Record<string, StatusConfig> = {
  running: { label: '运行中', variant: 'info', icon: 'Loader', animated: true },
  queued: { label: '排队中', variant: 'warning', icon: 'Clock' },
  pending: { label: '等待中', variant: 'warning', icon: 'Clock' },
  success: { label: '成功', variant: 'success', icon: 'CircleCheck' },
  failed: { label: '失败', variant: 'error', icon: 'CircleX' },
  cancelled: { label: '已取消', variant: 'default', icon: 'Ban' },
}

// ─── Instance status ───
export const instanceStatusMap: Record<string, StatusConfig> = {
  idle: { label: '空闲', variant: 'default', icon: 'Minus' },
  running: { label: '运行中', variant: 'info', icon: 'Loader', animated: true },
}

// ─── Health status (backup targets) ───
export const healthStatusMap: Record<string, StatusConfig> = {
  healthy: { label: '健康', variant: 'success', icon: 'HeartPulse' },
  degraded: { label: '异常', variant: 'warning', icon: 'AlertTriangle' },
  unreachable: { label: '不可达', variant: 'error', icon: 'Unplug' },
}

// ─── Risk severity ───
export const riskSeverityMap: Record<string, StatusConfig> = {
  critical: { label: '严重', variant: 'error', icon: 'ShieldAlert' },
  high: { label: '高', variant: 'error', icon: 'ShieldAlert' },
  medium: { label: '中等', variant: 'warning', icon: 'AlertTriangle' },
  low: { label: '低', variant: 'info', icon: 'Info' },
  warning: { label: '警告', variant: 'warning', icon: 'AlertTriangle' },
  info: { label: '信息', variant: 'info', icon: 'Info' },
}

// ─── Risk resolved status ───
export const riskResolvedMap: Record<string, StatusConfig> = {
  resolved: { label: '已解决', variant: 'success', icon: 'CircleCheck' },
  unresolved: { label: '未解决', variant: 'warning', icon: 'AlertTriangle' },
}

// ─── Policy / Backup type ───
export const backupTypeMap: Record<string, StatusConfig> = {
  rolling: { label: '滚动', variant: 'rolling', icon: 'RefreshCw' },
  cold: { label: '冷备', variant: 'cold', icon: 'Snowflake' },
}

// ─── Generic lookup helper ───
const defaultConfig: StatusConfig = { label: '', variant: 'default', icon: 'Circle' }

export function getStatusConfig(
  map: Record<string, StatusConfig>,
  status: string | undefined | null,
): StatusConfig {
  if (!status) return defaultConfig
  return map[status] ?? { ...defaultConfig, label: status }
}
