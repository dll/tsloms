# QA Report — CD-P0-01 数据库版本化迁移回归

- **任务**: CD-P0-01「数据库版本化迁移」回归测试
- **提交**: `7603076`（dev 重构：`db.go::InitDB/InitTestDB` 由无条件 AutoMigrate → `MigrateDatabaseVersioned` 显式版本化迁移）
- **新增文件**: `packages/server/internal/model/versioned_migrate.go` / `versioned_migrate_test.go`
- **QA 执行人**: qa-regression 子代理
- **日期**: 2026-08-19
- **执行环境**: `packages/server`（Windows / pwsh / go v1.x via local toolchain）
- **结论**: ✅ **未发现回归**（14 个含测试的包全绿，无 FAIL）

---

## 1. 执行命令与结果

在 `packages/server` 下依次执行：

| 命令 | 结果 |
|------|------|
| `go build ./...` | ✅ 通过（BUILD_EXIT=0） |
| `go vet ./...`   | ✅ 通过（VET_EXIT=0） |
| `go test ./... -count=1` | ✅ 全部通过（TEST_EXIT=0） |

重新添加 QA 回归用例后再跑一遍 `go build ./...` / `go vet ./...` / `go test ./... -count=1`，仍全部通过。

### 逐包结果（`go test ./... -count=1`）

共 **15 个包**清单中，**14 个含测试用例的包全部 `ok`（PASS，无 FAIL）**；`cmd/licensegen` 无测试文件（`[no test files]`，属正常，非失败）。

| 包 | 结果 | 耗时(约) |
|----|------|---------|
| `cmd/coveragecheck` | ✅ ok | 49.9s |
| `cmd/licensegen` | ⚪ 无测试文件 | — |
| `cmd/server` | ✅ ok | 20.9s |
| `internal/ai` | ✅ ok | 38.4s |
| `internal/caselib` | ✅ ok | 16.8s |
| `internal/config` | ✅ ok | 8.1s |
| `internal/faultcode` | ✅ ok | 10.4s |
| `internal/handler` | ✅ ok | 69.7s |
| `internal/license` | ✅ ok | 13.2s |
| `internal/logger` | ✅ ok | 5.7s |
| `internal/middleware` | ✅ ok | 9.2s |
| `internal/model` | ✅ ok | 14.9s |
| `internal/mqtt` | ✅ ok | 13.7s |
| `internal/recognition` | ✅ ok | 7.5s |
| `internal/service` | ✅ ok | 6.9s |

**无任何 `FAIL` / `panic`。**（日志中出现的 `TestResponse_failHelpers`、`TestValidate_PartialChannelPowerLossPanic` 为测试名，其断言均为 `--- PASS`。）

---

## 2. 重点回归：依赖数据库迁移语义的既有测试

以下既有测试全部 **PASS**，确认重构后业务语义未退化：

| 用例 | 文件 | 结果 | 说明 |
|------|------|------|------|
| `TestMigrateLegacyDeviceMaterials` / `...NoData` | `db_test.go` | ✅ PASS | 旧表 `device_materials` 合并入 `materials` / 空表删表（版本 0003） |
| `TestSeedRBAC` / `TestSeedRBAC_Idempotent` | `rbac_test.go` | ✅ PASS | SeedRBAC 种子（权限字典 + 4 内置角色） |
| `TestSeedSuperAdmin_AccountCreated` + `_PasswordValidates` + `_GeneratedRandom` | `p0_superadmin_test.go` | ✅ PASS | 超管 `419116` 首建（版本 0004），bcrypt 加密、幂等 |
| `TestPatrolModel_AutoMigrateCreatesTables` | `p1_patrol_test.go` | ✅ PASS | patrol 表经版本 0001 结构基座自动建表 |
| `TestEnsureActiveWorkOrder_ConcurrentConverges` | `workorder_test.go` | ✅ PASS | `uk_wo_active_scope` 唯一索引并发收敛（版本 0002） |
| handler/service 依赖 `InitTestDB` 的用例（抽样） | `internal/handler`、`internal/service` | ✅ PASS | 两包全绿，说明 InitTestDB 一键建表+种子语义对 80+ 处依赖保持兼容 |

---

## 3. 新增回归守护用例（QA 添加）

dev 自带的 `versioned_migrate_test.go` 已含幂等/版本记录/FreshDB 抽样三例。**QA 额外新增独立守护文件** `packages/server/internal/model/qa_versioned_migrate_regression_test.go`（仅新增测试文件，未改动任何实现/业务源码），含 4 个回归守护用例，全部 PASS：

| 用例 | 守护点 |
|------|--------|
| `TestQA_VersionedMigrate_IdempotentNoSideEffect` | 二次执行严格幂等：版本数不再增长、超管不重复创建、区划/角色种子不重复（比 dev 更进一步断言业务数据无副作用） |
| `TestQA_VersionedMigrate_AppliedRecordsComplete` | 版本表 `applied` 记录字段完整（`applied_by` / `applied_at` / `version` / `name`），版本唯一且顺序与 `orderedMigrations` 对齐 |
| `TestQA_VersionedMigrate_FreshDB_All38Tables` | 全新库一键迁移后 **38 张业务表全量逐张断言建表**（比 dev 抽样式更全面），`schema_migrations` 版本表亦存在 |
| `TestQA_InitTestDB_SeedSemanticsUnchanged` | **业务红线**：`InitTestDB()` 一键全量建表+种子语义与重构前一致（RBAC 权限字典/4 角色、超管首建、区划种子、`uk_wo_active_scope` 索引俱在） |

> 说明：dev 自带的三例与 QA 新增四例互补、不重复：QA 侧增加了「二次执行无业务副作用」「38 表全量逐张」「InitTestDB 种子语义红线」等更严格的断言视角。

---

## 4. 业务红线核查：`InitTestDB()` 语义一致性

**通过。** 重构后 `InitTestDB()` 仍走 `MigrateDatabaseVersioned`（SQLite 简化无锁无备份路径），产物与重构前 `AutoMigrate` 等价：

- 38 张业务表全部可建表（QA 守护用例③全量断言通过）；
- `schema_migrations` 版本表独立存在；
- `uk_wo_active_scope` 唯一索引（版本 0002）已建立；
- 种子俱在：RBAC 权限字典、4 内置角色、超管 `419116` 首建、区划 `SeedAreas`。

判定依据：`TestSeedRBAC`、`TestSeedSuperAdmin_AccountCreated`、`TestEffectivePermissions` 等依赖种子的测试均 PASS；QA 守护用例④直接断言 `InitTestDB()` 返回后种子数据存在。现有 80+ 处测试依赖该语义的用例（handler/service 全包绿）无退化。

---

## 5. 发现的问题

**未发现回归。** 所有执行命令、重点回归用例、新增守护用例均通过；无 FAIL、无 panic、无断言失败。

备注（非缺陷，仅供知悉）：
- `MigrateDatabaseVersioned` 在 SQLite 测试路径跳过 DDL 备份，MySQL 生产路径对含 DDL/DropTable 版本（0002/0003）强制备份 fail-closed——本次回归为 SQLite 内存库，未触及真实 MySQL 备份 syscall 分支；相关失败场景建议由独立 MySQL 集成冒烟覆盖（不在本次 `go test` 范围内）。
- 迁移日志中的 `[TSLOMS][migration:0004] 超级管理员...初始密码...` 为 `log.Printf` 一次性密码打印（审计 BLOCK-1 设计），属预期行为，本环境未配置密码故打印随机密码。

---

## 6. 涉及文件

- **新增（QA）**：`packages/server/internal/model/qa_versioned_migrate_regression_test.go`
  、`qa-report-db-migration.md`
- **未改动**：`versioned_migrate.go` / `db.go` / `migrate.go` 及其它任何实现/业务源码。
- **git status（本会话产物）**：新增以上两个文件（`??`）。另检测到工作区存在一处**非本会话产生**的既有未提交改动 `deploy/scripts/release-install.sh`（P0-03 systemd 单元校验内容，与数据库迁移任务无关，QA 未触碰）。本会话无其它文件污染；测试过程临时日志 `_qa_test_full.log` 已清理（且已被 .gitignore 覆盖）。
- 未 push。
