export type StatusTone = "default" | "primary" | "running" | "info" | "success" | "warning" | "danger"

export interface StatusToneDefinition {
	accent: string
	surface: string
	border: string
	text: string
	label: string
	icon: string
}

export const statusToneMap: Record<StatusTone, StatusToneDefinition> = {
	default: {
		accent: "var(--text-muted)",
		surface: "var(--surface-elevated)",
		border: "color-mix(in srgb, var(--border-default) 92%, transparent)",
		text: "var(--text-strong)",
		label: "默认",
		icon: "•",
	},
	primary: {
		accent: "var(--primary-500)",
		surface: "var(--surface-accent-soft)",
		border: "color-mix(in srgb, var(--primary-500) 34%, var(--border-default))",
		text: "var(--text-strong)",
		label: "品牌",
		icon: "●",
	},
	running: {
		accent: "var(--accent-mint-400)",
		surface: "color-mix(in srgb, var(--accent-mint-400) 16%, var(--surface-panel-solid))",
		border: "color-mix(in srgb, var(--accent-mint-400) 34%, var(--border-default))",
		text: "var(--text-strong)",
		label: "运行中",
		icon: "↻",
	},
	info: {
		accent: "var(--info-500)",
		surface: "var(--info-surface)",
		border: "color-mix(in srgb, var(--info-500) 34%, var(--border-default))",
		text: "var(--text-strong)",
		label: "信息",
		icon: "i",
	},
	success: {
		accent: "var(--success-500)",
		surface: "var(--success-surface)",
		border: "color-mix(in srgb, var(--success-500) 34%, var(--border-default))",
		text: "var(--text-strong)",
		label: "成功",
		icon: "✓",
	},
	warning: {
		accent: "var(--warning-500)",
		surface: "var(--warning-surface)",
		border: "color-mix(in srgb, var(--warning-500) 38%, var(--border-default))",
		text: "var(--text-strong)",
		label: "警告",
		icon: "!",
	},
	danger: {
		accent: "var(--error-500)",
		surface: "color-mix(in srgb, var(--error-500) 30%, var(--surface-panel-solid))",
		border: "color-mix(in srgb, var(--error-500) 84%, var(--border-default))",
		text: "var(--error-500)",
		label: "危险",
		icon: "!",
	},
}