# 05-04 前端实例详情页

## 前序任务简报

实例列表页已完成：admin/viewer 按权限展示实例列表，可创建实例并跳转详情。后端实例详情接口（含统计数据）、策略 CRUD 接口（含类型校验和手动触发）均已就位。

## 当前任务目标

实现实例详情页面（`/instances/:id`），使用 Tab 组织多个功能模块：概览、策略、备份、审计、设置。

## 实现指导

### 1. 路由与页面

- 路由：`/instances/:id`，已认证 + 实例权限
- 页面组件：`pages/instances/InstanceDetailPage.vue`
- 使用 AppTabs 切换 5 个 Tab

### 2. API 模块

```typescript
// api/instances.ts（补充）
function getInstance(id: number): Promise<InstanceDetail>
function getInstanceStats(id: number): Promise<InstanceStats>
function updateInstance(id: number, data: UpdateInstanceRequest): Promise<Instance>
function updateInstancePermissions(id: number, permissions: PermissionItem[]): Promise<void>

// api/policies.ts
function listPolicies(instanceId: number): Promise<Policy[]>
function createPolicy(instanceId: number, data: CreatePolicyRequest): Promise<Policy>
function updatePolicy(instanceId: number, policyId: number, data: UpdatePolicyRequest): Promise<Policy>
function deletePolicy(instanceId: number, policyId: number): Promise<void>
function triggerPolicy(instanceId: number, policyId: number): Promise<void>

// api/backups.ts
function listBackups(instanceId: number, params?: PaginationParams): Promise<PaginatedData<Backup>>
function getBackup(instanceId: number, backupId: number): Promise<BackupDetail>
```

### 3. 概览 Tab

- 顶部信息卡：实例名称、数据源类型+路径、状态
- 统计卡片展示（使用 AppCard）：
  - 备份总数
  - 成功率（百分比 + 颜色指示）
  - 总备份大小（人可读格式）
  - 容灾率占位（显示 "--"，后续阶段九接入数据）
- 当前运行任务：有运行中任务时显示进度条和状态（后续阶段十三接入实时进度）
- 最近 5 条备份记录：mini 表格（时间/类型/状态/大小）
- 注意：此 Tab 负责概览展示，详细数据由各子 Tab 提供

### 4. 策略 Tab

- 策略列表表格：
  - 名称、类型（滚动/冷）、目标名称、调度（cron 表达式或间隔描述）、启用状态（AppSwitch）、上次执行时间/状态、操作
- 操作列：编辑、手动触发、删除
- 新增策略按钮（admin）
- 新增/编辑 Modal 表单：
  - 策略名称（必填）
  - 类型（AppSelect：滚动/冷）
  - 目标（AppSelect：从目标列表过滤匹配类型的项）
  - 调度类型（AppSelect：间隔/Cron）
  - 调度值（间隔时支持人可读输入如"每 6 小时"转为秒数；Cron 时直接输入表达式）
  - 保留策略类型（AppSelect：按时间/按数量）
  - 保留值（按时间：天数；按数量：条数）
  - 冷备份选项（type=cold 时展开）：压缩开关、加密开关、加密密钥输入（加密开启时）、分卷开关、分卷大小（分卷开启时）
- 手动触发按钮：点击 → AppConfirm 确认 → Toast 提示任务已创建

### 5. 备份 Tab

- 备份列表表格：
  - 完成时间、类型（滚动/冷）、状态（AppBadge）、备份大小、数据原始大小、持续时间、操作
- 操作列：
  - 恢复按钮（后续阶段七实现逻辑，此处先渲染按钮但点击提示「功能开发中」）
  - 下载按钮（仅冷备份，后续实现）
- 支持分页

### 6. 审计 Tab

- 审计日志列表（后续阶段八接入数据）
- 此处先渲染空表格框架：时间、操作类型、操作人、详情
- 显示 AppEmpty（「暂无审计日志」）

### 7. 设置 Tab（admin 可见）

- 基础信息编辑表单：实例名称、数据源类型、路径、远程配置
- 访问权限配置：
  - 显示所有 viewer 用户列表（从 `/api/v1/users` 获取）
  - 每个用户一行：名称/邮箱 + 权限选择（无权限/只读/只读+下载）
  - 保存按钮批量更新权限

### 8. 调度值人可读转换

```typescript
// utils/schedule.ts
function formatScheduleValue(type: string, value: string): string
// interval: "3600" → "每 1 小时"，"86400" → "每 1 天"
// cron: 直接显示表达式

function parseIntervalInput(input: string): number // 秒数
// "6小时" → 21600, "30分钟" → 1800, "1天" → 86400
```

## 验收目标

1. 进入实例详情页显示 5 个 Tab：概览/策略/备份/审计/设置
2. 概览 Tab 正确展示统计卡片和最近备份
3. 策略 Tab 可完整 CRUD 策略，类型与目标联动过滤正确
4. 冷备份策略的压缩/加密/分卷选项仅在 type=cold 时显示
5. 手动触发策略后 Toast 提示任务已创建
6. 备份列表可分页展示
7. 设置 Tab 可编辑实例信息和配置权限
8. viewer 登录时设置 Tab 不可见
9. 深色/浅色主题下各 Tab 样式正确
