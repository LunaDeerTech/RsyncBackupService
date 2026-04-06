# 09-02 风险事件检测与生命周期

## 前序任务简报

容灾率计算引擎已完成：四维评分模型（新鲜度/恢复点/冗余度/稳定性）可计算实例容灾率，缓存机制和自动触发（备份完成/目标状态变化）已就位，API 可查询实例容灾评分。

## 当前任务目标

实现风险事件检测引擎，支持 7 种风险类型的自动检测、等级升级和解决标记。

## 实现指导

### 1. 风险检测器（`engine/risk_detector.go`）

```go
type RiskDetector struct {
    db       *store.DB
    drCache  *DRCache
    audit    *audit.Logger
}

func NewRiskDetector(db *store.DB, drCache *DRCache, audit *audit.Logger) *RiskDetector
```

### 2. 风险检测方法

每种风险通过特定的触发点调用：

```go
// 备份失败后调用
func (rd *RiskDetector) OnBackupFailed(ctx context.Context, instanceID int64, policyID int64) error

// 备份成功后调用（可能解决某些风险）
func (rd *RiskDetector) OnBackupSuccess(ctx context.Context, instanceID int64, policyID int64) error

// 健康检查完成后调用
func (rd *RiskDetector) OnHealthCheckComplete(ctx context.Context, targetID int64, status string) error

// 恢复失败后调用
func (rd *RiskDetector) OnRestoreFailed(ctx context.Context, instanceID int64) error

// 定期扫描调用（检查 overdue 和 cold_backup_missing）
func (rd *RiskDetector) PeriodicCheck(ctx context.Context) error
```

### 3. 风险检测规则

| 风险类型 | 触发条件 | 等级 |
|---------|---------|------|
| `backup_failed` | 备份任务失败 | 首次 warning，连续 ≥3 次 critical |
| `backup_overdue` | 距上次成功备份 > 计划周期 ×2 | ×2 warning，×3 critical |
| `cold_backup_missing` | 有滚动策略但无冷备份策略 | info |
| `target_unreachable` | 目标健康检查状态变为 unreachable | critical |
| `target_capacity_low` | 剩余容量 <20% | warning；<5% critical |
| `restore_failed` | 恢复任务失败 | critical |
| `credential_error` | SSH 连接认证失败（从备份/恢复任务的错误信息中检测） | critical |

### 4. Store 层

```go
func (db *DB) CreateRiskEvent(event *model.RiskEvent) error
func (db *DB) ListRiskEvents(query RiskEventQuery) ([]model.RiskEvent, int64, error)
func (db *DB) GetActiveRiskEvent(instanceID *int64, targetID *int64, source string) (*model.RiskEvent, error)
func (db *DB) ResolveRiskEvent(id int64) error
func (db *DB) UpdateRiskEventSeverity(id int64, severity string) error
func (db *DB) ListUnresolvedRiskEvents() ([]model.RiskEvent, error)
func (db *DB) CountConsecutiveFailures(instanceID int64, policyID int64) (int, error)
```

### 5. 风险生命周期管理

**产生**：
1. 检查是否已存在同类型未解决的风险事件
2. 不存在则创建新记录
3. 已存在则检查是否需要升级（如 warning → critical）
4. 写入 audit_log
5. 使关联实例的容灾率缓存失效

**升级**：
- `backup_failed`：查询连续失败次数，≥3 时升级为 critical
- `backup_overdue`：超时倍数增加时升级
- `target_capacity_low`：剩余容量继续下降时升级

**解决**：
- `backup_failed` / `backup_overdue`：备份成功后自动解决
- `target_unreachable`：健康检查恢复 healthy 后自动解决
- `target_capacity_low`：容量恢复后自动解决
- `cold_backup_missing`：创建冷备份策略后自动解决
- 标记 `resolved=true`、`resolved_at=now`
- 使关联实例的容灾率缓存失效

### 6. 定期扫描

在服务启动时注册定期检查（可与健康检查复用定时器，每 30 分钟执行一次 `PeriodicCheck`）：
- 扫描所有已启用策略，检查 `backup_overdue`
- 扫描所有有滚动策略但无冷策略的实例，检查 `cold_backup_missing`

### 7. 触发点集成

在以下位置调用 RiskDetector 方法：
- Worker Pool 任务完成回调：`OnBackupFailed` 或 `OnBackupSuccess`、`OnRestoreFailed`
- HealthChecker 完成检查后：`OnHealthCheckComplete`

## 验收目标

1. 备份失败时自动创建 `backup_failed` 风险事件
2. 连续 3 次失败后风险升级为 critical
3. 备份成功后 `backup_failed` 和 `backup_overdue` 风险自动解决
4. 目标变为 unreachable 时创建 critical 风险事件
5. 目标恢复 healthy 后风险自动解决
6. 定期扫描检测 `backup_overdue` 和 `cold_backup_missing`
7. 风险事件变化后关联实例容灾率缓存失效
8. 为各风险检测规则编写单元测试
