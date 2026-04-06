# 08-02 前端审计日志

## 前序任务简报

审计日志后端已完成：`internal/audit` 模块在所有关键业务操作（实例/策略/备份/恢复/用户/目标/远程/系统配置）中自动记录审计日志。查询 API 支持全局和实例级查询，支持时间范围和操作类型筛选，支持分页。

## 当前任务目标

在实例详情页的审计 Tab 接入审计日志数据，实现筛选和展示。

## 实现指导

### 1. API 模块

```typescript
// api/audit.ts
function listAuditLogs(params: AuditLogParams): Promise<PaginatedData<AuditLog>>
function listInstanceAuditLogs(instanceId: number, params: AuditLogParams): Promise<PaginatedData<AuditLog>>

interface AuditLogParams extends PaginationParams {
  start_date?: string  // ISO 日期
  end_date?: string
  action?: string      // 逗号分隔多个
}

interface AuditLog {
  id: number
  instance_id?: number
  user_id?: number
  user_name: string
  user_email: string
  action: string
  detail: Record<string, any>
  created_at: string
}
```

### 2. 实例详情审计 Tab

替换之前的占位空表格，接入实际数据：

- AppTable 列：时间、操作类型、操作人（名称/邮箱）、详情
- **操作类型展示**：将 action 枚举映射为中文标签
  - `instance.create` → 「创建实例」
  - `backup.complete` → 「备份完成」
  - `restore.trigger` → 「触发恢复」
  - 等等
- **详情列**：展示 detail JSON 中的关键信息，格式化为人可读文本
- **筛选栏**：
  - 时间范围：开始日期 + 结束日期（日期选择器或简单日期输入）
  - 操作类型：AppSelect 多选（或单选 + 全部选项）
- 支持分页

### 3. Action 中文映射

```typescript
// utils/audit.ts
const actionLabels: Record<string, string> = {
  'instance.create': '创建实例',
  'instance.update': '编辑实例',
  'instance.delete': '删除实例',
  'policy.create': '创建策略',
  'policy.update': '编辑策略',
  'policy.delete': '删除策略',
  'backup.trigger': '触发备份',
  'backup.complete': '备份完成',
  'backup.fail': '备份失败',
  'restore.trigger': '触发恢复',
  'restore.complete': '恢复完成',
  'restore.fail': '恢复失败',
  'user.create': '创建用户',
  'user.update': '编辑用户',
  'user.delete': '删除用户',
  'target.create': '创建目标',
  'target.update': '编辑目标',
  'target.delete': '删除目标',
  'remote.create': '创建远程配置',
  'remote.update': '编辑远程配置',
  'remote.delete': '删除远程配置',
  'system.config.update': '更新系统配置',
}
```

### 4. 详情格式化

```typescript
function formatAuditDetail(action: string, detail: Record<string, any>): string
// 根据 action 类型智能格式化 detail 对象为一行描述文本
// 如 backup.complete → "策略: daily-backup, 类型: 滚动, 耗时: 2分钟"
```

## 验收目标

1. 实例详情审计 Tab 展示该实例的审计日志
2. 操作类型以中文标签展示
3. 时间范围筛选功能可用
4. 操作类型筛选功能可用
5. 分页功能正常
6. 创建/编辑/删除实例后审计 Tab 出现对应记录
7. 备份完成/失败后审计 Tab 出现对应记录
8. 深色/浅色主题下样式正确
