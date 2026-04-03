# Phase 3: 存储目标页重构

> **For agentic workers:** Use superpowers:executing-plans to implement this phase. Each section is a sequential unit of work.

**目标：** 将 `StorageTargetsView.vue` 从「左分组表格 + 右表单同屏」布局改为「全宽分组列表 + Modal 表单」模式。增加删除确认 Dialog 和测试连通性操作 Modal。

**前置条件：** Phase 1（全局骨架）和 Phase 2（实例列表）已完成。

**设计规格来源：** `docs/superpowers/specs/2026-04-03-frontend-layout-redesign.md` 第 4.1、4.2、5.4 节。

---

## 1. 理解当前 StorageTargetsView 结构

**文件：** `web/src/views/StorageTargetsView.vue`（271 行）

**当前布局：**

```
<section class="page-view">
  <header> ... </header>
  <AppNotification ... />
  <section class="page-two-column">
    <div class="page-stack">             ← 左栏：分组表格
      <AppCard v-for="group in ...">
        <AppTable ... />
      </AppCard>
    </div>
    <AppCard title="新建/编辑存储目标">  ← 右栏：表单
      <form ... />
    </AppCard>
  </section>
</section>
```

**当前逻辑：**
- `form` reactive 对象管理新建/编辑状态
- `groupedTargets` computed 将目标按 rolling/cold 分组
- 行操作：编辑（填表单 + scrollTo）、测试（调 API testStorageTarget）、删除（直接调 API deleteStorageTarget，无确认）
- SSH 密钥列表用于远程目标下拉

---

## 2. 重构为全宽列表 + Modal

**文件：** `web/src/views/StorageTargetsView.vue`

### 2.1 新增导入

```typescript
import AppDialog from "../components/ui/AppDialog.vue"
import AppModal from "../components/ui/AppModal.vue"
```

### 2.2 新增状态变量

```typescript
const modalOpen = ref(false)
const deleteDialogOpen = ref(false)
const deleteTargetId = ref<number | null>(null)
const deleteTargetName = ref("")
```

### 2.3 新增 Modal 操作方法

```typescript
function openCreateModal(): void {
	resetForm()
	modalOpen.value = true
}

function openEditModal(target: StorageTargetSummary): void {
	form.id = String(target.id)
	form.name = target.name
	form.type = target.type
	form.host = target.host ?? ""
	form.port = String(target.port)
	form.user = target.user ?? ""
	form.sshKeyId = target.ssh_key_id ? String(target.ssh_key_id) : ""
	form.basePath = target.base_path
	formError.value = ""
	modalOpen.value = true
}

function closeModal(): void {
	if (isSubmitting.value) {
		return
	}
	modalOpen.value = false
}

function openDeleteDialog(target: StorageTargetSummary): void {
	deleteTargetId.value = target.id
	deleteTargetName.value = target.name
	deleteDialogOpen.value = true
}

function closeDeleteDialog(): void {
	deleteDialogOpen.value = false
	deleteTargetId.value = null
	deleteTargetName.value = ""
}

async function confirmDelete(): Promise<void> {
	if (deleteTargetId.value === null) {
		return
	}

	errorMessage.value = ""
	successMessage.value = ""

	try {
		await deleteStorageTarget(deleteTargetId.value)
		successMessage.value = `存储目标「${deleteTargetName.value}」已删除。`
		closeDeleteDialog()
		await loadData()
	} catch (error) {
		errorMessage.value = error instanceof ApiError ? error.message : "删除存储目标失败。"
		closeDeleteDialog()
	}
}
```

### 2.4 修改 submitForm

成功后关闭 Modal：

```typescript
async function submitForm(): Promise<void> {
	formError.value = ""
	successMessage.value = ""
	isSubmitting.value = true

	try {
		if (form.id === "") {
			await createStorageTarget(buildPayload())
			successMessage.value = "存储目标已创建。"
		} else {
			await updateStorageTarget(Number.parseInt(form.id, 10), buildPayload())
			successMessage.value = "存储目标已更新。"
		}

		modalOpen.value = false
		resetForm()
		await loadData()
	} catch (error) {
		formError.value = error instanceof ApiError ? error.message : "保存存储目标失败。"
	} finally {
		isSubmitting.value = false
	}
}
```

### 2.5 删除旧的 `editTarget` 和 `handleDelete` 方法

用 `openEditModal` 替换 `editTarget`，用 `openDeleteDialog` + `confirmDelete` 替换 `handleDelete`。

---

## 3. 重写 template

```vue
<template>
	<section class="page-view">
		<header class="page-header">
			<div>
				<h1 class="page-header__title">存储目标</h1>
				<p class="page-header__subtitle">按备份类型管理目标路径，并执行连通性测试。</p>
			</div>
			<AppButton @click="openCreateModal">新建目标</AppButton>
		</header>

		<AppNotification v-if="errorMessage" title="存储目标操作失败" tone="danger" :description="errorMessage" />
		<AppNotification v-if="successMessage" title="存储目标已更新" tone="success" :description="successMessage" />

		<div class="page-stack">
			<AppCard v-for="group in groupedTargets" :key="group.title" :title="group.title" :description="group.description">
				<AppTable
					:rows="group.items"
					:columns="[
						{ key: 'name', label: '名称' },
						{ key: 'type', label: '类型' },
						{ key: 'base_path', label: '基础路径' },
						{ key: 'updated_at', label: '更新时间' },
						{ key: 'actions', label: '操作' },
					]"
					row-key="id"
				>
					<template #cell-type="{ row }">
						<div class="page-stack">
							<AppTag :tone="row.type.endsWith('_ssh') ? 'info' : 'default'">{{ row.type }}</AppTag>
							<span v-if="row.host" class="page-muted">{{ row.user }}@{{ row.host }}:{{ row.port }}</span>
						</div>
					</template>
					<template #cell-updated_at="{ value }">
						<span>{{ formatDateTime(String(value)) }}</span>
					</template>
					<template #cell-actions="{ row }">
						<div class="page-action-row--wrap">
							<AppButton size="sm" variant="secondary" @click="openEditModal(row)">编辑</AppButton>
							<AppButton size="sm" variant="ghost" :loading="testingId === row.id" @click="handleTest(row.id)">测试</AppButton>
							<AppButton size="sm" variant="ghost" @click="openDeleteDialog(row)">删除</AppButton>
						</div>
					</template>
				</AppTable>
				<AppEmpty v-if="group.items.length === 0" title="当前没有目标" description="点击上方「新建目标」按钮添加存储目标。" compact />
			</AppCard>
		</div>

		<!-- 新建/编辑 Modal -->
		<AppModal :open="modalOpen" width="34rem" @close="closeModal">
			<section class="page-modal-form">
				<header class="page-modal-form__header">
					<h2 class="page-modal-form__title">{{ form.id === '' ? '新建存储目标' : '编辑存储目标' }}</h2>
					<p class="page-muted">本地目标只需基础路径，SSH 目标还需要连接信息。</p>
				</header>

				<form class="page-stack" @submit.prevent="submitForm">
					<div class="page-form-grid">
						<AppFormField label="名称" required>
							<AppInput v-model="form.name" placeholder="archive-primary" />
						</AppFormField>

						<AppFormField label="目标类型" required>
							<AppSelect v-model="form.type" :options="typeOptions" />
						</AppFormField>

						<AppFormField label="基础路径" required>
							<AppInput v-model="form.basePath" placeholder="/srv/backup" />
						</AppFormField>
					</div>

					<div v-if="isRemoteTarget" class="page-form-grid">
						<AppFormField label="主机" required>
							<AppInput v-model="form.host" placeholder="192.0.2.20" />
						</AppFormField>
						<AppFormField label="端口">
							<AppInput v-model="form.port" inputmode="numeric" />
						</AppFormField>
						<AppFormField label="用户" required>
							<AppInput v-model="form.user" placeholder="backup" />
						</AppFormField>
						<AppFormField label="SSH 密钥" required>
							<AppSelect v-model="form.sshKeyId" :options="sshKeyOptions" />
						</AppFormField>
					</div>

					<AppNotification
						v-if="form.type === 'rolling_ssh' || form.type === 'cold_ssh'"
						title="远程目标提示"
						tone="warning"
						description="SSH 目标的连通性测试会直接验证主机、用户和密钥组合。"
					/>

					<AppNotification v-if="formError" title="保存失败" tone="danger" :description="formError" />

					<div class="page-action-row--wrap">
						<AppButton type="submit" :loading="isSubmitting">{{ form.id === '' ? '创建目标' : '保存修改' }}</AppButton>
						<AppButton type="button" variant="ghost" @click="closeModal">取消</AppButton>
					</div>
				</form>
			</section>
		</AppModal>

		<!-- 删除确认 Dialog -->
		<AppDialog :open="deleteDialogOpen" title="确认删除存储目标" tone="danger" @close="closeDeleteDialog">
			<p>即将删除存储目标「{{ deleteTargetName }}」。关联的备份记录不会被删除，但该目标将不再可用于新的备份策略。</p>

			<template #actions>
				<AppButton variant="ghost" @click="closeDeleteDialog">取消</AppButton>
				<AppButton variant="danger" @click="confirmDelete">确认删除</AppButton>
			</template>
		</AppDialog>
	</section>
</template>
```

---

## 4. 添加 Modal 表单 CSS

在 `<style scoped>` 中添加与 Phase 2 相同的 Modal 表单样式（如果尚未抽取到全局）：

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

**建议：** 如果 Phase 2 已经完成，此时有两个组件使用相同的 `.page-modal-form` 样式。考虑将其移到全局 `web/src/styles/application.css` 中，并从两个组件的 scoped 样式中移除。

---

## 5. 关于用量条（磁盘使用状况）

设计规格（5.4 节）提到每行显示多色横条图展示磁盘使用状况。但规格同时注明：

> 用量数据依赖 API 返回的存储目标统计。若当前 API 不返回用量信息，此功能标记为 v1.1 增强，v1 版本仍以当前 `available_bytes` + `backup_count` 显示。

当前 `StorageTargetSummary` 类型只有基础字段，没有 `used_bytes` 或 `backup_bytes` 等详细用量字段。`DashboardStorageSummary` 有 `available_bytes` 和 `backup_count`，但这是仪表盘级别的汇总数据。

**本 phase 处理方式：** 不实现多色用量条。如果当前 API 可以返回 `available_bytes`，可以在表格中增加一列简单显示剩余空间。否则保持现有表格列不变。用量条功能标记为 v1.1 增强。

---

## 6. 验证与提交

1. 确认编译通过：

```bash
npm --prefix web run build
```

2. 提交：

```bash
git add -A
git commit -m "refactor(web): Phase 3 — StorageTargetsView full-width + Modal + delete confirm"
```

---

## 7. 启动服务并测试

启动完整服务：

```bash
make run
```

同时启动前端开发服务器：

```bash
npm --prefix web run dev
```

然后使用 `askQuestion` 工具向用户提出以下测试问题：

**问题标题：** Phase 3 存储目标页测试

**测试清单（请用户逐项确认）：**

1. **全宽布局** — 存储目标页的分组表格（滚动备份目标、冷备份目标）是否占据全部内容宽度？是否没有右侧表单区域？
2. **新建 Modal** — 点击「新建目标」按钮是否弹出居中 Modal？Modal 表单是否包含名称、目标类型、基础路径等字段？选择 SSH 类型后是否出现主机/端口/用户/密钥字段？
3. **编辑 Modal** — 点击表格行的「编辑」按钮是否弹出 Modal 且预填当前目标数据？
4. **删除确认** — 点击「删除」按钮是否弹出 danger 风格的确认 Dialog？确认后是否删除目标并刷新列表？取消后是否关闭 Dialog 且不执行删除？
5. **测试连通性** — 点击「测试」按钮是否调用连通性测试 API 并显示成功/失败消息？
6. **Modal 关闭** — ESC 键、遮罩层点击、取消按钮是否都能关闭 Modal？
7. **创建/编辑流程** — 提交表单后 Modal 是否自动关闭且列表自动刷新？
8. **空态提示** — 当某个分组没有目标时，是否显示引导性空态文案？
9. **SSH 远程目标提示** — 创建/编辑 SSH 目标时，Modal 中是否显示黄色警告提示？
