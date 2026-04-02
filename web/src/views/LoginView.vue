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
	username: "admin",
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
		<section class="login-view__hero">
			<div class="login-view__hero-copy">
				<p class="login-view__eyebrow">Balanced Flux Gateway</p>
				<h1 class="login-view__title">Rsync Backup Service</h1>
				<p class="login-view__subtitle">
					进入统一运维控制台，查看运行中任务、实例状态、恢复风险确认与权限设置。
				</p>
			</div>

			<ul class="login-view__highlights" aria-label="能力概览">
				<li>实时任务进度、速率与剩余时间</li>
				<li>实例详情 Tab、恢复确认与二次认证</li>
				<li>通知、审计和实例权限管理</li>
			</ul>
		</section>

		<AppCard class="login-view__card">
			<template #header>
				<div class="login-view__card-header">
					<div class="login-view__card-heading">
						<p class="login-view__card-eyebrow">Session Access</p>
						<h2 class="login-view__card-title">登录到控制台</h2>
						<p class="login-view__card-subtitle">使用现有账户进入受保护页面。</p>
					</div>
					<AppButton variant="secondary" size="sm" :aria-label="themeActionLabel" @click="toggleTheme">
						{{ ui.theme === "light" ? "深色主题" : "浅色主题" }}
					</AppButton>
				</div>
			</template>

			<form class="login-view__form" @submit.prevent="handleSubmit">
				<AppFormField label="用户名" required>
					<AppInput v-model="form.username" autocomplete="username" />
				</AppFormField>

				<AppFormField label="密码" required>
					<AppPasswordInput v-model="form.password" autocomplete="current-password" />
				</AppFormField>

				<AppNotification
					v-if="errorMessage"
					title="登录失败"
					tone="danger"
					:description="errorMessage"
				/>

				<AppButton type="submit" size="lg" :loading="isSubmitting">登录</AppButton>
			</form>

			<p class="login-view__hint">登录后会回到原目标页面；未指定时默认进入仪表盘。</p>
		</AppCard>
	</div>
</template>

<style scoped>
.login-view {
	display: grid;
	grid-template-columns: minmax(0, 1.15fr) minmax(20rem, 26rem);
	gap: var(--space-6);
	align-items: stretch;
	min-height: 100vh;
	padding: var(--space-6);
	background:
		radial-gradient(circle at top left, color-mix(in srgb, var(--primary-500) 18%, transparent), transparent 35%),
		radial-gradient(circle at bottom right, color-mix(in srgb, var(--accent-mint-400) 16%, transparent), transparent 40%),
		var(--surface-base);
}

.login-view__hero,
.login-view__card {
	align-self: stretch;
}

.login-view__hero {
	display: grid;
	gap: var(--space-6);
	padding: clamp(1.8rem, 3vw, 2.8rem);
	border: var(--border-width) solid var(--shell-panel-border);
	border-radius: var(--radius-card);
	background:
		linear-gradient(160deg, color-mix(in srgb, var(--surface-panel) 96%, transparent), transparent),
		var(--shell-panel-bg);
	box-shadow: var(--shell-panel-shadow);
	position: relative;
	overflow: hidden;
}

.login-view__hero::after {
	content: "";
	position: absolute;
	inset: auto -10% -20% 40%;
	height: 18rem;
	border-radius: 999px;
	background: var(--shell-hero-glow);
	filter: blur(24px);
	pointer-events: none;
}

.login-view__hero-copy,
.login-view__highlights {
	position: relative;
	z-index: 1;
}

.login-view__eyebrow,
.login-view__card-eyebrow,
.login-view__hint,
.login-view__card-subtitle {
	margin: 0;
	color: var(--text-muted);
}

.login-view__eyebrow,
.login-view__card-eyebrow {
	font-size: 0.78rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
}

.login-view__title {
	margin: 0;
	max-width: 12ch;
	color: var(--text-strong);
	font-size: clamp(2.4rem, 4vw, 4.1rem);
	line-height: 0.94;
	letter-spacing: -0.05em;
}

.login-view__subtitle {
	margin: 0;
	max-width: 36rem;
	color: var(--text-muted);
	font-size: 1rem;
	line-height: 1.7;
}

.login-view__highlights {
	display: grid;
	gap: var(--space-3);
	padding: 0;
	margin: 0;
	list-style: none;
}

.login-view__highlights li {
	padding: 0.95rem 1rem;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-control);
	background: color-mix(in srgb, var(--surface-elevated) 92%, var(--surface-panel-solid));
	color: var(--text-strong);
	line-height: 1.6;
}

.login-view__card-header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: var(--space-4);
	flex-wrap: wrap;
}

.login-view__card-heading,
.login-view__form {
	display: grid;
	gap: var(--space-4);
}

.login-view__card-title {
	margin: var(--space-2) 0 0;
	color: var(--text-strong);
	font-size: 1.52rem;
	line-height: 1.1;
	letter-spacing: -0.03em;
}

.login-view__hint {
	font-size: 0.9rem;
	line-height: 1.6;
}

@media (max-width: 920px) {
	.login-view {
		grid-template-columns: 1fr;
		padding: var(--space-4);
	}
}
</style>