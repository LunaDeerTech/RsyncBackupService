<script setup lang="ts">
import { computed, ref, useId, type ComponentPublicInstance } from "vue"

export type AppTabItem = {
	value: string
	label: string
	disabled?: boolean
}

interface AppTabsProps {
	modelValue: string
	tabs: AppTabItem[]
	ariaLabel?: string
}

const props = withDefaults(defineProps<AppTabsProps>(), {
	ariaLabel: "内容切换",
})

const emit = defineEmits<{
	"update:modelValue": [value: string]
}>()

const baseId = useId()
const tabRefs = ref<Array<HTMLButtonElement | null>>([])

const activeTab = computed(
	() => props.tabs.find((tab) => tab.value === props.modelValue && tab.disabled !== true) ?? props.tabs.find((tab) => !tab.disabled),
)
const activeTabId = computed(() => (activeTab.value ? `tab-${baseId}-${activeTab.value.value}` : undefined))

function getPanelId(value: string): string {
	return `tab-panel-${baseId}-${value}`
}

function selectTab(value: string): void {
	if (props.tabs.find((tab) => tab.value === value)?.disabled) {
		return
	}

	emit("update:modelValue", value)
}

function setTabRef(element: Element | ComponentPublicInstance | null, index: number): void {
	tabRefs.value[index] = element instanceof HTMLButtonElement ? element : null
}

function focusTab(index: number): void {
	tabRefs.value[index]?.focus()
}

function findNextIndex(startIndex: number, direction: 1 | -1): number {
	const length = props.tabs.length

	for (let offset = 1; offset <= length; offset += 1) {
		const candidateIndex = (startIndex + direction * offset + length) % length
		if (!props.tabs[candidateIndex]?.disabled) {
			return candidateIndex
		}
	}

	return startIndex
}

function onKeydown(event: KeyboardEvent, index: number): void {
	let nextIndex: number | null = null

	if (event.key === "ArrowRight") {
		nextIndex = findNextIndex(index, 1)
	}

	if (event.key === "ArrowLeft") {
		nextIndex = findNextIndex(index, -1)
	}

	if (event.key === "Home") {
		nextIndex = props.tabs.findIndex((tab) => !tab.disabled)
	}

	if (event.key === "End") {
		nextIndex = [...props.tabs].reverse().findIndex((tab) => !tab.disabled)
		if (nextIndex !== -1) {
			nextIndex = props.tabs.length - 1 - nextIndex
		}
	}

	if (nextIndex === null || nextIndex < 0) {
		return
	}

	event.preventDefault()
	selectTab(props.tabs[nextIndex].value)
	focusTab(nextIndex)
}
</script>

<template>
	<section class="app-tabs">
		<div class="app-tabs__list" role="tablist" :aria-label="ariaLabel">
			<button
				v-for="(tab, index) in tabs"
				:key="tab.value"
				:ref="(element) => setTabRef(element, index)"
				type="button"
				class="app-tabs__tab"
				:id="`tab-${baseId}-${tab.value}`"
				role="tab"
				:data-active="activeTab?.value === tab.value ? 'true' : 'false'"
				:aria-selected="activeTab?.value === tab.value ? 'true' : 'false'"
				:aria-controls="$slots.default ? getPanelId(tab.value) : undefined"
				:tabindex="activeTab?.value === tab.value ? 0 : -1"
				:disabled="tab.disabled"
				@click="selectTab(tab.value)"
				@keydown="onKeydown($event, index)"
			>
				{{ tab.label }}
			</button>
		</div>

		<template v-if="$slots.default && activeTabId">
			<div
				v-for="tab in tabs"
				:key="tab.value"
				:id="getPanelId(tab.value)"
				class="app-tabs__panel"
				role="tabpanel"
				:aria-labelledby="`tab-${baseId}-${tab.value}`"
				:hidden="activeTab?.value !== tab.value"
			>
				<slot :active-tab="tab" :selected-value="activeTab?.value" />
			</div>
		</template>
	</section>
</template>

<style scoped>
.app-tabs {
	display: grid;
	gap: var(--space-4);
}

.app-tabs__list {
	display: inline-flex;
	flex-wrap: wrap;
	gap: var(--space-2);
	padding: 0.28rem;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: calc(var(--radius-card) - 2px);
	background: var(--tab-list-bg);
	width: fit-content;
	max-width: 100%;
}

.app-tabs__tab {
	min-height: 2.55rem;
	padding: 0.7rem 1rem;
	border: var(--border-width) solid transparent;
	border-radius: calc(var(--radius-button) - 2px);
	background: transparent;
	color: var(--text-muted);
	font: inherit;
	font-size: 0.92rem;
	font-weight: 700;
	cursor: pointer;
	transition:
		background var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.app-tabs__tab:hover:not(:disabled) {
	background: var(--button-ghost-bg-hover);
	color: var(--text-strong);
}

.app-tabs__tab[data-active="true"] {
	border-color: var(--tab-active-border);
	background: var(--tab-active-bg);
	color: var(--tab-active-text);
	box-shadow: inset 0 -1px 0 color-mix(in srgb, var(--accent-mint-400) 30%, transparent);
}

.app-tabs__tab:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.app-tabs__tab:disabled {
	color: var(--text-muted);
	cursor: not-allowed;
	opacity: var(--state-disabled-opacity);
}

.app-tabs__panel {
	padding: var(--space-4);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-panel) 92%, transparent);
	box-shadow: var(--shadow-ambient);
	backdrop-filter: blur(16px);
	-webkit-backdrop-filter: blur(16px);
}
</style>