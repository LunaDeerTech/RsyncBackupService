# Task 13 Prompt — 页面实现、实时任务界面与恢复确认流程

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 13「页面实现、实时任务界面与恢复确认流程」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 13
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的页面路由、核心页面与恢复流程部分
- docs/superpowers/specs/2026-04-01-rsync-component-style-design.md 中的页面应用指导与危险操作页面规则

前序任务简报
- Task 01 到 Task 09 已完成后端主能力、API、WebSocket 和系统接口。
- Task 10 到 Task 12 已完成前端基础设施、主题系统和核心 UI 组件。
- 当前缺少真正可用的登录页、实例列表/详情页、资源管理页、通知页、审计页和设置页，以及对实时任务和恢复确认流程的前端承接。

当前任务目标
- 实现 API 模块、登录页、仪表盘、实例列表页、实例详情页及其子 Tab、资源管理页、通知页、审计页、设置页。
- 实现实时任务订阅 composable，并接入运行中任务、进度、ETA 与中继模式提示。
- 实现恢复风险确认、密码二次认证输入和用户管理/实例权限设置页。
- 编写页面级测试，并让它们通过。

当前任务实现指导
- 本任务聚焦页面与工作流，不要重写前端基础设施或组件库。
- 页面层尽量薄，API 调用集中到 `web/src/api/*`，实时逻辑集中到 `useRealtimeTasks`。
- 实例详情页按设计文档使用 Tab 分区：概览、策略、备份历史、恢复、通知订阅。
- 恢复流程必须突出风险文案、确认路径、覆盖语义和二次认证，不得弱化危险态。
- 如果策略/存储组合触发 remote-to-remote relay，要在页面中显示中继模式提示。
- 设置页以用户管理、密码修改和实例权限管理为核心，不扩展额外系统管理功能。

关键方法/接口定义
```ts
export async function login(payload: { username: string; password: string }): Promise<LoginResponse>
export async function listInstances(): Promise<InstanceSummary[]>
export async function getInstanceDetail(id: number): Promise<InstanceDetail>
export async function startRestore(payload: RestorePayload): Promise<RestoreRecord>
```

```ts
export function useRealtimeTasks(): {
  tasks: Ref<RunningTaskViewModel[]>
  connect(): void
  disconnect(): void
}
```

```ts
export async function listUsers(): Promise<UserSummary[]>
export async function updateInstancePermissions(instanceId: number, payload: PermissionPayload[]): Promise<void>
```

单元测试要求
- 先补登录页、实例列表页、恢复页和关键工作流测试，再实现页面层逻辑。
- 至少覆盖：登录提交、实例列表渲染、恢复前二次认证弹窗、基础实时任务订阅行为。
- 如果需要用户协助做一次真实页面流程或实时任务界面确认，可使用 `askQuestion` 请求帮助，并明确页面路径、交互步骤和预期结果。

验收目标
- `npm --prefix web run test -- src/views/LoginView.spec.ts src/views/InstancesListView.spec.ts src/views/instance/RestoreTab.spec.ts` 通过。
- `npm --prefix web run build` 通过。
- 登录、实例页、恢复确认、实时任务和设置页工作流可跑通。
- 本任务不提前实现静态资源嵌入、Docker 打包或 Playwright 端到端用例。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：页面职责是否过重、API 调用是否集中、危险操作 UX 是否清晰、实时订阅是否能正确清理。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 13 验收回归 prompt”。
- 这段回归 prompt 至少包含：`npm --prefix web run test -- src/views/LoginView.spec.ts src/views/InstancesListView.spec.ts src/views/instance/RestoreTab.spec.ts` 和 `npm --prefix web run build`。
- 如果需要用户协助做一次真实页面工作流回归，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先按“页面层、API 模块、实时 composable、危险操作流程”四块说明本任务边界。
- 完成后总结新增页面、关键工作流和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到页面职责或交互语义需要确认，请暂停并提问。
