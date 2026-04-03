<script setup lang="ts">
import { computed } from "vue"

import AppCard from "../components/ui/AppCard.vue"
import { useAuthStore } from "../stores/auth"

const auth = useAuthStore()
const username = computed(() => auth.currentUser?.username ?? "—")
const isAdmin = computed(() => auth.currentUser?.is_admin === true)
</script>

<template>
	<section class="page-view">
		<AppCard title="当前会话" description="你的登录账户信息。">
			<p>用户名：{{ username }}</p>
			<p>角色：{{ isAdmin ? "管理员" : "普通用户" }}</p>
		</AppCard>

		<p class="page-muted">密码修改功能将在后续阶段实现。</p>
	</section>
</template>