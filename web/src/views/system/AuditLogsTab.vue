<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { listAuditLogs } from "../../api/audit"
import { ApiError } from "../../api/client"
import { listUsers } from "../../api/users"
import type { AuditLogQuery, AuditLogItem, UserSummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatDateTime, stringifyJson } from "../../utils/formatters"

const logs = ref<AuditLogItem[]>([])
const users = ref<UserSummary[]>([])
const total = ref(0)
const errorMessage = ref("")
const isLoading = ref(false)

const filters = reactive({
	action: "",
	resourceType: "",
	userId: "",
	startTime: "",
	endTime: "",
	page: 1,
})

const userOptions = computed(() => [
	{ value: "", label: "全部用户" },
	...users.value.map((item) => ({ value: String(item.id), label: item.username })),
])

function toISODateTime(value: string): string | undefined {
	const trimmed = value.trim()
	if (trimmed === "") {
		return undefined
	}

	const date = new Date(trimmed)
	return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function hasExplicitFilters(): boolean {
	return (
		filters.action.trim() !== ""
		|| filters.resourceType.trim() !== ""
		|| filters.userId !== ""
		|| filters.startTime.trim() !== ""
		|| filters.endTime.trim() !== ""
	)
	}

function buildQuery(): AuditLogQuery {
	if (!hasExplicitFilters()) {
		return {
			page: filters.page,
			page_size: 20,
		}
	}

	return {
		action: filters.action.trim() || undefined,
		resource_type: filters.resourceType.trim() || undefined,
		user_id: filters.userId === "" ? undefined : Number.parseInt(filters.userId, 10),
		start_time: toISODateTime(filters.startTime),
		end_time: toISODateTime(filters.endTime),
		page: filters.page,
		page_size: 20,
	}
	}

async function loadData(): Promise<void> {
	isLoading.value = true
	errorMessage.value = ""

	try {
		const [response, userItems] = await Promise.all([listAuditLogs(buildQuery()), listUsers()])
		logs.value = response.items
		total.value = response.total
		users.value = userItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载审计日志失败。"
	} finally {
		isLoading.value = false
	}
	}

function resetFilters(): void {
	filters.action = ""
	filters.resourceType = ""
	filters.userId = ""
	filters.startTime = ""
	filters.endTime = ""
	filters.page = 1
	void loadData()
	}

function previousPage(): void {
	if (filters.page <= 1) {
		return
	}

	filters.page -= 1
	void loadData()
	}

function nextPage(): void {
	const maxPage = Math.max(1, Math.ceil(total.value / 20))
	if (filters.page >= maxPage) {
		return
	}

	filters.page += 1
	void loadData()
	}

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="审计日志加载失败" tone="danger" :description="errorMessage" />

		<AppCard title="筛选器" description="按动作、资源类型、用户和时间范围过滤。">
			<form class="page-stack" @submit.prevent="loadData">
				<div class="page-form-grid">
					<AppFormField label="动作">
						<AppInput v-model="filters.action" placeholder="instances.restore" />
					</AppFormField>
					<AppFormField label="资源类型">
						<AppInput v-model="filters.resourceType" placeholder="restore_records" />
					</AppFormField>
					<AppFormField label="用户">
						<AppSelect v-model="filters.userId" :options="userOptions" />
					</AppFormField>
					<AppFormField label="开始时间">
						<AppInput v-model="filters.startTime" type="datetime-local" />
					</AppFormField>
					<AppFormField label="结束时间">
						<AppInput v-model="filters.endTime" type="datetime-local" />
					</AppFormField>
				</div>
				<div class="page-action-row--wrap">
					<AppButton type="submit" :loading="isLoading">应用筛选</AppButton>
					<AppButton type="button" variant="ghost" @click="resetFilters">重置</AppButton>
				</div>
			</form>
		</AppCard>

		<AppCard title="日志表格" :description="`当前第 ${filters.page} 页，共 ${total} 条记录。`">
			<AppTable
				:rows="logs"
				:columns="[
					{ key: 'created_at', label: '时间' },
					{ key: 'username', label: '用户' },
					{ key: 'action', label: '动作' },
					{ key: 'resource_type', label: '资源' },
					{ key: 'detail', label: '详情' },
				]"
				row-key="id"
			>
				<template #cell-created_at="{ value }">
					<span>{{ formatDateTime(String(value)) }}</span>
				</template>
				<template #cell-action="{ value }">
					<AppTag tone="default">{{ String(value) }}</AppTag>
				</template>
				<template #cell-resource_type="{ row }">
					<div class="page-stack">
						<span>{{ row.resource_type }}</span>
						<span class="page-muted">#{{ row.resource_id }} · {{ row.ip_address }}</span>
					</div>
				</template>
				<template #cell-detail="{ value }">
					<pre class="audit-log__detail">{{ stringifyJson(value) }}</pre>
				</template>
			</AppTable>

			<AppEmpty v-if="!isLoading && logs.length === 0" title="当前没有匹配日志" compact />

			<template #footer>
				<div class="page-action-row--wrap">
					<AppButton variant="ghost" :disabled="filters.page === 1" @click="previousPage">上一页</AppButton>
					<AppButton variant="secondary" :disabled="filters.page * 20 >= total" @click="nextPage">下一页</AppButton>
				</div>
			</template>
		</AppCard>
	</section>
</template>

<style scoped>
.audit-log__detail {
	margin: 0;
	padding: 0.75rem 0.85rem;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-control);
	background: color-mix(in srgb, var(--surface-elevated) 94%, var(--surface-panel-solid));
	color: var(--text-strong);
	font-family: "JetBrains Mono", "Fira Code", ui-monospace, monospace;
	font-size: 0.8rem;
	line-height: 1.55;
	white-space: pre-wrap;
	word-break: break-word;
}
</style>