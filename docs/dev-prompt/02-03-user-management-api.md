# 02-03 用户管理接口

## 前序任务简报

认证系统已完成：注册/登录/刷新令牌接口可用，JWT 认证中间件、角色检查中间件（`RequireAuth`、`RequireAdmin`）和实例权限中间件已就位。admin 和 viewer 角色隔离已生效。

## 当前任务目标

实现管理员用户管理 API（CRUD）和个人信息/密码修改 API。

## 实现指导

### 1. 管理员接口（admin 权限）

**GET `/api/v1/users`**：
- 返回所有用户列表（不含 `password_hash`）
- 支持分页

**POST `/api/v1/users`**：
- 请求体：`{ "email": "...", "name": "...", "role": "admin|viewer" }`
- 逻辑：验证邮箱格式和唯一性 → 生成随机密码 → 创建用户 → 尝试 SMTP 发送密码（未配置时日志输出）
- admin 可创建任意角色的用户

**PUT `/api/v1/users/:id`**：
- 请求体：`{ "name": "...", "role": "..." }`
- 约束：不能修改自己的角色（防止唯一 admin 降级）
- 不能通过此接口修改密码

**DELETE `/api/v1/users/:id`**：
- 约束：不能删除自己、不能删除最后一个 admin
- 删除用户时同步清理其 `instance_permissions` 和 `notification_subscriptions`

### 2. 个人信息接口（已认证用户）

**GET `/api/v1/users/me`**：
- 返回当前登录用户的信息（id, email, name, role, created_at）

**PUT `/api/v1/users/me/password`**：
- 请求体：`{ "old_password": "...", "new_password": "..." }`
- 校验旧密码正确性 → 哈希新密码 → 更新

**PUT `/api/v1/users/me/profile`**：
- 请求体：`{ "name": "..." }`
- 仅允许修改显示名称

### 3. 输入校验

- 邮箱格式校验（正则或 `net/mail.ParseAddress`）
- 密码长度校验：新密码至少 8 位
- 角色值校验：仅允许 `admin` / `viewer`

## 验收目标

1. admin 可查看、创建、编辑、删除用户
2. 创建用户后日志输出随机密码（SMTP 未配置时）
3. admin 无法删除自己
4. admin 无法将自己降级为 viewer（如果是唯一 admin）
5. viewer 访问 `/api/v1/users` 返回 403
6. 任何已认证用户可获取自己信息（`/users/me`）
7. 修改密码时旧密码错误返回业务错误
8. 为用户管理接口编写关键路径的单元测试
