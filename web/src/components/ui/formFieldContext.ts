import {
	computed,
	inject,
	provide,
	ref,
	useId,
	type ComputedRef,
	type InjectionKey,
} from "vue"

type FormFieldContext = {
	controlId: ComputedRef<string>
	labelId: ComputedRef<string>
	descriptionId: ComputedRef<string | undefined>
	errorId: ComputedRef<string | undefined>
	describedBy: ComputedRef<string | undefined>
	invalid: ComputedRef<boolean>
	required: ComputedRef<boolean>
	setControlId: (id?: string) => void
}

const formFieldContextKey: InjectionKey<FormFieldContext> = Symbol("form-field-context")

type ProvideFormFieldOptions = {
	error: ComputedRef<string | undefined>
	description: ComputedRef<string | undefined>
	required: ComputedRef<boolean>
	invalid: ComputedRef<boolean>
}

export function provideFormFieldContext(options: ProvideFormFieldOptions): FormFieldContext {
	const baseId = useId()
	const overrideControlId = ref<string | undefined>()
	const controlId = computed(() => overrideControlId.value ?? `field-${baseId}`)
	const labelId = computed(() => `field-label-${baseId}`)
	const descriptionId = computed(() => (options.description.value ? `field-description-${baseId}` : undefined))
	const errorId = computed(() => (options.error.value ? `field-error-${baseId}` : undefined))
	const describedBy = computed(() => {
		const ids = [descriptionId.value, errorId.value].filter((value): value is string => typeof value === "string")
		return ids.length > 0 ? ids.join(" ") : undefined
	})

	const context: FormFieldContext = {
		controlId,
		labelId,
		descriptionId,
		errorId,
		describedBy,
		invalid: options.invalid,
		required: options.required,
		setControlId: (id?: string) => {
			overrideControlId.value = id
		},
	}

	provide(formFieldContextKey, context)

	return context
}

export function useFormFieldContext(): FormFieldContext | null {
	return inject(formFieldContextKey, null)
}