# 09-03 前端容灾率展示

## 前序任务简报

容灾率计算引擎（四维评分 + 缓存 + 自动触发）和风险事件检测（7 种类型 + 等级升级 + 自动解决）均已完成。API `GET /api/v1/instances/:id/disaster-recovery` 可返回完整的容灾评分数据。

## 当前任务目标

在实例详情概览 Tab 接入容灾率数据展示，在实例列表增加容灾率列。

## 实现指导

### 1. API 模块

```typescript
// api/instances.ts（补充）
function getDisasterRecovery(instanceId: number): Promise<DisasterRecoveryScore>

interface DisasterRecoveryScore {
  total: number
  level: 'safe' | 'caution' | 'risk' | 'danger'
  freshness: number
  recovery_points: number
  redundancy: number
  stability: number
  deductions: string[]
  calculated_at: string
}
```

### 2. 实例详情概览 Tab — 容灾率卡片

替换之前的 "--" 占位，展示完整的容灾率信息：

- **总分区域**：大字号显示分数（如 "82"），旁边显示等级标签（AppBadge）
  - 等级颜色：safe=success, caution=warning, risk=error(orange), danger=error(red)
- **等级中文映射**：safe→「安全」, caution→「注意」, risk→「风险」, danger→「危险」
- **四项分项**：水平排列或 2x2 网格
  - 每项显示：名称 + 分数 + 小型进度条
  - 名称：备份新鲜度、恢复点可用性、冗余与隔离度、执行稳定性
- **扣分原因**：列表展示，每条一行，使用 warning/error 色图标前缀

### 3. 容灾率圆形/环形图（可选增强）

- 使用 CSS 实现简单的环形进度指示器（不需要引入图表库）
- 中心显示分数
- 环形颜色跟随等级

### 4. 实例列表页增加容灾率列

在 InstanceListPage 的 AppTable 中增加「容灾率」列：
- 显示分数 + 等级颜色小圆点
- 列表 API 需要返回每个实例的容灾率（或前端对每个实例单独请求）
- 建议：后端在实例列表接口中附加容灾率总分和等级（利用缓存），避免 N+1 请求

### 5. 后端实例列表扩展（可选）

如果前端逐一请求容灾率影响性能，可在后端 `GET /api/v1/instances` 返回数据中增加：

```json
{
  "id": 1,
  "name": "...",
  "dr_score": 82,
  "dr_level": "caution",
  ...
}
```

### 6. 等级样式 Token

```typescript
// utils/disaster-recovery.ts
function getDRLevelColor(level: string): string
// safe → 'var(--success-500)', caution → 'var(--warning-500)', etc.

function getDRLevelLabel(level: string): string
// safe → '安全', caution → '注意', risk → '风险', danger → '危险'

function getDRLevelBadgeVariant(level: string): BadgeVariant
// safe → 'success', caution → 'warning', risk → 'error', danger → 'error'
```

## 验收目标

1. 实例详情概览 Tab 显示容灾率总分、等级和四项分项
2. 等级颜色正确对应 safe/caution/risk/danger
3. 扣分原因列表展示清晰
4. 实例列表页显示每个实例的容灾率分数和等级色标
5. 无策略的实例显示 danger 级别
6. 深色/浅色主题下容灾率卡片样式正确
