<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../../api/client"
import {
	createNotificationChannel,
	deleteNotificationChannel,
	listNotificationChannels,
	testNotificationChannel,
	updateNotificationChannel,
} from "../../api/notifications"
import type { NotificationChannelPayload, NotificationChannelSummary } from "../../api/types"
import AppButton from "../../components/ui/AppButton.vue"
import AppCard from "../../components/ui/AppCard.vue"
import AppDialog from "../../components/ui/AppDialog.vue"
import AppEmpty from "../../components/ui/AppEmpty.vue"
import AppFormField from "../../components/ui/AppFormField.vue"
import AppInput from "../../components/ui/AppInput.vue"
import AppModal from "../../components/ui/AppModal.vue"
import AppNotification from "../../components/ui/AppNotification.vue"
import AppSwitch from "../../components/ui/AppSwitch.vue"
import AppTable from "../../components/ui/AppTable.vue"
import AppTag from "../../components/ui/AppTag.vue"
import { formatDateTime } from "../../utils/formatters"

const formModalTitleId = "system-notifications-form-modal-title"
const testModalTitleId = "system-notifications-test-modal-title"

const channels = ref<NotificationChannelSummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")

const modalOpen = ref(false)
const isSubmitting = ref(false)
const formErrorMessage = ref("")
const form = reactive({
	id: "",
	name: "",
	enabled: true,
	host: "",
	port: "587",
	username: "",
	password: "",
	from: "",
	tls: true,
})

const testModalOpen = ref(false)
const isTesting = ref(false)
const testChannelId = ref<number | null>(null)
const testEmail = ref("")
const testResult = ref("")

const deleteDialogOpen = ref(false)
const deleteChannelId = ref<number | null>(null)
const deleteChannelName = ref("")

const smtpChannels = computed(() => channels.value.filter((item) => item.type === "smtp"))
const formModalTitle = computed(() => (form.id === "" ? "新建 SMTP 渠道" : "编辑 SMTP 渠道"))

function readConfig(channel: NotificationChannelSummary): Record<string, unknown> {
	const config = channel.config
	if (config === null || Array.isArray(config) || typeof config !== "object") {
		return {}
	}

	return config as Record<string, unknown>
}

function resetForm(): void {
	form.id = ""
	form.name = ""
	form.enabled = true
	form.host = ""
	form.port = "587"
	form.username = ""
	form.password = ""
	form.from = ""
	form.tls = true
	formErrorMessage.value = ""
}

function buildPayload(): NotificationChannelPayload {
	return {
		name: form.name.trim(),
		type: "smtp",
		config: {
			host: form.host.trim(),
			port: Number.parseInt(form.port, 10) || 587,
			username: form.username.trim(),
			password: form.password.trim() === "" ? undefined : form.password.trim(),
			from: form.from.trim(),
			tls: form.tls,
		},
		enabled: form.enabled,
	}
}

async function loadChannels(): Promise<void> {
	errorMessage.value = ""

	try {
		channels.value = await listNotificationChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载通知渠道失败。"
	}
}

function openCreateModal(): void {
	resetForm()
	modalOpen.value = true
}

function openEditModal(channel: NotificationChannelSummary): void {
	const config = readConfig(channel)
	form.id = String(channel.id)
	form.name = channel.name
	form.enabled = channel.enabled
	form.host = typeof config.host === "string" ? config.host : ""
	form.port = typeof config.port === "number" ? String(config.port) : "587"
	form.username = typeof config.username === "string" ? config.username : ""
	form.password = ""
	form.from = typeof config.from === "string" ? config.from : ""
	form.tls = typeof config.tls === "boolean" ? config.tls : true
	formErrorMessage.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}

	formErrorMessage.value = ""
	modalOpen.value = false
}

async function submitForm(): Promise<void> {
	isSubmitting.value = true
	formErrorMessage.value = ""
	successMessage.value = ""

	try {
		if (form.id === "") {
			await createNotificationChannel(buildPayload())
			successMessage.value = "通知渠道已创建。"
		} else {
			await updateNotificationChannel(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "通知渠道已更新。"
		}

		modalOpen.value = false
		resetForm()
		await loadChannels()
	} catch (error) {
		formErrorMessage.value = error instanceof ApiError ? error.message : "保存通知渠道失败。"
	} finally {
		isSubmitting.value = false
	}
}

function openTestModal(channelId: number): void {
	testChannelId.value = channelId
	testEmail.value = ""
	testResult.value = ""
	testModalOpen.value = true
}

function closeTestModal(): void {
	if (isTesting.value) {
		return
	}

	testModalOpen.value = false
}

async function submitTest(): Promise<void> {
	if (testChannelId.value === null || testEmail.value.trim() === "") {
		return
	}

	isTesting.value = true
	testResult.value = ""

	try {
		await testNotificationChannel(testChannelId.value, { email: testEmail.value.trim() })
		testResult.value = "测试通知已发送。"
	} catch (error) {
		testResult.value = error instanceof ApiError ? error.message : "测试通知发送失败。"
	} finally {
		isTesting.value = false
	}
}

function openDeleteDialog(channel: NotificationChannelSummary): void {
	deleteChannelId.value = channel.id
	deleteChannelName.value = channel.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteChannelId.value = null
	deleteChannelName.value = ""
	}

async function confirmDelete(): Promise<void> {
	if (deleteChannelId.value === null) {
		return
	}

	try {
		await deleteNotificationChannel(deleteChannelId.value)
		successMessage.value = `通知渠道「${deleteChannelName.value}」已删除。`
		closeDeleteDialog()
		await loadChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除通知渠道失败。"
		closeDeleteDialog()
	}
	}

onMounted(() => {
	void loadChannels()
})
</script>

<template>
	<section class="page-view">
		<AppNotification v-if="errorMessage" title="通知渠道操作失败" tone="danger" :description="errorMessage" announce />
		<AppNotification v-if="successMessage" title="通知渠道已更新" tone="success" :description="successMessage" />

		<AppCard>
			<template #header>
				<div class="system-notifications__card-header">
					<div class="system-notifications__card-heading">
						<h2 class="system-notifications__card-title">SMTP 渠道列表</h2>
						<p class="system-notifications__card-description">统一管理 SMTP 发送通道，并从独立测试弹层发起验证。</p>
					</div>
					<AppButton size="sm" @click="openCreateModal">新建渠道</AppButton>
				</div>
			</template>

			<AppTable
				:rows="smtpChannels"
				:columns="[
					{ key: 'name', label: '名称' },
					{ key: 'type', label: '类型' },
					{ key: 'enabled', label: '状态' },
					{ key: 'updated_at', label: '更新时间' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? "启用" : "停用" }}</AppTag>
				</template>
				<template #cell-updated_at="{ value }">
					<span>{{ formatDateTime(String(value)) }}</span>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<AppButton size="sm" variant="ghost" @click="openTestModal(row.id)">测试</AppButton>
						<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
					</div>
				</template>
			</AppTable>
			<AppEmpty v-if="smtpChannels.length === 0" title="暂无通知渠道" compact />
		</AppCard>

		<AppModal
			:open="modalOpen"
			:close-on-overlay="!isSubmitting"
			:labelled-by="formModalTitleId"
			width="34rem"
			@close="closeModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="formModalTitleId" class="page-modal-form__title">{{ formModalTitle }}</h2>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<AppNotification v-if="formErrorMessage" title="保存通知渠道失败" tone="danger" :description="formErrorMessage" announce />
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="smtp-main" />
						</AppFormField>
						<AppFormField label="SMTP Host" required>
							<AppInput v-model="form.host" placeholder="smtp.example.com" />
						</AppFormField>
						<AppFormField label="SMTP Port" required>
							<AppInput v-model="form.port" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="From" required>
							<AppInput v-model="form.from" placeholder="backup@example.com" />
						</AppFormField>
					</div>

					<div class="page-form-grid">
						<AppFormField label="用户名" required>
							<AppInput v-model="form.username" />
						</AppFormField>
						<AppFormField label="密码">
							<AppInput v-model="form.password" type="password" autocomplete="new-password" />
						</AppFormField>
						<AppFormField label="启用 TLS">
							<AppSwitch v-model="form.tls" />
						</AppFormField>
						<AppFormField label="启用渠道">
							<AppSwitch v-model="form.enabled" />
						</AppFormField>
					</div>

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? "创建渠道" : "保存修改" }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<AppModal
			:open="testModalOpen"
			:close-on-overlay="!isTesting"
			:labelled-by="testModalTitleId"
			width="28rem"
			@close="closeTestModal"
		>
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 :id="testModalTitleId" class="page-modal-form__title">测试通知</h2>
					<p class="page-muted">通过该渠道发送一封测试邮件。</p>
				</header>

				<div class="page-stack">
					<AppFormField label="测试收件邮箱" required>
						<AppInput v-model="testEmail" placeholder="ops@example.com" />
					</AppFormField>

					<AppNotification
						v-if="testResult"
						:title="testResult.includes('成功') || testResult.includes('已发送') ? '发送成功' : '发送失败'"
						:tone="testResult.includes('成功') || testResult.includes('已发送') ? 'success' : 'danger'"
						:description="testResult"
						announce
					/>

					<div class="page-action-row--wrap">
						<AppButton variant="secondary" :loading="isTesting" :disabled="testEmail.trim() === ''" @click="submitTest">发送测试</AppButton>
						<AppButton variant="ghost" @click="closeTestModal">关闭</AppButton>
					</div>
				</div>
			</section>
		</AppModal>

		<AppDialog :open="deleteDialogOpen" title="确认删除通知渠道" tone="danger" @close="closeDeleteDialog">
			<p>即将删除通知渠道「{{ deleteChannelName }}」。关联的订阅将无法继续发送通知。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>

<style scoped>
.system-notifications__card-header {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.system-notifications__card-heading {
	display: grid;
	gap: var(--space-2);
}

.system-notifications__card-title,
.system-notifications__card-description {
	margin: 0;
}

.system-notifications__card-title {
	color: var(--text-strong);
	font-size: 1.08rem;
	line-height: 1.15;
	letter-spacing: -0.03em;
}

.system-notifications__card-description {
	color: var(--text-muted);
	font-size: 0.92rem;
	line-height: 1.6;
}
</style>