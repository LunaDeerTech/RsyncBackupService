<script setup lang="ts">
import AppBadge from "../components/ui/AppBadge.vue"
import AppBreadcrumb from "../components/ui/AppBreadcrumb.vue"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppProgress from "../components/ui/AppProgress.vue"
import AppSpinner from "../components/ui/AppSpinner.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTag from "../components/ui/AppTag.vue"
import AppTimeline from "../components/ui/AppTimeline.vue"
import AppToastHost from "../components/ui/AppToastHost.vue"

interface TableColumn<T> {
	key: keyof T | string
	label: string
}

interface InstanceRow {
	id: number
	name: string
	strategy: string
	throughput: string
	lastRun: string
	status: "running" | "success" | "warning" | "danger"
}

const breadcrumbItems = [
	{ label: "组件系统", to: "/ui-preview" },
	{ label: "Task 12 预览", current: true },
]

const tableColumns: TableColumn<InstanceRow>[] = [
	{ key: "name", label: "实例" },
	{ key: "strategy", label: "策略" },
	{ key: "throughput", label: "吞吐" },
	{ key: "lastRun", label: "最近执行" },
	{ key: "status", label: "状态" },
]

const tableRows: InstanceRow[] = [
	{ id: 1, name: "prod-main", strategy: "滚动备份", throughput: "12.34MB/s", lastRun: "2 分钟前", status: "running" },
	{ id: 2, name: "archive-vault", strategy: "冷备归档", throughput: "8.12MB/s", lastRun: "12 分钟前", status: "success" },
	{ id: 3, name: "analytics-dr", strategy: "增量同步", throughput: "4.20MB/s", lastRun: "29 分钟前", status: "warning" },
	{ id: 4, name: "edge-cache", strategy: "滚动备份", throughput: "0MB/s", lastRun: "失败于 41 分钟前", status: "danger" },
]

const timelineItems = [
	{
		id: 1,
		title: "prod-main 进入实时同步窗口",
		description: "流量恢复正常，吞吐稳定在 12MB/s 左右。",
		timestamp: "21:03",
		tone: "success" as const,
	},
	{
		id: 2,
		title: "analytics-dr 保留策略将于今晚收敛",
		description: "检测到归档空间接近阈值，建议清理过旧快照。",
		timestamp: "20:42",
		tone: "warning" as const,
	},
	{
		id: 3,
		title: "edge-cache SSH 连接中断",
		description: "系统已暂停重试并发出危险通知。",
		timestamp: "20:19",
		tone: "danger" as const,
	},
]

function statusTone(status: InstanceRow["status"]) {
	if (status === "running") {
		return "running"
	}

	if (status === "success") {
		return "success"
	}

	if (status === "warning") {
		return "warning"
	}

	return "danger"
}

function statusLabel(status: InstanceRow["status"]) {
	if (status === "running") {
		return "运行中"
	}

	if (status === "success") {
		return "已完成"
	}

	if (status === "warning") {
		return "待关注"
	}

	return "失败"
}

function statusToneFromValue(value: unknown) {
	return statusTone(value as InstanceRow["status"])
}

function statusLabelFromValue(value: unknown) {
	return statusLabel(value as InstanceRow["status"])
}
</script>

<template>
	<div class="ui-data-preview">
		<AppToastHost label="Task 12 Toast 预览">
			<AppNotification announce title="prod-main 正在写入新快照" tone="success" description="写入速率 12.34MB/s，预计剩余 1 分 23 秒。" />
			<AppNotification announce title="edge-cache 备份中断" tone="danger" description="SSH 连接已断开，任务被安全终止。" />
		</AppToastHost>

		<section class="ui-data-preview__hero">
			<div class="ui-data-preview__hero-copy">
				<AppBreadcrumb :items="breadcrumbItems" />
				<div class="ui-data-preview__hero-flags">
					<AppBadge tone="primary">Task 12</AppBadge>
					<AppTag tone="info">高密度表格</AppTag>
					<AppTag tone="warning">状态语义</AppTag>
				</div>
				<p class="ui-data-preview__eyebrow">Balanced Flux / Data Display</p>
				<h1 class="ui-data-preview__title">表格、状态、反馈与高密度信息组件</h1>
				<p class="ui-data-preview__body">
					这个临时页面只展示 Task 12 的展示型组件：高密度表格保持冷静骨架，运行态反馈提供文字元信息，语义通知与标签不会混用品牌主色承担危险态。
				</p>
			</div>

			<AppCard
				tone="running"
				eyebrow="Live Summary"
				title="运行态概览"
				description="动效只服务状态感知，持续运动只保留给运行中的进度层。"
			>
				<div class="ui-data-preview__hero-metrics">
					<div>
						<span>活跃任务</span>
						<strong>04</strong>
					</div>
					<div>
						<span>平均吞吐</span>
						<strong>9.12MB/s</strong>
					</div>
					<div>
						<span>待处理告警</span>
						<strong>01</strong>
					</div>
				</div>
			</AppCard>
		</section>

		<section class="ui-data-preview__grid">
			<AppCard
				title="实例扫描表"
				description="表格维持紧凑边界、弱 hover 和明确列头，不使用毛玻璃与大面积渐变。"
			>
				<AppTable aria-label="Task 12 实例列表预览" :rows="tableRows" :columns="tableColumns" row-key="id" dense>
					<template #cell-throughput="{ value }">
						<AppBadge tone="primary">{{ value }}</AppBadge>
					</template>

					<template #cell-status="{ value }">
						<AppTag :tone="statusToneFromValue(value)">{{ statusLabelFromValue(value) }}</AppTag>
					</template>
				</AppTable>
			</AppCard>

			<div class="ui-data-preview__stack">
				<AppCard title="运行中反馈" description="每条进度都同时给出百分比和速率或剩余时间。">
					<AppProgress aria-label="prod-main 运行进度" :percentage="45" speed-text="12.34MB/s" eta-text="1m 23s" tone="running" />
					<AppProgress aria-label="archive-vault 归档进度" :percentage="100" speed-text="归档完成" tone="success" />
					<AppProgress aria-label="edge-cache 重试进度" :percentage="18" eta-text="等待 SSH 重连" tone="danger" />
				</AppCard>

				<AppCard title="语义标签与徽标" description="品牌色只服务主强调，信息态与危险态单独走语义色。" compact>
					<div class="ui-data-preview__token-row">
						<AppTag tone="primary">品牌主色</AppTag>
						<AppTag tone="success">成功</AppTag>
						<AppTag tone="warning">警告</AppTag>
						<AppTag tone="danger">危险</AppTag>
					</div>
					<div class="ui-data-preview__token-row">
						<AppBadge tone="primary">18</AppBadge>
						<AppBadge tone="info">API</AppBadge>
						<AppBadge tone="success">OK</AppBadge>
						<AppBadge tone="danger">ERR</AppBadge>
					</div>
				</AppCard>
			</div>
		</section>

		<section class="ui-data-preview__grid ui-data-preview__grid--bottom">
			<AppCard title="通知与时间线" description="通知卡强调语义与后果说明，时间线保持节奏整齐、便于扫视。">
				<div class="ui-data-preview__notification-stack">
					<AppNotification title="增量同步已开始" tone="info" description="系统将在当前窗口内持续写入最新差异。" timestamp="刚刚" />
					<AppNotification title="archive-vault 已完成归档" tone="success" description="冷备卷已落盘并生成校验记录。" timestamp="5 分钟前" />
				</div>
				<AppTimeline :items="timelineItems" />
			</AppCard>

			<div class="ui-data-preview__stack">
				<AppCard title="空状态" description="没有记录时，用稳定卡片而不是装饰型插画占据注意力。">
					<AppEmpty title="还没有恢复记录" description="当用户第一次进入恢复历史时，这里展示下一步操作，而不是空白表格。">
						<template #actions>
							<AppButton variant="secondary">创建恢复任务</AppButton>
						</template>
					</AppEmpty>
				</AppCard>

				<AppCard title="Spinner" description="加载态只保留短促旋转，不附加持续发光。" compact>
					<div class="ui-data-preview__spinner-row">
						<AppSpinner tone="default" label="读取实例列表" />
						<AppSpinner tone="running" label="同步运行态" />
						<AppSpinner tone="danger" label="等待重试" />
					</div>
				</AppCard>
			</div>
		</section>
	</div>
</template>

<style scoped>
.ui-data-preview {
	position: relative;
	display: grid;
	gap: var(--space-6);
	max-width: 88rem;
	min-height: 100vh;
	padding: clamp(var(--space-4), 3.2vw, var(--space-8));
	margin: 0 auto;
	background:
		linear-gradient(90deg, color-mix(in srgb, var(--border-default) 18%, transparent) 1px, transparent 1px),
		linear-gradient(color-mix(in srgb, var(--border-default) 16%, transparent) 1px, transparent 1px),
		radial-gradient(circle at top left, color-mix(in srgb, var(--primary-300) 24%, transparent), transparent 34%),
		radial-gradient(circle at bottom right, color-mix(in srgb, var(--accent-mint-400) 14%, transparent), transparent 30%),
		linear-gradient(180deg, var(--surface-canvas), var(--surface-subtle));
	background-size: 2.6rem 2.6rem, 2.6rem 2.6rem, auto, auto, auto;
	background-position: center;
}

.ui-data-preview__hero,
.ui-data-preview__grid {
	display: grid;
	gap: var(--space-4);
}

.ui-data-preview__hero {
	grid-template-columns: minmax(0, 1.35fr) minmax(20rem, 0.86fr);
	align-items: stretch;
}

.ui-data-preview__grid {
	grid-template-columns: minmax(0, 1.4fr) minmax(20rem, 0.92fr);
	align-items: start;
}

.ui-data-preview__grid--bottom {
	grid-template-columns: repeat(2, minmax(0, 1fr));
}

.ui-data-preview__hero-copy,
.ui-data-preview__stack,
.ui-data-preview__notification-stack {
	display: grid;
	gap: var(--space-4);
}

.ui-data-preview__hero-copy {
	padding: clamp(var(--space-5), 4vw, var(--space-8));
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 86%, transparent);
	border-radius: calc(var(--radius-dialog) + 2px);
	background:
		linear-gradient(140deg, color-mix(in srgb, var(--surface-panel) 96%, transparent), color-mix(in srgb, var(--surface-elevated) 84%, transparent));
	box-shadow: var(--shadow-ambient);
}

.ui-data-preview__hero-flags,
.ui-data-preview__token-row,
.ui-data-preview__spinner-row {
	display: flex;
	gap: var(--space-3);
	flex-wrap: wrap;
	align-items: center;
}

.ui-data-preview__eyebrow,
.ui-data-preview__body {
	margin: 0;
	color: var(--text-muted);
}

.ui-data-preview__eyebrow {
	font-size: 0.8rem;
	font-weight: 800;
	letter-spacing: 0.1em;
	text-transform: uppercase;
}

.ui-data-preview__title {
	margin: 0;
	max-width: 12ch;
	color: var(--text-strong);
	font-size: clamp(2.3rem, 5vw, 4rem);
	line-height: 0.98;
	letter-spacing: -0.05em;
}

.ui-data-preview__body {
	max-width: 42rem;
	font-size: 1rem;
	line-height: 1.7;
}

.ui-data-preview__hero-metrics {
	display: grid;
	grid-template-columns: repeat(3, minmax(0, 1fr));
	gap: var(--space-3);
}

.ui-data-preview__hero-metrics div {
	display: grid;
	gap: var(--space-2);
	padding: var(--space-3);
	border-radius: calc(var(--radius-control) + 2px);
	background: color-mix(in srgb, var(--surface-panel-solid) 88%, transparent);
	box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--border-default) 76%, transparent);
}

.ui-data-preview__hero-metrics span {
	color: var(--text-muted);
	font-size: 0.8rem;
	font-weight: 600;
	letter-spacing: 0.04em;
	text-transform: uppercase;
}

.ui-data-preview__hero-metrics strong {
	color: var(--text-strong);
	font-size: 1.2rem;
	line-height: 1;
}

@media (max-width: 1080px) {
	.ui-data-preview__hero,
	.ui-data-preview__grid,
	.ui-data-preview__grid--bottom {
		grid-template-columns: 1fr;
	}

	.ui-data-preview__hero-metrics {
		grid-template-columns: 1fr;
	}
}

@media (max-width: 720px) {
	.ui-data-preview {
		padding-bottom: 12rem;
	}

	.ui-data-preview__hero-copy {
		padding: var(--space-5);
	}
}
</style>