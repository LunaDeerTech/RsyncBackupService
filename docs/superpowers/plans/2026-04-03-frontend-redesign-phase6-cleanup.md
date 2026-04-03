# Phase 6: 清理与收尾

> **For agentic workers:** Use superpowers:executing-plans to implement this phase. Each section is a sequential unit of work.

**目标：** 删除废弃的旧文件、提取共用 CSS、统一收尾验证，确保整个前端重构完整可用。

**前置条件：** Phase 1–5 均已完成，所有新页面功能正常。

---

## 1. 删除废弃视图文件

以下 4 个视图文件的功能已被 Phase 5 的新组件替代，不再被路由引用：

```bash
rm web/src/views/SettingsView.vue
rm web/src/views/SSHKeysView.vue
rm web/src/views/NotificationsView.vue
rm web/src/views/AuditLogsView.vue
```

**验证：** 删除后运行 `grep -rn "SettingsView\|SSHKeysView\|NotificationsView\|AuditLogsView" web/src/`，确认无残留引用。如果有，修掉。

---

## 2. 清理路由文件中的旧导入

**文件：** `web/src/router/index.ts`

确认以下路由 **不再存在**（Phase 1 应当已经移除）：

- `/settings`
- `/ssh-keys`
- `/notifications`
- `/audit-logs`

如果还有残留引用或 lazy import，删除它们。

---

## 3. 清理 navigation.ts 中的旧条目

**文件：** `web/src/router/navigation.ts`

确认旧的 7 项扁平导航列表已被 Phase 1 的分组导航替换。如果文件中还保留了旧的 `navigationItems` export，删除之。

---

## 4. 提取共用 CSS

**文件：** `web/src/styles/application.css`

在多个 Phase（2、3、4、5）中反复使用了 `.page-modal-form` 系列类名。如果它们还是仅在各组件的 `<style scoped>` 中定义（Phase 2/3 已写入 scoped），则提取到全局 `application.css`：

```css
/* ─ Modal form layout ─ */
.page-modal-form {
	display: flex;
	flex-direction: column;
	gap: var(--space-5);
	padding: var(--space-5) var(--space-6);
}

.page-modal-form__header {
	display: flex;
	flex-direction: column;
	gap: var(--space-1);
}

.page-modal-form__title {
	margin: 0;
	font-size: var(--text-lg);
	font-weight: 700;
	color: var(--text-strong);
}
```

然后从各组件的 `<style scoped>` 中删除重复的 `.page-modal-form` 定义。涉及的文件：
- `web/src/views/InstancesListView.vue`
- `web/src/views/StorageTargetsView.vue`
- `web/src/views/instance/StrategiesTab.vue`
- `web/src/views/instance/RestoreTab.vue`
- `web/src/views/instance/SubscriptionsTab.vue`
- `web/src/views/system/UsersTab.vue`
- `web/src/views/system/SSHKeysTab.vue`
- `web/src/views/system/NotificationChannelsTab.vue`

**注意：** 去掉 scoped 中的定义后，确认全局定义的优先级仍然正确。如果 scoped 版本有额外属性，需要合并。

---

## 5. 全面编译验证

```bash
npm --prefix web run build
```

修复所有 TypeScript / Vue 编译错误。

---

## 6. 运行测试

```bash
npm --prefix web run test
```

修复所有失败的测试。如果旧文件的测试引用了已删除的组件，更新或删除相应测试。

---

## 7. 提交

```bash
git add -A
git commit -m "refactor(web): Phase 6 — cleanup old views, extract shared CSS, final verification"
```

---

## 8. 启动服务并测试

启动完整服务：

```bash
make run
```

同时启动前端开发服务器：

```bash
npm --prefix web run dev
```

然后使用 `askQuestion` 工具向用户提出以下测试问题：

**问题标题：** Phase 6 清理与收尾最终验证

**测试清单（请用户逐项确认）：**

1. **完整导航** — 侧边栏显示分组导航：「仪表盘」+「备份实例」（核心） +「存储目标」（存储），是否正确？管理员是否额外看到「系统管理」（管理）？底部是否有主题切换和用户区域？
2. **所有页面可达** — 仪表盘、备份实例列表、实例详情（5 Tab）、存储目标、系统管理（4 Tab）、个人信息 — 所有页面是否均可无报错加载？
3. **Modal 全局一致** — 所有创建/编辑表单是否均通过 Modal 呈现？关闭行为是否一致（ESC / 点击遮罩 / 取消按钮）？
4. **Dialog 全局一致** — 所有删除确认是否通过 danger Dialog？是否显示红色调？
5. **无残留页面** — 访问 `/settings`、`/ssh-keys`、`/notifications`、`/audit-logs` 是否被重定向到首页或 404？
6. **移动端响应** — 窄屏下页面布局是否合理？Modal 是否接近全宽？
7. **暗色主题** — 切换暗色主题后所有页面颜色是否正常？是否有白底色残留？
8. **普通用户权限** — 普通用户登录后只能看到自己有权限的实例，无法访问系统管理页面。
9. **控制台错误** — 浏览器开发者工具控制台是否有 Vue 警告或 JS 错误？
10. **整体满意度** — 新布局是否符合最初的设计预期？有无需要调整的细节？
