<script setup lang="ts">
import { primaryNavigation } from "../router/navigation"
</script>

<template>
	<aside class="sidebar-nav">
		<div class="sidebar-nav__brand">
			<p class="sidebar-nav__eyebrow">Balanced Flux</p>
			<p class="sidebar-nav__title">Rsync Backup Service</p>
			<p class="sidebar-nav__caption">前端基础设施、主题与受保护布局已经就绪。</p>
		</div>

		<nav class="sidebar-nav__links" aria-label="主导航">
			<RouterLink v-for="item in primaryNavigation" :key="item.name" :to="item.to" custom>
				<template #default="{ href, navigate, isActive }">
					<a
						:href="href"
						class="sidebar-nav__link"
						:class="{ 'sidebar-nav__link--active': isActive }"
						@click="navigate"
					>
						<span class="sidebar-nav__link-label">{{ item.label }}</span>
						<span class="sidebar-nav__link-caption">{{ item.eyebrow }}</span>
					</a>
				</template>
			</RouterLink>
		</nav>
	</aside>
</template>

<style scoped>
.sidebar-nav {
	display: flex;
	flex-direction: column;
	gap: var(--space-5);
	padding: var(--space-5);
	border: var(--border-width) solid var(--shell-sidebar-border);
	border-radius: var(--radius-card);
	background: var(--shell-sidebar-bg);
	box-shadow: var(--shell-sidebar-shadow);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
}

.sidebar-nav__brand {
	display: grid;
	gap: var(--space-2);
	padding-bottom: var(--space-4);
	border-bottom: var(--border-width) solid var(--border-default);
}

.sidebar-nav__eyebrow {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.76rem;
	font-weight: 600;
	letter-spacing: 0.08em;
	text-transform: uppercase;
}

.sidebar-nav__title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1.35rem;
	font-weight: 700;
	line-height: 1.1;
	letter-spacing: -0.03em;
}

.sidebar-nav__caption {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.95rem;
}

.sidebar-nav__links {
	display: grid;
	gap: var(--space-2);
}

.sidebar-nav__link {
	display: grid;
	gap: 0.15rem;
	padding: 0.9rem 1rem;
	border: var(--border-width) solid transparent;
	border-radius: var(--radius-control);
	color: var(--text-muted);
	background: transparent;
	text-decoration: none;
	transition:
		transform var(--duration-fast) ease,
		background-color var(--duration-fast) ease,
		border-color var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.sidebar-nav__link:hover {
	transform: translateX(2px);
	background: var(--nav-link-bg-hover);
	border-color: var(--border-default);
	color: var(--text-strong);
}

.sidebar-nav__link:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.sidebar-nav__link--active {
	background: var(--nav-link-bg-active);
	border-color: color-mix(in srgb, var(--primary-500) 28%, var(--border-default));
	color: var(--nav-link-text-active);
	box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.06);
}

.sidebar-nav__link-label {
	font-weight: 700;
	font-size: 0.96rem;
}

.sidebar-nav__link-caption {
	font-size: 0.78rem;
	letter-spacing: 0.05em;
	text-transform: uppercase;
	opacity: 0.84;
}

@media (max-width: 960px) {
	.sidebar-nav {
		padding: var(--space-4);
	}

	.sidebar-nav__links {
		grid-template-columns: repeat(auto-fit, minmax(9rem, 1fr));
	}
	}
</style>