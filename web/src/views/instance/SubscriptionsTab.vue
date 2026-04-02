<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from "vue"

import { ApiError } from "../../api/client"
import { deleteSubscription, listNotificationChannels, listSubscriptions, upsertSubscription } from "../../api/notifications"
import type { JsonValue, NotificationChannelSummary, NotificationSubscription } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
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
const isSubmitting = ref(false)

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

function extractRecipientEmail(value: JsonValue): string {
	if (value === null || Array.isArray(value) || typeof value !== "object") {
		return ""
	}

	return typeof value.email === "string" ? value.email : ""
}

function syncFormFromSubscription(channelId: string): void {
	const subscription = subscriptions.value.find((item) => String(item.channel_id) === channelId)
	if (!subscription) {
		form.recipientEmail = ""
		form.enabled = true
		selectedEvents.value = ["backup_success", "backup_failed"]
		return
	}

	form.recipientEmail = extractRecipientEmail(subscription.channel_config)
	form.enabled = subscription.enabled
	selectedEvents.value = [...subscription.events]
}

function toggleEvent(value: string): void {
	selectedEvents.value = selectedEvents.value.includes(value)
		? selectedEvents.value.filter((item) => item !== value)
		: [...selectedEvents.value, value]
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

		if (form.channelId === "" && channelItems.length > 0) {
			form.channelId = String(channelItems[0].id)
		}
		syncFormFromSubscription(form.channelId)
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
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存通知订阅失败。"
	} finally {
		isSubmitting.value = false
	}
}

async function removeSubscription(subscriptionId: number): Promise<void> {
	try {
		await deleteSubscription(subscriptionId)
		successMessage.value = "通知订阅已删除。"
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除通知订阅失败。"
	}
}

watch(
	() => form.channelId,
	(value) => {
		syncFormFromSubscription(value)
	},
)

onMounted(() => {
	void loadData()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="通知订阅加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="订阅已更新" tone="success" :description="successMessage" />

		<section class="page-two-column">
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
						<AppButton size="sm" variant="ghost" @click="removeSubscription(row.id)">删除</AppButton>
					</template>
				</AppTable>
				<AppEmpty v-if="subscriptions.length === 0" title="当前没有订阅" compact />
			</AppCard>

			<AppCard title="添加或更新订阅" description="为当前实例选择事件并填写 SMTP 收件地址。">
				<AppEmpty
					v-if="channels.length === 0"
					title="暂无可用通知渠道"
					description="请先在通知渠道页面创建并启用至少一个 SMTP 渠道。"
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

					<AppButton type="submit" :loading="isSubmitting">保存订阅</AppButton>
				</form>
			</AppCard>
		</section>
	</section>
</template>