# 04-03 前端远程配置页面

## 前序任务简报

远程配置后端已完成：`GET/POST/PUT/DELETE /api/v1/remotes` 和 `POST /api/v1/remotes/:id/test`（连接测试）接口可用。创建时通过 multipart 上传私钥，API 响应不返回私钥路径。前端 AppLayout 布局、基础 UI 组件库（Button、Input、Modal、Table、Toast 等）已就位。

## 当前任务目标

实现远程配置管理页面（`/system/remotes`），包含列表展示、新增/编辑弹窗、连接测试和删除功能。

## 实现指导

### 1. 路由与页面

- 路由：`/system/remotes`，admin 权限
- 页面组件：`pages/system/RemoteConfigPage.vue`

### 2. API 模块

```typescript
// api/remotes.ts
function listRemotes(params?: PaginationParams): Promise<PaginatedData<RemoteConfig>>
function createRemote(formData: FormData): Promise<RemoteConfig>
function updateRemote(id: number, formData: FormData): Promise<RemoteConfig>
function deleteRemote(id: number): Promise<void>
function testRemoteConnection(id: number): Promise<{ success: boolean; message: string }>
```

### 3. 列表页

- 使用 AppTable 展示：名称、类型（SSH/云存储）、主机、端口、用户名、创建时间、操作
- 操作列：编辑按钮、测试按钮、删除按钮
- 页面顶部：标题「远程配置」+ 新增按钮
- 支持分页

### 4. 新增/编辑 Modal

- 使用 AppModal + AppFormItem 构建表单
- 字段：
  - 名称（AppInput，必填）
  - 类型（AppSelect：SSH / 云存储，云存储暂禁用或提示「即将支持」）
  - SSH 表单（type=ssh 时显示）：
    - 主机地址（AppInput，必填）
    - 端口（AppInput type=number，默认 22）
    - 用户名（AppInput，必填）
    - 私钥文件（文件上传 input，新建时必填，编辑时可选）
- 提交时使用 FormData 封装（因为有文件上传）
- 提交成功后刷新列表 + Toast 提示

### 5. 连接测试

- 点击测试按钮后：按钮显示 loading 状态 → 调用 test 接口 → Toast 显示测试结果（成功/失败 + 信息）

### 6. 删除

- 点击删除按钮 → AppConfirm 确认 → 调用删除接口
- 后端返回「被引用无法删除」时，Toast 显示错误信息

### 7. TypeScript 类型

```typescript
// types/remote.ts
interface RemoteConfig {
  id: number
  name: string
  type: 'ssh' | 'cloud'
  host: string
  port: number
  username: string
  cloud_provider?: string
  created_at: string
  updated_at: string
}
```

## 验收目标

1. admin 可从侧边导航进入远程配置页面
2. 可新增 SSH 远程配置（含私钥文件上传），列表刷新显示
3. 可编辑远程配置，选择性地更新私钥
4. 测试连接按钮可调用后端 test 接口并显示结果
5. 删除配置有确认弹窗，被引用时显示错误提示
6. 表单必填校验生效
7. 深色/浅色主题下样式正确
