<script setup lang="ts">
import { computed } from "vue"
import { useRoute } from "vue-router"

const route = useRoute()

const title = computed(() =>
	typeof route.meta.title === "string" ? route.meta.title : "Rsync Backup Service",
)

const subtitle = computed(() =>
	typeof route.meta.description === "string" ? route.meta.description : "",
)
</script>

<template>
	<header class="top-bar">
		<h1 class="top-bar__title">{{ title }}</h1>
		<p v-if="subtitle" class="top-bar__subtitle">{{ subtitle }}</p>
	</header>
</template>

<style scoped>
.top-bar {
	display: grid;
	gap: var(--space-2);
	padding: var(--space-5);
	border: var(--border-width) solid var(--shell-panel-border);
	border-radius: var(--radius-card);
	background: var(--shell-topbar-bg);
	box-shadow: var(--shell-panel-shadow);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
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
	margin: 0;
	max-width: 48rem;
	color: var(--text-muted);
	font-size: 0.96rem;
}

@media (max-width: 960px) {
	.top-bar {
		padding: var(--space-4);
	}
}
</style>