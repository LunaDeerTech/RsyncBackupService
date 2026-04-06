# 12-01 用户管理与注册开关页面

## 前序任务简报

系统所有后端功能已完成（认证/实例/策略/备份引擎/恢复/审计/容灾率/风险/通知/仪表盘）。前端主要业务页面已实现（登录/实例列表/实例详情/远程配置/备份目标/仪表盘/SMTP 配置）。后端用户管理 API（`GET/POST/PUT/DELETE /api/v1/users`）和注册开关 API 在阶段二和十已实现。

## 当前任务目标

实现用户管理页面和注册开关前端界面。

## 实现指导

### 1. 路由与页面

- 路由：`/system/users`，admin 权限
- 页面组件：`pages/system/UserManagementPage.vue`

### 2. API 模块

```typescript
// api/users.ts（补充）
function listUsers(params?: PaginationParams): Promise<PaginatedData<User>>
function createUser(data: { email: string; name?: string; role: string }): Promise<User>
function updateUser(id: number, data: { name?: string; role?: string }): Promise<User>
function deleteUser(id: number): Promise<void>
```

### 3. 用户列表

- AppTable 展示：邮箱、名称、角色（AppBadge：admin=primary, viewer=default）、创建时间、操作
- 操作列：编辑、删除
- 页面顶部：标题「用户管理」+ 新增用户按钮 + 注册开关
- 支持分页

### 4. 新增用户 Modal

- 表单字段：
  - 邮箱（必填，email 格式验证）
  - 名称（可选）
  - 角色（AppSelect：admin / viewer）
- 提交后后端生成随机密码并通过 SMTP 发送
- Toast 提示「用户已创建，密码已发送至邮箱」
- SMTP 未配置时提示「密码已输出到服务器日志」

### 5. 编辑用户 Modal

- 可编辑字段：名称、角色
- 不能修改邮箱和密码（密码通过个人中心修改）
- 不能修改自己的角色

### 6. 删除用户

- AppConfirm 确认弹窗
- admin 不可删除自己
- 最后一个 admin 不可删除
- 提示将同步清理该用户所有权限和订阅配置

### 7. 注册开关

- 在用户列表页面顶部或旁边放置 AppSwitch
- 标签：「允许新用户注册」
- 切换时调用 `PUT /api/v1/system/registration` 并 Toast 提示

## 验收目标

1. 用户列表正确展示所有用户
2. 可创建新用户（admin/viewer 角色）
3. 创建后 Toast 提示密码发送方式
4. 可编辑用户名称和角色
5. 不可删除自己或最后一个 admin
6. 注册开关可正常切换，影响注册页面行为
7. 深色/浅色主题下样式正确
