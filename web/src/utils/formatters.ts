import type {
	BackupStatus,
	JsonValue,
	SourceType,
	StrategySummary,
} from "../api/types"
import type { StatusTone } from "../components/ui/statusTone"

export function formatDateTime(value?: string | null): string {
	if (!value) {
		return "—"
	}

	const date = new Date(value)
	if (Number.isNaN(date.getTime())) {
		return value
	}

	return new Intl.DateTimeFormat("zh-CN", {
		year: "numeric",
		month: "2-digit",
		day: "2-digit",
		hour: "2-digit",
		minute: "2-digit",
	}).format(date)
}

export function formatRemainingTime(value?: string | null, nowValue: number | Date = Date.now()): string {
	if (!value) {
		return "时间未知"
	}

	const target = new Date(value)
	if (Number.isNaN(target.getTime())) {
		return "时间未知"
	}

	const now = typeof nowValue === "number" ? nowValue : nowValue.getTime()
	const diffMs = target.getTime() - now
	if (diffMs <= 0) {
		return "即将启动"
	}

	let remainingSeconds = Math.ceil(diffMs / 1000)
	const days = Math.floor(remainingSeconds / 86400)
	remainingSeconds -= days * 86400
	const hours = Math.floor(remainingSeconds / 3600)
	remainingSeconds -= hours * 3600
	const minutes = Math.floor(remainingSeconds / 60)
	const seconds = remainingSeconds - minutes * 60

	const parts: string[] = []
	if (days > 0) {
		parts.push(`${days} 天`)
	}
	if (hours > 0) {
		parts.push(`${hours} 小时`)
	}
	if (minutes > 0) {
		parts.push(`${minutes} 分钟`)
	}
	if (parts.length === 0) {
		parts.push(`${seconds} 秒`)
	}

	return parts.slice(0, 2).join(" ")
}

export function formatBytes(bytes?: number): string {
	if (!bytes || bytes <= 0) {
		return "0 B"
	}

	const units = ["B", "KB", "MB", "GB", "TB"]
	let value = bytes
	let unitIndex = 0

	while (value >= 1024 && unitIndex < units.length - 1) {
		value /= 1024
		unitIndex += 1
	}

	const precision = value >= 100 || unitIndex === 0 ? 0 : value >= 10 ? 1 : 2
	return `${value.toFixed(precision)} ${units[unitIndex]}`
}

export function formatStatusLabel(status?: string): string {
	switch ((status ?? "").trim()) {
		case "running":
			return "运行中"
		case "success":
			return "成功"
		case "failed":
			return "失败"
		case "cancelled":
			return "已取消"
		default:
			return status && status.trim() !== "" ? status : "未知"
	}
}

export function statusTone(status?: string): StatusTone {
	switch ((status ?? "").trim()) {
		case "running":
			return "running"
		case "success":
			return "success"
		case "failed":
			return "danger"
		case "cancelled":
			return "warning"
		default:
			return "default"
	}
}

export function formatSource(sourceType: SourceType | string, sourcePath: string, sourceHost?: string): string {
	if (sourceType === "remote" && sourceHost) {
		return `${sourceHost}:${sourcePath}`
	}

	return sourcePath
}

export function formatBackupType(type: string): string {
	return type === "cold" ? "冷备份" : type === "rolling" ? "滚动备份" : type || "未知"
}

export function formatSchedule(strategy: StrategySummary): string {
	if (strategy.cron_expr && strategy.cron_expr.trim() !== "") {
		return `Cron: ${strategy.cron_expr}`
	}

	if (strategy.interval_seconds > 0) {
		return `每 ${strategy.interval_seconds} 秒`
	}

	return "未配置"
}

export function parseJsonInput(text: string): JsonValue {
	const trimmed = text.trim()
	if (trimmed === "") {
		return {}
	}

	return JSON.parse(trimmed) as JsonValue
}

export function stringifyJson(value: JsonValue | undefined): string {
	if (value === undefined) {
		return "{}"
	}

	return JSON.stringify(value, null, 2)
}

export function splitLines(value: string): string[] {
	return value
		.split(/[\n,]/)
		.map((item) => item.trim())
		.filter((item) => item !== "")
}