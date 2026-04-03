<template>
	<div class="app-shell" data-testid="app-shell">
		<aside class="app-shell__sidebar">
			<SidebarNav />
		</aside>
		<div class="app-shell__body">
			<main class="app-shell__content">
				<RouterView />
			</main>
		</div>
	</div>
</template>

<script setup lang="ts">
import SidebarNav from "./SidebarNav.vue"
</script>

<style scoped>
.app-shell {
	display: grid;
	grid-template-columns: var(--shell-sidebar-width) minmax(0, 1fr);
	min-height: 100vh;
	min-height: 100dvh;
	align-items: start;
	gap: var(--space-4);
	padding: var(--space-4);
}

.app-shell__sidebar {
	position: sticky;
	top: var(--space-4);
	height: calc(100dvh - var(--space-8));
	min-height: 0;
	align-self: start;
}

.app-shell__body {
	display: flex;
	flex-direction: column;
	min-width: 0;
	gap: var(--space-4);
}

.app-shell__content {
	flex: 1;
	min-width: 0;
	padding-bottom: var(--space-6);
}

@media (max-width: 960px) {
	.app-shell {
		grid-template-columns: 1fr;
		padding: var(--space-3);
	}

	.app-shell__sidebar {
		position: static;
		top: auto;
		height: auto;
	}
}
</style>