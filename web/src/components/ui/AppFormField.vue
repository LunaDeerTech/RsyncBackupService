<script setup lang="ts">
import { computed } from "vue"

import { provideFormFieldContext } from "./formFieldContext"

interface AppFormFieldProps {
	label: string
	error?: string
	description?: string
	required?: boolean
}

const props = withDefaults(defineProps<AppFormFieldProps>(), {
	error: undefined,
	description: undefined,
	required: false,
})

const error = computed(() => props.error)
const description = computed(() => props.description)
const required = computed(() => props.required)
const invalid = computed(() => Boolean(props.error))

const field = provideFormFieldContext({
	error,
	description,
	required,
	invalid,
})

const controlId = computed(() => field.controlId.value)
const labelId = computed(() => field.labelId.value)
const descriptionId = computed(() => field.descriptionId.value)
const errorId = computed(() => field.errorId.value)
</script>

<template>
	<div class="app-form-field" :data-invalid="invalid ? 'true' : 'false'">
		<div class="app-form-field__header">
			<label :for="controlId" :id="labelId" class="app-form-field__label">
				<span>{{ label }}</span>
			</label>
			<span v-if="required" class="app-form-field__required-text">必填</span>
		</div>

		<div class="app-form-field__control">
			<slot />
		</div>

		<p v-if="description" :id="descriptionId" class="app-form-field__description">
			{{ description }}
		</p>

		<p v-if="error" :id="errorId" class="app-form-field__error" role="alert">
			<span class="app-form-field__error-icon" aria-hidden="true">!</span>
			<span>{{ error }}</span>
		</p>
	</div>
</template>

<style scoped>
.app-form-field {
	display: grid;
	gap: var(--space-2);
}

.app-form-field__header {
	display: flex;
	justify-content: space-between;
	gap: var(--space-3);
	align-items: center;
	}

.app-form-field__label {
	display: inline-flex;
	align-items: center;
	gap: var(--space-2);
	color: var(--text-strong);
	font-size: 0.92rem;
	font-weight: 600;
}

.app-form-field__required-text,
.app-form-field__description {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.84rem;
}

.app-form-field__control {
	display: grid;
	}

.app-form-field__error {
	display: inline-flex;
	align-items: flex-start;
	gap: var(--space-2);
	margin: 0;
	color: var(--error-text);
	font-size: 0.84rem;
	font-weight: 600;
	line-height: 1.45;
}

.app-form-field__error-icon {
	display: inline-grid;
	place-items: center;
	width: 1rem;
	height: 1rem;
	margin-top: 0.08rem;
	border-radius: 999px;
	background: color-mix(in srgb, var(--error-500) 16%, transparent);
	color: var(--error-text);
	font-size: 0.76rem;
	font-weight: 800;
	flex-shrink: 0;
}
</style>