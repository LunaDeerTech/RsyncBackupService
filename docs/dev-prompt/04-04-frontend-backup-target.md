# 04-04 前端备份目标页面

## 前序任务简报

备份目标后端已完成：`GET/POST/PUT/DELETE /api/v1/targets` 和 `POST /api/v1/targets/:id/health-check` 接口可用，健康检查引擎支持本地/SSH 类型，后台每 30 分钟自动检查。前端远程配置管理页面已实现。

## 当前任务目标

实现备份目标管理页面（`/targets`），包含列表展示（含容量和健康状态）、新增/编辑弹窗和手动健康检查。

## 实现指导

### 1. 路由与页面

- 路由：`/targets`，admin 权限
- 页面组件：`pages/targets/BackupTargetPage.vue`

### 2. API 模块

```typescript
// api/targets.ts
function listTargets(params?: PaginationParams): Promise<PaginatedData<BackupTarget>>
function createTarget(data: CreateTargetRequest): Promise<BackupTarget>
function updateTarget(id: number, data: UpdateTargetRequest): Promise<BackupTarget>
function deleteTarget(id: number): Promise<void>
function healthCheck(id: number): Promise<{ health_status: string; health_message: string }>
```

### 3. 列表页

- 使用 AppTable 展示列：
  - 名称
  - 备份类型（AppBadge：滚动/冷）
  - 存储类型（local/ssh/cloud）
  - 存储路径
  - 容量使用：AppProgress 显示已用/总容量百分比，下方文本显示 "X GB / Y GB"
  - 健康状态：AppBadge（healthy=success, degraded=warning, unreachable=error）
  - 上次检查时间
  - 操作列：编辑、健康检查、删除

### 4. 新增/编辑 Modal

- 表单字段：
  - 名称（必填）
  - 备份类型（AppSelect：滚动/冷，**创建后不可修改**）
  - 存储类型（AppSelect：local/ssh/cloud，选项根据备份类型联动过滤——滚动仅 local/ssh，冷支持全部，cloud 暂禁用；**创建后不可修改**）
  - 存储路径（必填）
  - 关联远程配置（AppSelect，storage_type=ssh 时显示，从远程配置列表动态加载）

### 5. 健康检查

- 点击健康检查按钮 → loading 状态 → 调用 healthCheck 接口 → 刷新该行数据 → Toast 显示结果

### 6. 容量展示

- 容量为 null 时显示「未检测」
- 容量数值采用人可读格式：`formatBytes(bytes)` → "1.2 GB"、"500 MB" 等
- 容量使用率 >80% 时进度条变 warning，>95% 变 error

### 7. TypeScript 类型

```typescript
interface BackupTarget {
  id: number
  name: string
  backup_type: 'rolling' | 'cold'
  storage_type: 'local' | 'ssh' | 'cloud'
  storage_path: string
  remote_config_id?: number
  total_capacity_bytes?: number
  used_capacity_bytes?: number
  last_health_check?: string
  health_status: 'healthy' | 'degraded' | 'unreachable'
  health_message: string
  created_at: string
  updated_at: string
}
```

## 验收目标

1. admin 可从侧边导航进入备份目标页面
2. 可新增本地/SSH 类型备份目标，备份类型与存储类型联动过滤正确
3. 列表正确展示容量进度条和健康状态标签
4. 手动健康检查按钮点击后更新该行的状态和容量
5. 删除目标有确认弹窗，被策略引用时显示错误
6. 容量数值以人可读格式展示
7. 深色/浅色主题切换样式正确
