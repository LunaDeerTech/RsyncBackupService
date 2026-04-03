<script setup lang="ts">
import { ref } from "vue"

import AppTabs from "../components/ui/AppTabs.vue"
import AuditLogsTab from "./system/AuditLogsTab.vue"
import NotificationChannelsTab from "./system/NotificationChannelsTab.vue"
import SSHKeysTab from "./system/SSHKeysTab.vue"
import UsersTab from "./system/UsersTab.vue"

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
		<header class="page-header page-header--inset page-header--shell-aligned">
			<div class="page-header__content">
				<p class="page-header__eyebrow">SYSTEM</p>
				<h1 class="page-header__title">系统管理</h1>
				<p class="page-header__subtitle">用户管理、SSH 密钥、通知渠道与审计日志。</p>
			</div>
		</header>

		<AppTabs v-model="activeTab" :tabs="tabs" aria-label="系统管理标签" />

		<UsersTab v-if="activeTab === 'users'" />
		<SSHKeysTab v-else-if="activeTab === 'ssh-keys'" />
		<NotificationChannelsTab v-else-if="activeTab === 'notifications'" />
		<AuditLogsTab v-else-if="activeTab === 'audit-logs'" />
	</section>
</template>