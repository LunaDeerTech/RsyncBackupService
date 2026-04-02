<script setup lang="ts">
import { computed, useAttrs, watchSyncEffect } from "vue"

import { useFormFieldContext } from "./formFieldContext"

interface AppInputProps {
	modelValue: string
	disabled?: boolean
	invalid?: boolean
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppInputProps>(), {
	disabled: false,
	invalid: false,
})

const emit = defineEmits<{
	"update:modelValue": [value: string]
}>()

const attrs = useAttrs()
const field = useFormFieldContext()

watchSyncEffect(() => {
	field?.setControlId(typeof attrs.id === "string" ? attrs.id : undefined)
})

const inputType = computed(() => (typeof attrs.type === "string" ? attrs.type : "text"))
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
</script>

<template>
	<input
		v-bind="$attrs"
		:id="controlId"
		:type="inputType"
		class="app-input"
		:value="modelValue"
		:disabled="disabled"
		:required="isRequired"
		:data-invalid="isInvalid ? 'true' : 'false'"
		:aria-invalid="isInvalid ? 'true' : undefined"
		:aria-labelledby="labelledBy"
		:aria-describedby="describedBy"
		@input="onInput"
	/>
</template>

<style scoped>
.app-input {
	width: 100%;
	min-height: 2.9rem;
	padding: 0.82rem 0.96rem;
	border: var(--border-width) solid var(--control-border);
	border-radius: var(--radius-control);
	background: var(--control-bg);
	box-shadow: inset 0 1px 0 color-mix(in srgb, var(--surface-panel-solid) 34%, transparent);
	color: var(--control-text);
	font: inherit;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		opacity var(--duration-fast) ease;
}

.app-input::placeholder {
	color: var(--control-placeholder);
}

.app-input:hover:not(:disabled) {
	background: var(--control-bg-hover);
	border-color: var(--control-border-hover);
}

.app-input:focus-visible {
	outline: none;
	border-color: var(--control-border-hover);
	box-shadow: var(--state-focus-ring), var(--control-shadow-focus);
}

.app-input[data-invalid="true"] {
	border-color: var(--control-border-invalid);
	box-shadow: var(--control-shadow-danger);
}

.app-input[data-invalid="true"]:focus-visible {
	box-shadow: var(--state-focus-ring-danger), var(--control-shadow-danger);
	}

.app-input:disabled {
	background: var(--control-bg-disabled);
	color: var(--text-muted);
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
}
</style>