<script setup lang="ts">
import { computed } from "vue"

import { statusToneMap, type StatusTone } from "./statusTone"

interface AppBadgeProps {
	tone?: StatusTone
	value?: string | number
}

const props = withDefaults(defineProps<AppBadgeProps>(), {
	tone: "primary",
	value: undefined,
})

const toneDefinition = computed(() => statusToneMap[props.tone])
const toneStyle = computed(() => ({
	"--app-badge-accent": toneDefinition.value.accent,
	"--app-badge-surface": toneDefinition.value.surface,
	"--app-badge-border": toneDefinition.value.border,
	"--app-badge-text": props.tone === "danger" ? "var(--text-strong)" : toneDefinition.value.text,
}))
</script>

<template>
	<span class="app-badge" :data-tone="tone" :style="toneStyle">
		<slot>{{ value }}</slot>
	</span>
</template>

<style scoped>
.app-badge {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	min-width: 1.85rem;
	min-height: 1.55rem;
	padding: 0.2rem 0.55rem;
	border: var(--border-width) solid var(--app-badge-border);
	border-radius: 999px;
	background: var(--app-badge-surface);
	color: var(--app-badge-text);
	font-size: 0.74rem;
	font-weight: 800;
	line-height: 1;
	letter-spacing: 0.06em;
	text-transform: uppercase;
}
</style>