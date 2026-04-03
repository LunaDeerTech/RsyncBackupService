<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../../api/client"
import { deleteSubscription, listNotificationChannels, listSubscriptions, upsertSubscription } from "../../api/notifications"
import type { JsonValue, NotificationChannelSummary, NotificationSubscription } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSelect from "../../components/ui/AppSelect.vue"
import AppSwitch from "../../components/ui/AppSwitch.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"

const props = defineProps<{ instanceId: number }>()

const channels = ref<NotificationChannelSummary[]>([])
const subscriptions = ref<NotificationSubscription[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const modalOpen = ref(false)
const deleteDialogOpen = ref(false)
const editingSubscriptionId = ref<number | null>(null)
const deleteSubscriptionId = ref<number | null>(null)
const isSubmitting = ref(false)
const modalTitleId = "subscriptions-tab-modal-title"
const channelSetupHint = "请先在系统管理 > 通知渠道中创建并启用至少一个 SMTP 渠道。"

const form = reactive({
	channelId: "",
	recipientEmail: "",
	enabled: true,
})

const selectedEvents = ref<string[]>(["backup_success", "backup_failed"])
const supportedEvents = [
	{ value: "backup_success", label: "备份成功" },
	{ value: "backup_failed", label: "备份失败" },
	{ value: "restore_complete", label: "恢复成功" },
	{ value: "restore_failed", label: "恢复失败" },
]

const channelOptions = computed(() => [
	{ value: "", label: "选择通知渠道" },
	...channels.value.map((item) => ({ value: String(item.id), label: `${item.name} · ${item.type}` })),
])
const isEditing = computed(() => editingSubscriptionId.value !== null)

function extractRecipientEmail(value: JsonValue): string {
	if (value === null || Array.isArray(value) || typeof value !== "object") {
		return ""
	}

	return typeof value.email === "string" ? value.email : ""
}

function toggleEvent(value: string): void {
	selectedEvents.value = selectedEvents.value.includes(value)
		? selectedEvents.value.filter((item) => item !== value)
		: [...selectedEvents.value, value]
}

function openCreateModal(): void {
	editingSubscriptionId.value = null
	form.channelId = channels.value.length > 0 ? String(channels.value[0].id) : ""
	form.recipientEmail = ""
	form.enabled = true
	selectedEvents.value = ["backup_success", "backup_failed"]
	modalOpen.value = true
}

function openEditModal(subscription: NotificationSubscription): void {
	editingSubscriptionId.value = subscription.id
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
	editingSubscriptionId.value = null
}

function openDeleteDialog(subscriptionId: number): void {
	deleteSubscriptionId.value = subscriptionId
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteSubscriptionId.value = null
}

async function loadData(): Promise<void> {
	errorMessage.value = ""

	try {
		const [channelItems, subscriptionItems] = await Promise.all([
			listNotificationChannels(),
			listSubscriptions(props.instanceId),
		])
		channels.value = channelItems
		subscriptions.value = subscriptionItems
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载通知订阅失败。"
	}
}

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
		editingSubscriptionId.value = null
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存通知订阅失败。"
	} finally {
		isSubmitting.value = false
	}
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

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="通知订阅加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="订阅已更新" tone="success" :description="successMessage" />

		<div class="page-action-row--wrap page-actions-end">
			<AppButton :disabled="channels.length === 0" @click="openCreateModal">添加订阅</AppButton>
		</div>

		<AppNotification v-if="channels.length === 0" title="暂无可用通知渠道" tone="warning" :description="channelSetupHint" />

		<AppCard title="当前订阅" description="订阅属于当前登录用户与当前实例。">
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
			<AppEmpty
				v-if="subscriptions.length === 0"
				title="当前没有订阅"
				:description="channels.length === 0 ? channelSetupHint : '点击「添加订阅」按钮为该实例配置事件通知。'"
				compact
			/>
		</AppCard>

		<AppModal
			:open="modalOpen"
			:close-on-overlay="!isSubmitting"
			:labelled-by="modalTitleId"
			width="32rem"
			@close="closeModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="modalTitleId" class="page-modal-form__title">{{ editingSubscriptionId === null ? '添加订阅' : '编辑订阅' }}</h2>
				</header>

				<AppEmpty
					v-if="channels.length === 0"
					title="暂无可用通知渠道"
					:description="channelSetupHint"
					compact
				/>

				<form v-else class="page-stack" @submit.prevent="submitForm">
					<AppFormField label="通知渠道" required>
						<AppSelect v-model="form.channelId" :options="channelOptions" :disabled="isEditing" />
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

		<AppDialog :open="deleteDialogOpen" title="确认删除订阅" tone="danger" @close="closeDeleteDialog">
			<p>删除该订阅后将不再接收对应事件的通知。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>

<style scoped>
.page-actions-end {
	justify-content: flex-end;
}
</style>