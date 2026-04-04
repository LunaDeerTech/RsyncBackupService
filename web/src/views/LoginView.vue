<script setup lang="ts">
import { computed, reactive, ref } from "vue"
import { useRoute, useRouter } from "vue-router"

import { ApiError } from "../api/client"
import AppButton from "../components/ui/AppButton.vue"
import AppCard from "../components/ui/AppCard.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppNotification from "../components/ui/AppNotification.vue"
import AppPasswordInput from "../components/ui/AppPasswordInput.vue"
import { useSession } from "../composables/useSession"
import { useUiStore } from "../stores/ui"

const route = useRoute()
const router = useRouter()
const session = useSession()
const ui = useUiStore()

const form = reactive({
	username: "",
	password: "",
})

const errorMessage = ref("")
const isSubmitting = ref(false)

const redirectTarget = computed(() => {
	const redirect = route.query.redirect
	return typeof redirect === "string" && redirect.startsWith("/") ? redirect : "/"
})

const themeActionLabel = computed(() =>
	ui.theme === "light" ? "切换到深色主题" : "切换到浅色主题",
)

function toggleTheme(): void {
	ui.setTheme(ui.theme === "light" ? "dark" : "light")
}

async function handleSubmit(): Promise<void> {
	errorMessage.value = ""
	isSubmitting.value = true

	try {
		await session.login(form)
		await router.push(redirectTarget.value)
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "登录失败，请检查服务状态后重试。"
	} finally {
		isSubmitting.value = false
	}
}
</script>

<template>
	<div class="login-view" data-testid="login-view">
		<div class="login-view__container">
			<header class="login-view__brand">
				<h1 class="login-view__title">Rsync Backup Service</h1>
				<p class="login-view__intro">使用账户访问备份控制台。</p>
			</header>

			<AppCard class="login-view__card">
				<template #header>
					<div class="login-view__card-header">
						<div class="login-view__card-heading">
							<h2 class="login-view__card-title">登录到控制台</h2>
							<p class="login-view__card-subtitle">输入你的账户信息以继续访问受保护页面。</p>
						</div>
						<AppButton variant="ghost" size="sm" :aria-label="themeActionLabel" @click="toggleTheme">
							{{ ui.theme === "light" ? "深色主题" : "浅色主题" }}
						</AppButton>
					</div>
				</template>

				<form class="login-view__form" @submit.prevent="handleSubmit">
					<AppFormField label="用户名" required>
						<AppInput v-model="form.username" autocomplete="username" placeholder="请输入用户名" />
					</AppFormField>

					<AppFormField label="密码" required>
						<AppPasswordInput
							v-model="form.password"
							autocomplete="current-password"
							placeholder="请输入密码"
						/>
					</AppFormField>

					<AppNotification
						v-if="errorMessage"
						title="登录失败"
						tone="danger"
						announce
						:description="errorMessage"
					/>

					<AppButton type="submit" size="lg" :loading="isSubmitting">登录</AppButton>
				</form>

				<p class="login-view__hint">登录后会回到原目标页面；未指定时会进入你的默认首页。</p>
			</AppCard>
		</div>
	</div>
</template>

<style scoped>
.login-view {
	min-height: 100vh;
	display: grid;
	place-items: center;
	padding: var(--space-6);
	background:
		radial-gradient(circle at top, color-mix(in srgb, var(--primary-500) 10%, transparent), transparent 42%),
		radial-gradient(circle at bottom, color-mix(in srgb, var(--accent-mint-400) 8%, transparent), transparent 48%),
		var(--surface-base);
}

.login-view__container {
	width: min(100%, 29rem);
	display: grid;
	gap: var(--space-5);
}

.login-view__brand {
	display: grid;
	gap: var(--space-2);
	justify-items: center;
	text-align: center;
}

.login-view__title {
	margin: 0;
	color: var(--text-strong);
	font-size: clamp(1.9rem, 4vw, 2.6rem);
	line-height: 1;
	letter-spacing: -0.04em;
}

.login-view__intro,
.login-view__card-subtitle,
.login-view__hint {
	margin: 0;
	color: var(--text-muted);
	line-height: 1.6;
}

.login-view__intro {
	font-size: 0.98rem;
}

.login-view__card {
	width: 100%;
	padding: clamp(1.2rem, 3vw, 1.8rem);
	border-color: color-mix(in srgb, var(--border-default) 92%, transparent);
	background:
		linear-gradient(180deg, color-mix(in srgb, var(--surface-panel) 96%, transparent), transparent),
		var(--shell-panel-bg);
	box-shadow: var(--shell-panel-shadow);
}

.login-view__card-header {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: var(--space-4);
}

.login-view__card-heading,
.login-view__form {
	display: grid;
	gap: var(--space-3);
}

.login-view__card-title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1.48rem;
	line-height: 1.1;
	letter-spacing: -0.03em;
}

.login-view__hint {
	font-size: 0.9rem;
}

@media (max-width: 920px) {
	.login-view {
		padding: var(--space-4);
	}

	.login-view__container {
		gap: var(--space-4);
	}

	.login-view__card-header {
		align-items: flex-start;
	}
}
</style>