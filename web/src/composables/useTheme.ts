import { watchEffect } from "vue"

import { applyDocumentTheme, useUiStore } from "../stores/ui"

export function useTheme(): void {
	const ui = useUiStore()

	watchEffect(() => {
		applyDocumentTheme(ui.theme)
	})
}