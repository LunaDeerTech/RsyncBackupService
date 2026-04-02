<script setup lang="ts">
interface AppEmptyProps {
	title: string
	description?: string
	compact?: boolean
}

withDefaults(defineProps<AppEmptyProps>(), {
	description: undefined,
	compact: false,
})
</script>

<template>
	<section class="app-empty" :data-compact="compact ? 'true' : 'false'">
		<div class="app-empty__icon" aria-hidden="true">
			<slot name="icon">
				<span class="app-empty__spark" />
			</slot>
		</div>

		<div class="app-empty__content">
			<h2 class="app-empty__title">{{ title }}</h2>
			<p v-if="description" class="app-empty__description">{{ description }}</p>
		</div>

		<div v-if="$slots.actions" class="app-empty__actions">
			<slot name="actions" />
		</div>
	</section>
</template>

<style scoped>
.app-empty {
	display: grid;
	justify-items: center;
	gap: var(--space-4);
	padding: var(--space-6);
	border: var(--border-width) dashed color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-elevated) 88%, var(--surface-panel-solid));
	text-align: center;
}

.app-empty[data-compact="true"] {
	padding: var(--space-5);
}

.app-empty__icon {
	display: grid;
	place-items: center;
	width: 4rem;
	height: 4rem;
	border-radius: 1.25rem;
	background: color-mix(in srgb, var(--primary-300) 16%, var(--surface-panel-solid));
}

.app-empty__spark {
	width: 1.25rem;
	height: 1.25rem;
	border-radius: 0.45rem;
	border: 2px solid color-mix(in srgb, var(--primary-500) 48%, transparent);
	transform: rotate(45deg);
	}

.app-empty__content {
	display: grid;
	gap: var(--space-2);
	max-width: 22rem;
}

.app-empty__title,
.app-empty__description {
	margin: 0;
}

.app-empty__title {
	color: var(--text-strong);
	font-size: 1.02rem;
	line-height: 1.2;
}

.app-empty__description {
	color: var(--text-muted);
	font-size: 0.9rem;
	line-height: 1.6;
}

.app-empty__actions {
	display: flex;
	gap: var(--space-3);
	flex-wrap: wrap;
	justify-content: center;
}
</style>