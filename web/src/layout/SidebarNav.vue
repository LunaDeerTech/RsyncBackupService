<script setup lang="ts">
import { computed, onMounted, watch } from "vue"
import { useRoute, useRouter } from "vue-router"

import { getCurrentUser } from "../api/auth"
import { ApiError } from "../api/client"
import { useSession } from "../composables/useSession"
import { navigationGroups } from "../router/navigation"
import { useAuthStore } from "../stores/auth"
import { useUiStore } from "../stores/ui"

const route = useRoute()
const router = useRouter()
const session = useSession()
const auth = useAuthStore()
const ui = useUiStore()

const isAdmin = computed(() => auth.currentUser?.is_admin === true)
const username = computed(() => auth.currentUser?.username ?? "用户")
const userInitial = computed(() => username.value.charAt(0).toUpperCase())
const themeLabel = computed(() => (ui.theme === "light" ? "深色主题" : "浅色主题"))

const visibleGroups = computed(() =>
	navigationGroups
		.filter((group) => !group.requiresAdmin || isAdmin.value)
		.map((group) => ({
			...group,
			items: group.items.filter((item) => !item.requiresAdmin || isAdmin.value),
		}))
		.filter((group) => group.items.length > 0),
)

async function hydrateCurrentUser(): Promise<void> {
	if (auth.accessToken === null || auth.currentUser !== null) {
		return
	}

	try {
		auth.setCurrentUser(await getCurrentUser())
	} catch (error) {
		if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
			auth.clearSession()
		}
	}
}

watch(
	() => auth.accessToken,
	(accessToken) => {
		if (accessToken === null) {
			auth.setCurrentUser(null)
			return
		}

		void hydrateCurrentUser()
	},
	{ immediate: true },
)

onMounted(() => {
	void hydrateCurrentUser()
})

function toggleTheme(): void {
	ui.setTheme(ui.theme === "light" ? "dark" : "light")
}

function isItemActive(itemTo: string): boolean {
	if (route.path === itemTo) {
		return true
	}

	if (itemTo === "/") {
		return false
	}

	return route.path.startsWith(`${itemTo}/`)
}

async function handleLogout(): Promise<void> {
	session.logout()
	await router.push("/login")
}
</script>

<template>
	<nav class="sidebar-nav" aria-label="主导航">
		<div class="sidebar-nav__brand">
			<p class="sidebar-nav__brand-title">Rsync Backup Service</p>
			<p class="sidebar-nav__brand-subtitle">运维备份控制台</p>
		</div>

		<div class="sidebar-nav__groups">
			<div v-for="group in visibleGroups" :key="group.label" class="sidebar-nav__group">
				<p class="sidebar-nav__group-label">{{ group.label }}</p>
				<div class="sidebar-nav__links">
					<RouterLink v-for="item in group.items" :key="item.to" :to="item.to" custom>
						<template #default="{ href, navigate }">
							<a
								:href="href"
								class="sidebar-nav__link"
								:class="{ 'sidebar-nav__link--active': isItemActive(item.to) }"
								@click="navigate"
							>
								<span class="sidebar-nav__link-label">{{ item.label }}</span>
								<span v-if="item.caption" class="sidebar-nav__link-caption">{{ item.caption }}</span>
							</a>
						</template>
					</RouterLink>
				</div>
			</div>
		</div>

		<div class="sidebar-nav__footer">
			<div class="sidebar-nav__footer-actions">
				<button type="button" class="sidebar-nav__footer-btn" :aria-label="themeLabel" @click="toggleTheme">
					{{ ui.theme === "light" ? "🌙" : "☀️" }}
				</button>
				<button type="button" class="sidebar-nav__footer-btn" aria-label="退出登录" @click="handleLogout">
					退出
				</button>
			</div>
			<RouterLink to="/profile" class="sidebar-nav__user">
				<span class="sidebar-nav__user-avatar">{{ userInitial }}</span>
				<span class="sidebar-nav__user-name">{{ username }}</span>
			</RouterLink>
		</div>
	</nav>
</template>

<style scoped>
.sidebar-nav {
	display: flex;
	flex-direction: column;
	height: 100%;
	min-height: 0;
	gap: var(--space-5);
	padding: var(--space-5);
	border: var(--border-width) solid var(--shell-sidebar-border);
	border-radius: var(--radius-card);
	background: var(--shell-sidebar-bg);
	box-shadow: var(--shell-sidebar-shadow);
	overflow: hidden;
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
}

.sidebar-nav__brand {
	display: grid;
	gap: var(--space-1);
}

.sidebar-nav__brand-title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1.08rem;
	font-weight: 800;
	letter-spacing: -0.03em;
}

.sidebar-nav__brand-subtitle {
	margin: 0;
	color: var(--text-muted);
	font-size: 0.82rem;
	font-weight: 600;
	line-height: 1.5;
}

.sidebar-nav__groups {
	display: flex;
	flex: 1;
	min-height: 0;
	flex-direction: column;
	gap: var(--space-5);
	overflow-y: auto;
}

.sidebar-nav__group {
	display: flex;
	flex-direction: column;
	gap: var(--space-2);
}

.sidebar-nav__group-label {
	margin: 0;
	padding: 0 var(--space-3);
	color: var(--text-muted);
	font-size: 0.72rem;
	font-weight: 700;
	letter-spacing: 0.1em;
	text-transform: uppercase;
}

.sidebar-nav__links {
	display: flex;
	flex-direction: column;
	gap: var(--space-1);
}

.sidebar-nav__link {
	display: grid;
	gap: var(--space-1);
	padding: var(--space-3);
	border: var(--border-width) solid transparent;
	border-radius: var(--radius-card);
	background: transparent;
	color: var(--text-default);
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

.sidebar-nav__footer {
	display: flex;
	flex-direction: column;
	gap: var(--space-3);
	padding-top: var(--space-4);
	border-top: var(--border-width) solid var(--border-default);
}

.sidebar-nav__footer-actions {
	display: flex;
	align-items: center;
	gap: var(--space-2);
}

.sidebar-nav__footer-btn {
	padding: var(--space-2) var(--space-3);
	border: var(--border-width) solid var(--border-default);
	border-radius: var(--radius-button);
	background: transparent;
	color: var(--text-muted);
	font: inherit;
	font-size: 0.84rem;
	font-weight: 600;
	cursor: pointer;
	transition:
		background-color var(--duration-fast) ease,
		color var(--duration-fast) ease;
}

.sidebar-nav__footer-btn:hover {
	background: var(--nav-link-bg-hover);
	color: var(--text-strong);
}

.sidebar-nav__footer-btn:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.sidebar-nav__user {
	display: flex;
	align-items: center;
	gap: var(--space-3);
	padding: var(--space-2);
	border-radius: var(--radius-card);
	color: var(--text-default);
	text-decoration: none;
	transition: background-color var(--duration-fast) ease;
}

.sidebar-nav__user:hover {
	background: var(--nav-link-bg-hover);
}

.sidebar-nav__user:focus-visible {
	outline: none;
	box-shadow: var(--state-focus-ring);
}

.sidebar-nav__user-avatar {
	display: inline-grid;
	place-items: center;
	width: 2rem;
	height: 2rem;
	border-radius: 999px;
	background: color-mix(in srgb, var(--primary-500) 18%, var(--surface-elevated));
	color: var(--text-strong);
	font-size: 0.84rem;
	font-weight: 700;
}

.sidebar-nav__user-name {
	font-size: 0.9rem;
	font-weight: 600;
}

@media (max-width: 960px) {
	.sidebar-nav {
		padding: var(--space-4);
		overflow: visible;
	}

	.sidebar-nav__links {
		flex-direction: row;
		flex-wrap: wrap;
		gap: var(--space-2);
	}

	.sidebar-nav__groups {
		overflow-y: visible;
	}

	.sidebar-nav__footer {
		flex-direction: row;
		justify-content: space-between;
		align-items: center;
	}
}
</style>