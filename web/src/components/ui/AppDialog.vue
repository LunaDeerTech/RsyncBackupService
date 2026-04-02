<script setup lang="ts">
import { computed, useId, useSlots } from "vue"

import AppModal from "./AppModal.vue"

interface AppDialogProps {
	open: boolean
	title: string
	tone?: "default" | "danger"
}

const props = withDefaults(defineProps<AppDialogProps>(), {
	tone: "default",
})

const emit = defineEmits<{
	close: []
}>()

const slots = useSlots()
const titleId = `dialog-title-${useId()}`
const descriptionId = `dialog-description-${useId()}`
const hasDescription = computed(() => slots.default !== undefined)

function onClose(): void {
	emit("close")
}
</script>

<template>
	<AppModal
		:open="open"
		:tone="tone"
		:labelled-by="titleId"
		:described-by="hasDescription ? descriptionId : undefined"
		@close="onClose"
	>
		<section class="app-dialog" :data-tone="tone">
			<header class="app-dialog__header">
				<div v-if="tone === 'danger'" class="app-dialog__tone-chip">
					<span class="app-dialog__tone-icon" aria-hidden="true">!</span>
					<span>危险操作</span>
				</div>
				<h2 :id="titleId" class="app-dialog__title">
					{{ title }}
				</h2>
			</header>

			<div v-if="hasDescription" :id="descriptionId" class="app-dialog__body">
				<slot />
			</div>

			<footer v-if="$slots.actions" class="app-dialog__actions">
				<slot name="actions" />
			</footer>
		</section>
	</AppModal>
</template>

<style scoped>
.app-dialog {
	display: grid;
	gap: var(--space-4);
	padding: var(--space-5);
}

.app-dialog__header {
	display: grid;
	gap: var(--space-3);
}

.app-dialog__tone-chip {
	display: inline-flex;
	align-items: center;
	gap: var(--space-2);
	justify-self: start;
	padding: 0.4rem 0.7rem;
	border: var(--border-width) solid var(--state-danger-border);
	border-radius: 999px;
	background: var(--dialog-danger-banner);
	color: var(--error-text);
	font-size: 0.78rem;
	font-weight: 700;
	letter-spacing: 0.04em;
	text-transform: uppercase;
}

.app-dialog__tone-icon {
	display: inline-grid;
	place-items: center;
	width: 0.95rem;
	height: 0.95rem;
	border-radius: 999px;
	background: color-mix(in srgb, var(--error-500) 18%, transparent);
	font-size: 0.72rem;
}

.app-dialog__title {
	margin: 0;
	color: var(--text-strong);
	font-size: clamp(1.2rem, 2vw, 1.5rem);
	line-height: 1.15;
	letter-spacing: -0.03em;
}

.app-dialog__body {
	color: var(--text-muted);
	font-size: 0.96rem;
	line-height: 1.6;
}

.app-dialog__body :deep(p) {
	margin: 0;
}

.app-dialog__actions {
	display: flex;
	justify-content: flex-end;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.app-dialog__actions :deep(button:not(.app-button)) {
	min-height: 2.65rem;
	padding: 0.76rem 1rem;
	border: var(--border-width) solid var(--button-secondary-border);
	border-radius: var(--radius-button);
	background: var(--button-secondary-bg);
	color: var(--text-strong);
	font: inherit;
	font-weight: 700;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease;
}

.app-dialog__actions :deep(button:not(.app-button):hover:not(:disabled)) {
	background: var(--button-ghost-bg-hover);
	border-color: var(--control-border-hover);
}

.app-dialog__actions :deep(button:not(.app-button):focus-visible) {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

@media (max-width: 640px) {
	.app-dialog {
		padding: var(--space-4);
	}

	.app-dialog__actions {
		justify-content: stretch;
	}

	.app-dialog__actions :deep(button:not(.app-button)) {
		flex: 1 1 10rem;
	}
}
</style>