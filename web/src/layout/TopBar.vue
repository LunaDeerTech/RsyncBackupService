<script setup lang="ts">
import { computed } from "vue"
import { useRoute, useRouter } from "vue-router"

import { useSession } from "../composables/useSession"
import { useUiStore } from "../stores/ui"

const route = useRoute()
const router = useRouter()
const session = useSession()
const ui = useUiStore()

const title = computed(() =>
	typeof route.meta.title === "string" ? route.meta.title : "Rsync Backup Service",
)

const subtitle = computed(() =>
	typeof route.meta.description === "string"
		? route.meta.description
		: "当前任务只交付前端基础设施，后续业务内容将在这里承载。",
)

const themeActionLabel = computed(() =>
	ui.theme === "light" ? "切换到深色主题" : "切换到浅色主题",
)

function toggleTheme(): void {
	ui.setTheme(ui.theme === "light" ? "dark" : "light")
}

async function handleLogout(): Promise<void> {
	session.logout()
	await router.push("/login")
}
</script>

<template>
	<header class="top-bar">
		<div class="top-bar__heading">
			<p class="top-bar__eyebrow">Task 10 Infrastructure</p>
			<p class="top-bar__title">{{ title }}</p>
			<p class="top-bar__subtitle">{{ subtitle }}</p>
		</div>

		<div class="top-bar__actions">
			<span class="top-bar__badge">本地会话已加载</span>
			<button type="button" class="top-bar__button" :aria-label="themeActionLabel" @click="toggleTheme">
				{{ ui.theme === "light" ? "深色主题" : "浅色主题" }}
			</button>
			<button type="button" class="top-bar__button top-bar__button--ghost" @click="handleLogout">
				退出会话
			</button>
		</div>
	</header>
</template>

<style scoped>
.top-bar {
	display: flex;
	justify-content: space-between;
	gap: var(--space-4);
	align-items: flex-start;
	padding: var(--space-5);
	border: var(--border-width) solid var(--shell-panel-border);
	border-radius: var(--radius-card);
	background: var(--shell-topbar-bg);
	box-shadow: var(--shell-panel-shadow);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
}

.top-bar__heading {
	display: grid;
	gap: var(--space-2);
	min-width: 0;
}

.top-bar__eyebrow,
.top-bar__subtitle {
	margin: 0;
	color: var(--text-muted);
}

.top-bar__eyebrow {
	font-size: 0.76rem;
	font-weight: 600;
	letter-spacing: 0.08em;
	text-transform: uppercase;
}

.top-bar__title {
	margin: 0;
	color: var(--text-strong);
	font-size: clamp(1.35rem, 2.2vw, 2rem);
	font-weight: 700;
	line-height: 1.05;
	letter-spacing: -0.04em;
}

.top-bar__subtitle {
	max-width: 48rem;
	font-size: 0.96rem;
}

.top-bar__actions {
	display: flex;
	flex-wrap: wrap;
	justify-content: flex-end;
	gap: var(--space-3);
	align-items: center;
}

.top-bar__badge {
	display: inline-flex;
	align-items: center;
	padding: 0.58rem 0.85rem;
	border: var(--border-width) solid color-mix(in srgb, var(--accent-mint-400) 38%, var(--border-default));
	border-radius: 999px;
	background: color-mix(in srgb, var(--accent-mint-400) 12%, var(--surface-elevated));
	color: var(--text-strong);
	font-size: 0.86rem;
	font-weight: 600;
	white-space: nowrap;
}

.top-bar__button {
	padding: 0.72rem 1rem;
	border: none;
	border-radius: var(--radius-button);
	background: var(--button-primary-bg);
	color: var(--button-primary-text);
	font-weight: 700;
	cursor: pointer;
	transition:
		transform var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		opacity var(--duration-fast) ease;
}

.top-bar__button--ghost {
	background: var(--button-secondary-bg);
	color: var(--button-secondary-text);
	box-shadow: inset 0 0 0 var(--border-width) var(--shell-panel-border);
	font-weight: 600;
}

.top-bar__button:hover {
	transform: translateY(-1px);
	box-shadow: var(--button-primary-hover-shadow);
}

.top-bar__button--ghost:hover {
	box-shadow: inset 0 0 0 var(--border-width) var(--border-default);
	background: var(--nav-link-bg-hover);
	}

.top-bar__button:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

@media (max-width: 960px) {
	.top-bar {
		flex-direction: column;
		padding: var(--space-4);
	}

	.top-bar__actions {
		width: 100%;
		justify-content: stretch;
	}

	.top-bar__actions > * {
		flex: 1 1 11rem;
	}
	}
</style>