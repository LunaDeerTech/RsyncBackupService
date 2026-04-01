# Task 10 Prompt — 前端基础设施、鉴权状态与 Balanced Flux Token 系统

你是 GitHub Copilot，运行在 VS Code 当前工作区中。请先阅读指定文件并理解已有实现，再直接在工作区内完成当前任务；只处理本任务边界，不要提前实现后续任务；遇到阻塞、歧义、需要确认或测试环境不足时先提问，不要猜测。

请实现 Rsync Backup Service 开发计划中的 Task 10「前端基础设施、鉴权状态与 Balanced Flux Token 系统」。

开始前请先阅读：
- docs/superpowers/plans/2026-04-01-rsync-backup-service-implementation-plan.md 中的 Task 10
- docs/superpowers/specs/2026-04-01-rsync-component-style-design.md 的视觉系统、token 分层、主题模型与可访问性部分
- docs/superpowers/specs/2026-04-01-rsync-backup-service-design.md 中的前端路由与认证要求

前序任务简报
- Task 01 已建立 Vue 3 + Vite 应用壳层和基础样式入口。
- Task 03 与 Task 09 已提供认证接口、系统接口和实时 API 基础。
- 当前还缺少统一 API client、前端鉴权状态管理、主题切换和应用布局壳层。

当前任务目标
- 实现统一 API client，支持 access token 注入与 401 后自动 refresh 重放。
- 实现 auth store、ui/theme store、session/theme composable。
- 落地 Balanced Flux 四层 token 体系和浅深主题 CSS 变量。
- 实现应用主布局、侧栏导航、顶栏和基础路由守卫。
- 编写相关前端测试，并让它们通过。

当前任务实现指导
- 本任务只建立前端基础设施和壳层，不实现具体业务页面。
- 主题系统必须遵循样式设计文档：`Balanced Flux`、`Cyan Mint` 主色、危险态独立颜色体系、浅深主题保持一致材料逻辑。
- API client 要集中处理 token 注入和 refresh，避免每个页面重复实现。
- 路由守卫要区分匿名页和登录后页，但不要在此阶段加入复杂权限 UI。
- 布局先服务后续页面承载，避免在本任务写入大量业务内容。

关键方法/接口定义
```ts
export async function apiFetch<T>(path: string, init?: RequestInit): Promise<T>
export async function refreshSession(): Promise<void>
```

```ts
export function useAuthStore(): {
  accessToken: string | null
  refreshToken: string | null
  setSession(tokens: { accessToken: string; refreshToken: string }): void
  clearSession(): void
}
```

```ts
export function useUiStore(): {
  theme: "light" | "dark"
  setTheme(theme: "light" | "dark"): void
}

export function useTheme(): void
```

单元测试要求
- 先补 auth store、theme store、route guard 和应用壳层测试，再实现前端基础设施。
- 至少覆盖：匿名访问重定向、主题切换、session 持久化或清理、应用布局根渲染。
- 如果需要用户协助做一次真实界面主题或登录壳层确认，可使用 `askQuestion` 请求帮助，并明确页面路径、预期现象和需要确认的点。

验收目标
- `npm --prefix web run test -- src/stores/auth.spec.ts src/layout/AppShell.spec.ts` 通过。
- `npm --prefix web run build` 通过。
- 路由守卫、主题切换和 API client 基础能力可用。
- 本任务不提前实现业务页面、复杂组件或实时任务展示。

代码评审要求
- 在实现和自测完成后，用代码评审视角检查本任务改动，重点关注：token 刷新是否集中处理、主题 token 是否符合设计文档、路由守卫是否清晰、布局是否可被后续页面复用。
- 如果发现问题，先修复并重新执行相关测试，再给出最终结论。

验收回归 prompt 要求
- 在完成本任务后，额外输出一段可直接复制给 Copilot 的“Task 10 验收回归 prompt”。
- 这段回归 prompt 至少包含：`npm --prefix web run test -- src/stores/auth.spec.ts src/layout/AppShell.spec.ts` 和 `npm --prefix web run build`。
- 如果需要用户协助做一次主题或壳层手工验证，也要在回归 prompt 中写出可通过 `askQuestion` 请求的步骤。

执行与反馈要求
- 开始前先说明本任务会交付哪些“基础设施”，哪些内容明确留给后续任务。
- 完成后总结 API client、store、token 系统、布局和验证结果。
- 最后单独给出一段“验收回归 prompt”，不要和修改摘要混在一起。
- 如遇到主题语义或状态管理边界不清，需要确认时请暂停并提问。
