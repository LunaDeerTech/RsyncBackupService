<script setup lang="ts">
import { ref } from "vue"

import AppTabs from "../components/ui/AppTabs.vue"

const activeTab = ref("users")

const tabs = [
	{ value: "users", label: "用户管理" },
	{ value: "ssh-keys", label: "SSH 密钥" },
	{ value: "notifications", label: "通知渠道" },
	{ value: "audit-logs", label: "审计日志" },
]
</script>

<template>
	<section class="page-view">
		<header class="page-header">
			<div>
				<h1 class="page-header__title">系统管理</h1>
				<p class="page-header__subtitle">用户管理、SSH 密钥、通知渠道与审计日志。</p>
			</div>
		</header>

		<AppTabs v-model="activeTab" :tabs="tabs" aria-label="系统管理标签" />

		<p class="page-muted">{{ activeTab }} — 此标签页内容将在后续阶段实现。</p>
	</section>
</template>