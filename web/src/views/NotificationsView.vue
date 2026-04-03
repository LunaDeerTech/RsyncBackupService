<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue"

import { ApiError } from "../api/client"
import {
	createNotificationChannel,
	deleteNotificationChannel,
	listNotificationChannels,
	testNotificationChannel,
	updateNotificationChannel,
} from "../api/notifications"
import type { NotificationChannelPayload, NotificationChannelSummary } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppEmpty from "../components/ui/AppEmpty.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppSwitch from "../components/ui/AppSwitch.vue"
import AppTable from "../components/ui/AppTable.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime } from "../utils/formatters"

type SMTPConfigForm = {
	host: string
	port: string
	username: string
	password: string
	from: string
	tls: boolean
}

const channels = ref<NotificationChannelSummary[]>([])
const errorMessage = ref("")
const successMessage = ref("")
const isSubmitting = ref(false)
const isTesting = ref(false)

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
	testEmail: "",
})

function readConfig(channel: NotificationChannelSummary): Partial<SMTPConfigForm> {
	const config = channel.config
	if (config === null || Array.isArray(config) || typeof config !== "object") {
		return {}
	}

	return {
		host: typeof config.host === "string" ? config.host : "",
		port: typeof config.port === "number" ? String(config.port) : "587",
		username: typeof config.username === "string" ? config.username : "",
		password: "",
		from: typeof config.from === "string" ? config.from : "",
		tls: typeof config.tls === "boolean" ? config.tls : true,
	}
}

const smtpChannels = computed(() => channels.value.filter((item) => item.type === "smtp"))

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
	form.testEmail = ""
}

function editChannel(channel: NotificationChannelSummary): void {
	const config = readConfig(channel)
	form.id = String(channel.id)
	form.name = channel.name
	form.enabled = channel.enabled
	form.host = config.host ?? ""
	form.port = config.port ?? "587"
	form.username = config.username ?? ""
	form.password = ""
	form.from = config.from ?? ""
	form.tls = config.tls ?? true
	window.scrollTo({ top: 0, behavior: "smooth" })
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

async function submitForm(): Promise<void> {
	isSubmitting.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		if (form.id === "") {
			await createNotificationChannel(buildPayload())
			successMessage.value = "通知渠道已创建。"
		} else {
			await updateNotificationChannel(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "通知渠道已更新。"
		}

		resetForm()
		await loadChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "保存通知渠道失败。"
	} finally {
		isSubmitting.value = false
	}
}

async function handleTest(channelId: number): Promise<void> {
	isTesting.value = true
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await testNotificationChannel(channelId, {
			email: form.testEmail.trim(),
		})
		successMessage.value = "测试通知已发送。"
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "测试通知发送失败。"
	} finally {
		isTesting.value = false
	}
}

async function handleDelete(channelId: number): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await deleteNotificationChannel(channelId)
		successMessage.value = `通知渠道 ${channelId} 已删除。`
		await loadChannels()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除通知渠道失败。"
	}
}

onMounted(() => {
	void loadChannels()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">NOTIFICATIONS</p>
				<h1 class="page-header__title">通知渠道</h1>
				<p class="page-header__subtitle">配置 SMTP 渠道，并用单独的收件配置发送测试通知。</p>
			</div>
			<div class="page-header__actions">
				<AppButton variant="secondary" @click="resetForm">新建渠道</AppButton>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="通知渠道操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="通知渠道已更新" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<AppCard title="SMTP 渠道列表" description="非管理员仅能在实例订阅页看到已启用渠道，这里展示完整配置摘要。">
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
						<AppTag :tone="value ? 'success' : 'warning'">{{ value ? '启用' : '停用' }}</AppTag>
					</template>
					<template #cell-updated_at="{ value }">
						<span>{{ formatDateTime(String(value)) }}</span>
					</template>
					<template #cell-actions="{ row }">
						<div class="page-action-row--wrap">
							<AppButton size="sm" variant="secondary" @click="editChannel(row)">编辑</AppButton>
							<AppButton size="sm" variant="ghost" @click="handleDelete(row.id)">删除</AppButton>
						</div>
					</template>
				</AppTable>
				<AppEmpty v-if="smtpChannels.length === 0" title="暂无通知渠道" compact />
			</AppCard>

			<div class="page-stack">
				<AppCard :title="form.id === '' ? '新建 SMTP 渠道' : '编辑 SMTP 渠道'" description="页面层只暴露 SMTP 需要的核心配置，不扩展额外通知后端。">
					<form class="page-stack" @submit.prevent="submitForm">
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
							<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? '创建渠道' : '保存修改' }}</AppButton>
							<AppButton type="button" variant="ghost" @click="resetForm">重置</AppButton>
						</div>
					</form>
				</AppCard>

				<AppCard title="测试通知" description="通过渠道配置发送一封测试邮件，接收方配置独立于渠道本身。">
					<div class="page-stack">
						<AppFormField label="测试收件邮箱" required>
							<AppInput v-model="form.testEmail" placeholder="ops@example.com" />
						</AppFormField>
						<div class="page-action-row--wrap">
							<AppButton
								variant="secondary"
								:loading="isTesting"
								:disabled="form.id === '' || form.testEmail.trim() === ''"
								@click="handleTest(Number.parseInt(form.id, 10))"
							>
								发送测试通知
							</AppButton>
						</div>
					</div>
				</AppCard>
			</div>
		</section>
	</section>
</template>