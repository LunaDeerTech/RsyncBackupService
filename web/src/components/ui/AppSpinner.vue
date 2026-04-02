<script setup lang="ts">
import { computed } from "vue"

interface AppSpinnerProps {
	label?: string
	size?: "sm" | "md" | "lg"
	tone?: "default" | "running" | "danger"
	announce?: boolean
}

const props = withDefaults(defineProps<AppSpinnerProps>(), {
	label: "加载中",
	size: "md",
	tone: "default",
	announce: false,
})

const toneStyle = computed(() => {
	if (props.tone === "running") {
		return {
			"--app-spinner-accent": "var(--accent-mint-400)",
			"--app-spinner-track": "color-mix(in srgb, var(--primary-500) 18%, transparent)",
		}
	}

	if (props.tone === "danger") {
		return {
			"--app-spinner-accent": "var(--error-500)",
			"--app-spinner-track": "color-mix(in srgb, var(--error-500) 18%, transparent)",
		}
	}

	return {
		"--app-spinner-accent": "var(--primary-500)",
		"--app-spinner-track": "color-mix(in srgb, var(--primary-500) 16%, transparent)",
	}
})
</script>

<template>
	<div class="app-spinner" :data-size="size" :style="toneStyle" :role="announce ? 'status' : undefined" :aria-live="announce ? 'polite' : undefined">
		<span class="app-spinner__ring" aria-hidden="true" />
		<span class="app-spinner__label">{{ label }}</span>
	</div>
</template>

<style scoped>
.app-spinner {
	display: inline-flex;
	align-items: center;
	gap: 0.7rem;
	color: var(--text-muted);
}

.app-spinner__ring {
	width: 1.1rem;
	height: 1.1rem;
	border: 2px solid var(--app-spinner-track);
	border-top-color: var(--app-spinner-accent);
	border-radius: 999px;
	animation: app-spinner-spin 960ms linear infinite;
}

.app-spinner[data-size="md"] .app-spinner__ring {
	width: 1.35rem;
	height: 1.35rem;
}

.app-spinner[data-size="lg"] .app-spinner__ring {
	width: 1.75rem;
	height: 1.75rem;
	border-width: 3px;
}

.app-spinner__label {
	font-size: 0.86rem;
	font-weight: 600;
}

@keyframes app-spinner-spin {
	from {
		transform: rotate(0deg);
	}

	to {
		transform: rotate(360deg);
	}
}

@media (prefers-reduced-motion: reduce) {
	.app-spinner__ring {
		animation: none;
	}
}
</style>