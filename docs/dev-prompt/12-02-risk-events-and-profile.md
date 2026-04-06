# 12-02 风险事件与个人中心页面

## 前序任务简报

用户管理页面和注册开关已完成。后端风险事件查询 API（`GET /api/v1/dashboard/risks`）和个人信息修改 API（`GET/PUT /api/v1/users/me`、`PUT /api/v1/users/me/password`、`PUT /api/v1/users/me/profile`）已在之前阶段实现。通知订阅组件（`NotificationSubscriptions.vue`）已在阶段十开发。

## 当前任务目标

实现风险事件列表页面和个人中心页面（含密码修改、名称修改和通知订阅）。

## 实现指导

### 1. 风险事件页面

**路由**：`/system/risks`，admin 权限
**页面组件**：`pages/system/RiskEventsPage.vue`

**API 模块**：
```typescript
// api/risks.ts
function listRiskEvents(params: RiskEventParams): Promise<PaginatedData<RiskEvent>>

interface RiskEventParams extends PaginationParams {
  severity?: string    // info / warning / critical
  source?: string      // 风险来源类型
  resolved?: boolean
}

interface RiskEvent {
  id: number
  instance_id?: number
  instance_name?: string
  target_id?: number
  target_name?: string
  severity: 'info' | 'warning' | 'critical'
  source: string
  message: string
  resolved: boolean
  created_at: string
  resolved_at?: string
}
```

**页面内容**：
- 筛选栏：
  - 严重等级筛选（AppSelect：全部/info/warning/critical）
  - 来源类型筛选（AppSelect）
  - 状态筛选（AppSelect：全部/未解决/已解决）
- AppTable 展示列：
  - 严重等级（AppBadge：info=info, warning=warning, critical=error）
  - 来源类型（中文映射）
  - 关联实例/目标名称（可点击跳转）
  - 描述
  - 状态（未解决/已解决 + 解决时间）
  - 创建时间
- 默认显示未解决的风险事件
- 支持分页

**风险来源中文映射**：
```typescript
const riskSourceLabels = {
  'backup_failed': '备份失败',
  'backup_overdue': '备份超期',
  'cold_backup_missing': '缺少冷备份',
  'target_unreachable': '目标不可达',
  'target_capacity_low': '目标容量不足',
  'restore_failed': '恢复失败',
  'credential_error': '凭证错误',
}
```

### 2. 个人中心页面

**路由**：`/profile`，已认证用户
**页面组件**：`pages/profile/ProfilePage.vue`

**页面分区**：

**2.1 个人信息区**：
- 显示当前邮箱（只读）、角色（只读）
- 修改名称表单：AppInput + 保存按钮
- 调用 `PUT /api/v1/users/me/profile`

**2.2 修改密码区**：
- 表单字段：旧密码、新密码、确认新密码
- 校验：新密码与确认密码一致、新密码长度 ≥8
- 调用 `PUT /api/v1/users/me/password`
- 成功后 Toast 提示，清空表单

**2.3 通知订阅区**：
- 嵌入 `NotificationSubscriptions.vue` 组件（阶段十已开发）
- 标题：「通知订阅」
- 说明：「开启后，当对应实例发生风险事件时将通过邮件通知您」

### 3. 各区域使用 AppCard 包裹

每个功能区域使用 AppCard 组件，标题清晰分隔。

## 验收目标

1. 风险事件页面可展示所有风险事件
2. 可按严重等级、来源类型和解决状态筛选
3. 点击关联实例名称可跳转到实例详情
4. 个人中心可修改名称并保存
5. 修改密码需旧密码正确、新密码一致
6. 通知订阅区正确显示可访问实例并支持切换开关
7. 深色/浅色主题下样式正确
