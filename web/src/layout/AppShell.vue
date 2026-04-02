<template>
	<div class="app-shell" data-testid="app-shell">
		<SidebarNav />
		<div class="app-shell__body">
			<TopBar />
			<main class="app-shell__content">
				<RouterView />
			</main>
		</div>
	</div>
</template>

<script setup lang="ts">
import SidebarNav from "./SidebarNav.vue"
import TopBar from "./TopBar.vue"
</script>

<style scoped>
.app-shell {
	display: grid;
	grid-template-columns: var(--shell-sidebar-width) minmax(0, 1fr);
	min-height: 100vh;
	gap: var(--space-4);
	padding: var(--space-4);
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
}
</style>