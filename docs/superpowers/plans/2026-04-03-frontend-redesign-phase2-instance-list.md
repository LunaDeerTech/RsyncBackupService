# Phase 2: 实例列表页重构

> **For agentic workers:** Use superpowers:executing-plans to implement this phase. Each section is a sequential unit of work.

**目标：** 将 `InstancesListView.vue` 从「左列表 + 右表单同屏」布局改为「全宽列表 + Modal 表单」模式。

**前置条件：** Phase 1 已完成（路由、导航、侧边栏、TopBar 已重构）。

**设计规格来源：** `docs/superpowers/specs/2026-04-03-frontend-layout-redesign.md` 第 4.1、5.2 节。

---

## 1. 理解当前 InstancesListView 结构

**文件：** `web/src/views/InstancesListView.vue`（362 行）

**当前布局：**

```
<section class="page-view">
  <header class="page-header"> ... </header>
  <AppNotification ... /> (多个)
  <section class="page-two-column">    ← 双栏布局
    <AppCard title="实例列表">        ← 左栏：列表 + 筛选
      ...
    </AppCard>
    <AppCard title="新建/编辑实例">   ← 右栏：表单
      ...
    </AppCard>
  </section>
</section>
```

**当前逻辑：**
- 表单状态（`form` reactive 对象 + resetForm/editInstance/buildPayload/submitForm）在组件内
- SSH 密钥列表用于远程源下拉选择
- 筛选器（搜索 + 启用状态）
- 中继模式警告通知
- 表格行操作：「编辑」按钮调用 `editInstance()` 填充右侧表单 + `scrollTo(top)`，「详情」链接跳转路由

---

## 2. 重构为全宽列表 + Modal

**文件：** `web/src/views/InstancesListView.vue`

**整体改动方向：**

1. 移除 `page-two-column` 双栏布局包裹
2. 列表卡片占据全宽
3. 表单区域移入 `AppModal` + `AppDialog` 结构
4. 新增「新建实例」按钮放在页面标题旁
5. 表格「编辑」操作改为打开 Modal 并预填表单

**完整重写 template 部分：**

```vue
<template>
	<section class="page-view">
		<header class="page-header">
			<div>
				<h1 class="page-header__title">备份实例</h1>
				<p class="page-header__subtitle">管理源路径、源主机和实例级恢复入口。</p>
			</div>
			<AppButton @click="openCreateModal">新建实例</AppButton>
		</header>

		<AppNotification v-if="errorMessage" title="实例列表加载失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="实例已保存" tone="success" :description="successMessage" />
		<AppNotification
			v-if="hasConfirmedRelayInstances"
			title="中继模式"
			tone="warning"
			description="存在源与目标均为远程主机的实例组合。恢复或滚动同步时将使用本机缓存目录中继，请确认磁盘空间。"
		/>
		<AppNotification
			v-if="hasPotentialRelayInstances"
			title="可能经过中继缓存"
			tone="warning"
			description="当前账户无法读取目标类型。若远程源绑定了 SSH 目标，恢复或滚动同步会经过本机缓存目录，请预留磁盘空间。"
		/>

		<AppCard title="实例列表" description="支持按名称、主机或路径筛选。">
			<div class="page-form-grid">
				<AppFormField label="搜索">
					<AppInput v-model="query" placeholder="名称 / 主机 / 路径" />
				</AppFormField>
				<AppFormField label="启用状态">
					<AppSelect
						v-model="enabledFilter"
						:options="[
							{ value: 'all', label: '全部' },
							{ value: 'enabled', label: '已启用' },
							{ value: 'disabled', label: '已停用' },
						]"
					/>
				</AppFormField>
			</div>

			<AppTable
				:rows="filteredInstances"
				:columns="[
					{ key: 'name', label: '实例' },
					{ key: 'source_path', label: '源路径' },
					{ key: 'strategy_count', label: '策略' },
					{ key: 'last_backup_status', label: '最近状态' },
					{ key: 'enabled', label: '启用' },
					{ key: 'actions', label: '操作' },
				]"
				row-key="id"
			>
				<template #cell-name="{ row }">
					<div class="instances-list__name-cell">
						<RouterLink class="instances-list__detail-link" :to="`/instances/${row.id}`">{{ row.name }}</RouterLink>
						<AppTag v-if="row.relay_mode" tone="warning">中继模式</AppTag>
						<AppTag v-else-if="row.relay_mode_uncertain" tone="warning">可能中继</AppTag>
					</div>
				</template>
				<template #cell-source_path="{ row }">
					<div class="instances-list__source-cell">
						<span>{{ formatSource(row.source_type, row.source_path, row.source_host) }}</span>
						<span class="page-muted">{{ formatDateTime(row.updated_at) }}</span>
					</div>
				</template>
				<template #cell-strategy_count="{ value }">
					<span>{{ value }} 条策略</span>
				</template>
				<template #cell-last_backup_status="{ row }">
					<div class="instances-list__status-cell">
						<AppTag :tone="statusTone(row.last_backup_status)">{{ formatStatusLabel(row.last_backup_status) }}</AppTag>
						<span class="page-muted">{{ formatDateTime(row.last_backup_at) }}</span>
					</div>
				</template>
				<template #cell-enabled="{ value }">
					<AppTag :tone="value ? 'success' : 'warning'">{{ value ? "已启用" : "已停用" }}</AppTag>
				</template>
				<template #cell-actions="{ row }">
					<div class="page-action-row--wrap">
						<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
						<RouterLink class="instances-list__detail-link" :to="`/instances/${row.id}`">详情</RouterLink>
					</div>
				</template>
			</AppTable>

			<AppEmpty
				v-if="!isLoading && filteredInstances.length === 0"
				title="当前没有实例"
				description="点击上方「新建实例」按钮创建第一个备份实例。"
				compact
			/>
		</AppCard>

		<!-- 新建/编辑 Modal -->
		<AppModal :open="modalOpen" width="34rem" @close="closeModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">{{ form.id === '' ? '新建实例' : '编辑实例' }}</h2>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="prod-web" />
						</AppFormField>

						<AppFormField label="源类型" required>
							<AppSelect
								v-model="form.sourceType"
								:options="[
									{ value: 'local', label: '本地路径' },
									{ value: 'remote', label: '远程主机' },
								]"
							/>
						</AppFormField>

						<AppFormField label="源路径" required>
							<AppInput v-model="form.sourcePath" placeholder="/srv/data" />
						</AppFormField>

						<AppFormField label="启用实例">
							<AppSwitch v-model="form.enabled" />
						</AppFormField>
					</div>

					<div v-if="form.sourceType === 'remote'" class="page-form-grid">
						<AppFormField label="源主机" required>
							<AppInput v-model="form.sourceHost" placeholder="192.0.2.10" />
						</AppFormField>

						<AppFormField label="端口">
							<AppInput v-model="form.sourcePort" inputmode="numeric" />
						</AppFormField>

						<AppFormField label="源用户" required>
							<AppInput v-model="form.sourceUser" placeholder="backup" />
						</AppFormField>

						<AppFormField label="SSH 密钥">
							<AppSelect v-model="form.sourceSSHKeyID" :options="sshKeyOptions" />
						</AppFormField>
					</div>

					<AppFormField label="排除模式" description="每行一个模式，例如 node_modules 或 *.tmp。">
						<textarea v-model="form.excludePatterns" class="instances-list__textarea" rows="4" />
					</AppFormField>

					<AppNotification v-if="formError" title="保存失败" tone="danger" :description="formError" />

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? "创建实例" : "保存修改" }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>
	</section>
</template>
```

---

## 3. 更新 script 部分

在 `<script setup>` 中做以下修改：

### 3.1 新增导入

添加 `AppModal` 的导入：

```typescript
import AppModal from "../components/ui/AppModal.vue"
```

### 3.2 移除不需要的导入

不再需要的导入（如果表单卡片不需要 `AppCard` 包裹则保留 `AppCard` 用于列表卡片）：
- 保留 `AppCard`（列表仍用卡片包裹）

### 3.3 新增 Modal 状态

```typescript
const modalOpen = ref(false)
```

### 3.4 新增 Modal 操作方法

```typescript
function openCreateModal(): void {
	resetForm()
	modalOpen.value = true
}

function openEditModal(instance: InstanceSummary): void {
	form.id = String(instance.id)
	form.name = instance.name
	form.sourceType = instance.source_type
	form.sourceHost = instance.source_host ?? ""
	form.sourcePort = String(instance.source_port)
	form.sourceUser = instance.source_user ?? ""
	form.sourceSSHKeyID = instance.source_ssh_key_id ? String(instance.source_ssh_key_id) : ""
	form.sourcePath = instance.source_path
	form.excludePatterns = instance.exclude_patterns.join("\n")
	form.enabled = instance.enabled
	formError.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}
	modalOpen.value = false
}
```

### 3.5 修改 submitForm

在 `submitForm` 成功路径中，关闭 Modal：

```typescript
async function submitForm(): Promise<void> {
	formError.value = ""
	successMessage.value = ""
	isSubmitting.value = true

	try {
		if (form.id === "") {
			await createInstance(buildPayload())
			successMessage.value = "实例已创建。"
		} else {
			await updateInstance(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "实例已更新。"
		}

		modalOpen.value = false
		resetForm()
		await loadData()
	} catch (error) {
		formError.value = error instanceof ApiError ? error.message : "保存实例失败。"
	} finally {
		isSubmitting.value = false
	}
}
```

### 3.6 删除旧的 editInstance 函数

旧的 `editInstance` 函数包含 `window.scrollTo({ top: 0, behavior: "smooth" })`。替换为 `openEditModal`。

### 3.7 页面标题按钮变更

原来的「新建实例」按钮调用 `resetForm`，现在改为调用 `openCreateModal`。

---

## 4. 新增 Modal 表单相关 CSS

在 `<style scoped>` 中添加 Modal 表单的通用样式：

```css
.page-modal-form {
	display: grid;
	gap: var(--space-4);
	padding: var(--space-5);
}

.page-modal-form__header {
	display: grid;
	gap: var(--space-2);
}

.page-modal-form__title {
	margin: 0;
	color: var(--text-strong);
	font-size: 1.25rem;
	font-weight: 700;
	letter-spacing: -0.02em;
}
```

**注意：** 如果 `page-modal-form` 系列样式会在多个页面复用，也可以将其添加到全局 `web/src/styles/application.css` 中。建议先在当前组件 scoped 中定义，等 Phase 3+ 需要复用时再抽取到全局。

---

## 5. 验证与提交

1. 确认编译通过：

```bash
npm --prefix web run build
```

2. 检查是否有 TypeScript 错误。

3. 如果有前端测试引用了 `InstancesListView` 中旧的函数名（如 `editInstance`），需要同步更新。

4. 提交：

```bash
git add -A
git commit -m "refactor(web): Phase 2 — InstancesListView full-width + Modal form"
```

---

## 6. 启动服务并测试

启动完整服务：

```bash
make run
```

同时启动前端开发服务器：

```bash
npm --prefix web run dev
```

然后使用 `askQuestion` 工具向用户提出以下测试问题：

**问题标题：** Phase 2 实例列表页测试

**测试清单（请用户逐项确认）：**

1. **全宽布局** — 实例列表页是否占据了整个内容区域的宽度？是否不再有左右双栏布局？
2. **新建 Modal** — 点击页面标题旁的「新建实例」按钮，是否弹出居中 Modal？Modal 中是否包含完整的实例创建表单（名称、源类型、源路径、启用开关、远程主机信息等）？
3. **编辑 Modal** — 点击表格行的「编辑」按钮，是否弹出 Modal 且表单已预填当前实例信息？
4. **Modal 关闭** — 按 ESC 键或点击遮罩层，Modal 是否正常关闭？点击「取消」按钮，Modal 是否关闭？
5. **创建流程** — 在 Modal 中填写表单并提交，是否成功创建实例？Modal 是否自动关闭且列表自动刷新？
6. **编辑流程** — 编辑已有实例并保存，是否成功更新？Modal 是否自动关闭且列表反映更新？
7. **筛选功能** — 搜索框和启用状态筛选是否仍然正常工作？
8. **详情跳转** — 表格中的实例名称链接和「详情」链接是否仍然正确跳转到实例详情页？
9. **空态提示** — 当没有实例时，空态提示文案是否显示为「点击上方「新建实例」按钮创建第一个备份实例。」？
10. **提交中状态** — 表单提交过程中，提交按钮是否显示 loading 状态？此时 Modal 是否不可关闭？
