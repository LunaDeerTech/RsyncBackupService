<script setup lang="ts">
import { computed, useAttrs, watchSyncEffect } from "vue"

import { useFormFieldContext } from "./formFieldContext"

interface AppSwitchProps {
	modelValue: boolean
	disabled?: boolean
	invalid?: boolean
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppSwitchProps>(), {
	disabled: false,
	invalid: false,
})

const emit = defineEmits<{
	"update:modelValue": [value: boolean]
}>()

const attrs = useAttrs()
const field = useFormFieldContext()

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
const describedBy = computed(() => {
	const attrValue = typeof attrs["aria-describedby"] === "string" ? attrs["aria-describedby"] : undefined
	const ids = [field?.describedBy.value, attrValue].filter((value): value is string => typeof value === "string")
	return ids.length > 0 ? ids.join(" ") : undefined
})

function onChange(event: Event): void {
	const target = event.target as HTMLInputElement
	emit("update:modelValue", target.checked)
}
</script>

<template>
	<input
		v-bind="$attrs"
		:id="controlId"
		class="app-switch"
		type="checkbox"
		role="switch"
		:checked="modelValue"
		:disabled="disabled"
		:data-invalid="isInvalid ? 'true' : 'false'"
		:aria-checked="modelValue ? 'true' : 'false'"
		:aria-invalid="isInvalid ? 'true' : undefined"
		:aria-labelledby="labelledBy"
		:aria-describedby="describedBy"
		@change="onChange"
	/>
</template>

<style scoped>
.app-switch {
	position: relative;
	width: 3.15rem;
	height: 1.9rem;
	margin: 0;
	border: var(--border-width) solid var(--control-border);
	border-radius: 999px;
	background: color-mix(in srgb, var(--surface-elevated) 96%, transparent);
	appearance: none;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		opacity var(--duration-fast) ease;
}

.app-switch::after {
	content: "";
	position: absolute;
	top: 0.18rem;
	left: 0.2rem;
	width: 1.34rem;
	height: 1.34rem;
	border-radius: 999px;
	background: var(--surface-panel-solid);
	box-shadow: 0 8px 18px color-mix(in srgb, var(--shadow-ambient) 24%, transparent);
	transition:
		transform var(--duration-fast) ease,
		background var(--duration-fast) ease;
}

.app-switch:checked {
	border-color: color-mix(in srgb, var(--primary-500) 38%, var(--border-default));
	background: color-mix(in srgb, var(--primary-500) 28%, var(--surface-panel-solid));
}

.app-switch:checked::after {
	transform: translateX(1.23rem);
	background: color-mix(in srgb, var(--accent-mint-400) 86%, var(--surface-panel-solid));
}

.app-switch:hover:not(:disabled) {
	border-color: var(--control-border-hover);
}

.app-switch:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.app-switch[data-invalid="true"] {
	border-color: var(--control-border-invalid);
}

.app-switch[data-invalid="true"]:focus-visible {
	box-shadow: var(--state-focus-ring-danger);
}

.app-switch:disabled {
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
}
</style>