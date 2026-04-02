<script setup lang="ts">
import { computed, ref, useAttrs, watchSyncEffect } from "vue"

import { useFormFieldContext } from "./formFieldContext"

interface AppPasswordInputProps {
	modelValue: string
	disabled?: boolean
	invalid?: boolean
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppPasswordInputProps>(), {
	disabled: false,
	invalid: false,
})

const emit = defineEmits<{
	"update:modelValue": [value: string]
}>()

const attrs = useAttrs()
const field = useFormFieldContext()
const revealed = ref(false)

watchSyncEffect(() => {
	field?.setControlId(typeof attrs.id === "string" ? attrs.id : undefined)
})

const controlId = computed(() => (typeof attrs.id === "string" ? attrs.id : field?.controlId.value))
const labelledBy = computed(() => {
	const attrValue = typeof attrs["aria-labelledby"] === "string" ? attrs["aria-labelledby"] : undefined
	const ids = [field?.labelId.value, attrValue].filter((value): value is string => typeof value === "string")
	return ids.length > 0 ? ids.join(" ") : undefined
})
const isInvalid = computed(() => props.invalid || field?.invalid.value === true)
const isRequired = computed(() => field?.required.value === true)
const describedBy = computed(() => {
	const attrValue = typeof attrs["aria-describedby"] === "string" ? attrs["aria-describedby"] : undefined
	const ids = [field?.describedBy.value, attrValue].filter((value): value is string => typeof value === "string")
	return ids.length > 0 ? ids.join(" ") : undefined
})

function onInput(event: Event): void {
	const target = event.target as HTMLInputElement
	emit("update:modelValue", target.value)
}

function toggleVisibility(): void {
	revealed.value = !revealed.value
}
</script>

<template>
	<div class="app-password-input" :data-invalid="isInvalid ? 'true' : 'false'" :data-disabled="disabled ? 'true' : 'false'">
		<input
			v-bind="$attrs"
			:id="controlId"
			class="app-password-input__control"
			:type="revealed ? 'text' : 'password'"
			:value="modelValue"
			:disabled="disabled"
			:required="isRequired"
			:aria-invalid="isInvalid ? 'true' : undefined"
			:aria-labelledby="labelledBy"
			:aria-describedby="describedBy"
			@input="onInput"
		/>
		<button
			type="button"
			class="app-password-input__toggle"
			:disabled="disabled"
			:aria-label="revealed ? '隐藏密码' : '显示密码'"
			@click="toggleVisibility"
		>
			{{ revealed ? "隐藏" : "显示" }}
		</button>
	</div>
</template>

<style scoped>
.app-password-input {
	display: grid;
	grid-template-columns: minmax(0, 1fr) auto;
	align-items: center;
	gap: var(--space-3);
	padding: 0.22rem 0.26rem 0.22rem 0.22rem;
	border: var(--border-width) solid var(--control-border);
	border-radius: var(--radius-control);
	background: var(--control-bg);
	box-shadow: inset 0 1px 0 color-mix(in srgb, var(--surface-panel-solid) 34%, transparent);
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		opacity var(--duration-fast) ease;
}

.app-password-input:hover:not([data-disabled="true"]) {
	background: var(--control-bg-hover);
	border-color: var(--control-border-hover);
}

.app-password-input:focus-within {
	border-color: var(--control-border-hover);
	box-shadow: var(--state-focus-ring), var(--control-shadow-focus);
}

.app-password-input[data-invalid="true"] {
	border-color: var(--control-border-invalid);
	box-shadow: var(--control-shadow-danger);
}

.app-password-input[data-invalid="true"]:focus-within {
	box-shadow: var(--state-focus-ring-danger), var(--control-shadow-danger);
}

.app-password-input[data-disabled="true"] {
	background: var(--control-bg-disabled);
	opacity: var(--state-disabled-opacity);
	}

.app-password-input__control {
	width: 100%;
	min-width: 0;
	padding: 0.58rem 0.7rem;
	border: none;
	background: transparent;
	color: var(--control-text);
	font: inherit;
}

.app-password-input__control::placeholder {
	color: var(--control-placeholder);
}

.app-password-input__control:focus-visible {
	outline: none;
	box-shadow: none;
}

.app-password-input__control:disabled {
	color: var(--text-muted);
	cursor: not-allowed;
}

.app-password-input__toggle {
	min-height: 2.35rem;
	padding: 0.6rem 0.82rem;
	border: var(--border-width) solid transparent;
	border-radius: calc(var(--radius-control) - 4px);
	background: transparent;
	color: var(--text-muted);
	font: inherit;
	font-size: 0.84rem;
	font-weight: 700;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.app-password-input__toggle:hover:not(:disabled) {
	border-color: var(--button-secondary-border);
	background: var(--button-ghost-bg-hover);
	color: var(--text-strong);
}

.app-password-input__toggle:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.app-password-input[data-invalid="true"] .app-password-input__toggle:focus-visible {
	box-shadow: var(--state-focus-ring-danger);
}

.app-password-input__toggle:disabled {
	cursor: not-allowed;
	color: var(--text-muted);
}
</style>