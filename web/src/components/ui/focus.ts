const focusableSelector = [
	"a[href]",
	"button:not([disabled])",
	"input:not([disabled]):not([type='hidden'])",
	"select:not([disabled])",
	"textarea:not([disabled])",
	"[tabindex]:not([tabindex='-1'])",
].join(",")

function isHTMLElement(candidate: Element | null): candidate is HTMLElement {
	return candidate instanceof HTMLElement
}

function isFocusable(element: HTMLElement): boolean {
	return !element.hasAttribute("hidden") && element.getAttribute("aria-hidden") !== "true"
}

export function getFocusableElements(container: ParentNode): HTMLElement[] {
	return Array.from(container.querySelectorAll(focusableSelector)).filter(isHTMLElement).filter(isFocusable)
}

export function focusFirstElement(container: HTMLElement): boolean {
	const [first] = getFocusableElements(container)

	if (!first) {
		return false
	}

	first.focus()
	return true
}

export function cycleFocus(container: HTMLElement, direction: 1 | -1): void {
	const focusable = getFocusableElements(container)

	if (focusable.length === 0) {
		container.focus()
		return
	}

	const activeElement = document.activeElement
	const currentIndex = focusable.findIndex((element) => element === activeElement)
	const fallbackIndex = direction === 1 ? 0 : focusable.length - 1

	if (currentIndex === -1) {
		focusable[fallbackIndex]?.focus()
		return
	}

	const nextIndex = (currentIndex + direction + focusable.length) % focusable.length
	focusable[nextIndex]?.focus()
}