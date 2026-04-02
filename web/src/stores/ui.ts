import { reactive } from "vue"

export type ThemeMode = "light" | "dark"

export const THEME_STORAGE_KEY = "rbs.ui.theme"

type UiStore = {
	theme: ThemeMode
	setTheme(theme: ThemeMode): void
}

function readStoredTheme(): ThemeMode | null {
	if (typeof localStorage === "undefined") {
		return null
	}

	const raw = localStorage.getItem(THEME_STORAGE_KEY)
	if (raw === "light" || raw === "dark") {
		return raw
	}

	if (raw !== null) {
		localStorage.removeItem(THEME_STORAGE_KEY)
	}

	return null
}

function resolveInitialTheme(): ThemeMode {
	const storedTheme = readStoredTheme()
	if (storedTheme !== null) {
		return storedTheme
	}

	if (
		typeof window !== "undefined" &&
		typeof window.matchMedia === "function" &&
		window.matchMedia("(prefers-color-scheme: dark)").matches
	) {
		return "dark"
	}

	return "light"
}

function persistTheme(theme: ThemeMode): void {
	if (typeof localStorage === "undefined") {
		return
	}

	localStorage.setItem(THEME_STORAGE_KEY, theme)
}

export function applyDocumentTheme(theme: ThemeMode): void {
	if (typeof document === "undefined") {
		return
	}

	document.documentElement.dataset.theme = theme
	document.documentElement.style.colorScheme = theme
}

const initialTheme = resolveInitialTheme()

const uiStore = reactive<UiStore>({
	theme: initialTheme,
	setTheme(theme) {
		uiStore.theme = theme
		persistTheme(theme)
		applyDocumentTheme(theme)
	},
})

applyDocumentTheme(initialTheme)

export function useUiStore(): UiStore {
	return uiStore
}