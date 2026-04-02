export type NavigationItem = {
	name: string
	to: string
	childPath: string
	label: string
	eyebrow: string
	description: string
	requiresAdmin?: boolean
}

export const primaryNavigation: NavigationItem[] = [
	{
		name: "dashboard",
		to: "/",
		childPath: "",
		label: "仪表盘",
		eyebrow: "Overview",
		description: "统计卡片、运行中任务、最近备份与存储空间概览。",
		requiresAdmin: true,
	},
	{
		name: "instances",
		to: "/instances",
		childPath: "instances",
		label: "备份实例",
		eyebrow: "Instances",
		description: "实例列表、详情入口和恢复工作流承载页。",
	},
	{
		name: "storageTargets",
		to: "/storage-targets",
		childPath: "storage-targets",
		label: "存储目标",
		eyebrow: "Storage",
		description: "本地与 SSH 存储目标管理、测试和维护。",
		requiresAdmin: true,
	},
	{
		name: "sshKeys",
		to: "/ssh-keys",
		childPath: "ssh-keys",
		label: "SSH 密钥",
		eyebrow: "Keys",
		description: "SSH 密钥登记、连通性验证与清理。",
		requiresAdmin: true,
	},
	{
		name: "notifications",
		to: "/notifications",
		childPath: "notifications",
		label: "通知渠道",
		eyebrow: "Notify",
		description: "SMTP 渠道管理与订阅前置配置。",
		requiresAdmin: true,
	},
	{
		name: "auditLogs",
		to: "/audit-logs",
		childPath: "audit-logs",
		label: "审计日志",
		eyebrow: "Audit",
		description: "按用户、动作和时间范围筛选关键操作。",
		requiresAdmin: true,
	},
	{
		name: "settings",
		to: "/settings",
		childPath: "settings",
		label: "系统设置",
		eyebrow: "Settings",
		description: "用户管理、密码修改和实例权限设置。",
		requiresAdmin: true,
	},
]