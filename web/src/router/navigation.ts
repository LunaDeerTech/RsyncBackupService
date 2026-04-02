export type NavigationItem = {
	name: string
	to: string
	childPath: string
	label: string
	eyebrow: string
	description: string
}

export const primaryNavigation: NavigationItem[] = [
	{
		name: "dashboard",
		to: "/",
		childPath: "",
		label: "仪表盘",
		eyebrow: "Overview",
		description: "Task 10 先交付主布局、路由守卫和主题系统，统计卡片与实时概览将在后续任务接入。",
	},
	{
		name: "instances",
		to: "/instances",
		childPath: "instances",
		label: "备份实例",
		eyebrow: "Instances",
		description: "实例列表、搜索、筛选和操作区由后续任务实现，这里先保留稳定的页面承载位。",
	},
	{
		name: "storageTargets",
		to: "/storage-targets",
		childPath: "storage-targets",
		label: "存储目标",
		eyebrow: "Storage",
		description: "存储目标表单、连通性测试和危险确认将在后续任务中基于当前壳层接入。",
	},
	{
		name: "sshKeys",
		to: "/ssh-keys",
		childPath: "ssh-keys",
		label: "SSH 密钥",
		eyebrow: "Keys",
		description: "当前只保留受保护路由和主题一致的容器，密钥管理视图稍后实现。",
	},
	{
		name: "notifications",
		to: "/notifications",
		childPath: "notifications",
		label: "通知渠道",
		eyebrow: "Notify",
		description: "通知渠道页面会复用当前布局、顶栏和 token 系统，不在本任务提前展开。",
	},
	{
		name: "auditLogs",
		to: "/audit-logs",
		childPath: "audit-logs",
		label: "审计日志",
		eyebrow: "Audit",
		description: "审计日志的筛选和详情面板将在后续任务实现，这里只接通基础路由骨架。",
	},
	{
		name: "settings",
		to: "/settings",
		childPath: "settings",
		label: "系统设置",
		eyebrow: "Settings",
		description: "用户管理、密码修改和系统设置稍后填充；本任务只交付可复用的壳层基础设施。",
	},
]