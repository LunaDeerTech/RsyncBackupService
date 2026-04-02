<script setup lang="ts">
import { RouterLink } from "vue-router"

export type AppBreadcrumbItem = {
	label: string
	to?: string
	current?: boolean
}

interface AppBreadcrumbProps {
	items: AppBreadcrumbItem[]
}

defineProps<AppBreadcrumbProps>()
</script>

<template>
	<nav class="app-breadcrumb" aria-label="面包屑导航">
		<ol class="app-breadcrumb__list">
			<li v-for="(item, index) in items" :key="`${item.label}-${index}`" class="app-breadcrumb__item">
				<RouterLink v-if="item.to && !item.current" v-slot="{ href, navigate }" custom :to="item.to">
					<a class="app-breadcrumb__link" :href="href" @click="navigate">
						{{ item.label }}
					</a>
				</RouterLink>
				<span v-else class="app-breadcrumb__current" :aria-current="item.current ? 'page' : undefined">
					{{ item.label }}
				</span>
				<span v-if="index < items.length - 1" class="app-breadcrumb__separator" aria-hidden="true">/</span>
			</li>
		</ol>
	</nav>
</template>

<style scoped>
.app-breadcrumb__list {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: var(--space-2);
	padding: 0;
	margin: 0;
	list-style: none;
}

.app-breadcrumb__item {
	display: inline-flex;
	align-items: center;
	gap: var(--space-2);
	min-width: 0;
}

.app-breadcrumb__link,
.app-breadcrumb__current {
	display: inline-flex;
	align-items: center;
	padding: 0.34rem 0.56rem;
	border-radius: 999px;
	text-decoration: none;
	font-size: 0.86rem;
	font-weight: 600;
	line-height: 1;
	transition:
		background var(--duration-fast) ease,
		color var(--duration-fast) ease,
		box-shadow var(--duration-fast) ease;
}

.app-breadcrumb__link {
	color: var(--text-muted);
}

.app-breadcrumb__link:hover {
	background: var(--breadcrumb-link-bg-hover);
	color: var(--text-strong);
}

.app-breadcrumb__link:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.app-breadcrumb__current {
	background: color-mix(in srgb, var(--primary-300) 12%, var(--surface-panel-solid));
	color: var(--text-strong);
}

.app-breadcrumb__separator {
	color: var(--breadcrumb-separator);
	font-size: 0.84rem;
	font-weight: 600;
}
</style>