# 03-02 基础 UI 组件库

## 前序任务简报

`AppLayout` 布局组件已完成：桌面端左侧固定导航 + 右侧内容区，移动端抽屉式导航。全局 CSS token 体系已定义（品牌色、语义色、表面色、文本色、边框、圆角、间距），浅色/深色主题切换可用。图标库已集成。

## 当前任务目标

按照 `docs/component-style-design.md` 风格规范，实现项目所需的通用 UI 组件库。

## 实现指导

所有组件应使用 CSS 变量（token）而非硬编码颜色，确保主题切换兼容。组件放在 `web/src/components/` 目录下。

### 1. 操作型组件

**AppButton**：
- Props：`variant`（`primary` | `outline` | `danger` | `ghost`）、`size`（`sm` | `md` | `lg`）、`disabled`、`loading`
- Loading 状态显示 spinner 并禁用点击
- 使用品牌色 `--primary-500/600` 作为 primary 样式

**AppInput**：
- Props：`type`（`text` | `password` | `number` | `email`）、`modelValue`、`placeholder`、`disabled`、`error`（错误提示文本）
- 支持 `v-model`
- 聚焦时显示 `--border-focus` 边框
- 有 error 时边框变为 `--error-500`，下方显示错误文本

**AppSelect**：
- Props：`modelValue`、`options`（`{ label, value }[]`）、`placeholder`、`disabled`
- 支持 `v-model`
- 原生 `<select>` 即可，样式与 AppInput 统一

**AppSwitch**：
- Props：`modelValue`（boolean）、`disabled`
- 支持 `v-model`
- 开启状态使用主品牌色

**AppCheckbox**：
- Props：`modelValue`（boolean）、`label`、`disabled`
- 支持 `v-model`

### 2. 数据展示组件

**AppTable**：
- Props：`columns`（`{ key, title, width?, sortable?, render? }[]`）、`data`（行数据数组）、`loading`
- 支持列排序（点击表头切换 asc/desc）
- 加载中显示骨架屏或 spinner
- 空数据显示 EmptyState
- Slot：`#cell-[key]` 自定义单元格渲染

**AppBadge / AppTag**：
- Props：`variant`（`success` | `warning` | `error` | `info` | `default`）
- 用于状态标签展示：备份状态、健康状态、角色标签等
- 带语义色背景 + 文字

**AppProgress**：
- Props：`value`（0-100）、`variant`（`primary` | `success` | `warning` | `error`）、`size`（`sm` | `md`）
- 水平进度条，带颜色渲变

### 3. 反馈组件

**AppModal**：
- Props：`visible`（v-model）、`title`、`width`（可选）
- Slots：默认内容、`#footer`（操作按钮区）
- 遮罩层点击关闭（可配置）
- ESC 键关闭
- 打开/关闭动画

**AppToast**：
- 全局 Toast 通知系统
- `useToastStore` composable：`toast.success(msg)`、`toast.error(msg)`、`toast.warning(msg)`、`toast.info(msg)`
- 右上角弹出，自动消失（3-5 秒）
- 支持同时显示多条，从上到下堆叠

**AppConfirm**：
- 基于 AppModal 的确认对话框
- `useConfirm` composable：`confirm({ title, message, confirmText, danger? }) → Promise<boolean>`
- danger 模式：确认按钮使用 error 色

### 4. 布局组件

**AppCard**：
- Props：`title`（可选）、`padding`（可选）
- 圆角（`--radius-lg`）+ 边框 + 背景色（`--surface-raised`）
- Slot：默认内容、`#header`

**AppTabs**：
- Props：`tabs`（`{ key, label }[]`）、`activeKey`（v-model）
- Tab 头部样式：横向排列，当前 tab 底部高亮线（使用品牌色）
- Slot：`#tab-[key]` 各 tab 内容

**AppDrawer**：
- Props：`visible`（v-model）、`title`、`side`（`left` | `right`）、`width`
- 侧边滑出面板 + 遮罩层

**AppEmpty**：
- Props：`message`（默认"暂无数据"）、`icon`（可选）
- 空状态占位组件

### 5. 表单组件

**AppFormItem**：
- Props：`label`、`required`（显示 * 号）、`error`（错误提示）
- 布局：label 在上，输入组件在下（slot），错误提示在最下
- 用于包裹 AppInput / AppSelect 等

**AppFormGroup**：
- 表单分组容器，提供统一的间距

### 6. 分页组件

**AppPagination**：
- Props：`page`、`pageSize`、`total`
- Events：`update:page`、`update:pageSize`
- 显示：上一页/下一页、页码、总条数、每页条数选择

## 验收目标

1. 所有组件在浅色/深色主题下样式正确
2. Button 的 4 种 variant 视觉可区分，loading 状态有 spinner
3. Input 聚焦边框变色，error 时红色边框 + 提示文字
4. Table 可传入 columns 和 data 渲染，空数据显示 EmptyState
5. Modal 可打开/关闭，支持遮罩点击和 ESC 关闭
6. Toast 可从任意组件调用，右上角弹出并自动消失
7. Confirm 返回 Promise，用户确认 resolve true，取消 resolve false
8. Tabs 切换平滑，当前 tab 有高亮指示
9. Pagination 翻页和切换 pageSize 事件正确触发
