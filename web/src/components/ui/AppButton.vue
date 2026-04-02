<script setup lang="ts">
import { computed, useAttrs } from "vue"

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger"
export type ButtonSize = "sm" | "md" | "lg"

interface AppButtonProps {
	variant?: ButtonVariant
	size?: ButtonSize
	loading?: boolean
	disabled?: boolean
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppButtonProps>(), {
	variant: "primary",
	size: "md",
	loading: false,
	disabled: false,
})

const attrs = useAttrs()

const buttonType = computed(() => (typeof attrs.type === "string" ? attrs.type : "button"))
const isDisabled = computed(() => props.disabled || props.loading)
</script>

<template>
	<button
		v-bind="$attrs"
		:type="buttonType"
		class="app-button"
		:data-variant="variant"
		:data-size="size"
		:disabled="isDisabled"
		:aria-busy="loading ? 'true' : undefined"
	>
		<span v-if="loading" class="app-button__spinner" aria-hidden="true" />
		<span class="app-button__content">
			<slot />
		</span>
	</button>
</template>

<style scoped>
.app-button {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	gap: var(--space-2);
	min-width: 0;
	border: var(--border-width) solid transparent;
	border-radius: var(--radius-button);
	background: var(--button-primary-bg);
	box-shadow: var(--button-primary-shadow);
	color: var(--button-primary-text);
	font-weight: 700;
	line-height: 1;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		transform var(--duration-fast) ease,
		opacity var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.app-button[data-size="sm"] {
	min-height: 2.25rem;
	padding: 0.62rem 0.9rem;
	font-size: 0.82rem;
}

.app-button[data-size="md"] {
	min-height: 2.75rem;
	padding: 0.82rem 1.08rem;
	font-size: 0.92rem;
}

.app-button[data-size="lg"] {
	min-height: 3.2rem;
	padding: 0.98rem 1.32rem;
	font-size: 1rem;
	}

.app-button[data-variant="secondary"] {
	border-color: var(--button-secondary-border);
	background: var(--button-secondary-bg);
	box-shadow: inset 0 0 0 var(--border-width) color-mix(in srgb, var(--border-default) 36%, transparent);
	color: var(--button-secondary-text);
}

.app-button[data-variant="ghost"] {
	background: transparent;
	box-shadow: none;
	color: var(--button-ghost-text);
}

.app-button[data-variant="danger"] {
	background: var(--button-danger-bg);
	box-shadow: var(--button-danger-shadow);
	color: var(--button-danger-text);
}

.app-button:hover:not(:disabled) {
	transform: translateY(-1px);
	box-shadow: var(--button-primary-hover-shadow);
}

.app-button[data-variant="secondary"]:hover:not(:disabled),
.app-button[data-variant="ghost"]:hover:not(:disabled) {
	background: var(--button-ghost-bg-hover);
	box-shadow: inset 0 0 0 var(--border-width) var(--control-border-hover);
	}

.app-button[data-variant="danger"]:hover:not(:disabled) {
	box-shadow: var(--button-danger-hover-shadow);
}

.app-button:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.app-button[data-variant="danger"]:focus-visible {
	box-shadow: var(--state-focus-ring-danger);
}

.app-button:disabled {
	background: var(--button-disabled-bg);
	box-shadow: none;
	color: var(--button-disabled-text);
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
	transform: none;
}

.app-button__content {
	display: inline-flex;
	align-items: center;
	gap: inherit;
}

.app-button__spinner {
	width: 0.95rem;
	height: 0.95rem;
	border: 2px solid currentColor;
	border-right-color: transparent;
	border-radius: 999px;
	animation: button-spin 780ms linear infinite;
	opacity: 0.76;
}

@keyframes button-spin {
	from {
		transform: rotate(0deg);
	}

	to {
		transform: rotate(360deg);
	}
}
</style>