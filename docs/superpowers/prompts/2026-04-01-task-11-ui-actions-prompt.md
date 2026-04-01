# Task 11 Prompt — 输入、操作与危险交互组件

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 11「输入、操作与危险交互组件」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 11
- docs/superpowers/specs/2026-04-01-rsync-component-style-design.md 中的操作型实体组件、危险态和可访问性规则

前序任务简报
- Task 10 已建立前端应用壳层、主题 token 和基础状态管理。
- 当前前端已有布局与样式变量，但还没有可复用的按钮、输入、对话框等交互组件。

当前任务目标
- 实现按钮、输入、文本域、选择器、开关、模态框、对话框、表单字段、密码输入、Tabs 和 Breadcrumb。
- 落实焦点环、危险态、禁用态、键盘可访问性和表单错误展示。
- 编写交互组件测试，并让它们通过。

当前任务实现指导
- 本任务只实现操作型与危险交互组件，不实现表格、进度、通知等数据展示组件。
- 危险态必须使用独立 error 体系，不允许借用品牌主色弱化风险语义。
- 输入类组件要优先保证键盘焦点清晰、错误文案明确和结构一致。
- 对话框要有基本 focus trap 和首个可聚焦元素处理。
- 组件实现要依赖 Task 10 的 token 系统，不要在组件里写死大量颜色值。

关键方法/接口定义
```ts
type ButtonVariant = "primary" | "secondary" | "ghost" | "danger"

interface AppButtonProps {
  variant?: ButtonVariant
  size?: "sm" | "md" | "lg"
  loading?: boolean
  disabled?: boolean
}
```

```ts
interface AppDialogProps {
  open: boolean
  title: string
  tone?: "default" | "danger"
}

interface AppFormFieldProps {
  label: string
  error?: string
  required?: boolean
}
```

```ts
interface AppInputProps {
  modelValue: string
  disabled?: boolean
  invalid?: boolean
}
```

单元测试要求
- 先补按钮、输入、对话框和焦点管理相关组件测试，再实现交互组件。
- 至少覆盖：danger variant、focus trap、错误态渲染、禁用态和基础键盘可达性。
- 如果需要用户协助做一次真实键盘导航或焦点可见性检查，可使用 `askQuestion` 请求帮助，并明确步骤和预期行为。

验收目标
- `npm --prefix web run test -- src/components/ui/AppButton.spec.ts src/components/ui/AppDialog.spec.ts` 通过。
- 关键交互组件可复用、焦点清晰、危险态语义明确。
- 本任务不提前实现表格、卡片、进度、通知或业务页面。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：组件 API 是否稳定、危险态是否误用品牌色、焦点管理是否可靠、表单错误反馈是否清楚。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 11 验收回归 prompt”。
- 这段回归 prompt 至少包含：`npm --prefix web run test -- src/components/ui/AppButton.spec.ts src/components/ui/AppDialog.spec.ts` 和 `npm --prefix web run build`。
- 如果需要用户协助做一次键盘无障碍验证，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先说明本任务中“输入组件”“操作组件”“危险交互组件”的划分。
- 完成后总结组件清单、无障碍处理、样式约束和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到组件 API 选择不确定，需要确认时请暂停并提问。
