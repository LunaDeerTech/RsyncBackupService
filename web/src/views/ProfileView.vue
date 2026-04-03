<script setup lang="ts">
import { onMounted, reactive, ref } from "vue"

import { changePassword, getCurrentUser } from "../api/auth"
import { ApiError } from "../api/client"
import type { AuthUser } from "../api/types"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppPasswordInput from "../components/ui/AppPasswordInput.vue"
import AppTag from "../components/ui/AppTag.vue"
import { formatDateTime } from "../utils/formatters"

const currentUser = ref<AuthUser | null>(null)
const errorMessage = ref("")
const successMessage = ref("")
const isLoading = ref(true)

const passwordForm = reactive({
	currentPassword: "",
	newPassword: "",
})

async function loadUser(): Promise<void> {
	isLoading.value = true
	errorMessage.value = ""

	try {
		currentUser.value = await getCurrentUser()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "加载用户信息失败。"
	} finally {
		isLoading.value = false
	}
}

async function submitChangePassword(): Promise<void> {
	errorMessage.value = ""
	successMessage.value = ""

	try {
		await changePassword({
			current_password: passwordForm.currentPassword,
			new_password: passwordForm.newPassword,
		})
		successMessage.value = "密码已修改成功。"
		passwordForm.currentPassword = ""
		passwordForm.newPassword = ""
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "修改密码失败。"
	}
}

onMounted(() => {
	void loadUser()
})
</script>

<template>
	<section class="page-view">
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">PROFILE</p>
				<h1 class="page-header__title">个人信息</h1>
				<p class="page-header__subtitle">查看会话信息和修改密码。</p>
			</div>
		</header>

		<AppNotification v-if="errorMessage" title="个人信息操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="操作成功" tone="success" :description="successMessage" />

		<section class="page-two-column">
			<AppCard title="当前会话" description="确认当前登录用户信息。">
				<dl v-if="currentUser" class="page-detail-list">
					<div>
						<dt>用户名</dt>
						<dd>{{ currentUser.username }}</dd>
					</div>
					<div>
						<dt>角色</dt>
						<dd>
							<AppTag :tone="currentUser.is_admin ? 'success' : 'default'">{{ currentUser.is_admin ? "管理员" : "普通用户" }}</AppTag>
						</dd>
					</div>
					<div>
						<dt>创建时间</dt>
						<dd>{{ formatDateTime(currentUser.created_at) }}</dd>
					</div>
					<div>
						<dt>更新时间</dt>
						<dd>{{ formatDateTime(currentUser.updated_at) }}</dd>
					</div>
				</dl>
				<p v-else-if="!isLoading" class="page-muted">当前用户信息不可用。</p>
			</AppCard>

			<AppCard title="修改密码" description="修改当前登录用户的密码。">
				<form class="page-stack" @submit.prevent="submitChangePassword">
					<AppFormField label="当前密码" required>
						<AppPasswordInput v-model="passwordForm.currentPassword" autocomplete="current-password" />
					</AppFormField>
					<AppFormField label="新密码" required>
						<AppPasswordInput v-model="passwordForm.newPassword" autocomplete="new-password" />
					</AppFormField>
					<AppButton type="submit">保存新密码</AppButton>
				</form>
			</AppCard>
		</section>
	</section>
</template>