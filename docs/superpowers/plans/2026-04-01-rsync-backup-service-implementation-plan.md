# Rsync Backup Service Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 交付首版可上线的 Linux-only Rsync Backup Service，包含认证、实例/策略/存储管理、滚动备份、冷备份、恢复、通知、审计日志、实时进度和单二进制部署能力。

**Architecture:** 采用 Go 单体应用，按 `config -> repository -> service -> executor/scheduler -> api` 分层，前端使用 Vue 3 SPA 独立构建后通过 `embed.FS` 嵌入。实现顺序遵循“先可运行骨架，再核心执行链路，再 API/实时，再 Balanced Flux 前端，再部署验收”的纵向切片，以便每个阶段都能形成可验证增量。

**Tech Stack:** Go, Gin, GORM, SQLite, bcrypt, JWT, robfig/cron, gorilla/websocket, Vue 3, Vue Router, Pinia, CSS variables, Vitest, Playwright, Docker, Makefile

---

## 实施前提

- 当前仓库仍处于文档阶段，本计划按绿地项目从零搭建。
- 为满足设计文档中的“用户管理”“实例权限分配”“设置页”能力，计划补充管理员专用用户与权限管理 API；这是对现有设计的必要落地补完，而不是范围扩张。
- `StorageBackend` 与 `Notifier` 先实现 `LocalStorage`、`SSHStorage`、`SMTPNotifier`；S3、WebDAV、rclone 保留接口，不进入 v1 开发任务。
- 前端组件与页面必须执行 `Balanced Flux` 视觉方向，使用 `Cyan Mint` 主品牌色、独立危险态颜色体系、浅深主题一致的 token 结构。

## 交付顺序

1. 建立后端/前端工程骨架、配置体系、测试脚本和基础构建链路。
2. 落实数据模型、认证授权、资源 CRUD、调度和任务执行核心链路。
3. 打通滚动备份、冷备份、恢复、通知、审计和实时进度 API。
4. 完成 Balanced Flux 组件库、核心页面、危险操作确认和最终部署验收。

### Task 1: 仓库骨架与开发工具链

**Files:**
- Create: `go.mod`
- Create: `cmd/server/main.go`
- Create: `internal/app/app.go`
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `web/package.json`
- Create: `web/tsconfig.json`
- Create: `web/vite.config.ts`
- Create: `web/vitest.config.ts`
- Create: `web/index.html`
- Create: `web/src/main.ts`
- Create: `web/src/App.vue`
- Create: `web/src/router/index.ts`
- Create: `web/src/test/setup.ts`
- Create: `web/src/App.spec.ts`
- Create: `web/src/styles/reset.css`
- Create: `web/src/styles/tokens.css`
- Create: `web/src/styles/theme-light.css`
- Create: `web/src/styles/theme-dark.css`
- Create: `.env.example`
- Create: `Makefile`
- Create: `Dockerfile`
- Create: `docker-compose.yml`
- Test: `internal/config/config_test.go`
- Test: `web/src/App.spec.ts`

- [ ] **Step 1: 编写配置加载失败测试**

```go
func TestLoadRejectsMissingJWTSecret(t *testing.T) {
    t.Setenv("RBS_JWT_SECRET", "")

    _, err := Load()
    if err == nil || !strings.Contains(err.Error(), "RBS_JWT_SECRET") {
        t.Fatalf("expected JWT secret validation error, got %v", err)
    }
}
```

Run: `go test ./internal/config -run TestLoadRejectsMissingJWTSecret -v`
Expected: FAIL with a validation error because `Load()` does not exist yet.

- [ ] **Step 2: 实现配置加载与应用启动骨架**

```go
type Config struct {
    Port          int
    DataDir       string
    JWTSecret     string
    AdminUser     string
    AdminPassword string
}

func Load() (Config, error) {
    // 从环境变量读取配置，校验 JWT secret 与数据目录。
}
```

```go
func main() {
    cfg, err := config.Load()
    if err != nil {
        log.Fatal(err)
    }
    if err := app.New(cfg).Run(); err != nil {
        log.Fatal(err)
    }
}
```

- [ ] **Step 3: 编写前端启动冒烟测试**

```ts
it("renders the application root", async () => {
  render(App)
  expect(screen.getByTestId("app-root")).toBeInTheDocument()
})
```

Run: `npm --prefix web run test -- src/App.spec.ts`
Expected: FAIL because the Vite app has not been scaffolded yet.

- [ ] **Step 4: 搭建 Vue/Vite 基础壳层与主题入口**

```ts
const app = createApp(App)
app.use(router)
app.mount("#app")
```

```vue
<template>
  <div data-testid="app-root">
    <RouterView />
  </div>
</template>
```

```css
:root {
  color-scheme: light dark;
  --radius-card: 16px;
  --color-primary-500: #63d9ff;
}
```

- [ ] **Step 5: 补齐构建命令与基础运行脚本**

```makefile
build:
	cd web && npm run build
	go build -o rsync-backup-service ./cmd/server

test:
	go test ./...
	npm --prefix web run test
```

- [ ] **Step 6: 运行骨架验证并提交**

Run: `go test ./internal/config -v`
Expected: PASS

Run: `npm --prefix web run test -- src/App.spec.ts`
Expected: PASS

Run: `npm --prefix web run build`
Expected: Vite build completes without errors.

```bash
git add go.mod cmd/server/main.go internal/config internal/app web .env.example Makefile Dockerfile docker-compose.yml
git commit -m "chore: scaffold backend and frontend workspaces"
```

### Task 2: 数据模型、SQLite 初始化与管理员引导

**Files:**
- Create: `internal/model/user.go`
- Create: `internal/model/ssh_key.go`
- Create: `internal/model/backup_instance.go`
- Create: `internal/model/storage_target.go`
- Create: `internal/model/strategy.go`
- Create: `internal/model/backup_record.go`
- Create: `internal/model/restore_record.go`
- Create: `internal/model/notification_channel.go`
- Create: `internal/model/notification_subscription.go`
- Create: `internal/model/instance_permission.go`
- Create: `internal/model/audit_log.go`
- Create: `internal/repository/db.go`
- Create: `internal/repository/migrate.go`
- Create: `internal/repository/seed.go`
- Create: `internal/repository/migrate_test.go`
- Modify: `internal/app/app.go`
- Modify: `cmd/server/main.go`
- Test: `internal/repository/migrate_test.go`

- [ ] **Step 1: 编写迁移与初始管理员测试**

```go
func TestMigrateAndSeedCreatesAdmin(t *testing.T) {
    db := openTestDB(t)
    cfg := config.Config{AdminUser: "admin", AdminPassword: "secret"}

    if err := MigrateAndSeed(db, cfg); err != nil {
        t.Fatalf("migrate failed: %v", err)
    }

    var user model.User
    if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
        t.Fatalf("expected seeded admin, got %v", err)
    }
    if !user.IsAdmin {
        t.Fatal("expected first seeded user to be admin")
    }
}
```

Run: `go test ./internal/repository -run TestMigrateAndSeedCreatesAdmin -v`
Expected: FAIL because repository bootstrap is not implemented.

- [ ] **Step 2: 定义 GORM 模型与关联关系**

```go
type Strategy struct {
    ID               uint
    InstanceID       uint
    Name             string
    BackupType       string
    CronExpr         *string
    IntervalSeconds  int
    RetentionDays    int
    RetentionCount   int
    ColdVolumeSize   *string
    StorageTargets   []StorageTarget `gorm:"many2many:strategy_storage_bindings"`
    CreatedAt        time.Time
    UpdatedAt        time.Time
}
```

- [ ] **Step 3: 实现 SQLite 打开、自动迁移与首个管理员播种**

```go
func OpenSQLite(dataDir string) (*gorm.DB, error) {
    dbPath := filepath.Join(dataDir, "rbs.sqlite")
    return gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
}

func MigrateAndSeed(db *gorm.DB, cfg config.Config) error {
    if err := db.AutoMigrate(allModels()...); err != nil {
        return err
    }
    return EnsureAdminUser(db, cfg.AdminUser, cfg.AdminPassword)
}
```

- [ ] **Step 4: 在应用启动时串联数据目录、数据库和播种逻辑**

```go
type App struct {
    Config config.Config
    DB     *gorm.DB
}

func New(cfg config.Config) *App {
    return &App{Config: cfg}
}
```

- [ ] **Step 5: 运行迁移测试并提交**

Run: `go test ./internal/repository -v`
Expected: PASS

Run: `go test ./...`
Expected: PASS for all currently implemented backend packages.

```bash
git add internal/model internal/repository internal/app cmd/server/main.go
git commit -m "feat: add sqlite models and bootstrap migration"
```

### Task 3: 认证、权限与审计中间件基础

**Files:**
- Create: `internal/service/auth_service.go`
- Create: `internal/service/user_service.go`
- Create: `internal/service/permission_service.go`
- Create: `internal/api/router.go`
- Create: `internal/api/middleware/jwt.go`
- Create: `internal/api/middleware/verify.go`
- Create: `internal/api/middleware/audit.go`
- Create: `internal/api/handler/auth_handler.go`
- Create: `internal/api/handler/user_handler.go`
- Create: `internal/api/handler/permission_handler.go`
- Create: `internal/api/auth_test.go`
- Create: `internal/api/middleware/auth_test.go`
- Modify: `internal/app/app.go`
- Test: `internal/api/auth_test.go`
- Test: `internal/api/middleware/auth_test.go`

- [ ] **Step 1: 编写登录、续期、二次认证和权限拦截测试**

```go
func TestLoginIssuesAccessAndRefreshTokens(t *testing.T) {
    router := newAuthTestRouter(t)

    req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"username":"admin","password":"secret"}`))
    req.Header.Set("Content-Type", "application/json")

    resp := httptest.NewRecorder()
    router.ServeHTTP(resp, req)

    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.Code)
    }
}
```

```go
func TestVerifyProtectedRouteRejectsMissingToken(t *testing.T) {
    router := newAuthTestRouter(t)
    req := httptest.NewRequest(http.MethodPost, "/api/instances/1/restore", nil)
    resp := httptest.NewRecorder()
    router.ServeHTTP(resp, req)
    if resp.Code != http.StatusUnauthorized {
        t.Fatalf("expected 401, got %d", resp.Code)
    }
}
```

Run: `go test ./internal/api/... -run 'TestLoginIssuesAccessAndRefreshTokens|TestVerifyProtectedRouteRejectsMissingToken' -v`
Expected: FAIL because auth services and middleware do not exist yet.

- [ ] **Step 2: 实现密码哈希、JWT 与一次性 verify token 服务**

```go
type TokenPair struct {
    AccessToken  string `json:"access_token"`
    RefreshToken string `json:"refresh_token"`
}

func (s *AuthService) Login(ctx context.Context, username, password string) (TokenPair, error) {
    // 校验 bcrypt，签发 2h access_token 与 7d refresh_token。
}
```

- [ ] **Step 3: 实现 JWT、管理员、实例权限与审计中间件**

```go
func RequireInstanceRole(minRole string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 超级管理员直接通过，否则检查 instance_permissions。
    }
}
```

```go
func AuditLogger(repo repository.AuditLogRepository) gin.HandlerFunc {
    return func(c *gin.Context) {
        c.Next()
        // 在非 GET 或显式审计资源上写入 audit_logs。
    }
}
```

- [ ] **Step 4: 实现认证相关路由与管理员用户/权限管理 API**

```go
auth := api.Group("/auth")
auth.POST("/login", handler.Login)
auth.POST("/refresh", handler.Refresh)
auth.POST("/verify", middleware.RequireJWT(), handler.Verify)
auth.GET("/me", middleware.RequireJWT(), handler.Me)
auth.PUT("/password", middleware.RequireJWT(), handler.ChangePassword)
```

```go
admin := api.Group("/users", middleware.RequireJWT(), middleware.RequireAdmin())
admin.GET("", userHandler.List)
admin.POST("", userHandler.Create)
admin.PUT("/:id/password", userHandler.ResetPassword)
```

- [ ] **Step 5: 运行认证与中间件测试并提交**

Run: `go test ./internal/service ./internal/api/... -v`
Expected: PASS

```bash
git add internal/service/auth_service.go internal/service/user_service.go internal/service/permission_service.go internal/api
git commit -m "feat: add auth, permission, and audit foundations"
```

### Task 4: 备份实例、策略、存储目标与 SSH 密钥管理

**Files:**
- Create: `internal/repository/instance_repository.go`
- Create: `internal/repository/strategy_repository.go`
- Create: `internal/repository/storage_target_repository.go`
- Create: `internal/repository/ssh_key_repository.go`
- Create: `internal/service/instance_service.go`
- Create: `internal/service/strategy_service.go`
- Create: `internal/service/storage_target_service.go`
- Create: `internal/service/ssh_key_service.go`
- Create: `internal/storage/backend.go`
- Create: `internal/storage/local.go`
- Create: `internal/storage/ssh.go`
- Create: `internal/api/handler/instance_handler.go`
- Create: `internal/api/handler/strategy_handler.go`
- Create: `internal/api/handler/storage_target_handler.go`
- Create: `internal/api/handler/ssh_key_handler.go`
- Create: `internal/service/resource_validation_test.go`
- Modify: `internal/api/router.go`
- Test: `internal/service/resource_validation_test.go`

- [ ] **Step 1: 编写资源验证测试，锁定关键业务约束**

```go
func TestCreateStrategyRejectsMixedCronAndInterval(t *testing.T) {
    svc := newStrategyServiceForTest(t)
    _, err := svc.Create(context.Background(), CreateStrategyRequest{
        Name:            "nightly",
        BackupType:      "rolling",
        CronExpr:        ptr("0 0 * * *"),
        IntervalSeconds: 3600,
    })
    if err == nil {
        t.Fatal("expected validation error")
    }
}
```

```go
func TestRegisterSSHKeyRejectsWorldReadableFile(t *testing.T) {
    svc := newSSHKeyServiceForTest(t)
    err := svc.Register(context.Background(), "prod", "./testdata/id_rsa_0644")
    if err == nil || !strings.Contains(err.Error(), "0600") {
        t.Fatalf("expected mode validation error, got %v", err)
    }
}
```

Run: `go test ./internal/service -run 'TestCreateStrategyRejectsMixedCronAndInterval|TestRegisterSSHKeyRejectsWorldReadableFile' -v`
Expected: FAIL because services do not exist yet.

- [ ] **Step 2: 实现资源仓储与业务服务**

```go
func (s *StrategyService) ValidateCreate(req CreateStrategyRequest) error {
    if req.CronExpr != nil && req.IntervalSeconds > 0 {
        return errors.New("cron_expr and interval_seconds are mutually exclusive")
    }
    if req.RetentionDays < 0 || req.RetentionCount < 0 {
        return errors.New("retention values must be >= 0")
    }
    return nil
}
```

- [ ] **Step 3: 实现本地/SSH 存储后端接口与连通性测试能力**

```go
type StorageBackend interface {
    Type() string
    Upload(ctx context.Context, localPath, remotePath string) error
    Download(ctx context.Context, remotePath, localPath string) error
    List(ctx context.Context, prefix string) ([]StorageObject, error)
    Delete(ctx context.Context, path string) error
    SpaceAvailable(ctx context.Context, path string) (uint64, error)
    TestConnection(ctx context.Context) error
}
```

- [ ] **Step 4: 暴露资源 CRUD 与测试连通性 API**

```go
api.GET("/instances", instanceHandler.List)
api.POST("/instances", instanceHandler.Create)
api.GET("/instances/:id", instanceHandler.Get)
api.PUT("/instances/:id", instanceHandler.Update)
api.DELETE("/instances/:id", middleware.RequireVerifyToken(), instanceHandler.Delete)
```

```go
api.POST("/storage-targets/:id/test", storageTargetHandler.TestConnection)
api.POST("/ssh-keys/:id/test", sshKeyHandler.TestConnection)
```

- [ ] **Step 5: 运行资源管理测试并提交**

Run: `go test ./internal/service ./internal/storage ./internal/api/... -v`
Expected: PASS

```bash
git add internal/repository internal/service internal/storage internal/api/handler internal/api/router.go
git commit -m "feat: add resource management and storage backends"
```

### Task 5: 调度器、运行任务注册表与冲突控制

**Files:**
- Create: `internal/scheduler/scheduler.go`
- Create: `internal/scheduler/registry.go`
- Create: `internal/executor/task_manager.go`
- Create: `internal/executor/task_lock.go`
- Create: `internal/service/scheduler_service.go`
- Create: `internal/scheduler/scheduler_test.go`
- Create: `internal/executor/task_manager_test.go`
- Modify: `internal/service/strategy_service.go`
- Modify: `internal/app/app.go`
- Test: `internal/scheduler/scheduler_test.go`
- Test: `internal/executor/task_manager_test.go`

- [ ] **Step 1: 编写调度与冲突跳过测试**

```go
func TestSchedulerSkipsConflictingRun(t *testing.T) {
    manager := NewTaskManager()
    lockKey := "instance:1:target:2"

    if _, ok := manager.TryStart(lockKey, func() {}); !ok {
        t.Fatal("expected first task to acquire lock")
    }
    if _, ok := manager.TryStart(lockKey, func() {}); ok {
        t.Fatal("expected second task to be rejected")
    }
}
```

Run: `go test ./internal/scheduler ./internal/executor -run 'TestSchedulerSkipsConflictingRun' -v`
Expected: FAIL because scheduler primitives do not exist yet.

- [ ] **Step 2: 实现任务注册表、取消句柄与实例+目标锁**

```go
type RunningTask struct {
    ID        string
    LockKey   string
    StartedAt time.Time
    Cancel    context.CancelFunc
}
```

```go
func (m *TaskManager) TryStart(lockKey string, cancel context.CancelFunc) (RunningTask, bool) {
    // 已存在相同 lockKey 时返回 false。
}
```

- [ ] **Step 3: 实现 cron/interval 统一调度封装**

```go
func (s *Scheduler) RegisterStrategy(strategy model.Strategy, run func(context.Context) error) error {
    // cron_expr 使用 robfig/cron，interval_seconds 使用 wrapper goroutine + ticker。
}
```

- [ ] **Step 4: 在策略变更时自动刷新调度注册表**

```go
func (s *StrategyService) Create(ctx context.Context, req CreateStrategyRequest) (*model.Strategy, error) {
    strategy, err := s.repo.Create(ctx, req)
    if err != nil {
        return nil, err
    }
    return strategy, s.scheduler.RefreshStrategy(*strategy)
}
```

- [ ] **Step 5: 运行调度测试并提交**

Run: `go test ./internal/scheduler ./internal/executor -v`
Expected: PASS

```bash
git add internal/scheduler internal/executor/task_manager.go internal/executor/task_lock.go internal/service/scheduler_service.go internal/service/strategy_service.go internal/app/app.go
git commit -m "feat: add scheduler and task conflict control"
```

### Task 6: 滚动备份执行链路、进度解析与保留策略

**Files:**
- Create: `internal/executor/runner.go`
- Create: `internal/executor/rsync.go`
- Create: `internal/executor/progress.go`
- Create: `internal/executor/snapshot.go`
- Create: `internal/executor/rolling_executor.go`
- Create: `internal/service/executor_service.go`
- Create: `internal/service/retention_service.go`
- Create: `internal/executor/progress_test.go`
- Create: `internal/executor/rolling_executor_test.go`
- Create: `internal/service/retention_service_test.go`
- Modify: `internal/model/backup_record.go`
- Test: `internal/executor/progress_test.go`
- Test: `internal/executor/rolling_executor_test.go`
- Test: `internal/service/retention_service_test.go`

- [ ] **Step 1: 编写进度解析与双远程中继测试**

```go
func TestParseProgress2Line(t *testing.T) {
    progress, ok := ParseProgress2("1,234,567  45%  12.34MB/s  0:01:23")
    if !ok || progress.Percentage != 45 {
        t.Fatalf("unexpected parse result: %+v", progress)
    }
}
```

```go
func TestBuildRollingPlanForRemoteToRemote(t *testing.T) {
    plan := BuildRollingPlan(testRemoteSource(), testRemoteTarget())
    if !plan.RequiresRelay {
        t.Fatal("expected relay mode for remote to remote rolling backup")
    }
}
```

```go
func TestMapRsyncExitCodeProducesReadableError(t *testing.T) {
    err := MapRsyncExitCode(23)
    if err == nil || !strings.Contains(err.Error(), "partial transfer") {
        t.Fatalf("expected readable rsync error, got %v", err)
    }
}
```

Run: `go test ./internal/executor -run 'TestParseProgress2Line|TestBuildRollingPlanForRemoteToRemote|TestMapRsyncExitCodeProducesReadableError' -v`
Expected: FAIL because parser and plan builder do not exist.

- [ ] **Step 2: 实现 rsync 命令构建器与 runner 抽象**

```go
type CommandSpec struct {
    Name string
    Args []string
    Dir  string
}

type Runner interface {
    Run(ctx context.Context, spec CommandSpec, onStdout func(string)) error
}

func WithExecutionTimeout(ctx context.Context, maxSeconds int) (context.Context, context.CancelFunc) {
    if maxSeconds <= 0 {
        return context.WithCancel(ctx)
    }
    return context.WithTimeout(ctx, time.Duration(maxSeconds)*time.Second)
}
```

- [ ] **Step 3: 实现快照目录、`--link-dest` 与 relay cache 规划**

```go
type RollingPlan struct {
    RequiresRelay bool
    SnapshotPath  string
    RelayCacheDir string
    LinkDest      string
}
```

- [ ] **Step 4: 实现进度滑动窗口、备份记录更新与保留策略清理**

```go
func EstimateRemaining(totalSize, transferred uint64, avgBytesPerSecond float64) time.Duration {
    if avgBytesPerSecond <= 0 {
        return 0
    }
    return time.Duration(float64(totalSize-transferred)/avgBytesPerSecond) * time.Second
}
```

```go
func (s *RetentionService) Cleanup(ctx context.Context, strategy model.Strategy, target model.StorageTarget) error {
    // 同时支持 retention_count 与 retention_days，按并集删除。
}
```

```go
func (s *ExecutorService) CheckTargetSpace(ctx context.Context, backend storage.StorageBackend, path string, estimatedSize uint64) error {
    // 空间不足时记录 warning audit/log，但不阻断执行，让 rsync/tar 自身给出最终错误。
}
```

- [ ] **Step 5: 运行滚动备份单元测试并提交**

Run: `go test ./internal/executor ./internal/service -v`
Expected: PASS

Run: `go test ./internal/executor -run TestParseProgress2Line -v`
Expected: PASS with parsed percentage, speed and ETA checks.

```bash
git add internal/executor internal/service/executor_service.go internal/service/retention_service.go internal/model/backup_record.go
git commit -m "feat: implement rolling backup execution and retention"
```

### Task 7: 冷备份、恢复与危险操作保护

**Files:**
- Create: `internal/executor/archive.go`
- Create: `internal/executor/cold_executor.go`
- Create: `internal/executor/restore.go`
- Create: `internal/service/restore_service.go`
- Create: `internal/executor/archive_test.go`
- Create: `internal/service/restore_service_test.go`
- Create: `internal/api/handler/backup_handler.go`
- Create: `internal/api/handler/restore_handler.go`
- Modify: `internal/api/router.go`
- Test: `internal/executor/archive_test.go`
- Test: `internal/service/restore_service_test.go`

- [ ] **Step 1: 编写冷备份打包与恢复约束测试**

```go
func TestBuildSplitArchiveCommand(t *testing.T) {
    spec := BuildArchiveCommand("/src", "/tmp/archive", ptr("1G"))
    if spec.Name != "sh" || !strings.Contains(strings.Join(spec.Args, " "), "split -b 1G") {
        t.Fatalf("unexpected split archive command: %+v", spec)
    }
}
```

```go
func TestRestoreRequiresVerifyToken(t *testing.T) {
    svc := newRestoreServiceForTest(t)
    _, err := svc.Start(context.Background(), RestoreRequest{})
    if err == nil {
        t.Fatal("expected verify token validation error")
    }
}
```

Run: `go test ./internal/executor ./internal/service -run 'TestBuildSplitArchiveCommand|TestRestoreRequiresVerifyToken' -v`
Expected: FAIL because cold executor and restore service do not exist yet.

- [ ] **Step 2: 实现冷备份打包、分卷与上传流程**

```go
func BuildArchiveCommand(sourceDir, outputBase string, volumeSize *string) CommandSpec {
    if volumeSize == nil {
        return CommandSpec{Name: "tar", Args: []string{"czf", outputBase + ".tar.gz", "-C", sourceDir, "."}}
    }
    return CommandSpec{Name: "sh", Args: []string{"-c", "tar czf - -C '" + sourceDir + "' . | split -b " + *volumeSize + " - '" + outputBase + ".tar.gz.part_'"}}
}
```

- [ ] **Step 3: 实现归档下载、解压恢复与 RestoreRecord 持久化**

```go
func (s *RestoreService) Start(ctx context.Context, req RestoreRequest) (*model.RestoreRecord, error) {
    // 下载归档或定位快照，校验 verify token，执行恢复并写入 restore_records。
}
```

- [ ] **Step 4: 暴露备份历史、可恢复快照/归档、恢复发起 API**

```go
api.GET("/instances/:id/backups", backupHandler.List)
api.GET("/instances/:id/snapshots", backupHandler.ListSnapshots)
api.POST("/instances/:id/restore", middleware.RequireVerifyToken(), restoreHandler.Create)
api.GET("/restore-records", restoreHandler.List)
```

- [ ] **Step 5: 运行冷备份/恢复测试并提交**

Run: `go test ./internal/executor ./internal/service ./internal/api/... -v`
Expected: PASS

```bash
git add internal/executor/archive.go internal/executor/cold_executor.go internal/executor/restore.go internal/service/restore_service.go internal/api/handler/backup_handler.go internal/api/handler/restore_handler.go internal/api/router.go
git commit -m "feat: add cold backup and restore flows"
```

### Task 8: 通知渠道、用户订阅与审计查询

**Files:**
- Create: `internal/notify/notifier.go`
- Create: `internal/notify/smtp.go`
- Create: `internal/notify/template.go`
- Create: `internal/service/notification_service.go`
- Create: `internal/service/audit_service.go`
- Create: `internal/api/handler/notification_handler.go`
- Create: `internal/api/handler/audit_handler.go`
- Create: `internal/notify/smtp_test.go`
- Create: `internal/service/notification_service_test.go`
- Modify: `internal/api/router.go`
- Test: `internal/notify/smtp_test.go`
- Test: `internal/service/notification_service_test.go`

- [ ] **Step 1: 编写 SMTP 配置校验与重试测试**

```go
func TestSMTPNotifierValidatesConfig(t *testing.T) {
    notifier := SMTPNotifier{}
    err := notifier.Validate(json.RawMessage(`{"host":"","port":587}`))
    if err == nil {
        t.Fatal("expected validation error")
    }
}
```

```go
func TestNotifyInstanceSubscribersRetriesWithBackoff(t *testing.T) {
    svc := newNotificationServiceForTest(t)
    if err := svc.Notify(context.Background(), sampleEvent()); err != nil {
        t.Fatalf("expected retryable send to eventually succeed, got %v", err)
    }
}
```

Run: `go test ./internal/notify ./internal/service -run 'TestSMTPNotifierValidatesConfig|TestNotifyInstanceSubscribersRetriesWithBackoff' -v`
Expected: FAIL because notifier implementation does not exist yet.

- [ ] **Step 2: 实现 Notifier 接口、SMTP 渠道配置和 HTML 模板发送**

```go
type NotifyEvent struct {
    Type       string
    Instance   string
    Strategy   string
    Message    string
    Detail     any
    OccurredAt time.Time
}
```

```go
func (n *SMTPNotifier) Send(ctx context.Context, event NotifyEvent) error {
    // 30 秒超时，指数退避重试 3 次。
}
```

- [ ] **Step 3: 实现通知渠道 CRUD、实例级订阅与权限校验发送**

```go
api.GET("/notification-channels", notificationHandler.ListChannels)
api.POST("/notification-channels", middleware.RequireAdmin(), notificationHandler.CreateChannel)
api.GET("/instances/:id/subscriptions", notificationHandler.ListSubscriptions)
api.POST("/instances/:id/subscriptions", notificationHandler.UpsertSubscription)
```

- [ ] **Step 4: 实现审计日志筛选查询服务与 API**

```go
func (s *AuditService) List(ctx context.Context, req ListAuditLogsRequest) ([]model.AuditLog, int64, error) {
    // 支持 user/action/time range 分页查询。
}
```

- [ ] **Step 5: 运行通知与审计测试并提交**

Run: `go test ./internal/notify ./internal/service ./internal/api/... -v`
Expected: PASS

```bash
git add internal/notify internal/service/notification_service.go internal/service/audit_service.go internal/api/handler/notification_handler.go internal/api/handler/audit_handler.go internal/api/router.go
git commit -m "feat: add notifications and audit queries"
```

### Task 9: HTTP API 汇总、系统接口与 WebSocket 实时推送

**Files:**
- Create: `internal/api/ws/hub.go`
- Create: `internal/api/ws/client.go`
- Create: `internal/api/handler/task_handler.go`
- Create: `internal/api/handler/system_handler.go`
- Create: `internal/service/dashboard_service.go`
- Create: `internal/api/ws/hub_test.go`
- Create: `internal/api/system_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/service/executor_service.go`
- Test: `internal/api/ws/hub_test.go`
- Test: `internal/api/system_test.go`

- [ ] **Step 1: 编写运行任务接口与 WebSocket 广播测试**

```go
func TestRunningTasksEndpointReturnsInMemoryTasks(t *testing.T) {
    router := newSystemTestRouter(t)
    req := httptest.NewRequest(http.MethodGet, "/api/tasks/running", nil)
    resp := httptest.NewRecorder()
    router.ServeHTTP(resp, req)
    if resp.Code != http.StatusOK {
        t.Fatalf("expected 200, got %d", resp.Code)
    }
}
```

```go
func TestProgressHubBroadcastsToClients(t *testing.T) {
    hub := NewHub()
    // 注册 client，推送一次 ProgressEvent，断言收到消息。
}
```

Run: `go test ./internal/api/... -run 'TestRunningTasksEndpointReturnsInMemoryTasks|TestProgressHubBroadcastsToClients' -v`
Expected: FAIL because task/system handlers and hub do not exist yet.

- [ ] **Step 2: 实现 WebSocket Hub 与 ProgressEvent 广播总线**

```go
type ProgressEvent struct {
    TaskID        string  `json:"task_id"`
    InstanceID    uint    `json:"instance_id"`
    Percentage    float64 `json:"percentage"`
    SpeedText     string  `json:"speed_text"`
    RemainingText string  `json:"remaining_text"`
    Status        string  `json:"status"`
}
```

- [ ] **Step 3: 实现运行任务、取消任务、系统状态与仪表盘聚合接口**

```go
api.GET("/tasks/running", taskHandler.ListRunning)
api.POST("/tasks/:id/cancel", taskHandler.Cancel)
api.GET("/system/status", systemHandler.Status)
api.GET("/system/dashboard", systemHandler.Dashboard)
api.GET("/audit-logs", auditHandler.List)
```

- [ ] **Step 4: 将执行器事件、调度状态与 WebSocket 推送串联**

```go
func (s *ExecutorService) PublishProgress(event ProgressEvent) {
    s.progressBus <- event
}
```

- [ ] **Step 5: 运行 API 汇总测试并提交**

Run: `go test ./internal/api/... ./internal/service -v`
Expected: PASS

```bash
git add internal/api/ws internal/api/handler/task_handler.go internal/api/handler/system_handler.go internal/service/dashboard_service.go internal/service/executor_service.go internal/api/router.go
git commit -m "feat: add realtime progress and system apis"
```

### Task 10: 前端基础设施、鉴权状态与 Balanced Flux Token 系统

**Files:**
- Create: `web/src/api/client.ts`
- Create: `web/src/api/types.ts`
- Create: `web/src/stores/auth.ts`
- Create: `web/src/stores/ui.ts`
- Create: `web/src/composables/useSession.ts`
- Create: `web/src/composables/useTheme.ts`
- Create: `web/src/layout/AppShell.vue`
- Create: `web/src/layout/SidebarNav.vue`
- Create: `web/src/layout/TopBar.vue`
- Create: `web/src/stores/auth.spec.ts`
- Create: `web/src/layout/AppShell.spec.ts`
- Modify: `web/src/router/index.ts`
- Modify: `web/src/styles/tokens.css`
- Modify: `web/src/styles/theme-light.css`
- Modify: `web/src/styles/theme-dark.css`
- Test: `web/src/stores/auth.spec.ts`
- Test: `web/src/layout/AppShell.spec.ts`

- [ ] **Step 1: 编写前端鉴权、主题切换与路由守卫测试**

```ts
it("redirects anonymous users to /login", async () => {
  const router = createTestRouter()
  await router.push("/")
  expect(router.currentRoute.value.path).toBe("/login")
})
```

```ts
it("applies dark theme tokens to the document root", async () => {
  const ui = useUiStore()
  ui.setTheme("dark")
  expect(document.documentElement.dataset.theme).toBe("dark")
})
```

Run: `npm --prefix web run test -- src/stores/auth.spec.ts src/layout/AppShell.spec.ts`
Expected: FAIL because auth store, app shell and theme store do not exist yet.

- [ ] **Step 2: 实现统一 API client、access token/refresh token 管理与 401 自动续期**

```ts
export async function apiFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  // 自动附加 access token；401 时尝试 refresh，再重放一次请求。
}
```

- [ ] **Step 3: 落地 Balanced Flux 四层 token 架构**

```css
:root {
  --primary-500: #63d9ff;
  --accent-mint-400: #7ef2d4;
  --error-500: #e77483;
  --surface-1: #ffffff;
  --surface-2: #f6fafd;
  --focus-ring: 0 0 0 3px color-mix(in srgb, var(--primary-500) 32%, transparent);
}

:root[data-theme="dark"] {
  --bg-canvas: #0e1726;
  --surface-1: #142133;
  --text-strong: #eaf5ff;
}
```

- [ ] **Step 4: 实现登录外壳、主布局与侧栏导航**

```vue
<template>
  <div class="app-shell">
    <SidebarNav />
    <main class="app-shell__content"><RouterView /></main>
  </div>
</template>
```

- [ ] **Step 5: 运行前端基础设施测试并提交**

Run: `npm --prefix web run test -- src/stores/auth.spec.ts src/layout/AppShell.spec.ts`
Expected: PASS

Run: `npm --prefix web run build`
Expected: PASS

```bash
git add web/src/api web/src/stores web/src/composables web/src/layout web/src/router/index.ts web/src/styles
git commit -m "feat: add frontend app shell and design tokens"
```

### Task 11: 输入、操作与危险交互组件

**Files:**
- Create: `web/src/components/ui/AppButton.vue`
- Create: `web/src/components/ui/AppInput.vue`
- Create: `web/src/components/ui/AppTextarea.vue`
- Create: `web/src/components/ui/AppSelect.vue`
- Create: `web/src/components/ui/AppSwitch.vue`
- Create: `web/src/components/ui/AppModal.vue`
- Create: `web/src/components/ui/AppDialog.vue`
- Create: `web/src/components/ui/AppFormField.vue`
- Create: `web/src/components/ui/AppPasswordInput.vue`
- Create: `web/src/components/ui/AppTabs.vue`
- Create: `web/src/components/ui/AppBreadcrumb.vue`
- Create: `web/src/components/ui/AppButton.spec.ts`
- Create: `web/src/components/ui/AppDialog.spec.ts`
- Test: `web/src/components/ui/AppButton.spec.ts`
- Test: `web/src/components/ui/AppDialog.spec.ts`

- [ ] **Step 1: 编写按钮、焦点环与危险确认样式测试**

```ts
it("renders danger button with error semantics instead of brand color", () => {
  render(AppButton, { props: { variant: "danger" } })
  expect(screen.getByRole("button")).toHaveAttribute("data-variant", "danger")
})
```

```ts
it("keeps destructive dialog copy visible and focus-trapped", async () => {
  render(AppDialog, { props: { tone: "danger", open: true } })
  expect(screen.getByRole("dialog")).toBeInTheDocument()
})
```

Run: `npm --prefix web run test -- src/components/ui/AppButton.spec.ts src/components/ui/AppDialog.spec.ts`
Expected: FAIL because UI primitives do not exist yet.

- [ ] **Step 2: 实现按钮、输入与选择器的 token 绑定和状态反馈**

```vue
<button class="app-button" :data-variant="variant" :data-size="size">
  <slot />
</button>
```

```css
.app-button[data-variant="primary"] {
  background: var(--primary-500);
}

.app-button[data-variant="danger"] {
  background: var(--error-500);
}
```

- [ ] **Step 3: 实现密码输入、模态框、对话框与焦点管理**

```ts
const firstFocusable = dialogRef.value?.querySelector<HTMLElement>("button, input, [tabindex]")
firstFocusable?.focus()
```

- [ ] **Step 4: 实现表单字段容器、错误文案与键盘无障碍交互**

```vue
<AppFormField label="确认密码" :error="errorMessage" required>
  <AppPasswordInput v-model="password" />
</AppFormField>
```

- [ ] **Step 5: 运行交互组件测试并提交**

Run: `npm --prefix web run test -- src/components/ui/AppButton.spec.ts src/components/ui/AppDialog.spec.ts`
Expected: PASS

```bash
git add web/src/components/ui
git commit -m "feat: add interactive ui primitives"
```

### Task 12: 表格、状态、反馈与高密度信息组件

**Files:**
- Create: `web/src/components/ui/AppTable.vue`
- Create: `web/src/components/ui/AppCard.vue`
- Create: `web/src/components/ui/AppTag.vue`
- Create: `web/src/components/ui/AppBadge.vue`
- Create: `web/src/components/ui/AppProgress.vue`
- Create: `web/src/components/ui/AppToastHost.vue`
- Create: `web/src/components/ui/AppNotification.vue`
- Create: `web/src/components/ui/AppTimeline.vue`
- Create: `web/src/components/ui/AppEmpty.vue`
- Create: `web/src/components/ui/AppSpinner.vue`
- Create: `web/src/components/ui/AppTable.spec.ts`
- Create: `web/src/components/ui/AppProgress.spec.ts`
- Create: `web/src/components/ui/AppNotification.spec.ts`
- Test: `web/src/components/ui/AppTable.spec.ts`
- Test: `web/src/components/ui/AppProgress.spec.ts`
- Test: `web/src/components/ui/AppNotification.spec.ts`

- [ ] **Step 1: 编写高密度列表和运行态反馈测试**

```ts
it("renders progress text alongside gradient bar", () => {
  render(AppProgress, { props: { percentage: 45, speedText: "12.34MB/s", etaText: "1m 23s" } })
  expect(screen.getByText("45%"))
  expect(screen.getByText("12.34MB/s"))
})
```

```ts
it("keeps table row hover subtle in dense mode", () => {
  render(AppTable, { props: { dense: true, rows: [{ id: 1, name: "prod" }], columns: [{ key: "name", label: "名称" }] } })
  expect(screen.getByRole("table")).toHaveAttribute("data-density", "dense")
})
```

Run: `npm --prefix web run test -- src/components/ui/AppTable.spec.ts src/components/ui/AppProgress.spec.ts src/components/ui/AppNotification.spec.ts`
Expected: FAIL because these display components do not exist yet.

- [ ] **Step 2: 实现表格、卡片、标签和状态徽标组件**

```vue
<span class="app-tag" :data-tone="tone">
  <slot />
</span>
```

```css
.app-tag[data-tone="success"] { background: color-mix(in srgb, var(--success-500) 18%, transparent); }
.app-tag[data-tone="error"] { background: color-mix(in srgb, var(--error-500) 18%, transparent); }
```

- [ ] **Step 3: 实现进度条、通知卡、时间线与全局 Toast 容器**

```vue
<div class="app-progress__meta">
  <span>{{ percentage }}%</span>
  <span>{{ speedText }}</span>
  <span>{{ etaText }}</span>
</div>
```

- [ ] **Step 4: 校准视觉纪律，确保表格区域无毛玻璃、危险态无品牌色污染**

```css
.app-table,
.app-table * {
  backdrop-filter: none;
}

.app-notification[data-tone="danger"] {
  border-color: color-mix(in srgb, var(--error-500) 58%, var(--border-default));
}
```

- [ ] **Step 5: 运行数据展示组件测试并提交**

Run: `npm --prefix web run test -- src/components/ui/AppTable.spec.ts src/components/ui/AppProgress.spec.ts src/components/ui/AppNotification.spec.ts`
Expected: PASS

```bash
git add web/src/components/ui/AppTable.vue web/src/components/ui/AppCard.vue web/src/components/ui/AppTag.vue web/src/components/ui/AppBadge.vue web/src/components/ui/AppProgress.vue web/src/components/ui/AppToastHost.vue web/src/components/ui/AppNotification.vue web/src/components/ui/AppTimeline.vue web/src/components/ui/AppEmpty.vue web/src/components/ui/AppSpinner.vue web/src/components/ui/*.spec.ts
git commit -m "feat: add data display and status components"
```

### Task 13: 页面实现、实时任务界面与恢复确认流程

**Files:**
- Create: `web/src/api/auth.ts`
- Create: `web/src/api/instances.ts`
- Create: `web/src/api/strategies.ts`
- Create: `web/src/api/storageTargets.ts`
- Create: `web/src/api/sshKeys.ts`
- Create: `web/src/api/backups.ts`
- Create: `web/src/api/notifications.ts`
- Create: `web/src/api/audit.ts`
- Create: `web/src/api/system.ts`
- Create: `web/src/api/users.ts`
- Create: `web/src/views/LoginView.vue`
- Create: `web/src/views/DashboardView.vue`
- Create: `web/src/views/InstancesListView.vue`
- Create: `web/src/views/InstanceDetailView.vue`
- Create: `web/src/views/instance/OverviewTab.vue`
- Create: `web/src/views/instance/StrategiesTab.vue`
- Create: `web/src/views/instance/BackupsTab.vue`
- Create: `web/src/views/instance/RestoreTab.vue`
- Create: `web/src/views/instance/SubscriptionsTab.vue`
- Create: `web/src/views/StorageTargetsView.vue`
- Create: `web/src/views/SSHKeysView.vue`
- Create: `web/src/views/NotificationsView.vue`
- Create: `web/src/views/AuditLogsView.vue`
- Create: `web/src/views/SettingsView.vue`
- Create: `web/src/composables/useRealtimeTasks.ts`
- Create: `web/src/views/LoginView.spec.ts`
- Create: `web/src/views/InstancesListView.spec.ts`
- Create: `web/src/views/instance/RestoreTab.spec.ts`
- Test: `web/src/views/LoginView.spec.ts`
- Test: `web/src/views/InstancesListView.spec.ts`
- Test: `web/src/views/instance/RestoreTab.spec.ts`

- [ ] **Step 1: 编写登录、实例列表和恢复确认页面测试**

```ts
it("submits login form and stores returned token pair", async () => {
  render(LoginView)
  await userEvent.type(screen.getByLabelText("用户名"), "admin")
  await userEvent.type(screen.getByLabelText("密码"), "secret")
  await userEvent.click(screen.getByRole("button", { name: "登录" }))
  expect(mockLogin).toHaveBeenCalled()
})
```

```ts
it("requires password confirmation before restore submit", async () => {
  render(RestoreTab)
  await userEvent.click(screen.getByRole("button", { name: "开始恢复" }))
  expect(screen.getByText(/二次认证/)).toBeInTheDocument()
})
```

Run: `npm --prefix web run test -- src/views/LoginView.spec.ts src/views/InstancesListView.spec.ts src/views/instance/RestoreTab.spec.ts`
Expected: FAIL because page views and API modules do not exist yet.

- [ ] **Step 2: 实现登录页、仪表盘与全局导航**

```vue
<AppCard class="login-card">
  <h1>Rsync Backup Service</h1>
  <form @submit.prevent="submit">
    <AppInput v-model="form.username" label="用户名" />
    <AppPasswordInput v-model="form.password" label="密码" />
  </form>
</AppCard>
```

- [ ] **Step 3: 实现实例列表、详情页 Tabs 与资源 CRUD 页面**

```vue
<AppTabs :items="tabs" v-model="activeTab" />
<RouterView />
```

```ts
const tabs = [
  { key: "overview", label: "概览" },
  { key: "strategies", label: "策略" },
  { key: "backups", label: "备份历史" },
  { key: "restore", label: "恢复" },
  { key: "subscriptions", label: "通知订阅" },
]
```

- [ ] **Step 4: 实现运行中任务、实时进度、双远程中继提示与仪表盘汇总**

```ts
export function useRealtimeTasks() {
  // 连接 /api/ws/progress，每秒更新运行中任务与 ETA。
}
```

```vue
<AppNotification v-if="isRelayMode" tone="info" title="中继模式">
  源与目标均为远程主机，将使用本机缓存目录转发，请确认磁盘空间充足。
</AppNotification>
```

- [ ] **Step 5: 实现恢复风险确认、用户管理和实例权限设置页**

```vue
<AppDialog :open="confirmOpen" tone="danger" title="确认恢复">
  <p>即将恢复到 {{ targetPath }}。覆盖模式不可撤销。</p>
  <AppPasswordInput v-model="verifyPassword" label="确认密码" />
</AppDialog>
```

- [ ] **Step 6: 运行页面测试与前端构建并提交**

Run: `npm --prefix web run test -- src/views/LoginView.spec.ts src/views/InstancesListView.spec.ts src/views/instance/RestoreTab.spec.ts`
Expected: PASS

Run: `npm --prefix web run build`
Expected: PASS

```bash
git add web/src/api web/src/views web/src/composables/useRealtimeTasks.ts
git commit -m "feat: add application pages and realtime workflows"
```

### Task 14: 单二进制打包、Docker 化与端到端验收

**Files:**
- Create: `internal/webui/embed.go`
- Create: `web/playwright.config.ts`
- Create: `web/tests/e2e/auth-and-dashboard.spec.ts`
- Create: `web/tests/e2e/manual-backup-flow.spec.ts`
- Create: `web/tests/e2e/restore-danger-flow.spec.ts`
- Create: `README.md`
- Modify: `Dockerfile`
- Modify: `docker-compose.yml`
- Modify: `Makefile`
- Modify: `.env.example`
- Modify: `cmd/server/main.go`
- Modify: `web/vite.config.ts`
- Test: `web/tests/e2e/auth-and-dashboard.spec.ts`
- Test: `web/tests/e2e/manual-backup-flow.spec.ts`
- Test: `web/tests/e2e/restore-danger-flow.spec.ts`

- [ ] **Step 1: 编写端到端关键路径测试**

```ts
test("admin can login and see dashboard metrics", async ({ page }) => {
  await page.goto("/login")
  await page.getByLabel("用户名").fill("admin")
  await page.getByLabel("密码").fill("secret")
  await page.getByRole("button", { name: "登录" }).click()
  await expect(page.getByText("运行中任务")).toBeVisible()
})
```

```ts
test("restore flow requires second password confirmation", async ({ page }) => {
  await page.goto("/instances/1")
  await page.getByRole("tab", { name: "恢复" }).click()
  await page.getByRole("button", { name: "开始恢复" }).click()
  await expect(page.getByText("确认密码")).toBeVisible()
})
```

Run: `npm --prefix web run test:e2e`
Expected: FAIL because embedded static serving and Playwright setup do not exist yet.

- [ ] **Step 2: 实现 `embed.FS` 静态资源服务与 SPA fallback**

```go
//go:embed dist
var Dist embed.FS

func RegisterStatic(router *gin.Engine) {
        // 在 internal/webui 包中暴露静态资源，提供 /assets/* 与非 API 路由 fallback 到 index.html。
}
```

```ts
export default defineConfig({
    build: {
        outDir: "../internal/webui/dist",
        emptyOutDir: true,
    },
})
```

- [ ] **Step 3: 完善 Docker、多阶段构建、Compose 与运维文档**

```dockerfile
FROM node:22-alpine AS web-build
WORKDIR /app
COPY web/package*.json ./web/
WORKDIR /app/web
RUN npm ci
COPY web .
RUN npm run build

FROM golang:1.24-alpine AS go-build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY .env.example Makefile ./
COPY --from=web-build /app/internal/webui/dist ./internal/webui/dist
RUN go build -o /out/rsync-backup-service ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache rsync openssh-client ca-certificates tzdata
COPY --from=go-build /out/rsync-backup-service /usr/local/bin/rsync-backup-service
ENTRYPOINT ["/usr/local/bin/rsync-backup-service"]
```

- [ ] **Step 4: 运行完整验收矩阵**

Run: `go test ./...`
Expected: PASS

Run: `npm --prefix web run test`
Expected: PASS

Run: `npm --prefix web run build`
Expected: PASS

Run: `npm --prefix web run test:e2e`
Expected: PASS

Run: `docker compose build`
Expected: PASS

- [ ] **Step 5: 提交发布候选版本**

```bash
git add internal/webui/embed.go web/vite.config.ts web/playwright.config.ts web/tests/e2e README.md Dockerfile docker-compose.yml Makefile .env.example cmd/server/main.go
git commit -m "chore: package embedded app and add e2e validation"
```

## 覆盖检查

- 单二进制、Docker、嵌入式前端、`.env` 与 Makefile：Task 1、Task 14
- SQLite、所有核心表、首次启动管理员：Task 2
- JWT、refresh token、二次认证、实例级权限、审计中间件：Task 3
- 备份实例、策略、存储目标、SSH 密钥、连通性测试：Task 4
- Cron/间隔调度、任务冲突跳过、取消执行：Task 5、Task 9
- 滚动备份、`--link-dest`、双远程中继、进度估算、保留策略、空间预检查、超时与 rsync exit code 映射：Task 6
- 冷备份、分卷、恢复、风险确认后端约束：Task 7
- SMTP 通知、用户订阅、审计查询：Task 8
- 系统状态、仪表盘、运行中任务、WebSocket 推送：Task 9、Task 13
- Balanced Flux 设计 token、浅深主题、危险态分离、组件族规范：Task 10、Task 11、Task 12
- 登录页、实例页、通知页、审计页、设置页、实时任务面板、恢复确认：Task 13

## 明确不在 v1 范围内

- S3、WebDAV、rclone 的实际存储实现
- 多品牌皮肤系统
- Windows/macOS 服务端运行支持
- 复杂报表、图表自定义和跨实例批量编排