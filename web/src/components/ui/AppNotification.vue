<script setup lang="ts">
import { computed } from "vue"

import { statusToneMap } from "./statusTone"

export interface AppNotificationProps {
	title: string
	tone?: "info" | "success" | "warning" | "danger"
	description?: string
	timestamp?: string
	announce?: boolean
}

const props = withDefaults(defineProps<AppNotificationProps>(), {
	tone: "info",
	description: undefined,
	timestamp: undefined,
	announce: false,
})

const toneDefinition = computed(() => statusToneMap[props.tone])
const liveRole = computed(() => {
	if (!props.announce) {
		return undefined
	}

	return props.tone === "danger" ? "alert" : "status"
})
const toneStyle = computed(() => ({
	"--app-notification-accent": toneDefinition.value.accent,
	"--app-notification-surface": toneDefinition.value.surface,
	"--app-notification-border": toneDefinition.value.border,
	"--app-notification-text": toneDefinition.value.text,
}))
</script>

<template>
	<article class="app-notification" :data-tone="tone" :role="liveRole" :style="toneStyle">
		<div class="app-notification__icon" aria-hidden="true">{{ toneDefinition.icon }}</div>

		<div class="app-notification__content">
			<div class="app-notification__header">
				<div class="app-notification__heading">
					<span class="app-notification__eyebrow">{{ toneDefinition.label }}</span>
					<h3 class="app-notification__title">{{ title }}</h3>
				</div>

				<time v-if="timestamp" class="app-notification__timestamp">{{ timestamp }}</time>
			</div>

			<p v-if="description" class="app-notification__description">{{ description }}</p>

			<div v-if="$slots.default" class="app-notification__body">
				<slot />
			</div>

			<footer v-if="$slots.actions" class="app-notification__actions">
				<slot name="actions" />
			</footer>
		</div>
	</article>
</template>

<style scoped>
.app-notification {
	display: grid;
	grid-template-columns: auto minmax(0, 1fr);
	gap: var(--space-3);
	padding: var(--space-4);
	border: var(--border-width) solid var(--app-notification-border);
	border-radius: var(--radius-card);
	background: var(--app-notification-surface);
	box-shadow: 0 14px 30px color-mix(in srgb, var(--border-default) 18%, transparent);
	color: var(--text-strong);
}

.app-notification__icon {
	display: grid;
	place-items: center;
	width: 2.2rem;
	height: 2.2rem;
	border-radius: 0.85rem;
	background: color-mix(in srgb, var(--app-notification-accent) 18%, var(--surface-panel-solid));
	border: var(--border-width) solid color-mix(in srgb, var(--app-notification-accent) 36%, transparent);
	color: var(--app-notification-text);
	font-size: 0.96rem;
	font-weight: 800;
}

.app-notification__content,
.app-notification__heading {
	display: grid;
	gap: var(--space-2);
}

.app-notification__header,
.app-notification__actions {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.app-notification__eyebrow,
.app-notification__timestamp,
.app-notification__description {
	margin: 0;
	color: var(--text-muted);
}

.app-notification__eyebrow {
	font-size: 0.76rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
	color: var(--app-notification-text);
}

.app-notification__title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1rem;
	line-height: 1.2;
}

.app-notification__timestamp {
	font-size: 0.78rem;
	white-space: nowrap;
}

.app-notification__description,
.app-notification__body {
	font-size: 0.9rem;
	line-height: 1.6;
}

.app-notification__body :deep(*) {
	margin: 0;
}

.app-notification__actions {
	padding-top: var(--space-1);
	}
</style>