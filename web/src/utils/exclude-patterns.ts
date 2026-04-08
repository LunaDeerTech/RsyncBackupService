export const EXCLUDE_PATTERN_HELP_EXAMPLES = [
  '*.log  匹配所有 .log 文件',
  'node_modules/  排除整个 node_modules 目录',
  'cache/**  排除 cache 目录及其所有子内容',
  'backup-?.tmp  匹配 backup-a.tmp 这类单字符文件名',
]

export function normalizeExcludePatternsInput(value: string): string[] {
  const lines = value
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)

  const normalized: string[] = []
  const seen = new Set<string>()
  for (const line of lines) {
    if (seen.has(line)) {
      continue
    }
    seen.add(line)
    normalized.push(line)
  }

  return normalized
}

export function excludePatternsToText(patterns?: string[]): string {
  if (!patterns || patterns.length === 0) {
    return ''
  }

  return patterns.join('\n')
}