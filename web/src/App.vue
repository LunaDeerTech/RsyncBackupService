<script setup lang="ts">
import { useTheme } from "./composables/useTheme"

useTheme()
</script>

<template>
	<div data-testid="app-root">
		<RouterView />
	</div>
</template>