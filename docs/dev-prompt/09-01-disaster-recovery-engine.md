# 09-01 容灾率计算引擎与触发

## 前序任务简报

备份/恢复引擎（阶段六/七）和审计日志（阶段八）全部完成。系统已具备完整的备份执行链路：调度器自动/手动触发 → 任务队列 → Worker 执行滚动/冷备份 → 保留清理 → 恢复。审计日志覆盖所有关键操作。

## 当前任务目标

实现容灾率计算引擎（四维评分模型）、计算结果缓存与自动触发机制，以及容灾率查询 API。

## 实现指导

### 1. 容灾率评分结构

```go
// service/disaster_recovery.go
type DisasterRecoveryScore struct {
    Total          float64  `json:"total"`            // 总分 0-100
    Level          string   `json:"level"`            // "safe" / "caution" / "risk" / "danger"
    Freshness      float64  `json:"freshness"`        // 备份新鲜度分项 0-100
    RecoveryPoints float64  `json:"recovery_points"`  // 恢复点可用性分项 0-100
    Redundancy     float64  `json:"redundancy"`       // 冗余与隔离度分项 0-100
    Stability      float64  `json:"stability"`        // 执行稳定性分项 0-100
    Deductions     []string `json:"deductions"`       // 主要扣分原因
    CalculatedAt   time.Time `json:"calculated_at"`
}
```

### 2. 计算逻辑

```go
type DRCalculator struct {
    db *store.DB
}

func NewDRCalculator(db *store.DB) *DRCalculator
func (c *DRCalculator) Calculate(ctx context.Context, instanceID int64) (*DisasterRecoveryScore, error)
```

总分公式：`Total = 0.35 × Freshness + 0.30 × RecoveryPoints + 0.20 × Redundancy + 0.15 × Stability`

**备份新鲜度（Freshness，满分 100）**：
- 查询实例所有已启用策略，计算最短备份周期（interval 秒数或 cron 下次间隔）
- 查询最近一次 `status=success` 的备份
- 距今 ≤ 1 周期：100 分
- 1~2 周期：线性 100→60
- 2~3 周期：线性 60→30
- >3 周期或无自动计划：0~20 分
- 无任何已启用策略：0 分，记录扣分原因

**恢复点可用性（RecoveryPoints，满分 100）**：
- 每个已启用策略检查：是否有至少一个 `status=success` 且未过保留期的备份
- 全部策略有可用恢复点：80 分起步
- 有多个连续可用恢复点：加分至 100
- 某策略无可用恢复点：该策略贡献 0 分
- 按策略数加权平均

**冗余与隔离度（Redundancy，满分 100）**：
- 收集实例所有策略引用的目标（去重）
- 计算健康目标数量和类型：
  - ≥2 健康目标且至少一个远程(ssh)：100
  - ≥2 健康目标均为本地：70
  - 1 个健康远程：60
  - 1 个健康本地：40
  - 异常目标每个扣 20

**执行稳定性（Stability，满分 100）**：
- 查询最近 10 次备份（所有策略合并）
- 成功率 ×80 + 无阻塞风险加分 20
- 连续失败 ≥3：扣至 20 以下
- 存在阻塞性风险（目标不可达/容量耗尽/凭证错误）：该项 0 分

### 3. 等级映射

```go
func scoreToLevel(total float64) string {
    switch {
    case total >= 85: return "safe"
    case total >= 70: return "caution"
    case total >= 40: return "risk"
    default: return "danger"
    }
}
```

### 4. 缓存机制

```go
type DRCache struct {
    mu    sync.RWMutex
    cache map[int64]*cachedScore // instanceID → score
}

type cachedScore struct {
    score     *DisasterRecoveryScore
    expiresAt time.Time
}

func (c *DRCache) Get(instanceID int64) (*DisasterRecoveryScore, bool) // 5 分钟过期
func (c *DRCache) Set(instanceID int64, score *DisasterRecoveryScore)
func (c *DRCache) Invalidate(instanceID int64)
```

### 5. 触发机制

在以下位置调用 `DRCache.Invalidate(instanceID)` 使缓存失效：

- **备份完成/失败后**：在 Worker Pool 的任务完成回调中
- **目标健康状态变化后**：在 HealthChecker 中，查找引用该目标的所有实例并 Invalidate
- **风险事件产生/解除后**：在 RiskDetector 中（阶段 09-02 实现）
- **策略创建/编辑/删除后**：在策略 Service 中

API 请求时调用 `DRCache.Get`，miss 时自动调用 `Calculate` 并 `Set`。

### 6. API 接口

**GET `/api/v1/instances/:id/disaster-recovery`**（已认证 + 实例权限）：
- 返回 `DisasterRecoveryScore`：总分、等级、四项分项、扣分原因列表、计算时间

## 验收目标

1. 容灾率计算返回 0-100 分和正确的等级
2. 四项分项各自独立计算，权重加和等于总分
3. 无任何策略的实例容灾率为 0（danger）
4. 有策略有成功备份的实例容灾率显著大于 0
5. 多目标（含远程）比单目标本地得分更高
6. 缓存在 5 分钟内返回缓存结果
7. 备份完成后缓存被自动清除
8. API 返回完整的评级信息和扣分原因
9. 为各分项计算逻辑编写单元测试
