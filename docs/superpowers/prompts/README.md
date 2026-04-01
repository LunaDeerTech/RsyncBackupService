# Rsync Backup Service Task Prompts

这些文档用于把开发计划拆成可直接投喂给 GitHub Copilot 的任务级 prompt。

下面每个任务文件的正文本身就是 prompt 本体：

- 直接复制整份文件内容给 Copilot 即可。
- 不需要再额外包一层“请执行以下 prompt”之类的说明。
- 每份文档都已按 Copilot 在 VS Code 工作区内执行任务的方式优化过边界、上下文和验收要求。
- 每份文档都包含单元测试要求、代码评审要求，以及让 Copilot 在完成后输出一段“验收回归 prompt”的要求。

使用建议：

1. 按任务顺序使用，尽量不要跳过前序任务。
2. 将对应任务文档的全文直接复制给 Copilot。
3. 每次只执行一个任务，等该任务验收通过后再进入下一个任务。
4. 如果 AI 提出阻塞、上下文缺失或需要确认，不要让它猜测，先补充信息再继续。

文件列表：

- `2026-04-01-task-01-foundation-prompt.md`
- `2026-04-01-task-02-model-bootstrap-prompt.md`
- `2026-04-01-task-03-auth-permission-prompt.md`
- `2026-04-01-task-04-resource-management-prompt.md`
- `2026-04-01-task-05-scheduler-prompt.md`
- `2026-04-01-task-06-rolling-backup-prompt.md`
- `2026-04-01-task-07-cold-backup-restore-prompt.md`
- `2026-04-01-task-08-notifications-audit-prompt.md`
- `2026-04-01-task-09-realtime-api-prompt.md`
- `2026-04-01-task-10-frontend-foundation-prompt.md`
- `2026-04-01-task-11-ui-actions-prompt.md`
- `2026-04-01-task-12-ui-data-display-prompt.md`
- `2026-04-01-task-13-pages-workflows-prompt.md`
- `2026-04-01-task-14-packaging-e2e-prompt.md`
