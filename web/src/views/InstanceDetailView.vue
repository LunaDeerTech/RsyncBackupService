<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import { ApiError } from "../api/client"
import { getInstanceDetail } from "../api/instances"
import { listStorageTargets } from "../api/storageTargets"
import { listStrategies } from "../api/strategies"
import type { InstanceDetail, StorageTargetSummary, StrategySummary } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppTabs from "../components/ui/AppTabs.vue"
import AppTag from "../components/ui/AppTag.vue"
import { useAuthStore } from "../stores/auth"
import { formatSource } from "../utils/formatters"
import BackupsTab from "./instance/BackupsTab.vue"
import OverviewTab from "./instance/OverviewTab.vue"
import RestoreTab from "./instance/RestoreTab.vue"
import StrategiesTab from "./instance/StrategiesTab.vue"
import SubscriptionsTab from "./instance/SubscriptionsTab.vue"

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()

const instance = ref<InstanceDetail | null>(null)
const strategies = ref<StrategySummary[]>([])
const storageTargets = ref<StorageTargetSummary[]>([])
const storageTargetsRestricted = ref(false)
const errorMessage = ref("")
const isLoading = ref(true)
const activeTab = ref("overview")

const maxStrategyPreviewRefreshDelay = 2_147_483_647
const strategyPreviewRefreshRetryDelay = 30_000

let strategyPreviewRefreshTimer: ReturnType<typeof setTimeout> | null = null
let strategyPreviewRefreshDisposed = false
let loadDetailRequestId = 0

const instanceId = computed(() => Number.parseInt(String(route.params.id ?? "0"), 10) || 0)
const isAdmin = computed(() => auth.currentUser?.is_admin === true)
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

const relayMode = computed(() => {
	if (instance.value?.source_type !== "remote") {
		return false
	}

	const targetMap = new Map(storageTargets.value.map((item) => [item.id, item]))
	return strategies.value.some((strategy) =>
		strategy.storage_target_ids.some((storageTargetId) => {
			const target = targetMap.get(storageTargetId)
			return target?.type === "rolling_ssh" || target?.type === "cold_ssh"
		}),
	)
})

const relayModePossible = computed(() => {
	if (instance.value?.source_type !== "remote" || !storageTargetsRestricted.value) {
		return false
	}

	return strategies.value.some((strategy) => strategy.storage_target_ids.length > 0)
})

const relayModeVisible = computed(() => relayMode.value || relayModePossible.value)

const relayModeTitle = computed(() => (relayMode.value ? "中继模式" : "可能经过中继缓存"))

const relayModeHint = computed(() => {
	if (!relayModeVisible.value) {
		return ""
	}

	if (relayModePossible.value) {
		return "当前账户无法读取目标类型。该实例源端位于远程主机；若策略绑定了 SSH 目标，恢复与滚动同步会经过本机缓存目录，请预留磁盘空间。"
	}

	return "该实例的源端与部分目标端都位于远程主机。恢复与滚动同步会经过本机缓存目录，请确认磁盘空间与网络带宽。"
})

const nextStrategyPreviewRunAt = computed(() => {
	const now = Date.now()
	let nextRunAt: number | null = null

	for (const strategy of strategies.value) {
		for (const runAt of strategy.upcoming_runs ?? []) {
			const timestamp = Date.parse(runAt)
			if (Number.isNaN(timestamp) || timestamp <= now) {
				continue
			}

			if (nextRunAt === null || timestamp < nextRunAt) {
				nextRunAt = timestamp
			}
		}
	}

	return nextRunAt
})

function clearStrategyPreviewRefreshTimer(): void {
	if (strategyPreviewRefreshTimer !== null) {
		clearTimeout(strategyPreviewRefreshTimer)
		strategyPreviewRefreshTimer = null
	}
}

async function refreshStrategiesPreview(): Promise<boolean> {
	const requestedInstanceId = instanceId.value

	try {
		const nextStrategies = await listStrategies(requestedInstanceId)
		if (strategyPreviewRefreshDisposed || requestedInstanceId !== instanceId.value) {
			return false
		}

		strategies.value = nextStrategies
		return true
	} catch {
		return false
	}
}

function scheduleStrategyPreviewRefresh(retryDelay?: number): void {
	if (strategyPreviewRefreshDisposed || isLoading.value || instance.value?.id !== instanceId.value) {
		return
	}

	clearStrategyPreviewRefreshTimer()
	if (retryDelay !== undefined) {
		strategyPreviewRefreshTimer = setTimeout(() => {
			scheduleStrategyPreviewRefresh()
		}, retryDelay)
		return
	}

	if (nextStrategyPreviewRunAt.value === null) {
		return
	}

	const delay = Math.max(nextStrategyPreviewRunAt.value - Date.now() + 1000, 1000)
	strategyPreviewRefreshTimer = setTimeout(() => {
		if (strategyPreviewRefreshDisposed) {
			return
		}

		if (delay > maxStrategyPreviewRefreshDelay) {
			scheduleStrategyPreviewRefresh()
			return
		}

		void refreshStrategiesPreview().then((didRefresh) => {
			if (strategyPreviewRefreshDisposed) {
				return
			}

			scheduleStrategyPreviewRefresh(didRefresh ? undefined : strategyPreviewRefreshRetryDelay)
		})
	}, Math.min(delay, maxStrategyPreviewRefreshDelay))
}

async function loadDetail(): Promise<void> {
	const requestedInstanceId = instanceId.value
	const requestId = loadDetailRequestId + 1
	loadDetailRequestId = requestId

	errorMessage.value = ""
	isLoading.value = true
	if (instance.value?.id !== requestedInstanceId) {
		instance.value = null
		strategies.value = []
		storageTargets.value = []
	}

	try {
		storageTargetsRestricted.value = false
		const [instanceItem, strategyItems, storageTargetItems] = await Promise.all([
			getInstanceDetail(requestedInstanceId),
			listStrategies(requestedInstanceId),
			listStorageTargets().catch((error: unknown) => {
				if (error instanceof ApiError && error.status === 403) {
					storageTargetsRestricted.value = true
					return []
				}

				throw error
			}),
		])

		if (
			strategyPreviewRefreshDisposed ||
			requestId !== loadDetailRequestId ||
			requestedInstanceId !== instanceId.value
		) {
			return
		}

		instance.value = instanceItem
		strategies.value = strategyItems
		storageTargets.value = storageTargetItems
	} catch (error) {
		if (
			strategyPreviewRefreshDisposed ||
			requestId !== loadDetailRequestId ||
			requestedInstanceId !== instanceId.value
		) {
			return
		}

		errorMessage.value = error instanceof ApiError ? error.message : "加载实例详情失败。"
	} finally {
		if (
			strategyPreviewRefreshDisposed ||
			requestId !== loadDetailRequestId ||
			requestedInstanceId !== instanceId.value
		) {
			return
		}

		isLoading.value = false
	}
}

function goBack(): void {
	void router.push("/instances")
}

onMounted(() => {
	void loadDetail()
})

watch(instanceId, () => {
	clearStrategyPreviewRefreshTimer()
	void loadDetail()
})

watch([strategies, instanceId, isLoading, () => instance.value?.id ?? 0], () => {
	scheduleStrategyPreviewRefresh()
})

watch(
	[tabs, activeTab],
	([nextTabs, nextActiveTab]) => {
		if (!nextTabs.some((tab) => tab.value === nextActiveTab)) {
			activeTab.value = nextTabs[0]?.value ?? "overview"
		}
	},
	{ immediate: true },
)

onBeforeUnmount(() => {
	strategyPreviewRefreshDisposed = true
	clearStrategyPreviewRefreshTimer()
})
</script>

<template>
	<section v-if="instance" class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">INSTANCE</p>
				<h1 class="page-header__title">{{ instance.name }}</h1>
				<p class="page-header__subtitle">{{ formatSource(instance.source_type, instance.source_path, instance.source_host) }}</p>
			</div>
			<div class="page-header__actions">
				<AppTag :tone="instance.enabled ? 'success' : 'warning'">{{ instance.enabled ? "已启用" : "已停用" }}</AppTag>
				<AppButton variant="secondary" @click="loadDetail">刷新</AppButton>
				<AppButton variant="ghost" @click="goBack">返回实例列表</AppButton>
			</div>
		</header>

		<AppTabs v-model="activeTab" :tabs="tabs" aria-label="实例详情标签" />

		<OverviewTab
			v-if="activeTab === 'overview'"
			:instance="instance"
			:strategies="strategies"
			:can-view-running-tasks="isAdmin"
			:relay-mode="relayModeVisible"
			:relay-mode-hint="relayModeHint"
			:relay-mode-title="relayModeTitle"
		/>
		<StrategiesTab v-else-if="activeTab === 'strategies'" :instance-id="instance.id" />
		<BackupsTab v-else-if="activeTab === 'backups'" :instance-id="instance.id" />
		<RestoreTab
			v-else-if="activeTab === 'restore'"
			:instance-id="instance.id"
			:instance="instance"
			:relay-mode="relayModeVisible"
			:relay-mode-hint="relayModeHint"
			:relay-mode-title="relayModeTitle"
		/>
		<SubscriptionsTab v-else-if="activeTab === 'subscriptions'" :instance-id="instance.id" />
	</section>

	<section v-else class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">INSTANCE</p>
				<h1 class="page-header__title">实例详情</h1>
				<p class="page-header__subtitle">载入实例、策略与恢复上下文。</p>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="实例详情加载失败" tone="danger" :description="errorMessage" />
		<p v-else-if="isLoading" class="page-muted">正在加载实例详情…</p>
	</section>
</template>