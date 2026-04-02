<script setup lang="ts">
interface AppToastHostProps {
	position?: "top-right" | "top-left" | "bottom-right" | "bottom-left"
	label?: string
	announce?: boolean
}

withDefaults(defineProps<AppToastHostProps>(), {
	position: "top-right",
	label: "系统通知",
	announce: false,
})
</script>

<template>
	<section class="app-toast-host" :data-position="position" role="region" :aria-live="announce ? 'polite' : undefined" :aria-label="label">
		<slot />
	</section>
</template>

<style scoped>
.app-toast-host {
	position: fixed;
	z-index: 60;
	display: grid;
	gap: var(--space-3);
	width: min(24rem, calc(100vw - 2rem));
	pointer-events: none;
}

.app-toast-host[data-position="top-right"] {
	top: var(--space-4);
	right: var(--space-4);
}

.app-toast-host[data-position="top-left"] {
	top: var(--space-4);
	left: var(--space-4);
}

.app-toast-host[data-position="bottom-right"] {
	right: var(--space-4);
	bottom: var(--space-4);
}

.app-toast-host[data-position="bottom-left"] {
	left: var(--space-4);
	bottom: var(--space-4);
}

.app-toast-host > * {
	pointer-events: auto;
}

@media (max-width: 640px) {
	.app-toast-host {
		left: var(--space-4);
		right: var(--space-4);
		width: auto;
	}
}
</style>