export const actionLabels: Record<string, string> = {
  'instance.create': '创建实例',
  'instance.update': '编辑实例',
  'instance.delete': '删除实例',
  'policy.create': '创建策略',
  'policy.update': '编辑策略',
  'policy.delete': '删除策略',
  'backup.trigger': '触发备份',
  'backup.complete': '备份完成',
  'backup.fail': '备份失败',
  'backup.retry': '备份重试',
  'backup.retry_exhausted': '重试耗尽',
  'backup.move_retry': '备份移动重试',
  'backup.move_retry_exhausted': '备份移动重试耗尽',
  'backup.download': '下载备份',
  'backup.cleanup_failed': '清理失败',
  'restore.trigger': '触发恢复',
  'restore.complete': '恢复完成',
  'restore.fail': '恢复失败',
  'user.create': '创建用户',
  'user.update': '编辑用户',
  'user.delete': '删除用户',
  'target.create': '创建目标',
  'target.update': '编辑目标',
  'target.delete': '删除目标',
  'remote.create': '创建远程配置',
  'remote.update': '编辑远程配置',
  'remote.delete': '删除远程配置',
  'system.config.update': '更新系统配置',
  'risk.create': '风险检测',
  'risk.escalate': '风险升级',
  'risk.resolve': '风险解决',
}

export function getActionLabel(action: string): string {
  return actionLabels[action] ?? action
}

export function getActionBadgeVariant(action: string): 'success' | 'warning' | 'error' | 'info' | 'default' {
  if (action.endsWith('.fail') || action === 'backup.cleanup_failed') return 'error'
  if (action === 'backup.move_retry_exhausted') return 'error'
  if (action === 'risk.create' || action === 'risk.escalate') return 'error'
  if (action === 'backup.retry' || action === 'backup.retry_exhausted' || action === 'backup.move_retry') return 'warning'
  if (action.endsWith('.delete')) return 'warning'
  if (action.endsWith('.complete') || action === 'risk.resolve') return 'success'
  if (action.endsWith('.create') || action.endsWith('.trigger')) return 'info'
  return 'default'
}

export const actionOptions = [
  { label: '全部', value: '' },
  ...Object.entries(actionLabels).map(([value, label]) => ({ label, value })),
]

const backupTypeLabels: Record<string, string> = { rolling: '滚动', cold: '冷备' }
const sourceTypeLabels: Record<string, string> = { local: '本地', ssh: 'SSH' }
const storageTypeLabels: Record<string, string> = { local: '本地', ssh: 'SSH', openlist: 'OpenList', cloud: '更多云存储' }
const remoteTypeLabels: Record<string, string> = { ssh: 'SSH', openlist: 'OpenList', cloud: '更多云存储' }
const triggerLabels: Record<string, string> = { manual: '手动', scheduled: '定时' }
const backupMoveOperationLabels: Record<string, string> = {
  local_move: '本地移动',
  ssh_rsync: 'SSH 传输',
  openlist_upload: 'OpenList 上传',
}

const riskSourceLabels: Record<string, string> = {
  backup_failure: '备份失败',
  target_unreachable: '目标不可达',
  low_dr_score: '容灾率低',
  no_recent_backup: '长期未备份',
  target_capacity_low: '目标容量不足',
  backup_overdue: '备份超期',
}

const severityLabels: Record<string, string> = {
  critical: '严重',
  high: '高',
  medium: '中',
  low: '低',
}

function fmt(key: string, val: unknown): string {
  if (val === undefined || val === null || val === '') return ''
  return `${key}: ${val}`
}

function fmtId(label: string, id: unknown): string {
  if (id === undefined || id === null) return ''
  return `${label} (#${id})`
}

export function formatAuditDetail(action: string, detail: Record<string, any>): string {
  if (!detail || Object.keys(detail).length === 0) return '-'

  // If detail is a string (e.g. cleanup_failed), return as-is
  if (typeof detail === 'string') return detail

  const parts: string[] = []

  if (action.startsWith('instance.')) {
    if (detail.name) parts.push(fmt('名称', detail.name))
    if (detail.source_type) parts.push(fmt('源类型', sourceTypeLabels[detail.source_type] ?? detail.source_type))
    if (detail.source_path) parts.push(fmt('路径', detail.source_path))
  } else if (action.startsWith('policy.')) {
    if (detail.name) parts.push(fmt('策略', detail.name))
    if (detail.type) parts.push(fmt('类型', backupTypeLabels[detail.type] ?? detail.type))
    if (detail.schedule_type) parts.push(fmt('调度', detail.schedule_type))
    if (detail.target_id) parts.push(fmtId('目标', detail.target_id))
    if (detail.enabled !== undefined) parts.push(fmt('启用', detail.enabled ? '是' : '否'))
    if (detail.retention_type) parts.push(fmt('保留策略', detail.retention_type === 'count' ? '按数量' : '按时间'))
    if (detail.retention_value) parts.push(fmt('保留值', detail.retention_value))
  } else if (action === 'backup.trigger') {
    if (detail.type) parts.push(fmt('类型', backupTypeLabels[detail.type] ?? detail.type))
    if (detail.trigger_source) parts.push(fmt('触发方式', triggerLabels[detail.trigger_source] ?? detail.trigger_source))
    if (detail.policy_name) parts.push(fmt('策略', `${detail.policy_name} (#${detail.policy_id})`))
    else if (detail.policy_id) parts.push(fmtId('策略', detail.policy_id))
    if (detail.backup_id) parts.push(fmtId('备份', detail.backup_id))
  } else if (action === 'backup.complete' || action === 'backup.fail') {
    if (detail.type) parts.push(fmt('类型', backupTypeLabels[detail.type] ?? detail.type))
    if (detail.trigger_source) parts.push(fmt('触发方式', triggerLabels[detail.trigger_source] ?? detail.trigger_source))
    if (detail.policy_name) parts.push(fmt('策略', `${detail.policy_name} (#${detail.policy_id})`))
    else if (detail.policy_id) parts.push(fmtId('策略', detail.policy_id))
    if (detail.duration_seconds != null) {
      const dur = Number(detail.duration_seconds)
      if (dur >= 60) parts.push(fmt('耗时', `${Math.floor(dur / 60)}分${dur % 60}秒`))
      else parts.push(fmt('耗时', `${dur}秒`))
    }
    if (detail.error_message) parts.push(fmt('错误', detail.error_message))
  } else if (action === 'backup.retry' || action === 'backup.retry_exhausted') {
    if (detail.type) parts.push(fmt('类型', backupTypeLabels[detail.type] ?? detail.type))
    if (detail.policy_name) parts.push(fmt('策略', `${detail.policy_name} (#${detail.policy_id})`))
    if (detail.attempt != null && detail.max_retries != null) parts.push(fmt('重试', `${detail.attempt}/${detail.max_retries}`))
    if (detail.next_delay) parts.push(fmt('下次等待', detail.next_delay))
    if (detail.error) parts.push(fmt('错误', detail.error))
    if (detail.final) parts.push('已耗尽全部重试次数')
  } else if (action === 'backup.move_retry' || action === 'backup.move_retry_exhausted') {
    if (detail.type) parts.push(fmt('类型', backupTypeLabels[detail.type] ?? detail.type))
    if (detail.policy_name) parts.push(fmt('策略', `${detail.policy_name} (#${detail.policy_id})`))
    if (detail.entry) parts.push(fmt('条目', detail.entry))
    if (detail.operation) parts.push(fmt('操作', backupMoveOperationLabels[detail.operation] ?? detail.operation))
    if (detail.attempt != null && detail.max_retries != null) parts.push(fmt('重试', `${detail.attempt}/${detail.max_retries}`))
    if (detail.source_path) parts.push(fmt('源路径', detail.source_path))
    if (detail.dest_path) parts.push(fmt('目标路径', detail.dest_path))
    if (detail.next_delay) parts.push(fmt('下次等待', detail.next_delay))
    if (detail.error) parts.push(fmt('错误', detail.error))
    if (detail.final) parts.push('当前条目已耗尽全部重试次数')
  } else if (action === 'backup.download') {
    if (detail.type) parts.push(fmt('类型', backupTypeLabels[detail.type] ?? detail.type))
    if (detail.backup_id) parts.push(fmtId('备份', detail.backup_id))
  } else if (action === 'backup.cleanup_failed') {
    // detail may be a plain string from retention cleaner
    return String(detail)
  } else if (action.startsWith('restore.')) {
    if (detail.backup_type) parts.push(fmt('备份类型', backupTypeLabels[detail.backup_type] ?? detail.backup_type))
    if (detail.restore_type) parts.push(fmt('恢复类型', detail.restore_type === 'source' ? '原路径' : '自定义'))
    if (detail.target_path) parts.push(fmt('目标路径', detail.target_path))
    if (detail.policy_name) parts.push(fmt('策略', `${detail.policy_name} (#${detail.policy_id})`))
    else if (detail.policy_id) parts.push(fmtId('策略', detail.policy_id))
    if (detail.backup_id) parts.push(fmtId('备份', detail.backup_id))
    if (detail.duration_seconds != null) {
      const dur = Number(detail.duration_seconds)
      if (dur >= 60) parts.push(fmt('耗时', `${Math.floor(dur / 60)}分${dur % 60}秒`))
      else parts.push(fmt('耗时', `${dur}秒`))
    }
    if (detail.error_message) parts.push(fmt('错误', detail.error_message))
  } else if (action.startsWith('user.')) {
    if (detail.name || detail.deleted_name) parts.push(fmt('用户', detail.name ?? detail.deleted_name))
    if (detail.email || detail.deleted_email) parts.push(fmt('邮箱', detail.email ?? detail.deleted_email))
    if (detail.role || detail.deleted_role) parts.push(fmt('角色', detail.role ?? detail.deleted_role))
    if (detail.source) parts.push(fmt('来源', detail.source === 'register' ? '自行注册' : detail.source))
  } else if (action.startsWith('target.')) {
    if (detail.name) parts.push(fmt('名称', detail.name))
    if (detail.backup_type) parts.push(fmt('备份类型', backupTypeLabels[detail.backup_type] ?? detail.backup_type))
    if (detail.storage_type) parts.push(fmt('存储类型', storageTypeLabels[detail.storage_type] ?? detail.storage_type))
    if (detail.storage_path) parts.push(fmt('路径', detail.storage_path))
  } else if (action.startsWith('remote.')) {
    if (detail.name) parts.push(fmt('名称', detail.name))
    if (detail.type) parts.push(fmt('类型', remoteTypeLabels[detail.type] ?? detail.type))
    if (detail.host) parts.push(fmt('主机', detail.host))
    if (detail.port) parts.push(fmt('端口', detail.port))
    if (detail.username) parts.push(fmt('用户名', detail.username))
  } else if (action.startsWith('risk.')) {
    if (detail.message) parts.push(detail.message)
    if (detail.source) parts.push(fmt('来源', riskSourceLabels[detail.source] ?? detail.source))
    if (detail.severity) parts.push(fmt('级别', severityLabels[detail.severity] ?? detail.severity))
    if (action === 'risk.escalate') {
      if (detail.from_severity && detail.to_severity) {
        parts.push(`${severityLabels[detail.from_severity] ?? detail.from_severity} → ${severityLabels[detail.to_severity] ?? detail.to_severity}`)
      }
    }
    if (detail.risk_event_id) parts.push(fmtId('事件', detail.risk_event_id))
  }

  if (parts.length === 0) {
    // Fallback: show key=value for first few fields
    const entries = Object.entries(detail).slice(0, 4)
    for (const [k, v] of entries) {
      parts.push(`${k}: ${v}`)
    }
  }

  return parts.join(', ') || '-'
}
