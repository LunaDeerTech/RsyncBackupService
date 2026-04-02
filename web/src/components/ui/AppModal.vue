<script setup lang="ts">
import { nextTick, onBeforeUnmount, ref, watch } from "vue"

import { cycleFocus, focusFirstElement } from "./focus"

interface AppModalProps {
	open: boolean
	labelledBy?: string
	describedBy?: string
	tone?: "default" | "danger"
	closeOnOverlay?: boolean
}

const props = withDefaults(defineProps<AppModalProps>(), {
	labelledBy: undefined,
	describedBy: undefined,
	tone: "default",
	closeOnOverlay: true,
})

const emit = defineEmits<{
	close: []
}>()

const panelRef = ref<HTMLElement | null>(null)
let previousFocusedElement: HTMLElement | null = null

async function focusPanel(): Promise<void> {
	if (!props.open) {
		return
	}

	await nextTick()

	if (!panelRef.value) {
		return
	}

	const focused = focusFirstElement(panelRef.value)

	if (!focused) {
		panelRef.value.focus()
	}
}

function restoreFocus(): void {
	if (previousFocusedElement && previousFocusedElement.isConnected) {
		previousFocusedElement.focus()
	}

	previousFocusedElement = null
}

function requestClose(): void {
	emit("close")
}

function onOverlayClick(event: MouseEvent): void {
	if (!props.closeOnOverlay || event.target !== event.currentTarget) {
		return
	}

	requestClose()
}

function onKeydown(event: KeyboardEvent): void {
	if (event.key === "Escape") {
		event.preventDefault()
		requestClose()
		return
	}

	if (event.key !== "Tab" || !panelRef.value) {
		return
	}

	event.preventDefault()
	cycleFocus(panelRef.value, event.shiftKey ? -1 : 1)
}

watch(
	() => props.open,
	async (open) => {
		if (open) {
			previousFocusedElement = document.activeElement instanceof HTMLElement ? document.activeElement : null
			await focusPanel()
			return
		}

		restoreFocus()
	},
	{ immediate: true },
)

onBeforeUnmount(() => {
	restoreFocus()
})
</script>

<template>
	<div v-if="open" class="app-modal" @click="onOverlayClick" @keydown="onKeydown">
		<div
			ref="panelRef"
			class="app-modal__panel"
			role="dialog"
			aria-modal="true"
			:data-tone="tone"
			:aria-labelledby="labelledBy"
			:aria-describedby="describedBy"
			tabindex="-1"
		>
			<slot />
		</div>
	</div>
</template>

<style scoped>
.app-modal {
	position: fixed;
	inset: 0;
	display: grid;
	place-items: center;
	padding: var(--space-4);
	background: var(--dialog-backdrop);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
	z-index: 50;
	animation: modal-fade-in var(--duration-base) ease;
}

.app-modal__panel {
	width: min(100%, 36rem);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 92%, transparent);
	border-radius: var(--radius-dialog);
	background: var(--dialog-surface);
	box-shadow: var(--shadow-ambient);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
	outline: none;
	animation: modal-panel-in var(--duration-base) ease;
}

.app-modal__panel:focus-visible {
	box-shadow: var(--state-focus-ring);
}

.app-modal__panel[data-tone="danger"] {
	border-color: var(--state-danger-border);
	box-shadow: 0 24px 60px color-mix(in srgb, var(--error-500) 18%, transparent);
}

.app-modal__panel[data-tone="danger"]:focus-visible {
	box-shadow: var(--state-focus-ring-danger);
}

@keyframes modal-fade-in {
	from {
		opacity: 0;
	}

	to {
		opacity: 1;
	}
}

@keyframes modal-panel-in {
	from {
		opacity: 0;
		transform: translateY(14px) scale(0.98);
	}

	to {
		opacity: 1;
		transform: translateY(0) scale(1);
	}
}
</style>