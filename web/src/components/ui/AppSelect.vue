<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useAttrs, useId, watchSyncEffect } from "vue"

import { useFormFieldContext } from "./formFieldContext"

export type AppSelectOption = {
	value: string
	label: string
	disabled?: boolean
}

interface AppSelectProps {
	modelValue: string
	options: AppSelectOption[]
	placeholder?: string
	disabled?: boolean
	invalid?: boolean
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppSelectProps>(), {
	placeholder: undefined,
	disabled: false,
	invalid: false,
})

const emit = defineEmits<{
	"update:modelValue": [value: string]
}>()

const attrs = useAttrs()
const field = useFormFieldContext()
const baseId = useId()
const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const open = ref(false)
const highlightedIndex = ref(-1)

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
const selectedIndex = computed(() => props.options.findIndex((option) => option.value === props.modelValue && option.disabled !== true))
const selectedOption = computed(() => (selectedIndex.value >= 0 ? props.options[selectedIndex.value] : undefined))
const displayLabel = computed(() => selectedOption.value?.label ?? props.placeholder ?? "请选择")
const listboxId = `select-listbox-${baseId}`
const activeOptionId = computed(() => {
	if (!open.value || highlightedIndex.value < 0) {
		return undefined
	}

	return `select-option-${baseId}-${props.options[highlightedIndex.value]?.value}`
})

function findEnabledIndex(startIndex: number, direction: 1 | -1): number {
	const total = props.options.length

	for (let offset = 1; offset <= total; offset += 1) {
		const candidateIndex = (startIndex + direction * offset + total) % total
		if (!props.options[candidateIndex]?.disabled) {
			return candidateIndex
		}
	}

	return startIndex
}

function syncHighlightedIndex(preferLast = false): void {
	if (selectedIndex.value >= 0) {
		highlightedIndex.value = selectedIndex.value
		return
	}

	const candidateIndex = preferLast
		? [...props.options].reverse().findIndex((option) => option.disabled !== true)
		: props.options.findIndex((option) => option.disabled !== true)

	if (candidateIndex === -1) {
		highlightedIndex.value = -1
		return
	}

	highlightedIndex.value = preferLast ? props.options.length - 1 - candidateIndex : candidateIndex
}

function openList(preferLast = false): void {
	if (props.disabled) {
		return
	}

	open.value = true
	syncHighlightedIndex(preferLast)
}

function closeList(): void {
	open.value = false
	highlightedIndex.value = -1
}

async function focusTrigger(): Promise<void> {
	await nextTick()
	triggerRef.value?.focus()
}

async function selectValue(option: AppSelectOption): Promise<void> {
	if (option.disabled) {
		return
	}

	emit("update:modelValue", option.value)
	closeList()
	await focusTrigger()
}

function moveHighlight(direction: 1 | -1): void {
	if (!open.value) {
		openList(direction === -1)
		return
	}

	if (highlightedIndex.value < 0) {
		syncHighlightedIndex(direction === -1)
		return
	}

	highlightedIndex.value = findEnabledIndex(highlightedIndex.value, direction)
}

function toggleList(): void {
	if (open.value) {
		closeList()
		return
	}

	openList()
}

function onTriggerKeydown(event: KeyboardEvent): void {
	if (event.key === "ArrowDown") {
		event.preventDefault()
		moveHighlight(1)
		return
	}

	if (event.key === "ArrowUp") {
		event.preventDefault()
		moveHighlight(-1)
		return
	}

	if (event.key === "Home") {
		if (!open.value) {
			return
		}

		event.preventDefault()
		highlightedIndex.value = props.options.findIndex((option) => option.disabled !== true)
		return
	}

	if (event.key === "End") {
		if (!open.value) {
			return
		}

		event.preventDefault()
		syncHighlightedIndex(true)
		return
	}

	if (event.key === "Enter" || event.key === " ") {
		event.preventDefault()

		if (!open.value) {
			openList()
			return
		}

		const option = props.options[highlightedIndex.value]
		if (option) {
			selectValue(option)
		}
		return
	}

	if (event.key === "Escape" && open.value) {
		event.preventDefault()
		closeList()
		return
	}

	if (event.key === "Tab" && open.value) {
		closeList()
	}
}

function onClickOutside(event: MouseEvent): void {
	if (!rootRef.value || !(event.target instanceof Node)) {
		return
	}

	if (!rootRef.value.contains(event.target)) {
		closeList()
	}
}

onMounted(() => {
	document.addEventListener("mousedown", onClickOutside)
})

onBeforeUnmount(() => {
	document.removeEventListener("mousedown", onClickOutside)
})
</script>

<template>
	<div ref="rootRef" class="app-select" :data-invalid="isInvalid ? 'true' : 'false'" :data-disabled="disabled ? 'true' : 'false'">
		<button
			v-bind="$attrs"
			ref="triggerRef"
			:id="controlId"
			type="button"
			class="app-select__trigger"
			role="combobox"
			:disabled="disabled"
			:aria-expanded="open ? 'true' : 'false'"
			aria-haspopup="listbox"
			:aria-controls="listboxId"
			:aria-activedescendant="activeOptionId"
			:aria-invalid="isInvalid ? 'true' : undefined"
			:aria-labelledby="labelledBy"
			:aria-describedby="describedBy"
			:aria-required="isRequired ? 'true' : undefined"
			@click="toggleList"
			@keydown="onTriggerKeydown"
		>
			<span class="app-select__value" :data-placeholder="selectedIndex < 0 ? 'true' : 'false'">
				{{ displayLabel }}
			</span>
			<span class="app-select__chevron" aria-hidden="true">⌄</span>
		</button>

		<div v-if="open" :id="listboxId" class="app-select__listbox" role="listbox" :aria-labelledby="controlId">
			<button
				v-for="(option, index) in options"
				:id="`select-option-${baseId}-${option.value}`"
				:key="option.value"
				type="button"
				class="app-select__option"
				role="option"
					tabindex="-1"
				:data-active="highlightedIndex === index ? 'true' : 'false'"
				:aria-selected="modelValue === option.value ? 'true' : 'false'"
				:disabled="option.disabled"
					@mousedown.prevent
				@click="selectValue(option)"
				@mouseenter="highlightedIndex = option.disabled ? highlightedIndex : index"
			>
				<span>{{ option.label }}</span>
				<span v-if="modelValue === option.value" class="app-select__check" aria-hidden="true">✓</span>
			</button>
		</div>
	</div>
</template>

<style scoped>
.app-select {
	position: relative;
	}

.app-select__trigger {
	width: 100%;
	min-height: 2.72rem;
	padding: 0.72rem 2.45rem 0.72rem 0.88rem;
	border: var(--border-width) solid var(--control-border);
	border-radius: calc(var(--radius-control) - 2px);
	background: var(--control-bg);
	box-shadow: inset 0 1px 0 color-mix(in srgb, var(--surface-panel-solid) 34%, transparent);
	color: var(--control-text);
	font: inherit;
	font-size: 0.91rem;
	text-align: left;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		opacity var(--duration-fast) ease;
}

.app-select__trigger:hover:not(:disabled) {
	background: var(--control-bg-hover);
	border-color: var(--control-border-hover);
}

.app-select__trigger:focus-visible {
	outline: none;
	border-color: var(--control-border-hover);
	box-shadow: var(--state-focus-ring), var(--control-shadow-focus);
}

.app-select[data-invalid="true"] .app-select__trigger {
	border-color: var(--control-border-invalid);
	box-shadow: var(--control-shadow-danger);
}

.app-select[data-invalid="true"] .app-select__trigger:focus-visible {
	box-shadow: var(--state-focus-ring-danger), var(--control-shadow-danger);
}

.app-select__trigger:disabled {
	background: var(--control-bg-disabled);
	color: var(--text-muted);
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
}

.app-select__value {
	display: block;
	min-width: 0;
	padding-right: var(--space-2);
	white-space: nowrap;
	overflow: hidden;
	text-overflow: ellipsis;
}

.app-select__value[data-placeholder="true"] {
	color: var(--control-placeholder);
}

.app-select__chevron {
	position: absolute;
	top: 50%;
	right: 0.88rem;
	transform: translateY(-50%);
	color: var(--text-muted);
	pointer-events: none;
	font-size: 0.94rem;
}

.app-select__listbox {
	position: absolute;
	top: calc(100% + var(--space-2));
	left: 0;
	right: 0;
	display: grid;
	gap: 0;
	padding: 0.38rem;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 90%, transparent);
	border-radius: calc(var(--radius-card) - 2px);
	background: color-mix(in srgb, var(--surface-panel) 96%, transparent);
	box-shadow: var(--shadow-ambient);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
	z-index: 20;
	animation: select-panel-in var(--duration-fast) ease;
}

.app-select__option {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: var(--space-3);
	width: 100%;
	padding: 0.62rem 0.74rem;
	border: var(--border-width) solid transparent;
	border-radius: calc(var(--radius-button) - 3px);
	background: transparent;
	color: var(--text-strong);
	font: inherit;
	font-size: 0.9rem;
	font-weight: 600;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.app-select__option[data-active="true"] {
	border-color: var(--tab-active-border);
	background: var(--tab-active-bg);
}

.app-select__option:hover:not(:disabled) {
	background: var(--button-ghost-bg-hover);
	border-color: color-mix(in srgb, var(--primary-300) 18%, var(--border-default));
}

.app-select__option:disabled {
	color: var(--text-muted);
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
}

.app-select__option:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.app-select__check {
	color: color-mix(in srgb, var(--accent-mint-400) 82%, var(--text-strong));
	font-size: 0.88rem;
	font-weight: 800;
}

@keyframes select-panel-in {
	from {
		opacity: 0;
		transform: translateY(-4px);
	}

	to {
		opacity: 1;
		transform: translateY(0);
	}
}
</style>