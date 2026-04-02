<script setup lang="ts">
import { computed, useAttrs, useId, watchEffect } from "vue"

export interface AppProgressProps {
	percentage: number
	speedText?: string
	etaText?: string
	tone?: "default" | "running" | "success" | "danger"
}

defineOptions({
	inheritAttrs: false,
})

const props = withDefaults(defineProps<AppProgressProps>(), {
	speedText: undefined,
	etaText: undefined,
	tone: "default",
})

const attrs = useAttrs()
const labelId = `app-progress-label-${useId()}`

const normalizedPercentage = computed(() => Math.max(0, Math.min(100, Math.round(props.percentage))))

const progressbarAttrs = computed(() => {
	const result: Record<string, unknown> = {}

	for (const [key, value] of Object.entries(attrs)) {
		if (key === "class" || key === "style" || key === "role" || key === "aria-label" || key === "aria-labelledby") {
			continue
		}

		result[key] = value
	}

	return result
})

const progressbarLabel = computed(() => (typeof attrs["aria-label"] === "string" ? attrs["aria-label"] : undefined))
const progressbarLabelledBy = computed(() =>
	typeof attrs["aria-labelledby"] === "string" ? attrs["aria-labelledby"] : labelId,
)

const toneStyle = computed(() => {
	if (props.tone === "running") {
		return {
			"--app-progress-fill-start": "var(--primary-500)",
			"--app-progress-fill-end": "var(--accent-mint-400)",
			"--app-progress-track": "color-mix(in srgb, var(--primary-300) 12%, var(--surface-elevated))",
			"--app-progress-border": "color-mix(in srgb, var(--primary-500) 30%, var(--border-default))",
		}
	}

	if (props.tone === "success") {
		return {
			"--app-progress-fill-start": "var(--success-500)",
			"--app-progress-fill-end": "var(--success-500)",
			"--app-progress-track": "var(--success-surface)",
			"--app-progress-border": "color-mix(in srgb, var(--success-500) 34%, var(--border-default))",
		}
	}

	if (props.tone === "danger") {
		return {
			"--app-progress-fill-start": "var(--error-500)",
			"--app-progress-fill-end": "var(--error-500)",
			"--app-progress-track": "color-mix(in srgb, var(--error-500) 26%, var(--surface-panel-solid))",
			"--app-progress-border": "color-mix(in srgb, var(--error-500) 74%, var(--border-default))",
		}
	}

	return {
		"--app-progress-fill-start": "var(--primary-500)",
		"--app-progress-fill-end": "var(--primary-500)",
		"--app-progress-track": "color-mix(in srgb, var(--surface-elevated) 92%, transparent)",
		"--app-progress-border": "color-mix(in srgb, var(--border-default) 94%, transparent)",
	}
})

const ariaValueText = computed(() => {
	const parts = [`${normalizedPercentage.value}%`]

	if (props.speedText) {
		parts.push(props.speedText)
	}

	if (props.etaText) {
		parts.push(props.etaText)
	}

	return parts.join(" ")
})

if (import.meta.env.DEV) {
	let hasWarnedForMissingMeta = false

	watchEffect(() => {
		const missingMeta = !props.speedText && !props.etaText

		if (missingMeta && !hasWarnedForMissingMeta) {
			console.warn("[AppProgress] Provide speedText or etaText to keep runtime metadata explicit.")
			hasWarnedForMissingMeta = true
		}

		if (!missingMeta) {
			hasWarnedForMissingMeta = false
		}
	})
}
</script>

<template>
	<section class="app-progress" :data-tone="tone" :style="toneStyle">
		<div class="app-progress__header">
			<div :id="labelId" class="app-progress__label">
				<slot name="label">执行进度</slot>
			</div>
			<strong class="app-progress__percentage">{{ normalizedPercentage }}%</strong>
		</div>

		<div
			v-bind="progressbarAttrs"
			class="app-progress__track"
			role="progressbar"
			aria-valuemin="0"
			aria-valuemax="100"
			:aria-valuenow="String(normalizedPercentage)"
			:aria-valuetext="ariaValueText"
			:aria-label="progressbarLabel"
			:aria-labelledby="progressbarLabel ? undefined : progressbarLabelledBy"
		>
			<div class="app-progress__fill" :style="{ width: `${normalizedPercentage}%` }" />
		</div>

		<div class="app-progress__meta">
			<span v-if="speedText" class="app-progress__meta-item">{{ speedText }}</span>
			<span v-if="etaText" class="app-progress__meta-item">{{ etaText }}</span>
			<span v-if="!speedText && !etaText" class="app-progress__meta-item">{{ tone === "danger" ? "等待处理" : "持续更新中" }}</span>
		</div>
	</section>
</template>

<style scoped>
.app-progress {
	display: grid;
	gap: var(--space-3);
	padding: var(--space-4);
	border: var(--border-width) solid var(--app-progress-border);
	border-radius: calc(var(--radius-card) - 2px);
	background: color-mix(in srgb, var(--surface-panel) 96%, transparent);
}

.app-progress__header,
.app-progress__meta {
	display: flex;
	justify-content: space-between;
	align-items: center;
	gap: var(--space-3);
	flex-wrap: wrap;
}

.app-progress__label,
.app-progress__meta-item {
	color: var(--text-muted);
	font-size: 0.82rem;
	font-weight: 600;
	letter-spacing: 0.02em;
}

.app-progress__percentage {
	color: var(--text-strong);
	font-size: 1rem;
	line-height: 1;
}

.app-progress__track {
	position: relative;
	height: 0.8rem;
	overflow: hidden;
	border-radius: 999px;
	background: var(--app-progress-track);
	box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--app-progress-border) 82%, transparent);
}

.app-progress__fill {
	position: relative;
	height: 100%;
	border-radius: inherit;
	background: linear-gradient(135deg, var(--app-progress-fill-start), var(--app-progress-fill-end));
	transition: width var(--duration-base) ease;
	overflow: hidden;
}

.app-progress[data-tone="running"] .app-progress__fill::after {
	content: "";
	position: absolute;
	inset: 0;
	background: linear-gradient(
		110deg,
		transparent 0%,
		color-mix(in srgb, white 24%, transparent) 45%,
		transparent 70%
	);
	animation: progress-stream 1200ms linear infinite;
	opacity: 0.76;
}

@keyframes progress-stream {
	from {
		transform: translateX(-100%);
	}

	to {
		transform: translateX(160%);
	}
}

@media (prefers-reduced-motion: reduce) {
	.app-progress__fill {
		transition: none;
	}

	.app-progress[data-tone="running"] .app-progress__fill::after {
		animation: none;
	}
}
</style>