<script setup lang="ts">
import { computed } from "vue"

import { statusToneMap, type StatusTone } from "./statusTone"

interface AppTagProps {
	tone?: StatusTone
	showDot?: boolean
}

const props = withDefaults(defineProps<AppTagProps>(), {
	tone: "default",
	showDot: true,
})

const toneDefinition = computed(() => statusToneMap[props.tone])
const toneStyle = computed(() => ({
	"--app-tag-accent": toneDefinition.value.accent,
	"--app-tag-surface": toneDefinition.value.surface,
	"--app-tag-border": toneDefinition.value.border,
	"--app-tag-text": props.tone === "danger" ? "var(--text-strong)" : toneDefinition.value.text,
}))
</script>

<template>
	<span class="app-tag" :data-tone="tone" :style="toneStyle">
		<span v-if="showDot" class="app-tag__dot" aria-hidden="true" />
		<slot />
	</span>
</template>

<style scoped>
.app-tag {
	display: inline-flex;
	align-items: center;
	gap: 0.45rem;
	min-height: 1.85rem;
	padding: 0.34rem 0.72rem;
	border: var(--border-width) solid var(--app-tag-border);
	border-radius: 999px;
	background: var(--app-tag-surface);
	color: var(--app-tag-text);
	font-size: 0.78rem;
	font-weight: 700;
	letter-spacing: 0.02em;
	white-space: nowrap;
}

.app-tag__dot {
	width: 0.46rem;
	height: 0.46rem;
	border-radius: 999px;
	background: var(--app-tag-accent);
}
</style>