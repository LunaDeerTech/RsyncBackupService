export type NavigationGroup = {
	label: string
	requiresAdmin?: boolean
	items: NavigationItem[]
}

export type NavigationItem = {
	label: string
	to: string
	caption?: string
	requiresAdmin?: boolean
}

export const navigationGroups: NavigationGroup[] = [
	{
		label: "工作区",
		items: [
			{
				label: "仪表盘",
				to: "/",
				caption: "DASHBOARD",
				requiresAdmin: true,
			},
			{
				label: "备份实例",
				to: "/instances",
				caption: "INSTANCES",
			},
		],
	},
	{
		label: "管理",
		requiresAdmin: true,
		items: [
			{
				label: "存储目标",
				to: "/storage-targets",
				caption: "STORAGE",
			},
			{
				label: "系统管理",
				to: "/system",
				caption: "SYSTEM",
			},
		],
	},
]