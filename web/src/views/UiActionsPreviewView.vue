<script setup lang="ts">
import { computed, ref } from "vue"

import AppBreadcrumb from "../components/ui/AppBreadcrumb.vue"
import AppButton from "../components/ui/AppButton.vue"
import AppDialog from "../components/ui/AppDialog.vue"
import AppFormField from "../components/ui/AppFormField.vue"
import AppInput from "../components/ui/AppInput.vue"
import AppModal from "../components/ui/AppModal.vue"
import AppPasswordInput from "../components/ui/AppPasswordInput.vue"
import AppSelect from "../components/ui/AppSelect.vue"
import AppSwitch from "../components/ui/AppSwitch.vue"
import AppTabs from "../components/ui/AppTabs.vue"
import AppTextarea from "../components/ui/AppTextarea.vue"

const instanceName = ref("prod-main")
const sourcePath = ref("/srv/data")
const notes = ref("删除前需要二次确认，并提示会一并删除关联策略。")
const storageTarget = ref("local")
const password = ref("secret-value")
const notificationsEnabled = ref(true)
const activeTab = ref("inputs")
const confirmDialogOpen = ref(false)
const modalOpen = ref(false)

const tabItems = [
	{ value: "inputs", label: "输入组件" },
	{ value: "actions", label: "操作组件" },
	{ value: "danger", label: "危险交互" },
]

const selectOptions = [
	{ value: "local", label: "本地目录" },
	{ value: "ssh", label: "SSH 远端" },
	{ value: "archive", label: "冷备归档" },
]

const breadcrumbItems = [
	{ label: "组件系统", to: "/ui-preview" },
	{ label: "Task 11 预览", current: true },
]

const activeTabLabel = computed(() => tabItems.find((item) => item.value === activeTab.value)?.label ?? "输入组件")
</script>

<template>
	<div class="ui-preview">
		<section class="ui-preview__hero">
			<div class="ui-preview__hero-copy">
				<AppBreadcrumb :items="breadcrumbItems" />
				<p class="ui-preview__eyebrow">Task 11 Preview</p>
				<h1 class="ui-preview__title">输入、操作与危险交互组件</h1>
				<p class="ui-preview__body">
					这个临时页面集中展示本任务落地的按钮、表单控件、Tabs、Breadcrumb、模态框和危险确认交互，便于快速核对焦点环、禁用态和风险语义。
				</p>
			</div>

			<div class="ui-preview__hero-actions">
				<AppButton variant="primary" @click="modalOpen = true">打开普通模态框</AppButton>
				<AppButton variant="danger" @click="confirmDialogOpen = true">打开危险确认</AppButton>
			</div>
		</section>

		<AppTabs v-model="activeTab" :tabs="tabItems" aria-label="Task 11 组件分类">
			<template #default="{ activeTab: currentTab }">
				<div class="ui-preview__tab-header">
					<h2 class="ui-preview__section-title">{{ activeTabLabel }}</h2>
					<p class="ui-preview__section-body">聚焦可复用 API、清晰焦点和独立危险态语义。</p>
				</div>

				<div v-if="currentTab?.value === 'inputs'" class="ui-preview__grid ui-preview__grid--inputs">
					<div class="ui-preview__card">
						<h3>基础表单</h3>
						<div class="ui-preview__stack">
							<AppFormField label="实例名称" description="名称将展示在实例列表和审计日志中。" required>
								<AppInput v-model="instanceName" placeholder="例如 prod-main" />
							</AppFormField>

							<AppFormField label="源路径" error="源路径不能为空，且必须是绝对路径。" required>
								<AppInput v-model="sourcePath" invalid placeholder="/srv/project" />
							</AppFormField>

							<AppFormField label="存储目标" description="切换不同目标时，错误态和禁用态保持一致。">
								<AppSelect v-model="storageTarget" :options="selectOptions" />
							</AppFormField>

							<AppFormField label="危险备注" description="错误文案需要颜色、图标和文本同时表达。">
								<AppTextarea v-model="notes" placeholder="填写操作说明或风险提示" />
							</AppFormField>
						</div>
					</div>

					<div class="ui-preview__card">
						<h3>密码与开关</h3>
						<div class="ui-preview__stack">
							<AppFormField label="加密密码" description="可切换明文查看，但保持键盘焦点清晰。" required>
								<AppPasswordInput v-model="password" placeholder="请输入密码" />
							</AppFormField>

							<AppFormField label="通知渠道" description="开关使用原生 checkbox 语义并映射为 switch。">
								<div class="ui-preview__switch-row">
									<AppSwitch v-model="notificationsEnabled" />
									<span>{{ notificationsEnabled ? "已启用" : "已停用" }}</span>
								</div>
							</AppFormField>

							<AppFormField label="禁用态输入" description="禁用后仍然保留轮廓，不让控件消失。">
								<AppInput model-value="/mnt/archive" disabled aria-label="禁用态路径" />
							</AppFormField>
						</div>
					</div>
				</div>

				<div v-else-if="currentTab?.value === 'actions'" class="ui-preview__grid ui-preview__grid--actions">
					<div class="ui-preview__card">
						<h3>按钮层级</h3>
						<div class="ui-preview__button-row">
							<AppButton variant="primary">保存策略</AppButton>
							<AppButton variant="secondary">测试连接</AppButton>
							<AppButton variant="ghost">取消</AppButton>
							<AppButton variant="primary" loading>提交中</AppButton>
						</div>
					</div>

					<div class="ui-preview__card">
						<h3>Breadcrumb</h3>
						<p class="ui-preview__card-copy">面包屑保持轻量，不抢走高风险操作的注意力。</p>
						<AppBreadcrumb :items="breadcrumbItems" />
					</div>
				</div>

				<div v-else class="ui-preview__grid ui-preview__grid--danger">
					<div class="ui-preview__card ui-preview__card--danger">
						<h3>危险按钮与说明</h3>
						<p class="ui-preview__card-copy">
							危险动作独立使用 error 体系，不复用 Cyan Mint 主品牌色，避免误导用户低估风险。
						</p>
						<div class="ui-preview__button-row">
							<AppButton variant="danger">删除实例</AppButton>
							<AppButton variant="danger" loading>删除中</AppButton>
						</div>
					</div>

					<div class="ui-preview__card">
						<h3>焦点管理</h3>
						<p class="ui-preview__card-copy">
							打开弹窗后，首个可聚焦元素会自动聚焦，Tab 与 Shift+Tab 在弹窗内部循环，Escape 触发关闭事件。
						</p>
						<div class="ui-preview__button-row">
							<AppButton variant="secondary" @click="modalOpen = true">预览普通模态框</AppButton>
							<AppButton variant="danger" @click="confirmDialogOpen = true">预览危险确认</AppButton>
						</div>
					</div>
				</div>
			</template>
		</AppTabs>

		<AppModal :open="modalOpen" labelled-by="preview-modal-title" described-by="preview-modal-body" @close="modalOpen = false">
			<section class="ui-preview__dialog-shell">
				<h2 id="preview-modal-title">普通模态框</h2>
				<p id="preview-modal-body">
					这个模态框复用和对话框相同的 focus trap。首个按钮会在打开后获得焦点，适合接入表单或说明性内容。
				</p>
				<div class="ui-preview__dialog-actions">
					<AppButton variant="ghost" @click="modalOpen = false">关闭</AppButton>
					<AppButton variant="primary">继续编辑</AppButton>
				</div>
			</section>
		</AppModal>

		<AppDialog :open="confirmDialogOpen" title="删除备份实例" tone="danger" @close="confirmDialogOpen = false">
			<p>此操作不可撤销，实例、策略和最近一次备份记录都会被永久移除。</p>

			<template #actions>
				<AppButton variant="ghost" @click="confirmDialogOpen = false">取消</AppButton>
				<AppButton variant="danger" @click="confirmDialogOpen = false">确认删除</AppButton>
			</template>
		</AppDialog>
	</div>
</template>

<style scoped>
.ui-preview {
	display: grid;
	gap: var(--space-6);
	padding: clamp(var(--space-4), 3.6vw, var(--space-8));
	max-width: 82rem;
	margin: 0 auto;
	min-height: 100vh;
	background:
		radial-gradient(circle at top left, color-mix(in srgb, var(--primary-300) 24%, transparent), transparent 32%),
		radial-gradient(circle at bottom right, color-mix(in srgb, var(--accent-mint-400) 14%, transparent), transparent 28%),
		linear-gradient(180deg, var(--surface-canvas), var(--surface-subtle));
}

.ui-preview__hero {
	display: grid;
	grid-template-columns: minmax(0, 1.25fr) minmax(15.5rem, 0.68fr);
	gap: var(--space-5);
	padding: var(--space-6);
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: calc(var(--radius-card) + 2px);
	background: color-mix(in srgb, var(--surface-panel) 92%, transparent);
	box-shadow: var(--shadow-ambient);
	backdrop-filter: blur(18px);
	-webkit-backdrop-filter: blur(18px);
}

.ui-preview__hero-copy {
	display: grid;
	gap: var(--space-3);
}

.ui-preview__eyebrow,
.ui-preview__section-body,
.ui-preview__body,
.ui-preview__card-copy {
	margin: 0;
	color: var(--text-muted);
}

.ui-preview__eyebrow {
	font-size: 0.78rem;
	font-weight: 700;
	letter-spacing: 0.08em;
	text-transform: uppercase;
}

.ui-preview__title,
.ui-preview__section-title,
.ui-preview__card h3,
.ui-preview__dialog-shell h2 {
	margin: 0;
	color: var(--text-strong);
	line-height: 1.05;
	letter-spacing: -0.04em;
}

.ui-preview__title {
	font-size: clamp(2.05rem, 4vw, 3.45rem);
}

.ui-preview__hero-actions,
.ui-preview__stack,
.ui-preview__dialog-shell {
	display: grid;
	gap: var(--space-4);
}

.ui-preview__hero-actions {
	align-content: center;
	}

.ui-preview__tab-header {
	display: grid;
	gap: var(--space-2);
	margin-bottom: var(--space-4);
}

.ui-preview__grid {
	display: grid;
	gap: var(--space-4);
}

.ui-preview__grid--inputs {
	grid-template-columns: repeat(2, minmax(0, 1fr));
}

.ui-preview__grid--actions,
.ui-preview__grid--danger {
	grid-template-columns: repeat(2, minmax(0, 1fr));
}

.ui-preview__card {
	display: grid;
	gap: var(--space-4);
	padding: 1.58rem 1.64rem;
	border: var(--border-width) solid color-mix(in srgb, var(--border-default) 88%, transparent);
	border-radius: var(--radius-card);
	background: color-mix(in srgb, var(--surface-panel) 94%, transparent);
	box-shadow: var(--shadow-ambient);
	backdrop-filter: blur(16px);
	-webkit-backdrop-filter: blur(16px);
}

.ui-preview__card--danger {
	border-color: var(--state-danger-border);
	background: color-mix(in srgb, var(--error-surface) 82%, var(--surface-panel));
}

.ui-preview__button-row,
.ui-preview__switch-row,
.ui-preview__dialog-actions {
	display: flex;
	gap: var(--space-3);
	align-items: center;
	flex-wrap: wrap;
}

.ui-preview__switch-row span {
	color: var(--text-strong);
	font-weight: 600;
}

.ui-preview__dialog-shell {
	padding: var(--space-5);
}

.ui-preview__dialog-shell p {
	margin: 0;
	color: var(--text-muted);
	line-height: 1.6;
}

@media (max-width: 960px) {
	.ui-preview__hero,
	.ui-preview__grid--inputs,
	.ui-preview__grid--actions,
	.ui-preview__grid--danger {
		grid-template-columns: 1fr;
	}

	.ui-preview__hero,
	.ui-preview__dialog-shell {
		padding: var(--space-4);
	}

	.ui-preview__card {
		padding: var(--space-5);
	}
}
</style>