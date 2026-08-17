# 软件验收审核报告（SAR）
## TSLOMS — 交通信号灯检测后台运维系统
### 版本 v7.0 | 审核日期：2026-08-17

---

## 目录

1. [执行摘要](#1-执行摘要)
2. [项目概况](#2-项目概况)
3. [功能完整性审核](#3-功能完整性审核)
4. [测试覆盖审核](#4-测试覆盖审核)
5. [代码质量审核](#5-代码质量审核)
6. [安全审核](#6-安全审核)
7. [文档完整性审核](#7-文档完整性审核)
8. [部署与运维就绪度审核](#8-部署与运维就绪度审核)
9. [发现问题汇总](#9-发现问题汇总)
10. [发布建议](#10-发布建议)
11. [审核结论](#11-审核结论)

---

## 1. 执行摘要

| 项目 | 结论 |
|------|------|
| **总体发布就绪度** | ⚠️ **有条件通过** — 须修复 2 个阻断项后方可发布 |
| 功能完整性 | ✅ 通过（110+ API 端点，37 个前端页面，无 TODO/FIXME 残留） |
| 测试覆盖 | ✅ 通过（后端 CI 门禁 ≥80%，81 个测试文件） |
| 代码质量 | ✅ 通过（无不当 panic/Fatal，代码结构清晰） |
| 安全状态 | ❌ **未通过** — 2 个高危问题，需修复后方可发布 |
| 文档完整性 | ⚠️ 部分通过（缺 Swagger/API 文档、CHANGELOG） |
| 部署就绪度 | ⚠️ 部分通过（Dockerfile Go 版本不匹配，compose 弱凭据） |

**阻断项（必须修复后才能发布）**：
- **BLOCK-1**：超管初始密码硬编码在源代码常量中（`seed.go`）
- **BLOCK-2**：`docker-compose.yml` 中 JWT Secret 为弱值 `tsloms-dev-secret`

---

## 2. 项目概况

| 属性 | 值 |
|------|-----|
| 系统全称 | 交通信号灯检测后台运维系统（TSLOMS） |
| 后端语言 | Go 1.25 |
| 前端框架 | Vue 3.4 + Element Plus 2.7 + Cesium 1.144 |
| 数据库 | MySQL 8.0（主）/ SQLite（测试） |
| 消息中间件 | EMQX 5.0（MQTT） |
| 缓存 | Redis 7.0 |
| 部署方式 | Docker Compose |
| 代码规模 | 后端约 19,700 行（95 个非测试源文件）；前端 37 个 Vue 组件 + 39 个 TS 模块 |
| 审核基准分支 | `main`（commit `4963248`） |

---

## 3. 功能完整性审核

### 3.1 后端 API 覆盖度

项目共注册约 **110 个 API 端点**，覆盖以下业务模块：

| 模块 | 端点数（估）| 状态 |
|------|------------|------|
| 认证（登录/注册/验证码） | 3 | ✅ |
| 设备管理（CRUD + 统计） | 12 | ✅ |
| 路口 / 区域 / 地图 | 8 | ✅ |
| 检测器接入（硬件/Mock/CSV） | 4 | ✅ |
| 自动巡检（任务/记录/排行） | 9 | ✅ |
| 预警管理 + 规则配置 | 7 | ✅ |
| 故障管理 + 证据 + 工单 | 14 | ✅ |
| 案例库 + 故障识别 | 6 | ✅ |
| 仪表盘统计 | 6 | ✅ |
| 日志查询 | 3 | ✅ |
| 用户 / 部门 / RBAC | 10 | ✅ |
| 媒体 / 固件 / 固件升级 | 8 | ✅ |
| 库存 / 成本 / 供应商 / 采购 | 10 | ✅ |
| 反馈 / 通知 | 5 | ✅ |
| AI 模块（预测/诊断/报告等） | ~25 | ✅ |
| 代理（高德/百度地图） | 2 | ✅ |

### 3.2 前端页面覆盖度

共 37 个 Vue 页面组件，覆盖所有主要业务场景：仪表盘、地图大屏、设备/路口管理、故障/工单、巡检、AI 工作台、库存四级页、系统设置、检测器接入等。

### 3.3 残留技术债务

通过对全仓库 Go 与 Vue/TS 源文件的 grep 扫描：

- **TODO / FIXME / HACK / PLACEHOLDER 注释**：**0 条**
- 功能完整性：✅ **通过**

---

## 4. 测试覆盖审核

### 4.1 后端测试

| 指标 | 值 |
|------|-----|
| Go 测试文件数 | **81 个** |
| 覆盖率门禁（CI） | ≥ 80%（`make coverage-check COV_THRESHOLD=80`） |
| CI 触发条件 | push/PR 至 `packages/server/**` + 手动触发 |
| CI 步骤 | gofmt → go vet → go build → go test → 覆盖率检查 |

覆盖范围：handler（约 45 个文件）、AI 模块（13 个）、model（8 个）、mqtt（5 个）、service（4 个）及其他辅助模块。

### 4.2 前端测试

| 指标 | 值 |
|------|-----|
| 前端测试文件数 | **1 个**（`format.test.ts`，工具函数） |
| 前端 CI 集成 | ❌ 未纳入 CI |
| Vitest 配置 | 存在，但未自动运行 |

> ⚠️ **观察项 OBS-1**：前端测试仅覆盖格式化工具函数，核心业务页面（仪表盘、地图大屏、AI 工作台等）无自动化测试，且未纳入 CI 流水线。建议后续迭代中补充端到端测试（Playwright/Cypress）。

### 4.3 测试结论

后端测试体系健全，CI 门禁有效。前端测试薄弱，但属于已知技术债务，不构成本次发布的阻断项。

**测试覆盖：✅ 通过（后端达标，前端为观察项）**

---

## 5. 代码质量审核

### 5.1 代码规模与结构

| 指标 | 值 |
|------|-----|
| 后端非测试源文件 | 95 个，约 19,700 行 |
| 前端 Vue 组件 | 37 个 |
| 前端 TS 模块 | 39 个 |
| 整体架构 | 分层清晰（handler → service → model），依赖注入合理 |

### 5.2 异常处理

| 检查项 | 结果 |
|--------|------|
| 非测试文件中的 `panic()` | 4 处，均属合理场景（见下） |
| 非 main.go 的 `log.Fatal` / `os.Exit` | 0 处 |

**panic 详情**：
- `cmd/licensegen/main.go:32` — 私钥格式错误，工具程序，合理
- `internal/model/db.go:75,80,84` — 在 `InitTestDB()` 内（测试辅助函数，虽文件名非 `_test.go`，但仅供测试调用），合理

### 5.3 依赖健康度

主要依赖均为近期活跃维护版本（gin 1.9.1、gorm 1.30.0、go-redis v9.5.1、jwt v5.2.2）。

> ⚠️ **观察项 OBS-2**：`packages/server/Dockerfile` 的 builder 阶段使用 `golang:1.22-alpine`，而 `go.mod` 声明 `go 1.25.0 / toolchain go1.26.6`。版本不匹配将导致 `docker build` 失败（Go 工具链会拒绝降级编译）。

**代码质量：✅ 通过（OBS-2 为待修正项，不阻断发布但须在构建前修复）**

---

## 6. 安全审核

本节报告由独立安全子任务完成，置信度阈值 ≥ 8/10。

### 6.1 高危漏洞

#### BLOCK-1：超管账号凭据硬编码于源代码
- **文件**：`packages/server/internal/model/seed.go:111-112`
- **严重性**：🔴 高危（Confidence 10/10）
- **描述**：超管账号用户名与初始密码以 Go 导出常量形式硬编码：
  ```go
  SuperAdminUsername = "419116"
  SuperAdminPassword = "Osgis!!!"
  ```
  `SeedSuperAdmin()` 在每次数据库迁移时自动调用（`migrate.go:118`），必然将此账号写入任何新部署的数据库。密码虽经 bcrypt 入库，但明文仍可从源码或编译产物中提取。
- **利用场景**：任何能读取源代码（含 git 历史）的人员可直接以超管身份登录，获得完整的 RBAC 管理权限，包括 `module:manage`、AI API key 替换等高权限操作。
- **修复方案**：参照 `ADMIN_INIT_PASSWORD` 的处理方式，从环境变量（如 `SUPER_ADMIN_PASSWORD`）读取初始密码；若未配置则用 `crypto/rand` 生成随机密码并一次性打印到 stdout。将 `SuperAdminUsername` / `SuperAdminPassword` 常量从代码中删除。

#### BLOCK-2：docker-compose.yml 中 JWT Secret 为弱值
- **文件**：`deploy/docker-compose.yml`
- **严重性**：🔴 高危（Confidence 9/10）
- **描述**：compose 文件中 `JWT_SECRET` 被设置为 `tsloms-dev-secret`，这是一个可公开预测的字符串。任何能访问此 compose 文件的人（包括 git 仓库协作者）可用此 secret 伪造任意用户的合法 JWT，绕过全部认证保护。
- **利用场景**：攻击者构造 JWT，设置 `user_id=1`（admin）或 `user_id=2`（super_admin），用 `tsloms-dev-secret` 签名，即可以最高权限访问所有 API。
- **修复方案**：将 compose 文件中的 `JWT_SECRET` 值替换为占位符（如 `${JWT_SECRET}`），并在 `.env.example` 中说明生成方式；在 `README` / 部署文档中明确要求生成至少 32 字节随机密钥。

### 6.2 中危 / 已缓解项

| 项目 | 状态 | 说明 |
|------|------|------|
| JWT 弱默认值（`config.go:55`） | ⚠️ 已有缓解 | `APP_ENV=production` 时 `main.go` 会拒绝弱 secret 启动，但非生产环境无拦截 |
| 数据库默认凭据（`config.go`） | ⚠️ 已有缓解 | 同上，生产检查已覆盖，但 compose 文件未覆盖此逻辑 |
| v-html 渲染（仪表盘） | ✅ 已排除 | 渲染值为服务端整数统计，非用户输入，不可利用 |
| math/rand 用于密码生成 | ✅ 已排除 | Go 1.20+ 自动随机种子，Confidence 6/10，低于阈值 |
| 文件上传 MIME 检查 | ✅ 已排除 | 仅静态存储不执行，无 RCE 路径 |

### 6.3 安全结论

**安全：❌ 未通过 — BLOCK-1、BLOCK-2 必须修复后方可发布**

---

## 7. 文档完整性审核

### 7.1 现有文档（docs/ 目录，27 个文件）

| 类别 | 文件 | 状态 |
|------|------|------|
| 产品需求文档（PRD） | v1.0 ～ v4.3（6 个） | ✅ |
| 系统架构报告（SAR） | v1.0 ～ v6.2（9 个）+ 本文 v7.0 | ✅ |
| 操作手册 | v1.1、v2.0 | ✅ |
| 设备协议 / AI 功能清单 | 已有 | ✅ |
| 部署故障排查 | 已有 | ✅ |
| 审计报告 | 已有 | ✅ |

### 7.2 缺失文档

| 缺失项 | 影响 | 优先级 |
|--------|------|--------|
| **API 文档（Swagger/OpenAPI）** | 前后端联调、第三方集成困难 | 中 |
| **CHANGELOG / RELEASE_NOTES** | 版本差异追踪困难 | 低 |
| **前端组件文档** | 二次开发成本高 | 低 |

> ⚠️ **观察项 OBS-3**：项目无 Swagger 注解或 OpenAPI 文档，建议在下一版本迭代中通过 `swaggo/swag` 自动生成。

**文档完整性：⚠️ 部分通过（功能性文档完整，API 规范文档缺失属后续改进项）**

---

## 8. 部署与运维就绪度审核

### 8.1 容器化

| 检查项 | 状态 |
|--------|------|
| Dockerfile 存在 | ✅ |
| docker-compose.yml 存在 | ✅ |
| 健康检查端点（`/api/v1/health`） | ✅ |
| 多服务编排（MySQL/Redis/EMQX/Server） | ✅ |
| `.env.example` 提供 | ✅ |

**Dockerfile Go 版本不匹配（OBS-2）**：`Dockerfile` 使用 `golang:1.22-alpine`，`go.mod` 要求 `go 1.25`，需将 builder 镜像更新为 `golang:1.25-alpine`（或更高）。此问题将直接导致 `docker build` 失败，**须在发布前修复**。

### 8.2 数据库迁移

- 使用 GORM `AutoMigrate` 在启动时自动执行，兼容性由 `legacy_migrate.go` 保障
- 无版本化迁移文件（如 Flyway / golang-migrate），升级时无法精确控制 schema 变更顺序

> ⚠️ **观察项 OBS-4**：生产环境建议引入版本化迁移工具（如 `golang-migrate`）替代纯 AutoMigrate，以避免升级时不可预期的 schema 变更。

### 8.3 环境配置

`.env.example` 内容完整，覆盖所有关键配置项，并对密钥强度有明确说明。`ADMIN_INIT_PASSWORD` 支持留空随机生成，配置体验良好。

### 8.4 可观测性

- 使用 `uber-go/zap` 结构化日志 ✅
- 健康检查端点 ✅
- 无 Prometheus/metrics 端点（属于后续增强项）

**部署就绪度：⚠️ 部分通过（OBS-2 Dockerfile 版本不匹配须修复，其余为观察项）**

---

## 9. 发现问题汇总

### 阻断项（Blocking Issues）— 发布前必须修复

| ID | 位置 | 描述 | 严重性 |
|----|------|------|--------|
| **BLOCK-1** | `packages/server/internal/model/seed.go:111-112` | 超管账号明文密码硬编码为源代码常量 | 🔴 高危 |
| **BLOCK-2** | `deploy/docker-compose.yml` | JWT_SECRET 设置为可预测弱值 `tsloms-dev-secret` | 🔴 高危 |

### 高优先级问题（须在发布前或首次生产部署前修复）

| ID | 位置 | 描述 |
|----|------|------|
| **HIGH-1** | `packages/server/Dockerfile` | builder 使用 `golang:1.22-alpine`，与 go.mod 的 `go 1.25` 不兼容，将导致 `docker build` 失败 |

### 观察项（Observations）— 建议后续迭代改进

| ID | 描述 | 建议时机 |
|----|------|----------|
| OBS-1 | 前端自动化测试仅 1 个工具函数测试，未纳入 CI | 下一迭代 |
| OBS-2 | 同 HIGH-1（Dockerfile 版本） | 发布前 |
| OBS-3 | 无 Swagger/OpenAPI 文档 | 下一迭代 |
| OBS-4 | 数据库迁移依赖 AutoMigrate，无版本化控制 | 下一迭代 |

---

## 10. 发布建议

### 必须执行（发布前）

1. **修复 BLOCK-1**：删除 `seed.go` 中的 `SuperAdminPassword` 常量，改从环境变量 `SUPER_ADMIN_PASSWORD` 读取；未配置时生成随机密码并打印一次到 stdout。同时更新 `.env.example` 和部署文档。

2. **修复 BLOCK-2**：将 `deploy/docker-compose.yml` 中 `JWT_SECRET: tsloms-dev-secret` 改为 `JWT_SECRET: ${JWT_SECRET}`，并要求部署方在 `.env` 文件中配置强密钥（≥32 字节随机值）。

3. **修复 HIGH-1**：将 `packages/server/Dockerfile` 第一行 `FROM golang:1.22-alpine` 改为 `FROM golang:1.25-alpine`（或与 go.mod toolchain 一致的版本）。

### 推荐执行（下一迭代）

4. 为前端核心业务流程添加端到端测试，并纳入 CI。
5. 使用 `swaggo/swag` 自动生成 Swagger/OpenAPI 文档。
6. 创建 `CHANGELOG.md`，建立规范化的版本记录机制。
7. 引入版本化数据库迁移工具（`golang-migrate`）。

---

## 11. 审核结论

> **TSLOMS v7.0 — 有条件通过（Conditional Pass）**
>
> 项目功能完整（110+ API，37 个前端页面，无 TODO 残留），后端测试体系健全（81 个测试文件，CI 覆盖率门禁 ≥80%），代码结构清晰，文档体系完善。
>
> **发布必要条件**：完成 BLOCK-1（超管密码硬编码）、BLOCK-2（compose 弱 JWT Secret）和 HIGH-1（Dockerfile 版本不匹配）三项修复后，可正式发布。
>
> 三项修复均为局部、低风险的配置/代码调整，预计工作量不超过 2 小时。

---

*本报告由 Claude Code 自动生成 | 审核基准：commit `4963248` | 审核日期：2026-08-17*
