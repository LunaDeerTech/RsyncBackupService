# 07-02 前端恢复交互

## 前序任务简报

恢复引擎与 API 已完成：滚动/冷备份恢复执行器可用，恢复 API 支持二次密码验证和实例名称确认，冷备份下载使用一次性临时令牌。恢复任务通过任务队列调度。

## 当前任务目标

在实例详情页的备份 Tab 中实现恢复操作交互和冷备份下载功能。

## 实现指导

### 1. API 模块

```typescript
// api/backups.ts（补充）
function restoreBackup(instanceId: number, backupId: number, data: RestoreRequest): Promise<void>
function downloadBackup(instanceId: number, backupId: number): Promise<{ download_url: string }>

interface RestoreRequest {
  restore_type: 'source' | 'custom'
  target_path?: string
  instance_name: string
  password: string
  encryption_key?: string
}
```

### 2. 备份列表操作列

在实例详情页的备份 Tab 表格操作列中：
- **恢复按钮**（admin 可见）：仅 `status=success` 的备份可点击
- **下载按钮**（admin 或拥有下载权限的 viewer）：仅冷备份且 `status=success` 时可点击

### 3. 恢复确认 Modal

点击恢复按钮后打开 AppModal，表单内容：

1. **恢复类型选择**：
   - 单选组：「恢复到原始位置」/「恢复到指定位置」
   - 选择「恢复到原始位置」时，显示警告提示：「将覆盖源路径的现有数据」
2. **目标路径输入**（仅「恢复到指定位置」时显示）
3. **加密密钥输入**（仅加密冷备份时显示，必填）
4. **确认区域**（danger 样式区域）：
   - 提示：「请输入实例名称 `xxx` 和您的账号密码以确认恢复操作」
   - 实例名称输入框
   - 当前密码输入框
5. **提交按钮**：AppButton variant=danger，文本「确认恢复」
   - 仅当实例名称匹配时启用按钮

### 4. 恢复提交后

- 提交成功：Toast 提示「恢复任务已创建」→ 关闭 Modal → 切换到概览 Tab
- 概览 Tab 显示当前运行的恢复任务（进度展示在阶段十三完善）
- 提交失败：Modal 内显示错误信息（密码错误 / 密钥不匹配等）

### 5. 冷备份下载

- 点击下载按钮 → 调用 `downloadBackup` 获取临时下载 URL → 触发浏览器下载（`window.open(url)` 或创建隐藏 `<a>` 标签点击）
- 下载中按钮显示 loading 状态

### 6. 备份详情展开

- 点击备份行可展开显示详细信息：
  - 快照路径
  - rsync 统计（文件数、传输大小、速率等）
  - 开始/完成时间
  - 失败原因（如有）

## 验收目标

1. 成功备份行显示恢复和下载按钮
2. 点击恢复打开确认 Modal，需填写正确的实例名称和密码
3. 实例名称不匹配时提交按钮禁用
4. 恢复到指定位置时目标路径必填
5. 加密冷备份恢复时密钥必填
6. 提交后 Toast 提示成功并跳转概览 Tab
7. 冷备份下载按钮点击后触发文件下载
8. 深色/浅色主题下 Modal 样式正确
