<script setup lang="ts">
import { computed, reactive, ref } from "vue"
import { useRoute, useRouter } from "vue-router"

import { ApiError } from "../api/client"
import { useSession } from "../composables/useSession"
import { useUiStore } from "../stores/ui"

const route = useRoute()
const router = useRouter()
const session = useSession()
const ui = useUiStore()

const credentials = reactive({
	username: "admin",
	password: "",
})

const errorMessage = ref("")
const isSubmitting = ref(false)

const redirectTarget = computed(() => {
	const redirect = route.query.redirect
	return typeof redirect === "string" && redirect.startsWith("/") ? redirect : "/"
})

const themeButtonLabel = computed(() =>
	ui.theme === "light" ? "切换到深色主题" : "切换到浅色主题",
)

function toggleTheme(): void {
	ui.setTheme(ui.theme === "light" ? "dark" : "light")
}

async function handleSubmit(): Promise<void> {
	errorMessage.value = ""
	isSubmitting.value = true

	try {
		await session.login(credentials)
		await router.push(redirectTarget.value)
	} catch (error) {
		errorMessage.value =
			error instanceof ApiError ? error.message : "登录失败，请检查网络状态或后端服务。"
	} finally {
		isSubmitting.value = false
	}
}
</script>

<template>
	<div class="login-shell" data-testid="login-shell">
		<div class="login-shell__frame">
			<section class="login-shell__hero">
				<p class="login-shell__eyebrow">Balanced Flux Foundation</p>
				<h1 class="login-shell__title">Rsync Backup Service</h1>
				<p class="login-shell__body">
					当前页面只承载登录壳层、鉴权状态与主题切换。业务页面、复杂组件和实时视图会在后续任务逐步接入。
				</p>
				<ul class="login-shell__highlights">
					<li>统一 API client 集中处理 token 注入与 refresh 重放</li>
					<li>Cyan Mint 品牌色与浅深主题 token 已建立</li>
					<li>匿名页与登录后页已经通过基础路由守卫隔离</li>
				</ul>
			</section>

			<section class="login-shell__card" aria-label="登录面板">
				<div class="login-shell__card-header">
					<div>
						<p class="login-shell__card-eyebrow">Session Gateway</p>
						<h2 class="login-shell__card-title">登录到应用壳层</h2>
					</div>
					<button
						type="button"
						class="login-shell__theme"
						:aria-label="themeButtonLabel"
						@click="toggleTheme"
					>
						{{ ui.theme === "light" ? "深色主题" : "浅色主题" }}
					</button>
				</div>

				<form class="login-shell__form" @submit.prevent="handleSubmit">
					<label class="login-shell__field">
						<span>用户名</span>
						<input v-model="credentials.username" name="username" autocomplete="username" />
					</label>
					<label class="login-shell__field">
						<span>密码</span>
						<input
							v-model="credentials.password"
							name="password"
							autocomplete="current-password"
							type="password"
						/>
					</label>

					<button class="login-shell__submit" type="submit" :disabled="isSubmitting">
						{{ isSubmitting ? "正在登录..." : "进入应用" }}
					</button>
				</form>

				<p v-if="errorMessage" class="login-shell__error" role="alert">{{ errorMessage }}</p>
				<p class="login-shell__note">
					如果 API 尚未启动，这个页面仍可用于预览登录壳层和主题切换效果。
				</p>
			</section>
		</div>
	</div>
</template>

<style scoped>
.login-shell {
	display: grid;
	min-height: 100vh;
	padding: var(--space-6);
	place-items: center;
}

.login-shell__frame {
	display: grid;
	grid-template-columns: minmax(0, 1.1fr) minmax(20rem, 26rem);
	gap: var(--space-6);
	width: min(72rem, 100%);
	align-items: stretch;
}

.login-shell__hero,
.login-shell__card {
	padding: var(--space-8);
	border: var(--border-width) solid var(--shell-panel-border);
	border-radius: var(--radius-card);
	background: var(--shell-panel-bg);
	box-shadow: var(--shell-panel-shadow);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
}

.login-shell__hero {
	display: flex;
	flex-direction: column;
	justify-content: center;
	gap: var(--space-4);
	position: relative;
	overflow: hidden;
	background:
		linear-gradient(160deg, color-mix(in srgb, var(--surface-panel) 92%, transparent), transparent),
		var(--shell-panel-bg);
}

.login-shell__hero::after {
	content: "";
	position: absolute;
	inset: auto -15% -28% 35%;
	height: 16rem;
	border-radius: 999px;
	background: var(--shell-hero-glow);
	filter: blur(16px);
	pointer-events: none;
}

.login-shell__eyebrow,
.login-shell__card-eyebrow {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.78rem;
	font-weight: 600;
	letter-spacing: 0.08em;
	text-transform: uppercase;
}

.login-shell__title {
	margin: 0;
	max-width: 12ch;
	color: var(--text-strong);
	font-size: clamp(2.4rem, 4vw, 4rem);
	line-height: 0.96;
	letter-spacing: -0.05em;
}

.login-shell__body,
.login-shell__note {
	margin: 0;
	max-width: 34rem;
	color: var(--text-muted);
	font-size: 1rem;
}

.login-shell__highlights {
	display: grid;
	gap: var(--space-3);
	padding: 0;
	margin: var(--space-3) 0 0;
	list-style: none;
}

.login-shell__highlights li {
	padding: var(--space-3) var(--space-4);
	border: var(--border-width) solid var(--border-default);
	border-radius: var(--radius-control);
	background: var(--surface-elevated);
	color: var(--text-strong);
	font-size: 0.96rem;
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
}

.login-shell__card {
	display: flex;
	flex-direction: column;
	gap: var(--space-5);
	justify-content: center;
}

.login-shell__card-header {
	display: flex;
	justify-content: space-between;
	gap: var(--space-4);
	align-items: flex-start;
}

.login-shell__card-title {
	margin: var(--space-2) 0 0;
	color: var(--text-strong);
	font-size: 1.5rem;
	line-height: 1.1;
	letter-spacing: -0.03em;
}

.login-shell__theme,
.login-shell__submit {
	border: none;
	border-radius: var(--radius-button);
	transition:
		transform var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		background-color var(--duration-fast) ease;
	cursor: pointer;
}

.login-shell__theme {
	padding: 0.7rem 1rem;
	background: var(--button-secondary-bg);
	color: var(--button-secondary-text);
	box-shadow: inset 0 0 0 var(--border-width) var(--shell-panel-border);
	white-space: nowrap;
}

.login-shell__form {
	display: grid;
	gap: var(--space-4);
}

.login-shell__field {
	display: grid;
	gap: var(--space-2);
	color: var(--text-strong);
	font-size: 0.94rem;
	font-weight: 600;
}

.login-shell__field input {
	width: 100%;
	padding: 0.95rem 1rem;
	border: var(--border-width) solid var(--control-border);
	border-radius: var(--radius-control);
	background: var(--control-bg);
	color: var(--control-text);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.04);
	transition:
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		transform var(--duration-fast) ease;
}

.login-shell__field input:focus-visible,
.login-shell__theme:focus-visible,
.login-shell__submit:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.login-shell__submit {
	padding: 1rem 1.15rem;
	background: var(--button-primary-bg);
	color: var(--button-primary-text);
	font-weight: 700;
	box-shadow: var(--button-primary-shadow);
}

.login-shell__submit:disabled {
	opacity: var(--state-disabled-opacity);
	cursor: wait;
	transform: none;
	box-shadow: none;
}

.login-shell__theme:hover,
.login-shell__submit:hover:not(:disabled) {
	transform: translateY(-1px);
}

.login-shell__error {
	margin: 0;
	padding: 0.9rem 1rem;
	border: var(--border-width) solid var(--state-danger-border);
	border-radius: var(--radius-control);
	background: var(--state-danger-surface);
	color: var(--error-text);
	font-size: 0.94rem;
}

@media (max-width: 900px) {
	.login-shell {
		padding: var(--space-4);
	}

	.login-shell__frame {
		grid-template-columns: 1fr;
	}

	.login-shell__hero,
	.login-shell__card {
		padding: var(--space-6);
	}

	.login-shell__card-header {
		flex-direction: column;
		align-items: stretch;
	}

	.login-shell__theme {
		width: 100%;
	}
	}
</style>