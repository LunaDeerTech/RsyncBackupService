export function formatScheduleValue(type: string, value: string): string {
  if (type === 'cron') return value

  const seconds = parseInt(value, 10)
  if (isNaN(seconds) || seconds <= 0) return value

  if (seconds >= 86400 && seconds % 86400 === 0) {
    const days = seconds / 86400
    return `每 ${days} 天`
  }
  if (seconds >= 3600 && seconds % 3600 === 0) {
    const hours = seconds / 3600
    return `每 ${hours} 小时`
  }
  if (seconds >= 60 && seconds % 60 === 0) {
    const minutes = seconds / 60
    return `每 ${minutes} 分钟`
  }
  return `每 ${seconds} 秒`
}

export function parseIntervalInput(input: string): number {
  const trimmed = input.trim()

  // Try pure number (treat as seconds)
  const pure = parseInt(trimmed, 10)
  if (String(pure) === trimmed && pure > 0) return pure

  // Match patterns like "6小时", "30分钟", "1天", "每6小时", "每 30 分钟"
  const match = trimmed.match(/^每?\s*(\d+)\s*(秒|分钟|小时|天)$/)
  if (match) {
    const num = parseInt(match[1], 10)
    switch (match[2]) {
      case '秒': return num
      case '分钟': return num * 60
      case '小时': return num * 3600
      case '天': return num * 86400
    }
  }

  return NaN
}
