# Task 12 Prompt — 表格、状态、反馈与高密度信息组件

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 12「表格、状态、反馈与高密度信息组件」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 12
- docs/superpowers/specs/2026-04-01-rsync-component-style-design.md 中的表格与日志组件、状态与概览组件、动效系统规则

前序任务简报
- Task 10 已提供前端基础设施和 token 系统。
- Task 11 已提供按钮、输入、对话框等交互型基础组件。
- 当前缺少高密度数据展示和状态反馈组件，无法支撑实例列表、历史记录、进度与通知场景。

当前任务目标
- 实现表格、卡片、标签、徽标、进度条、Toast 容器、通知卡、时间线、空状态和 Spinner。
- 落实高密度列表、运行态反馈、状态语义和视觉纪律。
- 编写相关组件测试，并让它们通过。

当前任务实现指导
- 本任务只实现数据展示与反馈组件，不进入业务页面逻辑。
- 表格区域必须保持高扫描效率，不使用毛玻璃和大面积渐变。
- 进度组件除了视觉条，还要提供百分比、速率或剩余时间等文本信息。
- 状态标签/通知要确保成功、警告、失败、信息和品牌主色语义不混淆。
- 动效只能服务于状态感知，避免持续炫目效果。

关键方法/接口定义
```ts
interface TableColumn<T> {
  key: keyof T | string
  label: string
}

interface AppTableProps<T> {
  rows: T[]
  columns: TableColumn<T>[]
  dense?: boolean
}
```

```ts
interface AppProgressProps {
  percentage: number
  speedText?: string
  etaText?: string
  tone?: "default" | "running" | "success" | "danger"
}
```

```ts
interface AppNotificationProps {
  title: string
  tone?: "info" | "success" | "warning" | "danger"
}
```

单元测试要求
- 先补表格、进度、通知等展示组件测试，再实现数据展示与状态反馈组件。
- 至少覆盖：高密度表格基础渲染、进度条文本元信息、通知语义状态、危险态颜色隔离。
- 如果需要用户协助做一次视觉密度或状态语义检查，可使用 `askQuestion` 请求帮助，并明确页面、组件和预期观察点。

验收目标
- `npm --prefix web run test -- src/components/ui/AppTable.spec.ts src/components/ui/AppProgress.spec.ts src/components/ui/AppNotification.spec.ts` 通过。
- 高密度表格、状态标签和运行态反馈组件满足样式文档约束。
- 本任务不提前实现具体页面或 API 调用。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：表格可读性、品牌色与语义色是否混淆、动效是否克制、组件职责是否清晰。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 12 验收回归 prompt”。
- 这段回归 prompt 至少包含：`npm --prefix web run test -- src/components/ui/AppTable.spec.ts src/components/ui/AppProgress.spec.ts src/components/ui/AppNotification.spec.ts` 和 `npm --prefix web run build`。
- 如果需要用户协助做一次视觉检查，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先说明表格/状态/反馈三类组件的职责划分。
- 完成后总结组件能力、视觉约束和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到组件 API 或状态语义需要确认，请暂停并提问。
