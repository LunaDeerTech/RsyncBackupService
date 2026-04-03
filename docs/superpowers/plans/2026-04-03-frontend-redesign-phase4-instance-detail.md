# Phase 4: 实例详情页重构

> **For agentic workers:** Use superpowers:executing-plans to implement this phase. Each section is a sequential unit of work.

**目标：** 重写 `OverviewTab` 为 V3-C 布局（KPI 行 + 双栏 + 时间线），将 `StrategiesTab`、`RestoreTab`、`SubscriptionsTab` 从双栏表单改为全宽列表 + Modal，并在 `InstanceDetailView` 中实现基于角色的 Tab 可见性。

**前置条件：** Phase 1（全局骨架）已完成。Phase 2、3 建议先完成以验证 Modal 模式可用。

**设计规格来源：** `docs/superpowers/specs/2026-04-03-frontend-layout-redesign.md` 第 5.3 节。

---

## 1. InstanceDetailView 角色控制

**文件：** `web/src/views/InstanceDetailView.vue`（175 行）

**当前状态：** 5 个 Tab（概览、策略、备份历史、恢复、通知订阅），全部对所有用户可见。

**目标状态：** 管理员看到 5 个 Tab，普通用户只看到「概览」和「备份历史」2 个 Tab。

### 1.1 导入 auth store

```typescript
import { useAuthStore } from "../stores/auth"
```

### 1.2 获取当前用户角色

```typescript
const auth = useAuthStore()
const isAdmin = computed(() => auth.currentUser?.is_admin === true)
```

### 1.3 动态计算 Tab 列表

替换静态 `tabs` 常量为计算属性：

```typescript
const tabs = computed(() => {
	const allTabs = [
		{ value: "overview", label: "概览" },
		{ value: "strategies", label: "策略", adminOnly: true },
		{ value: "backups", label: "备份历史" },
		{ value: "restore", label: "恢复", adminOnly: true },
		{ value: "subscriptions", label: "通知订阅", adminOnly: true },
	]

	return allTabs.filter((tab) => !tab.adminOnly || isAdmin.value)
})
```

### 1.4 确保默认 Tab 对普通用户有效

当前 `activeTab` 默认值是 `"overview"`，对普通用户有效，无需修改。

### 1.5 在 template 中更新显示条件

当前 template 中的 Tab 内容渲染用的是 `v-if="activeTab === 'xxx'"`，无需修改。但需要确认：当普通用户无法选中 `strategies`/`restore`/`subscriptions` Tab 时（因为 Tab 列表中不存在），对应的组件不会渲染。这已经由 `v-if/v-else-if` 保证。

---

## 2. OverviewTab 完全重写（V3-C 布局）

**文件：** `web/src/views/instance/OverviewTab.vue`（当前 114 行）

**当前状态：** 中继模式警告 + 双栏（基本信息 + 源配置）+ 策略摘要列表

**目标状态：** KPI 卡片行（5 列）+ 双栏（源配置 + 运行中任务）+ 最近活动时间线

### 2.1 新增 props

当前 props 有 `instance`、`strategies`、`relayMode`、`relayModeHint`、`relayModeTitle`。

需要额外接收备份记录用于 KPI 和时间线。修改 `InstanceDetailView.vue` 传递额外数据：

**方案 A：在 OverviewTab 内部自行获取数据。** 这更符合 Tab 的独立性，不增加父组件负担。

采用方案 A：在 OverviewTab 中调用 API 获取备份历史和运行中任务。

### 2.2 完整重写 OverviewTab

```vue
<script setup lang="ts">
import { computed, onMounted, ref } from "vue"

import { ApiError } from "../../api/client"
import { listBackups } from "../../api/backups"
import { listRunningTasks } from "../../api/system"
import type { BackupRecord, InstanceDetail, RunningTaskStatus, StrategySummary } from "../../api/types"
import AppCard from "../../components/ui/AppCard.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppProgress from "../../components/ui/AppProgress.vue"
import AppTag from "../../components/ui/AppTag.vue"
import AppTimeline from "../../components/ui/AppTimeline.vue"
import {
	formatBackupType,
	formatBytes,
	formatDateTime,
	formatSource,
	formatStatusLabel,
	statusTone,
} from "../../utils/formatters"

const props = defineProps<{
	instance: InstanceDetail
	strategies: StrategySummary[]
	relayMode: boolean
	relayModeHint?: string
	relayModeTitle?: string
}>()

const recentBackups = ref<BackupRecord[]>([])
const runningTasks = ref<RunningTaskStatus[]>([])
const isLoading = ref(true)

// KPI computed values
const activeStrategyCount = computed(() => props.strategies.filter((s) => s.enabled).length)

const totalBackupCount = computed(() => recentBackups.value.length)

const cumulativeBytes = computed(() =>
	recentBackups.value.reduce((sum, record) => sum + record.total_size, 0),
)

const successRate = computed(() => {
	const total = recentBackups.value.length
	if (total === 0) return "—"
	const successes = recentBackups.value.filter((r) => r.status === "success").length
	return `${Math.round((successes / total) * 100)}%`
})

const lastBackup = computed(() => {
	if (recentBackups.value.length === 0) return null
	return recentBackups.value[0]
})

const instanceTasks = computed(() =>
	runningTasks.value.filter((task) => task.instance_id === props.instance.id),
)

const timelineItems = computed(() =>
	recentBackups.value.slice(0, 10).map((record) => {
		const strategyName = props.strategies.find((s) => s.id === record.strategy_id)?.name ?? "手动"
		return {
			id: record.id,
			title: `${formatBackupType(record.backup_type)} · ${strategyName} → 目标 #${record.storage_target_id}`,
			description: record.error_message
				? `失败：${record.error_message}`
				: `${formatBytes(record.total_size)}，${record.files_transferred} 个文件`,
			timestamp: formatDateTime(record.finished_at ?? record.started_at),
			tone: statusTone(record.status),
		}
	}),
)

async function loadData(): Promise<void> {
	isLoading.value = true

	try {
		const [backups, tasks] = await Promise.all([
			listBackups(props.instance.id),
			listRunningTasks().catch(() => [] as RunningTaskStatus[]),
		])
		recentBackups.value = backups
		runningTasks.value = tasks
	} catch {
		// 静默处理，KPI 显示默认值
	} finally {
		isLoading.value = false
	}
}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification
			v-if="relayMode"
			:title="relayModeTitle || '远程到远程中继'"
			tone="warning"
			:description="relayModeHint || '该实例至少有一个远程源与远程目标的组合。执行链路会落到本机缓存目录，请检查可用磁盘空间。'"
		/>

		<!-- KPI 卡片行 -->
		<section class="page-kpi-grid" aria-label="实例关键指标">
			<AppCard title="活跃策略" compact>
				<p class="page-kpi__value">{{ activeStrategyCount }}</p>
			</AppCard>
			<AppCard title="备份总数" compact>
				<p class="page-kpi__value">{{ totalBackupCount }}</p>
			</AppCard>
			<AppCard title="累计容量" compact>
				<p class="page-kpi__value">{{ formatBytes(cumulativeBytes) }}</p>
			</AppCard>
			<AppCard title="成功率" compact>
				<p class="page-kpi__value">{{ successRate }}</p>
			</AppCard>
			<AppCard title="最近备份" compact>
				<div v-if="lastBackup" class="overview-tab__last-backup">
					<AppTag :tone="statusTone(lastBackup.status)">{{ formatStatusLabel(lastBackup.status) }}</AppTag>
					<span class="page-muted">{{ formatDateTime(lastBackup.finished_at ?? lastBackup.started_at) }}</span>
				</div>
				<p v-else class="page-kpi__value">—</p>
			</AppCard>
		</section>

		<!-- 双栏区域：源配置 + 运行中任务 -->
		<section class="page-two-column">
			<AppCard title="源配置" description="类型、路径、连接参数和排除规则。">
				<dl class="page-detail-list">
					<div>
						<dt>源类型</dt>
						<dd>{{ instance.source_type === "remote" ? "远程主机" : "本地路径" }}</dd>
					</div>
					<div>
						<dt>源位置</dt>
						<dd>{{ formatSource(instance.source_type, instance.source_path, instance.source_host) }}</dd>
					</div>
					<div v-if="instance.source_type === 'remote'">
						<dt>连接用户</dt>
						<dd>{{ instance.source_user || "—" }}</dd>
					</div>
					<div v-if="instance.source_type === 'remote'">
						<dt>端口</dt>
						<dd>{{ instance.source_port }}</dd>
					</div>
					<div>
						<dt>排除模式</dt>
						<dd>{{ instance.exclude_patterns.length > 0 ? instance.exclude_patterns.join("，") : "无" }}</dd>
					</div>
				</dl>
			</AppCard>

			<AppCard title="当前运行任务" description="该实例正在执行的备份或恢复任务。">
				<div v-if="instanceTasks.length > 0" class="page-stack">
					<article v-for="task in instanceTasks" :key="task.task_id" class="overview-tab__task">
						<div class="overview-tab__task-header">
							<p class="page-section__title">任务 {{ task.task_id }}</p>
							<AppTag :tone="statusTone(task.status)">{{ formatStatusLabel(task.status) }}</AppTag>
						</div>
						<AppProgress
							:percentage="task.percentage"
							:speed-text="task.speed_text"
							:eta-text="task.remaining_text"
							tone="running"
							:aria-label="`任务 ${task.task_id} 进度`"
						>
							<template #label>进度</template>
						</AppProgress>
					</article>
				</div>
				<AppEmpty v-else title="没有运行中任务" description="当备份或恢复执行时，这里会显示实时进度。" compact />
			</AppCard>
		</section>

		<!-- 最近备份活动时间线 -->
		<AppCard title="最近活动" description="最近 10 条备份记录，按完成时间倒序。">
			<AppTimeline v-if="timelineItems.length > 0" :items="timelineItems" compact />
			<AppEmpty v-else-if="!isLoading" title="暂无备份记录" compact />
		</AppCard>
	</section>
</template>

<style scoped>
.overview-tab__last-backup {
	display: flex;
	align-items: center;
	gap: var(--space-2);
	flex-wrap: wrap;
}

.overview-tab__task {
	display: grid;
	gap: var(--space-3);
	padding: var(--space-4);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-elevated) 92%, var(--surface-panel-solid));
}

.overview-tab__task-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: var(--space-3);
}
</style>
```

### 2.3 KPI CSS 复用说明

`page-kpi-grid` 和 `page-kpi__value` 这两个 CSS 类在 `DashboardView.vue` 中已有使用，应该已存在于全局 `application.css` 中。确认这些类存在。如果只在 DashboardView 的 scoped 样式中定义，则需要将其移到全局。

检查方法：在 `web/src/styles/application.css` 中搜索 `page-kpi-grid`。如果不存在，需要添加：

```css
.page-kpi-grid {
	display: grid;
	grid-template-columns: repeat(auto-fit, minmax(10rem, 1fr));
	gap: var(--space-4);
}

.page-kpi__value {
	margin: 0;
	color: var(--text-strong);
	font-size: clamp(1.5rem, 2.5vw, 2.2rem);
	font-weight: 800;
	letter-spacing: -0.04em;
	line-height: 1;
}
```

---

## 3. StrategiesTab 重构为全宽 + Modal

**文件：** `web/src/views/instance/StrategiesTab.vue`（280 行）

**当前状态：** `page-two-column` 双栏（左列表 + 右表单）

**目标状态：** 全宽策略表格 + Modal 新建/编辑策略 + 删除确认 Dialog

### 3.1 新增导入

```typescript
import AppDialog from "../../components/ui/AppDialog.vue"
import AppModal from "../../components/ui/AppModal.vue"
```

### 3.2 新增状态变量

```typescript
const modalOpen = ref(false)
const deleteDialogOpen = ref(false)
const deleteStrategyId = ref<number | null>(null)
const deleteStrategyName = ref("")
```

### 3.3 新增方法

```typescript
function openCreateModal(): void {
	resetForm()
	modalOpen.value = true
}

function openEditModal(strategy: StrategySummary): void {
	form.id = String(strategy.id)
	form.name = strategy.name
	form.backupType = strategy.backup_type
	form.scheduleMode = strategy.cron_expr ? "cron" : "interval"
	form.cronExpr = strategy.cron_expr ?? ""
	form.intervalSeconds = String(strategy.interval_seconds)
	form.retentionDays = String(strategy.retention_days)
	form.retentionCount = String(strategy.retention_count)
	form.coldVolumeSize = strategy.cold_volume_size ?? ""
	form.maxExecutionSeconds = String(strategy.max_execution_seconds)
	form.enabled = strategy.enabled
	selectedTargetIds.value = strategy.storage_target_ids.map((id) => String(id))
	formError.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}
	modalOpen.value = false
}

function openDeleteDialog(strategy: StrategySummary): void {
	deleteStrategyId.value = strategy.id
	deleteStrategyName.value = strategy.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteStrategyId.value = null
	deleteStrategyName.value = ""
}

async function confirmDelete(): Promise<void> {
	if (deleteStrategyId.value === null) {
		return
	}

	try {
		await deleteStrategy(deleteStrategyId.value)
		successMessage.value = `策略「${deleteStrategyName.value}」已删除。`
		closeDeleteDialog()
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除策略失败。"
		closeDeleteDialog()
	}
}
```

### 3.4 修改 submitForm

成功后关闭 Modal：在 `resetForm()` 之前加 `modalOpen.value = false`。

### 3.5 删除旧的 `editStrategy` 和 `removeStrategy` 方法

用 `openEditModal` 和 `openDeleteDialog` + `confirmDelete` 替代。

### 3.6 重写 template

```vue
<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="策略加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="策略已保存" tone="success" :description="successMessage" />

		<AppCard title="策略列表" description="每个策略绑定备份类型、调度和目标集合。">
			<template #header-actions>
				<AppButton size="sm" @click="openCreateModal">新建策略</AppButton>
			</template>

			<AppTable
				:rows="strategies"
				:columns="[
					{ key: 'name', label: '名称' },
					{ key: 'backup_type', label: '类型' },
					{ key: 'schedule', label: '调度' },
					{ key: 'storage_target_ids', label: '目标数' },
					{ key: 'enabled', label: '启用' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-backup_type="{ value }">
					<span>{{ formatBackupType(String(value)) }}</span>
				</template>
				<template #cell-schedule="{ row }">
					<span>{{ formatSchedule(row) }}</span>
				</template>
				<template #cell-storage_target_ids="{ value }">
					<span>{{ value.length }} 个目标</span>
				</template>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? "启用" : "停用" }}</AppTag>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="strategies.length === 0" title="该实例尚未配置备份策略。" compact />
		</AppCard>

		<!-- 策略 Modal -->
		<AppModal :open="modalOpen" width="36rem" @close="closeModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">{{ form.id === '' ? '新建策略' : '编辑策略' }}</h2>
					<p class="page-muted">滚动与冷备份使用不同的目标类型。</p>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="策略名称" required>
							<AppInput v-model="form.name" />
						</AppFormField>
						<AppFormField label="备份类型">
							<AppSelect
								v-model="form.backupType"
								:options="[
									{ value: 'rolling', label: '滚动备份' },
									{ value: 'cold', label: '冷备份' },
								]"
							/>
						</AppFormField>
						<AppFormField label="启用策略">
							<AppSwitch v-model="form.enabled" />
						</AppFormField>
					</div>

					<div class="page-form-grid">
						<AppFormField label="调度模式">
							<AppSelect
								v-model="form.scheduleMode"
								:options="[
									{ value: 'interval', label: '固定间隔' },
									{ value: 'cron', label: 'Cron 表达式' },
								]"
							/>
						</AppFormField>
						<AppFormField v-if="form.scheduleMode === 'interval'" label="间隔秒数">
							<AppInput v-model="form.intervalSeconds" inputmode="numeric" />
						</AppFormField>
						<AppFormField v-else label="Cron 表达式">
							<AppInput v-model="form.cronExpr" placeholder="0 0 * * *" />
						</AppFormField>
						<AppFormField label="最大执行秒数">
							<AppInput v-model="form.maxExecutionSeconds" inputmode="numeric" />
						</AppFormField>
					</div>

					<div class="page-form-grid">
						<AppFormField label="保留天数">
							<AppInput v-model="form.retentionDays" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="保留数量">
							<AppInput v-model="form.retentionCount" inputmode="numeric" />
						</AppFormField>
						<AppFormField v-if="form.backupType === 'cold'" label="冷备卷大小">
							<AppInput v-model="form.coldVolumeSize" placeholder="1G" />
						</AppFormField>
					</div>

					<div>
						<p class="page-section__title">存储目标</p>
						<p class="page-muted">仅展示与当前备份类型兼容的目标。</p>
						<div class="page-checkbox-list">
							<label v-for="target in compatibleTargets" :key="target.id" class="page-checkbox">
								<input
									type="checkbox"
									:checked="selectedTargetIds.includes(String(target.id))"
									@change="toggleTarget(String(target.id))"
								/>
								<span>{{ target.name }} · {{ target.type }}</span>
							</label>
						</div>
					</div>

					<AppNotification v-if="formError" title="保存失败" tone="danger" :description="formError" />

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? "创建策略" : "保存修改" }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<!-- 删除确认 Dialog -->
		<AppDialog :open="deleteDialogOpen" title="确认删除策略" tone="danger" @close="closeDeleteDialog">
			<p>即将删除策略「{{ deleteStrategyName }}」。该策略下的历史备份记录不会受影响。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>
```

**关于 `#header-actions` slot：** 检查 `AppCard` 组件是否支持该 slot。如果不支持，将「新建策略」按钮放在 `<AppCard>` 上方或在 `<section>` 的顶部独立放置：

```vue
<div class="page-action-row--wrap" style="justify-content: flex-end;">
	<AppButton @click="openCreateModal">新建策略</AppButton>
</div>
<AppCard title="策略列表" ...>
```

### 3.7 添加 Modal CSS

如果 `page-modal-form` 已在 Phase 2 抽取到全局 `application.css`，此处无需重复。如果尚未抽取，在 scoped 中添加。

---

## 4. RestoreTab 重构为全宽 + Modal

**文件：** `web/src/views/instance/RestoreTab.vue`（256 行）

**当前状态：** `page-two-column`（左恢复参数表单 + 右恢复记录）+ 确认 Dialog

**目标状态：** 全宽恢复记录表格 + 顶部「发起恢复」按钮 + Modal 表单（含内嵌确认/密码验证步骤）

### 4.1 修改概述

1. 将恢复参数表单和确认步骤合并到一个 Modal 中
2. 恢复记录表格占据全宽
3. 「发起恢复」按钮放在标题旁
4. Modal 流程：选择快照 → 填恢复路径 → 覆盖开关 → 风险警告 → 输入密码确认 → 提交

### 4.2 重写 template

```vue
<template>
	<section class="page-view">
		<AppNotification
			v-if="props.relayMode"
			:title="props.relayModeTitle"
			tone="warning"
			:description="props.relayModeHint || '所选实例存在远程到远程链路。恢复执行时会通过本机缓存目录中继，请确认剩余空间充足。'"
		/>
		<AppNotification v-if="errorMessage" title="恢复页面加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="恢复任务已创建" tone="success" :description="successMessage" />

		<AppCard title="恢复记录" description="显示当前实例的恢复任务记录。">
			<template #header-actions>
				<AppButton size="sm" variant="danger" :disabled="snapshots.length === 0" @click="openRestoreModal">发起恢复</AppButton>
			</template>

			<AppTable
				:rows="restoreRecords"
				:columns="[
					{ key: 'backup_record_id', label: '恢复源' },
					{ key: 'restore_target_path', label: '目标路径' },
					{ key: 'overwrite', label: '覆盖' },
					{ key: 'status', label: '状态' },
					{ key: 'started_at', label: '开始时间' },
				]"
				row-key="id"
			>
				<template #cell-overwrite="{ value }">
					<AppTag :tone="value ? 'danger' : 'info'">{{ value ? "覆盖" : "新位置" }}</AppTag>
				</template>
				<template #cell-status="{ value }">
					<AppTag :tone="statusTone(String(value))">{{ formatStatusLabel(String(value)) }}</AppTag>
				</template>
				<template #cell-started_at="{ row }">
					<div class="page-stack">
						<span>{{ formatDateTime(row.started_at) }}</span>
						<span class="page-muted">完成 {{ formatDateTime(row.finished_at) }}</span>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="restoreRecords.length === 0" title="尚无恢复记录" description="点击「发起恢复」按钮从备份快照恢复数据。" compact />
		</AppCard>

		<!-- 恢复 Modal -->
		<AppModal :open="restoreModalOpen" width="34rem" @close="closeRestoreModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">发起恢复</h2>
					<p class="page-muted">选择可恢复快照，指定目标路径并确认覆盖语义。</p>
				</header>

				<form class="page-stack" @submit.prevent="submitRestore">
					<AppFormField label="快照 / 归档" required>
						<AppSelect v-model="form.backupRecordId" :options="snapshotOptions" />
					</AppFormField>

					<AppFormField label="恢复目标路径" required>
						<AppInput v-model="form.restoreTargetPath" />
					</AppFormField>

					<AppFormField label="覆盖原位置">
						<AppSwitch v-model="form.overwrite" />
					</AppFormField>

					<AppNotification
						title="风险提示"
						tone="danger"
						:description="riskMessage"
					/>

					<div v-if="selectedSnapshot" class="page-stack">
						<p class="page-section__title">所选恢复源</p>
						<div class="page-action-row--wrap">
							<AppTag :tone="statusTone(selectedSnapshot.status)">{{ formatStatusLabel(selectedSnapshot.status) }}</AppTag>
							<AppTag tone="info">{{ formatStorageTargetContext(selectedSnapshot.storage_target_id) }}</AppTag>
							<span class="page-muted">{{ formatDateTime(selectedSnapshot.started_at) }}</span>
						</div>
						<p class="page-mono">{{ selectedSnapshot.snapshot_path }}</p>
					</div>

					<AppFormField label="确认密码" required description="输入当前账户密码以验证身份。">
						<AppPasswordInput v-model="password" autocomplete="current-password" />
					</AppFormField>

					<AppNotification v-if="verifyError" title="恢复未提交" tone="danger" :description="verifyError" />

					<div class="page-action-row--wrap">
						<AppButton
							type="submit"
							variant="danger"
							:loading="isSubmitting"
							:disabled="form.backupRecordId === '' || form.restoreTargetPath.trim() === '' || password.trim() === ''"
						>
							确认恢复
						</AppButton>
						<AppButton type="button" variant="ghost" @click="closeRestoreModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>
	</section>
</template>
```

### 4.3 script 修改

新增状态和方法：

```typescript
const restoreModalOpen = ref(false)

function openRestoreModal(): void {
	if (form.backupRecordId === "" && snapshots.value.length > 0) {
		form.backupRecordId = String(snapshots.value[0].id)
	}
	form.restoreTargetPath = props.instance.source_path
	form.overwrite = true
	password.value = ""
	verifyError.value = ""
	restoreModalOpen.value = true
}

function closeRestoreModal(): void {
	if (isSubmitting.value) {
		return
	}
	restoreModalOpen.value = false
	password.value = ""
	verifyError.value = ""
}
```

修改 `submitRestore`：成功后关闭 Modal 并清理状态：

```typescript
async function submitRestore(): Promise<void> {
	verifyError.value = ""
	isSubmitting.value = true

	try {
		const verification = await verifyPassword(password.value)
		const createdRecord = await startRestore({
			instance_id: props.instanceId,
			backup_record_id: Number.parseInt(form.backupRecordId, 10),
			restore_target_path: form.restoreTargetPath.trim(),
			overwrite: form.overwrite,
			verify_token: verification.verify_token,
		})

		successMessage.value = `恢复任务已提交，记录 ID ${createdRecord.id}。`
		restoreModalOpen.value = false
		password.value = ""
		await loadData()
	} catch (error) {
		verifyError.value = error instanceof ApiError ? error.message : "恢复提交失败。"
	} finally {
		isSubmitting.value = false
	}
}
```

移除旧的 `confirmOpen`、`openConfirm`、`closeConfirm` 和旧 `AppDialog` 使用。新版本将确认步骤（密码输入）内嵌到 Modal 底部，不再需要单独的确认 Dialog。

---

## 5. SubscriptionsTab 重构为全宽 + Modal

**文件：** `web/src/views/instance/SubscriptionsTab.vue`（208 行）

**当前状态：** `page-two-column`（左订阅列表 + 右订阅表单）

**目标状态：** 全宽订阅表格 + Modal 添加/编辑订阅 + 删除确认 Dialog

### 5.1 新增导入

```typescript
import AppDialog from "../../components/ui/AppDialog.vue"
import AppModal from "../../components/ui/AppModal.vue"
```

### 5.2 新增状态

```typescript
const modalOpen = ref(false)
const deleteDialogOpen = ref(false)
const deleteSubscriptionId = ref<number | null>(null)
```

### 5.3 新增方法

```typescript
function openCreateModal(): void {
	form.channelId = channels.value.length > 0 ? String(channels.value[0].id) : ""
	form.recipientEmail = ""
	form.enabled = true
	selectedEvents.value = ["backup_success", "backup_failed"]
	modalOpen.value = true
}

function openEditModal(subscription: NotificationSubscription): void {
	form.channelId = String(subscription.channel_id)
	form.recipientEmail = extractRecipientEmail(subscription.channel_config)
	form.enabled = subscription.enabled
	selectedEvents.value = [...subscription.events]
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}
	modalOpen.value = false
}

function openDeleteDialog(subscriptionId: number): void {
	deleteSubscriptionId.value = subscriptionId
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteSubscriptionId.value = null
}

async function confirmDelete(): Promise<void> {
	if (deleteSubscriptionId.value === null) {
		return
	}

	try {
		await deleteSubscription(deleteSubscriptionId.value)
		successMessage.value = "通知订阅已删除。"
		closeDeleteDialog()
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除通知订阅失败。"
		closeDeleteDialog()
	}
}
```

### 5.4 修改 submitForm

成功后关闭 Modal：

```typescript
async function submitForm(): Promise<void> {
	isSubmitting.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await upsertSubscription(props.instanceId, {
			channel_id: Number.parseInt(form.channelId, 10),
			events: selectedEvents.value,
			channel_config: { email: form.recipientEmail.trim() },
			enabled: form.enabled,
		})
		successMessage.value = "通知订阅已保存。"
		modalOpen.value = false
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存通知订阅失败。"
	} finally {
		isSubmitting.value = false
	}
}
```

### 5.5 重写 template

```vue
<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="通知订阅加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="订阅已更新" tone="success" :description="successMessage" />

		<AppCard title="当前订阅" description="订阅属于当前登录用户与当前实例。">
			<template #header-actions>
				<AppButton size="sm" :disabled="channels.length === 0" @click="openCreateModal">添加订阅</AppButton>
			</template>

			<AppTable
				:rows="subscriptions"
				:columns="[
					{ key: 'channel', label: '渠道' },
					{ key: 'events', label: '事件' },
					{ key: 'channel_config', label: '接收配置' },
					{ key: 'enabled', label: '状态' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-channel="{ row }">
					<span>{{ row.channel.name }}</span>
				</template>
				<template #cell-events="{ value }">
					<span>{{ value.join('，') }}</span>
				</template>
				<template #cell-channel_config="{ value }">
					<span>{{ extractRecipientEmail(value) || '—' }}</span>
				</template>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? '启用' : '停用' }}</AppTag>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row.id)">删除</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="subscriptions.length === 0" title="当前没有订阅" description="点击「添加订阅」按钮为该实例配置事件通知。" compact />
		</AppCard>

		<!-- 订阅 Modal -->
		<AppModal :open="modalOpen" width="32rem" @close="closeModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">{{ form.channelId === '' ? '添加订阅' : '编辑订阅' }}</h2>
				</header>

				<AppEmpty
					v-if="channels.length === 0"
					title="暂无可用通知渠道"
					description="请先在系统管理 > 通知渠道中创建并启用至少一个 SMTP 渠道。"
					compact
				/>

				<form v-else class="page-stack" @submit.prevent="submitForm">
					<AppFormField label="通知渠道" required>
						<AppSelect v-model="form.channelId" :options="channelOptions" />
					</AppFormField>

					<AppFormField label="收件邮箱" required>
						<AppInput v-model="form.recipientEmail" placeholder="ops@example.com" />
					</AppFormField>

					<div>
						<p class="page-section__title">订阅事件</p>
						<div class="page-checkbox-list">
							<label v-for="event in supportedEvents" :key="event.value" class="page-checkbox">
								<input type="checkbox" :checked="selectedEvents.includes(event.value)" @change="toggleEvent(event.value)" />
								<span>{{ event.label }}</span>
							</label>
						</div>
					</div>

					<AppFormField label="启用订阅">
						<AppSwitch v-model="form.enabled" />
					</AppFormField>

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">保存订阅</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<!-- 删除确认 Dialog -->
		<AppDialog :open="deleteDialogOpen" title="确认删除订阅" tone="danger" @close="closeDeleteDialog">
			<p>删除该订阅后将不再接收对应事件的通知。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>
```

### 5.6 关于 `#header-actions` slot

同 StrategiesTab，如果 `AppCard` 不支持 `#header-actions` slot，改为在卡片上方放置按钮行。

---

## 6. 验证与提交

1. 确认编译通过：

```bash
npm --prefix web run build
```

2. 提交：

```bash
git add -A
git commit -m "refactor(web): Phase 4 — instance detail tabs (overview V3-C, strategies/restore/subscriptions Modal)"
```

---

## 7. 启动服务并测试

启动完整服务：

```bash
make run
```

同时启动前端开发服务器：

```bash
npm --prefix web run dev
```

然后使用 `askQuestion` 工具向用户提出以下测试问题：

**问题标题：** Phase 4 实例详情页测试

**测试清单（请用户逐项确认）：**

1. **角色控制（管理员）** — 管理员登录后，实例详情页是否显示 5 个 Tab（概览、策略、备份历史、恢复、通知订阅）？
2. **角色控制（普通用户）** — 普通用户登录后，实例详情页是否只显示 2 个 Tab（概览、备份历史）？
3. **概览 Tab — KPI** — 概览 Tab 顶部是否显示 5 个 KPI 卡片（活跃策略、备份总数、累计容量、成功率、最近备份状态+时间）？
4. **概览 Tab — 双栏** — 是否有「源配置」定义列表和「当前运行任务」两列？无运行任务时是否显示空态？
5. **概览 Tab — 时间线** — 底部是否有「最近活动」时间线，显示最近备份记录？失败记录是否显示错误信息？
6. **策略 Tab — 全宽** — 策略 Tab 是否为全宽表格布局（无右侧表单）？
7. **策略 Tab — Modal** — 点击「新建策略」是否弹出 Modal？点击表格「编辑」是否弹出预填 Modal？
8. **策略 Tab — 删除** — 点击「删除」按钮是否弹出 danger 确认 Dialog？ 
9. **恢复 Tab — 布局** — 恢复 Tab 是否为全宽恢复记录表格 + 顶部「发起恢复」按钮？
10. **恢复 Tab — Modal 流程** — 点击「发起恢复」后 Modal 是否包含快照选择、目标路径、覆盖开关、风险提示和密码确认？提交后 Modal 是否关闭且记录列表刷新？
11. **订阅 Tab — 全宽** — 通知订阅 Tab 是否为全宽表格？
12. **订阅 Tab — Modal** — 添加/编辑订阅是否通过 Modal 操作？删除是否有确认 Dialog？
13. **备份历史 Tab** — 备份历史 Tab 是否保持原有布局不变（筛选器 + 表格 + 刷新按钮）？
