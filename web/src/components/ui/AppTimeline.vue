<script setup lang="ts">
import { computed } from "vue"

import { statusToneMap, type StatusTone } from "./statusTone"

export interface AppTimelineItem {
	id: string | number
	title: string
	description?: string
	timestamp?: string
	tone?: StatusTone
}

interface AppTimelineProps {
	items: AppTimelineItem[]
	compact?: boolean
}

const props = withDefaults(defineProps<AppTimelineProps>(), {
	compact: false,
})

function toneDefinition(tone: StatusTone = "default") {
	return statusToneMap[tone]
}

function toneStyle(tone: StatusTone = "default") {
	const definition = toneDefinition(tone)

	return {
		"--app-timeline-accent": definition.accent,
		"--app-timeline-surface": definition.surface,
		"--app-timeline-border": definition.border,
	}
}

const compactAttr = computed(() => (props.compact ? "true" : "false"))
</script>

<template>
	<ol class="app-timeline" :data-compact="compactAttr">
		<li v-for="item in items" :key="item.id" class="app-timeline__item" :style="toneStyle(item.tone)">
			<div class="app-timeline__marker" aria-hidden="true" />
			<div class="app-timeline__content">
				<div v-if="item.tone && item.tone !== 'default'" class="app-timeline__tone">
					<span class="app-timeline__tone-icon" aria-hidden="true">{{ toneDefinition(item.tone).icon }}</span>
					<span>{{ toneDefinition(item.tone).label }}</span>
				</div>

				<div class="app-timeline__header">
					<h3 class="app-timeline__title">{{ item.title }}</h3>
					<time v-if="item.timestamp" class="app-timeline__timestamp">{{ item.timestamp }}</time>
				</div>
				<p v-if="item.description" class="app-timeline__description">{{ item.description }}</p>
			</div>
		</li>
	</ol>
</template>

<style scoped>
.app-timeline {
	display: grid;
	gap: var(--space-4);
	padding: 0;
	margin: 0;
	list-style: none;
}

.app-timeline__item {
	position: relative;
	display: grid;
	grid-template-columns: auto minmax(0, 1fr);
	gap: var(--space-3);
}

.app-timeline__item:not(:last-child)::after {
	content: "";
	position: absolute;
	left: 0.48rem;
	top: 1.25rem;
	bottom: calc(var(--space-4) * -1);
	width: 1px;
	background: color-mix(in srgb, var(--app-timeline-accent) 24%, var(--border-default));
}

.app-timeline__marker {
	position: relative;
	margin-top: 0.34rem;
	width: 0.95rem;
	height: 0.95rem;
	border: 2px solid color-mix(in srgb, var(--app-timeline-accent) 48%, transparent);
	border-radius: 999px;
	background: color-mix(in srgb, var(--app-timeline-accent) 18%, transparent);
	box-shadow: 0 0 0 5px color-mix(in srgb, var(--app-timeline-surface) 64%, transparent);
	}

.app-timeline__content {
	display: grid;
	gap: var(--space-2);
	padding: 0 0 var(--space-4);
}

.app-timeline__tone {
	display: inline-flex;
	align-items: center;
	justify-self: start;
	gap: 0.45rem;
	padding: 0.28rem 0.62rem;
	border: var(--border-width) solid color-mix(in srgb, var(--app-timeline-accent) 34%, var(--border-default));
	border-radius: 999px;
	background: color-mix(in srgb, var(--app-timeline-surface) 84%, var(--surface-panel-solid));
	color: var(--text-strong);
	font-size: 0.72rem;
	font-weight: 700;
	letter-spacing: 0.04em;
	text-transform: uppercase;
}

.app-timeline__tone-icon {
	display: inline-grid;
	place-items: center;
	width: 0.9rem;
	height: 0.9rem;
	border-radius: 999px;
	background: color-mix(in srgb, var(--app-timeline-accent) 18%, transparent);
	color: var(--text-strong);
	font-size: 0.68rem;
}

.app-timeline[data-compact="true"] .app-timeline__content {
	padding-bottom: var(--space-3);
}

.app-timeline__header {
	display: flex;
	justify-content: space-between;
	align-items: flex-start;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.app-timeline__title,
.app-timeline__description,
.app-timeline__timestamp {
	margin: 0;
}

.app-timeline__title {
	color: var(--text-strong);
	font-size: 0.96rem;
	line-height: 1.3;
}

.app-timeline__description,
.app-timeline__timestamp {
	color: var(--text-muted);
	font-size: 0.85rem;
	line-height: 1.55;
}
</style>