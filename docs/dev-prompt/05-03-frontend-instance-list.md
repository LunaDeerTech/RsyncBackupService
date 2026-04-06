# 05-03 前端实例列表页

## 前序任务简报

实例后端 CRUD 接口已完成：admin 可见全量实例、viewer 可见授权实例，实例详情含统计数据，权限配置 API 就绪。策略后端已完成：CRUD + 手动触发、类型兼容校验、cron 校验、加密密钥哈希存储。前端 AppLayout、基础 UI 组件库、远程配置页面和备份目标页面均已就位。

## 当前任务目标

实现实例列表页面（`/instances`），包含列表展示和创建实例弹窗。

## 实现指导

### 1. 路由与页面

- 路由：`/instances`，已认证用户均可访问
- 页面组件：`pages/instances/InstanceListPage.vue`

### 2. API 模块

```typescript
// api/instances.ts
function listInstances(params?: PaginationParams): Promise<PaginatedData<InstanceListItem>>
function createInstance(data: CreateInstanceRequest): Promise<Instance>
function deleteInstance(id: number): Promise<void>
```

### 3. 列表页

- 使用 AppTable 展示列：
  - 名称（可点击跳转实例详情）
  - 数据源：类型 + 路径（如 "SSH: /data/myapp"）
  - 状态：AppBadge（idle=default, running=info/primary）
  - 上次备份结果：AppBadge（success/failed/无记录）
  - 上次备份时间：相对时间展示（如"5 分钟前"、"2 天前"）
  - 操作列：详情按钮、删除按钮（admin 可见）
- 页面顶部：标题「备份实例」+ 新增按钮（admin 可见）
- 支持分页
- 空状态：使用 AppEmpty 展示引导信息

### 4. 创建实例 Modal

- 表单字段：
  - 实例名称（必填）
  - 数据源类型（AppSelect：本地 / SSH）
  - 数据源路径（必填）
  - 关联远程配置（AppSelect，数据源类型=SSH 时显示并必填，选项从 `/api/v1/remotes` 动态加载）
- 提交成功后刷新列表 + Toast 提示

### 5. 删除实例

- 点击删除 → AppConfirm 确认（提示将删除所有关联的策略、备份和任务）
- running 状态下删除按钮禁用

### 6. TypeScript 类型

```typescript
interface Instance {
  id: number
  name: string
  source_type: 'local' | 'ssh'
  source_path: string
  remote_config_id?: number
  status: 'idle' | 'running'
  created_at: string
  updated_at: string
}

interface InstanceListItem extends Instance {
  last_backup_status?: 'success' | 'failed'
  last_backup_time?: string
  backup_count: number
}
```

### 7. 相对时间工具

```typescript
// utils/time.ts
function formatRelativeTime(dateStr: string): string
// "刚刚" / "5 分钟前" / "2 小时前" / "3 天前" / "2025-01-01"（超过 30 天显示完整日期）
```

## 验收目标

1. admin 登录后可见所有实例，viewer 仅可见授权实例
2. 列表展示实例名称、数据源、状态、上次备份信息
3. 可创建本地/SSH 类型实例，SSH 类型时关联远程配置
4. 点击实例名称可跳转到实例详情页（路由到 `/instances/:id`）
5. running 状态实例的删除按钮禁用
6. 空列表显示友好的空状态提示
7. 深色/浅色主题下样式正确
