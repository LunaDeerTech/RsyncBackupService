<script setup lang="ts">
import { computed } from "vue"

import type { InstanceDetail, StrategySummary } from "../../api/types"
import AppCard from "../../components/ui/AppCard.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatBackupType, formatDateTime, formatSchedule, formatSource } from "../../utils/formatters"

const props = defineProps<{
	instance: InstanceDetail
	strategies: StrategySummary[]
	relayMode: boolean
	relayModeHint?: string
	relayModeTitle?: string
}>()

const strategySummaries = computed(() =>
	props.strategies.map((strategy) => ({
		id: strategy.id,
		name: strategy.name,
		type: formatBackupType(strategy.backup_type),
		schedule: formatSchedule(strategy),
		targets: strategy.storage_target_ids.length,
		enabled: strategy.enabled,
	})),
)
</script>

<template>
	<section class="page-view">
		<AppNotification
			v-if="relayMode"
			:title="relayModeTitle || '远程到远程中继'"
			tone="warning"
			:description="relayModeHint || '该实例至少有一个远程源与远程目标的组合。执行链路会落到本机缓存目录，请检查可用磁盘空间。'"
		/>

		<section class="page-two-column">
			<AppCard title="基本信息" description="实例标识、创建时间和启用状态。">
				<dl class="page-detail-list">
					<div>
						<dt>ID</dt>
						<dd>{{ instance.id }}</dd>
					</div>
					<div>
						<dt>名称</dt>
						<dd>{{ instance.name }}</dd>
					</div>
					<div>
						<dt>创建者</dt>
						<dd>{{ instance.created_by }}</dd>
					</div>
					<div>
						<dt>创建时间</dt>
						<dd>{{ formatDateTime(instance.created_at) }}</dd>
					</div>
					<div>
						<dt>更新时间</dt>
						<dd>{{ formatDateTime(instance.updated_at) }}</dd>
					</div>
				</dl>
			</AppCard>

			<AppCard title="源配置" description="用于恢复默认目标路径和远程连接上下文。">
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
		</section>

		<AppCard title="策略摘要" description="策略的类型、调度与目标数量。">
			<ul v-if="strategySummaries.length > 0" class="page-inline-list">
				<li v-for="strategy in strategySummaries" :key="strategy.id">
					<div class="overview-tab__strategy-header">
						<strong>{{ strategy.name }}</strong>
						<AppTag :tone="strategy.enabled ? 'success' : 'warning'">{{ strategy.enabled ? "启用" : "停用" }}</AppTag>
					</div>
					<p class="page-muted">{{ strategy.type }} · {{ strategy.schedule }}</p>
					<p class="page-muted">{{ strategy.targets }} 个存储目标</p>
				</li>
			</ul>
			<p v-else class="page-muted">该实例尚未配置策略。</p>
		</AppCard>
	</section>
</template>

<style scoped>
.overview-tab__strategy-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: var(--space-3);
	flex-wrap: wrap;
}
</style>