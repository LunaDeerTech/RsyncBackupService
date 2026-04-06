# 04-01 远程配置后端

## 前序任务简报

阶段一至三已完成：后端工程骨架（Go HTTP Server + SQLite + 统一响应 + 中间件）就绪，认证系统（注册/登录/JWT/角色权限）完整可用，前端工程（Vue 3 + Tailwind + 主题系统）、AppLayout 布局和基础 UI 组件库已就位。

## 当前任务目标

实现 SSH/云远程配置的后端 CRUD 接口，包括私钥文件上传和 SSH 连接测试功能。

## 实现指导

### 1. 数据模型（`internal/model`）

```go
type RemoteConfig struct {
    ID             int64     `json:"id"`
    Name           string    `json:"name"`
    Type           string    `json:"type"` // "ssh" / "cloud"
    Host           string    `json:"host"`
    Port           int       `json:"port"`
    Username       string    `json:"username"`
    PrivateKeyPath string    `json:"-"`        // 不通过 API 返回
    CloudProvider  *string   `json:"cloud_provider,omitempty"`
    CloudConfig    *string   `json:"cloud_config,omitempty"`
    CreatedAt      time.Time `json:"created_at"`
    UpdatedAt      time.Time `json:"updated_at"`
}
```

### 2. Store 层

```go
func (db *DB) CreateRemoteConfig(rc *model.RemoteConfig) error
func (db *DB) GetRemoteConfigByID(id int64) (*model.RemoteConfig, error)
func (db *DB) ListRemoteConfigs() ([]model.RemoteConfig, error)
func (db *DB) UpdateRemoteConfig(rc *model.RemoteConfig) error
func (db *DB) DeleteRemoteConfig(id int64) error
func (db *DB) IsRemoteConfigInUse(id int64) (bool, error) // 检查是否被 instance 或 target 引用
```

### 3. Service 层

- **创建**：接收 multipart/form-data，包含 JSON 字段 + 私钥文件
  - 私钥文件保存到 `DATA_DIR/keys/<uuid>.pem`
  - 文件权限设为 `0600`
  - 数据库仅存储文件路径
- **编辑**：支持选择性更新私钥（有新文件则替换旧文件，无则保留原路径）
- **删除**：先检查是否被 `instances` 或 `backup_targets` 引用，有引用则拒绝删除；删除时同步删除磁盘上的私钥文件

### 4. API 接口

**GET `/api/v1/remotes`**（admin）：
- 返回远程配置列表，不含 `private_key_path`
- 支持分页

**POST `/api/v1/remotes`**（admin）：
- Content-Type: `multipart/form-data`
- 字段：`name`、`type`、`host`、`port`、`username`、`private_key`（文件）
- 校验：名称唯一性、host 非空、port 范围（1-65535）

**PUT `/api/v1/remotes/:id`**（admin）：
- Content-Type: `multipart/form-data`
- 可选更新字段
- 上传新私钥时替换旧文件

**DELETE `/api/v1/remotes/:id`**（admin）：
- 有引用时返回错误信息，提示哪些资源在使用

**POST `/api/v1/remotes/:id/test`**（admin）：
- SSH 连接测试：使用配置的 host/port/username/private_key 建立 SSH 连接
- 执行 `echo ok` 验证连通性
- 返回成功/失败信息
- 使用 `golang.org/x/crypto/ssh` 库

### 5. 安全约束

- 任何 API 响应中都不返回 `private_key_path`
- 私钥文件内容不通过任何 API 返回
- 上传的私钥文件进行基本格式校验（是否为有效的 PEM 格式）

## 验收目标

1. 可创建 SSH 类型远程配置，私钥文件存储到 `DATA_DIR/keys/` 下且权限为 0600
2. 列表接口不返回私钥路径信息
3. 连接测试接口对有效 SSH 配置返回成功
4. 删除被引用的远程配置返回错误
5. 删除未引用的远程配置成功，且磁盘上的私钥文件被清理
6. 为 store 层和连接测试编写单元测试
