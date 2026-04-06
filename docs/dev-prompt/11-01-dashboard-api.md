# 11-01 仪表盘后端接口

## 前序任务简报

阶段九（容灾率 + 风险事件）和阶段十（通知模块）已完成。系统核心功能已全面就绪：备份/恢复引擎、任务调度、保留清理、审计日志、容灾率评估、风险检测预警、邮件通知。前端实例管理/配置管理相关页面基本完整。

## 当前任务目标

实现仪表盘后端 API，提供系统全局的总览数据、风险事件列表、备份趋势、重点关注实例和即将执行的任务。

## 实现指导

### 1. 仪表盘总览（`GET /api/v1/dashboard/overview`，admin）

返回结构：
```go
type DashboardOverview struct {
    RunningTasks       int     `json:"running_tasks"`         // 运行中任务数
    QueuedTasks        int     `json:"queued_tasks"`          // 排队中任务数
    AbnormalInstances  int     `json:"abnormal_instances"`    // 容灾率 <70 的实例数
    UnresolvedRisks    int     `json:"unresolved_risks"`      // 未解决风险数
    SystemDRScore      float64 `json:"system_dr_score"`       // 系统综合容灾率（所有实例平均）
    SystemDRLevel      string  `json:"system_dr_level"`       // 系统容灾等级
    TargetHealthSummary struct {
        Healthy     int `json:"healthy"`
        Degraded    int `json:"degraded"`
        Unreachable int `json:"unreachable"`
    } `json:"target_health_summary"`
    TotalInstances int `json:"total_instances"`
    TotalBackups   int `json:"total_backups"`
}
```

### 2. 风险事件列表（`GET /api/v1/dashboard/risks`，admin）

- 返回未解决的风险事件列表，按 severity DESC + created_at DESC 排序
- 含关联的实例名称和目标名称
- 支持分页

### 3. 备份趋势（`GET /api/v1/dashboard/trends`，admin）

```go
type DashboardTrends struct {
    BackupResults []DailyBackupResult `json:"backup_results"` // 最近 7 天每天的成功/失败数量
    InstanceHealth struct {
        Safe    int `json:"safe"`
        Caution int `json:"caution"`
        Risk    int `json:"risk"`
        Danger  int `json:"danger"`
    } `json:"instance_health"` // 实例健康分布
}

type DailyBackupResult struct {
    Date    string `json:"date"`    // "2025-04-01"
    Success int    `json:"success"`
    Failed  int    `json:"failed"`
}
```

Store 查询：
```sql
SELECT DATE(completed_at) as date, 
       SUM(CASE WHEN status='success' THEN 1 ELSE 0 END) as success,
       SUM(CASE WHEN status='failed' THEN 1 ELSE 0 END) as failed
FROM backups 
WHERE completed_at >= date('now', '-7 days')
GROUP BY DATE(completed_at)
ORDER BY date
```

### 4. 重点关注实例（`GET /api/v1/dashboard/focus-instances`，admin）

- 返回容灾率最低或未解决风险最多的 5~8 个实例
- 每个实例包含：名称、容灾率分数/等级、未解决风险数、上次备份时间/状态

```go
type FocusInstance struct {
    ID              int64   `json:"id"`
    Name            string  `json:"name"`
    DRScore         float64 `json:"dr_score"`
    DRLevel         string  `json:"dr_level"`
    UnresolvedRisks int     `json:"unresolved_risks"`
    LastBackupTime  *time.Time `json:"last_backup_time"`
    LastBackupStatus string  `json:"last_backup_status"`
}
```

排序逻辑：优先取容灾率最低的，同分时风险数多的优先。

### 5. 即将执行的任务（`GET /api/v1/dashboard/upcoming-tasks`，admin）

- 返回未来 24 小时内即将执行的计划任务列表
- 需要调度器暴露方法获取各策略的下次触发时间

```go
// engine/scheduler.go（补充）
func (s *Scheduler) GetUpcomingTasks(within time.Duration) []UpcomingTask

type UpcomingTask struct {
    PolicyID     int64     `json:"policy_id"`
    PolicyName   string    `json:"policy_name"`
    InstanceID   int64     `json:"instance_id"`
    InstanceName string    `json:"instance_name"`
    Type         string    `json:"type"`           // "rolling" / "cold"
    NextRunAt    time.Time `json:"next_run_at"`
}
```

## 验收目标

1. Overview API 返回正确的运行任务数、异常实例数、未解决风险数
2. 系统综合容灾率是所有实例容灾率的平均值
3. 备份趋势返回最近 7 天的每日成功/失败数
4. 重点关注实例按容灾率升序排列
5. 即将执行的任务列表与调度器中的计划一致
6. 目标健康度摘要正确统计 healthy/degraded/unreachable
