<script setup lang="ts">
import { computed, useAttrs, watchSyncEffect } from "vue"

import { useFormFieldContext } from "./formFieldContext"

interface AppFileInputProps {
	disabled?: boolean
	invalid?: boolean
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppFileInputProps>(), {
	disabled: false,
	invalid: false,
})

const emit = defineEmits<{
	select: [file: File | null]
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
const describedBy = computed(() => {
	const attrValue = typeof attrs["aria-describedby"] === "string" ? attrs["aria-describedby"] : undefined
	const ids = [field?.describedBy.value, attrValue].filter((value): value is string => typeof value === "string")
	return ids.length > 0 ? ids.join(" ") : undefined
})
const isInvalid = computed(() => props.invalid || field?.invalid.value === true)

function onChange(event: Event): void {
	const target = event.target as HTMLInputElement
	const files = target.files
	if (files === null || files.length === 0) {
		emit("select", null)
		return
	}

	const selectedFile = typeof files.item === "function" ? files.item(0) : files[0]
	emit("select", selectedFile ?? null)
}
</script>

<template>
	<input
		v-bind="$attrs"
		:id="controlId"
		type="file"
		class="app-file-input"
		:disabled="disabled"
		:data-invalid="isInvalid ? 'true' : 'false'"
		:aria-invalid="isInvalid ? 'true' : undefined"
		:aria-labelledby="labelledBy"
		:aria-describedby="describedBy"
		@change="onChange"
	/>
</template>

<style scoped>
.app-file-input {
	width: 100%;
	min-height: 2.9rem;
	padding: 0.72rem 0.96rem;
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

.app-file-input:hover:not(:disabled) {
	background: var(--control-bg-hover);
	border-color: var(--control-border-hover);
}

.app-file-input:focus-visible {
	outline: none;
	border-color: var(--control-border-hover);
	box-shadow: var(--state-focus-ring), var(--control-shadow-focus);
}

.app-file-input[data-invalid="true"] {
	border-color: var(--control-border-invalid);
	box-shadow: var(--control-shadow-danger);
}

.app-file-input[data-invalid="true"]:focus-visible {
	box-shadow: var(--state-focus-ring-danger), var(--control-shadow-danger);
}

.app-file-input:disabled {
	background: var(--control-bg-disabled);
	color: var(--text-muted);
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
}
</style>