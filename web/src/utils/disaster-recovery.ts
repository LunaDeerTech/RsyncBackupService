export type DRLevel = 'safe' | 'caution' | 'risk' | 'danger'

export function getDRLevelColor(level: string): string {
  switch (level) {
    case 'safe': return 'var(--success-500)'
    case 'caution': return 'var(--warning-500)'
    case 'risk': return 'var(--error-500)'
    case 'danger': return 'var(--error-600)'
    default: return 'var(--text-muted)'
  }
}

export function getDRLevelLabel(level: string): string {
  switch (level) {
    case 'safe': return '安全'
    case 'caution': return '注意'
    case 'risk': return '风险'
    case 'danger': return '危险'
    default: return '未知'
  }
}

export function getDRLevelBadgeVariant(level: string): 'success' | 'warning' | 'error' | 'default' {
  switch (level) {
    case 'safe': return 'success'
    case 'caution': return 'warning'
    case 'risk': return 'error'
    case 'danger': return 'error'
    default: return 'default'
  }
}

export function getDRLevelRingColor(level: string): string {
  switch (level) {
    case 'safe': return 'var(--success-500)'
    case 'caution': return 'var(--warning-500)'
    case 'risk': return 'var(--error-500)'
    case 'danger': return 'var(--error-600)'
    default: return 'var(--border-default)'
  }
}
