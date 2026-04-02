<script setup lang="ts">
interface AppCardProps {
	eyebrow?: string
	title?: string
	description?: string
	tone?: "default" | "running" | "danger"
	compact?: boolean
}

withDefaults(defineProps<AppCardProps>(), {
	eyebrow: undefined,
	title: undefined,
	description: undefined,
	tone: "default",
	compact: false,
})
</script>

<template>
	<section class="app-card" :data-tone="tone" :data-compact="compact ? 'true' : 'false'">
		<header v-if="eyebrow || title || description || $slots.header" class="app-card__header">
			<slot name="header">
				<p v-if="eyebrow" class="app-card__eyebrow">{{ eyebrow }}</p>
				<h2 v-if="title" class="app-card__title">{{ title }}</h2>
				<p v-if="description" class="app-card__description">{{ description }}</p>
			</slot>
		</header>

		<div class="app-card__body">
			<slot />
		</div>

		<footer v-if="$slots.footer" class="app-card__footer">
			<slot name="footer" />
		</footer>
	</section>
</template>

<style scoped>
.app-card {
	position: relative;
	display: grid;
	gap: var(--space-4);
	padding: var(--space-5);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 90%, transparent);
	border-radius: var(--radius-card);
	background:
		linear-gradient(180deg, color-mix(in srgb, white 12%, transparent), transparent 22%),
		color-mix(in srgb, var(--surface-panel) 96%, transparent);
	box-shadow: var(--shadow-ambient);
	overflow: hidden;
}

.app-card::before {
	content: "";
	position: absolute;
	inset: 0;
	background: transparent;
	pointer-events: none;
	transition: opacity var(--duration-base) ease;
	opacity: 0;
}

.app-card[data-tone="running"]::before {
	background: linear-gradient(
		135deg,
		color-mix(in srgb, var(--primary-500) 18%, transparent),
		color-mix(in srgb, var(--accent-mint-400) 12%, transparent)
	);
	opacity: 1;
}

.app-card[data-tone="danger"] {
	border-color: color-mix(in srgb, var(--error-500) 74%, var(--border-default));
	background: color-mix(in srgb, var(--error-500) 22%, var(--surface-panel-solid));
	}

.app-card[data-compact="true"] {
	padding: var(--space-4);
	gap: var(--space-3);
}

.app-card__header,
.app-card__body,
.app-card__footer {
	position: relative;
	z-index: 1;
}

.app-card__header {
	display: grid;
	gap: var(--space-2);
}

.app-card__eyebrow,
.app-card__description {
	margin: 0;
	color: var(--text-muted);
}

.app-card__eyebrow {
	font-size: 0.76rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
}

.app-card__title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1.08rem;
	line-height: 1.15;
	letter-spacing: -0.03em;
}

.app-card__description {
	font-size: 0.92rem;
	line-height: 1.6;
}

.app-card__body {
	display: grid;
	gap: var(--space-3);
}

.app-card__footer {
	display: flex;
	gap: var(--space-3);
	flex-wrap: wrap;
}
</style>