# 13-01 任务进度实时展示

## 前序任务简报

系统全部功能页面基本完成：仪表盘、实例管理、备份目标、系统配置（远程配置/用户管理/SMTP 配置）、风险事件、个人中心。后端任务 API（`GET /api/v1/tasks/:id`）返回任务的 `progress`、`current_step`、`estimated_end`。备份/恢复执行过程中通过 progressCb 实时更新 task 记录。

## 当前任务目标

实现前端任务进度的实时轮询展示，提升用户对运行中任务的可观察性。

## 实现指导

### 1. 全局任务状态管理（`stores/task.ts`）

```typescript
interface TaskState {
  activeTasks: Task[]           // 全局运行中+排队中的任务
  pollingIntervalId: number | null
}

// useTaskStore
function startPolling(): void         // 开始全局任务轮询
function stopPolling(): void          // 停止轮询（用户登出时）
function fetchActiveTasks(): Promise<void>  // 获取活跃任务列表
function getTasksByInstance(instanceId: number): Task[]  // 获取特定实例的任务

// 单任务进度轮询
function watchTask(taskId: number, onUpdate: (task: Task) => void): () => void
// 返回停止函数，每 2 秒轮询一次，任务完成/失败时自动停止并触发 Toast
```

### 2. 全局任务轮询

- 用户登录后启动全局轮询（调用 `GET /api/v1/tasks` 获取活跃任务列表）
- 轮询间隔：10 秒（全局列表不需要太频繁）
- 用于：顶部栏任务指示器、仪表盘任务列表

### 3. 单任务进度轮询

- 当用户查看某个实例的运行中任务时，启动对该 task 的精细轮询
- 轮询间隔：2 秒
- 轮询 `GET /api/v1/tasks/:id`，获取 `progress`、`current_step`、`estimated_end`
- 任务完成（`success`/`failed`/`cancelled`）时：
  - 停止轮询
  - 触发 Toast 通知（成功：success toast; 失败：error toast）
  - 刷新关联数据（实例状态、备份列表等）

### 4. 实例详情概览 Tab — 任务进度

替换之前的占位区域，当实例有运行中任务时展示：

- **进度区域**（AppCard）：
  - 任务类型标签（滚动备份/冷备份/恢复）
  - AppProgress 进度条（0-100%）
  - 当前步骤描述（如"正在传输文件..."、"正在压缩..."）
  - 开始时间
  - 已运行时长（实时计时器）
  - 预计完成时间
  - 预计剩余时间
  - 取消按钮（admin）

- 无运行中任务时隐藏此区域

### 5. 仪表盘任务列表增强

将仪表盘「当前任务列表」的静态数据替换为 `useTaskStore` 中的实时数据：
- 每个任务行显示实时进度条
- 进度百分比文字

### 6. 顶部栏任务指示器

在 AppLayout 的顶部栏右侧添加运行中任务数指示器：
- 有运行中任务时显示数字气泡（如 Badge）
- 点击可展开小面板，列出运行中任务的简要信息
- 无任务时隐藏或显示灰色 0

### 7. 运行时长计时器

```typescript
// composables/useElapsedTime.ts
function useElapsedTime(startTime: string | null): Ref<string>
// 返回实时更新的运行时长字符串："00:01:23"
// 每秒更新一次
// startTime 为 null 时不计时
```

### 8. 取消任务

- 取消按钮点击 → AppConfirm 确认 → 调用 `POST /api/v1/tasks/:id/cancel`
- 取消后 Toast 提示、停止轮询

## 验收目标

1. 实例有运行中任务时详情概览显示进度条和步骤信息
2. 进度每 2 秒更新一次
3. 任务完成时弹出 Toast 通知（成功/失败）
4. 仪表盘任务列表显示实时进度
5. 顶部栏显示运行中任务数气泡
6. 已运行时长实时递增
7. admin 可点击取消按钮停止运行中任务
8. 页面离开时停止轮询（避免内存泄漏）
