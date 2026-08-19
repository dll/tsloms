# PM 核对清单：数据库版本化迁移（CD-P0-01）

> 角色：pm-tsloms（需求核对专员，只读）
> 任务：核对 `docs/TSLOMS-流水线CICDCO-v3.md` 的 **CD-P0-01** 与 `packages/server/internal/model/db.go`、`migrate.go`（及关联 file）的「数据库版本化迁移」重构范围。
> 基线文件：`db.go`、`migrate.go`、`seed.go`、`legacy_migrate.go`、`workorder.go`。
> 治理基线：仓库 `main`（v3 报告基线 `f9f1dfc`，AutoMigrate 仍在启动路径）。
> 结论性质：只读核对清单，**未修改任何源码**。

---

## 0. 问题还原（CD-P0-01 要求什么）

当前事实（来自代码）：

- `db.go::InitDB` 在建立连接后 **无条件调用 `AutoMigrate(db)`**（MySQL 生产 / SQLite 开发共用同一条路径）。
- `migrate.go::AutoMigrate` 一次把「**全量 GORM AutoMigrate（38 张表）** + **数据迁移/清理** + **种子**」全部压进启动路径。
- v3 报告指出的核心缺口：
  - 二进制可 `git revert`/回切，但 **DB 结构/数据不可随二进制回滚**；
  - `release-install.sh` 的 `mysqldump` 备份可选（缺 env / DB_NAME / DB_PASSWORD 只 WARN 继续）；
  - 无迁移版本表、无单实例锁、无超时、无失败补偿、无恢复演练；
  - **探活成功 ≠ 迁移安全**。

v3 CD-P0-01 整改目标（须达到）：
1. 迁移改为**显式版本化 migration**，**不在普通服务启动时隐式改库**；
2. 迁移前备份缺失/失败必须 **fail-closed**；
3. 数据库/分布式锁保证**单实例迁移**；
4. 大表迁移**超时**、向前兼容策略、回滚/恢复 runbook；
5. 至少一次真实**备份恢复 + 迁移失败演练**并保存时间线。

---

## 1. AutoMigrate 当前包含的全部内容

### 1.1 结构迁移（GORM 全量 AutoMigrate 创建/更新的表，共 38 个模型）

按 `migrate.go` 传入顺序：

| # | 模型 | 表 | 说明 |
|---|------|----|------|
| 1 | Device | devices | 设备 |
| 2 | PacketLog | packet_logs | 报文日志 |
| 3 | FaultRecord | fault_records | 故障 |
| 4 | FaultEvidence | fault_evidences | 故障证据 |
| 5 | FaultCase | fault_cases | 故障案例 |
| 6 | WorkOrder | work_orders | 工单（**含 fault_active_scope 派生列，唯一索引由 migrate 手动建**） |
| 7 | User | users | 用户 |
| 8 | Department | departments | 部门 |
| 9 | OperationLog | operation_logs | 操作日志 |
| 10 | DeviceMedia | device_media | 设备媒体 |
| 11 | Feedback | feedbacks | 反馈 |
| 12 | AIConfig | ai_configs | AI 配置 |
| 13 | AIUsage | ai_usages | AI 用量 |
| 14 | AIPrediction | ai_predictions | AI 预测 |
| 15 | FirmwarePackage | firmware_packages | 固件包 |
| 16 | FirmwareUpgradeRecord | firmware_upgrade_records | 固件升级记录 |
| 17 | Material | materials | 物料档案 |
| 18 | MaterialStock | material_stocks | 物料库存流水 |
| 19 | Supplier | suppliers | 供应商 |
| 20 | PurchaseOrder | purchase_orders | 采购单 |
| 21 | PurchaseOrderItem | purchase_order_items | 采购单明细 |
| 22 | RepairExpense | repair_expenses | 维修费用 |
| 23 | AIReport | ai_reports | AI 报告 |
| 24 | AIAdvice | ai_advices | AI 建议 |
| 25 | Permission | permissions | 权限字典 |
| 26 | Role | roles | 角色 |
| 27 | RolePermission | role_permissions | 角色-权限 |
| 28 | UserPermission | user_permissions | 用户-权限覆写 |
| 29 | Notification | notifications | 通知 |
| 30 | NotificationRead | notification_reads | 通知已读 |
| 31 | Warning | warnings | 预警（P0） |
| 32 | WarningRule | warning_rules | 预警规则（P0） |
| 33 | Area | areas | 行政区划（P0） |
| 34 | Crossing | crossings | 路口（P0） |
| 35 | PatrolTask | patrol_tasks | 巡检任务（P1） |
| 36 | PatrolRecord | patrol_records | 巡检记录（P1） |
| 37 | ModuleToggle | module_toggles | 模块开关 |
| 38 | LicenseState | license_states | 授权/试用状态 |

> 注意：**`device_materials` 不在 AutoMigrate 列表**。它是历史遗留表，仅在 `MigrateLegacyDeviceMaterials` 中检测 → 合并 → **删除**（全新库直接建旧表也会被删）。AutoMigrate 之外的单独结构变更只有它。

### 1.2 数据迁移 / 数据清理（AutoMigrate 内、结构迁移之后）

在 `migrate.go` 的 `AutoMigrate(db)` 内部顺序执行：

1. **状态升级 active → occurred**
   `db.Model(&FaultRecord{}).Where("status = ?", "active").Update("status", FaultStatusOccurred)`
   （四态模型引入后的历史数据升级。）
2. **`migrateWorkOrderActiveUnique(db)`**（`migrate.go` 内独立函数）
   - 若 `uk_wo_active_scope` 索引已存在 → 幂等跳过；
   - 否则：a) 清理同 `fault_id` 多条活跃工单（仅留 `MAX(id)`，其余置 `rejected` 并追加 `[系统迁移清理:重复自动派单]`）；b) 为活跃工单回填 `fault_active_scope = fault_id`；c) `CREATE UNIQUE INDEX uk_wo_active_scope ON work_orders(fault_active_scope)`。
3. **`MigrateLegacyDeviceMaterials(db)`**（`legacy_migrate.go`）
   - 旧 `device_materials` 表合并进 `materials`：按 `part_no`/`name` 作 code，分类推断，`device_hw_id` 从 uint32 转大写十六进制 UUID，写初始库存流水（`material_stocks`、`type=in`、note=`旧耗材台账合并初始库存`），同名物料只补设备绑定；**迁移完成后 `DropTable("device_materials")`**。
   - 幂等语义：同名物料已存在则跳过。

### 1.3 种子（seed）

4. **`SeedRBAC(db)`**（`seed.go`）
   - 权限字典按 `code` 幂等插入（缺则建，已存在则同步 name/module/sort）。
   - 内置角色 super_admin / admin / operator / viewer（缺则建）。
   - 角色默认权限**先删后插** `role_permissions`（保证与代码一致）。
5. **`SeedSuperAdmin(db, config.Get().SuperAdminPwd)`**（`seed.go`）
   - 账号 `419116`（username & phone_login），角色 super_admin；
   - 仅在 419116 不存在时创建；密码取 `SUPER_ADMIN_PASSWORD`，未配置则**生成随机强密码并返回打印一次**（审计 BLOCK-1：不硬编码明文）。
   - 返回值 `saPwd`；非空时 `migrate.go` 内 `log.Printf("[TSLOMS] 超级管理员账号 %s 已创建，初始密码…… 打印一次"`。
6. **`SeedAreas(db)`**（`migrate.go`）
   - 仅当 `areas` 表为空时写入最小层级示例（安徽省→合肥市→庐阳区→三孝口/逍遥津街道→龚湾社区→长江中路/宿州路）。幂等。

---

## 2. 迁移/种子性质分类（幂等可重放 vs 一次性有副作用）

> 结论先行：**当前 AutoMigrate 混装了「每启动重放安全」与「仅执行一次/有副作用」两类**。P0 版本化改造的核心就是把它们拆开。

| 迁移/种子 | 幂等可重放？ | 一次性/有副作用？ | 必须进版本化迁移？ |
|---|---|---|---|
| GORM `AutoMigrate(38 表)` 创建表/加列 | ✅ 补列加表天然幂等 | ⚠️ 不会删列/不会删表，历史遗留列永远残留；`fault_active_scope` 列有 `gorm:"index"` tag | **结构这一层**：建议进「首个版本化迁移 U1」（全量 ensure schema）；不作为「数据迁移」版本 |
| 状态升级 `active→occurred` | ✅ 全量 UPDATE 幂等（已 occurred 者不再匹配） | 无 | 直接迁移（低风险可重放） |
| `migrateWorkOrderActiveUnique`：清理重复活跃单 + 回填 scope + `CREATE UNIQUE INDEX` | ⚠️ 回填/清理幂等；但**索引创建**必须 `HasIndex` 前置判断（已做） | ⚠️ **`CREATE UNIQUE INDEX` 有副作用**：若历史数据重复活跃未清场会**失败**；索引一旦存在即修改 DB 约束，之后的业务并发派单依赖它 | ✅ **必须版本化**，且**失败即 fail-closed 阻断启动** |
| `MigrateLegacyDeviceMaterials` | ❌ **非幂等**：结束会 `DropTable("device_materials")`；删表是一次性 | ⚠️ **删表有副作用**、数据搬家后不可在同一库重放 | ✅ **必须版本化，且严格一次**。只应在检测到旧表存在且完成合并后执行一次，之后标记 `1` 永久跳过 |
| `SeedRBAC` | ✅ 幂等（按 code 找、角色先删后插 role_permissions） | 无（做加法/同步） | 可保留为**纯幂等启动逻辑**，不必版本化（但建议放入 U1 之后的 startup 引导，与迁移隔离） |
| `SeedSuperAdmin`（含随机密码打印） | ✅ 账号存在即幂等跳过 | ⚠️ **首次创建生成随机密码并打印到日志——有副作用/信息暴露**。重复调用不会触发，但属于「一次性初始化动作」 | 建议封装为**一次性初始化步骤**（失败可重试但不应在每次启动刷打印）；生成密码打印需放安全渠道 |
| `SeedAreas` | ✅ 表非空即跳过 | 无 | 可保留为纯幂等启动逻辑 |

### 结论矩阵（改造规划）
- **需版本化（一次性/有副作用）**：
  - `U2 建 uk_wo_active_scope 唯一索引`（含清理+回填前置）；
  - `U3 合并并删除 device_materials`；
  - `U0/种子-首次 超级管理员初始化`（若需，密码渠道化）。
- **可保留为纯幂等启动逻辑（每启动重放安全，不进版本表）**：
  - GORM `AutoMigrate`（作为 ensure-schema 基座）；
  - `active→occurred` 状态升级；
  - `SeedRBAC`、`SeedAreas`。
- **设计建议**：甚至可把「版本化迁移」限定为「结构基座 U1 + 数据一次性步骤 U2/U3」，而把幂等种子留在启动引导阶段，两者职责分开，代价最低。

---

## 3. 生产 MySQL vs 测试 SQLite 双方言差异风险

| 差异点 | MySQL（生产） | SQLite（测试/本地） | 风险与处理 |
|---|---|---|---|
| `CREATE UNIQUE INDEX` | 标准语法，索引一旦存在重复建会报错；`HasIndex` 可防 | 同标准，`HasIndex` 可用 | 已在 `migrateWorkOrderActiveUnique` 做 `HasIndex` 前置；**版本化改造必须保留** |
| 同表子查询 `UPDATE/DELETE` | **Error 1093**（不能直接从同表 SELECT 更新） | 允许 | 已用 `id NOT IN (SELECT MAX(id)...)` 规避；**版本化迁移切不可回归为直接子查询** |
| `fault_active_scope` 唯一语义 | NULL 不参与唯一，允许多条历史单 | 同 | 双端一致；建索引前必须回填/清理，否则唯一索引建立失败 |
| `DropTable` | 支持，DDL 隐式提交（**不能回滚**） | 支持 | `device_materials` 删除后不可逆 → **必须 fail-closed 备份后执行**；版本化方案需在 `migrate.device_materials_merge` 完成时记录版本标志，**防重启二次触发删表** |
| DDL 事务 | **不支持隐式提交**（无法 `ROLLBACK` DDL） | **支持事务内回滚** | 这是最大差异：**版本表 + 迁移执行不能指望 DDL 回滚**。正确做法是对 DDL「向前只加不改」，失败靠「记录 fail 状态 + 幂等重跑 + fail-closed」而非事务回滚 |
| 唯一/索引名冲突 | 索引名 `uk_wo_active_scope` 需全库唯一 | 同为库级命名空间 | 保持一致命名，避免重名 |
| `multiStatements=true`（DSN） | 已启用，可多语句 | N/A | 版本化迁移若用原生 SQL 多语句需留意解析；优先走 GORM 迁移器/逐语句 |
| 超时语义 | `LOCK`/长事务易锁表；`CREATE INDEX` 大表耗时长 | 无并发锁问题 | 生产大表建索引**必须设置超时（`SET SESSION lock_wait_timeout` / 语句超时）**，避免启动被长迁移拖死；测试无此场景，需注意两者行为差异 |
| 探活与迁移解耦 | 迁移在启动路径时探活假阳性 | —— | 版本化后迁移独立 job，探活只验服务，迁移成败靠迁移日志 + 版本表校验 |

---

## 4. 现有测试对 AutoMigrate / InitDB / InitTestDB 的依赖（改造兼容性）

### 4.1 依赖方式（已统计，覆盖面极广）
- **`InitTestDB()`** 是几乎所有测试的数据库夹具（含 db 打包测试），内部 = `gorm.Open(sqlite 内存)` + `SetMaxOpenConns(1)` + **`AutoMigrate(db)`**（当前包含全部结构+种子）。调用点超 **80 处**，分布在：`internal/ai`、`internal/caselib`、`internal/handler/*`、`internal/middleware`、`internal/mqtt`、`internal/service`、`internal/model`。
- 依赖 AutoMigrate 语义的具体测试：
  - `model/db_test.go::TestMigrateLegacyDeviceMaterials(NoData)`：**显式重建 `device_materials` 旧表后再调 `MigrateLegacyDeviceMaterials`**，断言旧表被删、物料/库存流水写入、同名跳过、设备绑定补充。**直接调用移入版本化后的单一步骤函数**，故该函数名/签名必须保留。
  - `model/rbac_test.go`：`InitTestDB()`（注释「会调用 AutoMigrate 与 SeedRBAC」）→ 断言权限/角色种子已写入。
  - `model/p0_superadmin_test.go`：`InitTestDB()` 后 `SeedSuperAdmin`，且注释「AutoMigrate 已预建超管（随机密码）」——**隐式依赖 AutoMigrate 已先跑 SeedSuperAdmin**；测试用 `db.Where(...).Delete(&User{})` 后再重建验证。
  - `model/p1_patrol_test.go::TestPatrolModel_AutoMigrateCreatesTables`：直接断言 `patrol_tasks` / `patrol_records` 表被 AutoMigrate 创建（**测试名含 AutoMigrate**）。
  - `model/workorder_test.go`：`TestEnsureActiveWorkOrder_ConcurrentConverges` 依赖 `uk_wo_active_scope` 唯一索引（注释明示）——**必须由迁移建立**，否则并发测试失效。
  - `internal/ai/anomaly_test.go`：注释「赋值全局 model.DB + AutoMigrate」。

### 4.2 改造成版本化后，测试如何兼容（结论）
- **关键设计**：**保留 `AutoMigrate(db)` 与 `InitTestDB()` 的函数形态**，仅让它们内部「调用统一基座迁移（ensure-schema + 数据一次性步骤）」。
- 推荐：
  1. `InitTestDB()` **继续直接全量 migrate**（测试不需版本表/锁/备份），即测试仍走「一次全量建表+种子」语义，几乎所有现有测试**零改动通过**。
  2. **新增独立迁移函数据 `MigrateDatabase(db)`（显式版本化）**，用作生产启动/CD 迁移 job 的入口；测试可另加 `MigrateDatabaseVersioned` 的版本表/锁/幂等单测，不破坏现有夹具。
  3. `db_test.go` 等直接调 `MigrateLegacyDeviceMaterials` 的测试**保持不变**，只要该函数签名保留。
  4. 版本化内部若把「种子」分离出迁移 job，需保证 `InitTestDB` 仍执行种子（否则 RBAC/superadmin/permission 测试会失去前置数据）。
- **风险点**：切勿把 `AutoMigrate` 改到「仅建结构、不跑种子」，否则 `rbac_test` / `p0_superadmin_test` 依赖的种子数据将缺失。**要么 `InitTestDB` 走完整启动引导，要么在迁移基座里统一 include 种子**。

---

## 5. 「最小可行版本化迁移」设计建议（供 dev-refactor 落地参考）

### 5.1 目标与边界
- **不在普通服务启动时隐式改库**：生产改为 CD 独立 migration job（或启动时仅在「版本 head == 当前」时跳过，差异检测）；开发/测试仍允许 `InitTestDB` 一键全量。
- **可回滚诉求的具体化**：由于 MySQL DDL 不可事务回滚，做到「向前只加、失败幂等重跑 + fail-closed + 强制备份」即达 P0；不强求「向下迁移 down」。

### 5.2 采用自建 version 表（不引入新依赖，风险最低）
> 无需 golang-migrate。当前结构已高度 GORM 化，自建 version 表 + 有序迁移函数列表最贴合、改造成本最小。

**新增版本元数据表：**
```
schema_migrations (
  id         INT PRIMARY KEY AUTO_INCREMENT,   -- 或 version uint
  version    VARCHAR(64) NOT NULL UNIQUE,       -- 'uint'-like: '0001_schema'
  name       VARCHAR(255),
  applied_at DATETIME,
  applied_by VARCHAR(64)                        -- 实例标识，便于审计
)
```
> 用 **`GET_LOCK('tsloms_migrate', timeout)`**（MySQL）实现单实例锁；SQLite 测试用文件/进程锁或 `BEGIN IMMEDIATE`。锁超时失败 → fail-closed 拒绝启动。

**有序迁移函数列表（顺序 + 标记）：**
| 版本 | 内容 | 性质 | 失败语义 |
|---|---|---|---|
| 0001_schema | GORM AutoMigrate 38 表（ensure-schema 基座） | 幂等 | fail-closed |
| 0002_active_scoped_index | `migrateWorkOrderActiveUnique`（清理+回填+建 uk_wo_active_scope） | **一次性 + 建约束** | fail-closed，失败置 version 表该版本为 failed/未 applied |
| 0003_device_materials_merge | `MigrateLegacyDeviceMaterials`（**含 DropTable 旧表**） | **一次性 + 删表** | **必须先强制备份**；fail-closed |
| 0004_superadmin_init | 超级管理员首建（密码渠道化，不打印明文到普通日志） | 一次性初始化 | 可重试 |

**执行器保证：**
1. 每个版本「先读 version 表 → 未 applied 才执行 → 成功写回 → 记录 applied_by/time」。
2. 每个版本独立检查幂等（如 `HasIndex`、`HasTable(device_materials)`）。
3. fail-closed 规则：任一版本失败 → **整体标记失败，进程退出/迁移 job 失败**，且绝不继续后续版本、绝不进入正常服务启动。
4. **锁**：MySQL 用 `GET_LOCK`；超时（默认如 300s，大表可配）失败即阻断。
5. **备份前置**：凡含 `DropTable`/DDL 的版本，执行前必须 `mysqldump`/备份成功；备份缺失或失败 → fail-closed（补齐 CD-P0-01 的「备份缺失必须失败」）。
6. **探活与迁移解耦**：健康接口只探服务存活；迁移成败以版本表 + 迁移 job 退出码为准（新增 CO 层面「版本 head 校验」巡检项）。

### 5.3 哪些启动逻辑保留不动、哪些封装成显式版本化

**保留为纯幂等启动引导（不动，仍可在启动时跑，安全）：**
- GORM `AutoMigrate` 的 **ensure-schema 基座能力**（可用 0001_schema 等价封装，避免启动重复全量 DDL；二选一即可）。
- `active→occurred` 状态升级（全量 UPDATE 幂等，重放安全）。
- `SeedRBAC`（幂等，按 code 同步）。
- `SeedAreas`（幂等，表非空跳过）。

**封装成显式版本化步骤（一次性/有副作用，不得隐式随启动重放）：**
- `migrateWorkOrderActiveUnique`（`CREATE UNIQUE INDEX`，历史清理非幂等→建约束有副作用）→ **0002**。
- `MigrateLegacyDeviceMaterials`（`DropTable` 删表，不可重放）→ **0003**。
- `SeedSuperAdmin` 首次建号 + 生成/打印密码（一次性初始化 + 信息暴露）→ **0004**（密码走安全渠道）。

### 5.4 对 dev-refactor 的落地注意（承接到「refactor-notes」）
- 保留 `AutoMigrate`、`InitTestDB`、`migrateWorkOrderActiveUnique`、`MigrateLegacyDeviceMaterials`、`SeedRBAC`、`SeedSuperAdmin`、`SeedAreas` 的**既有签名/语义**，避免破坏现有 80+ 处测试。
- `InitTestDB` 仍可「直接全量 migrate」；生产走新 `MigrateDatabaseVersioned(db)` 版本化入口。
- 新增 `schema_migrations` 表 + `GET_LOCK` 单实例锁 + 超时 + fail-closed 逻辑，属**新增**，不影响既有表清单。
- 迁移失败演练（备份恢复、模拟 create index 失败、模拟 device_materials_merge 中断）须留 timeline 记录，纳入审计（v3 §7.1 P0-01 验收证据）。

---

## 6. 风险清单（改造需在设计中显式处理）

| # | 风险 | 说明 | 建议 |
|---|---|---|---|
| R1 | DDL 事务语义差异 | MySQL 不可回滚、SQLite 可回滚 | 版本化不做「down 回滚」，仅 fail-closed + 幂等重跑 + 备份恢复 |
| R2 | 唯一索引建立失败 | 历史重复活跃工单未清场 | 保 0002 前置清理；失败阻断启动 |
| R3 | device_materials 二次删表 | 重启后重复触发 DropTable | 版本记录 + `HasTable` 检查；0003 严格一次 |
| R4 | 长迁移阻塞 | 大表 CREATE UNIQUE INDEX 慢 | 超时 + 后台迁移 job + 锁释放机制 |
| R5 | 探活假阳性 | 只验服务存活 | 探活解耦；CO 加版本 head 巡检 |
| R6 | 备份缺失仍继续 | CD-P0-01 明确要求 fail-closed | 备份前置强制校验 |
| R7 | 密码打印暴露 | SeedSuperAdmin 首次随机密码 log.Printf | 渠道化 + 不再普通日志明码 |
| R8 | 测试数据前置丢失 | 若种子脱离 AutoMigrate | InitTestDB 必须仍含种子或走启动引导 |
| R9 | 版本表/锁破坏 SQLite 测试 | 测试无 MySQL GET_LOCK | 锁抽象方言化；测试用简化锁或直接全量 |

---

## 7. 交接项（供 dev-refactor 实施，均不改业务逻辑）
- 目标：把「结构基座（幂等）」与「一次性副作用迁移 + 强制备份 + 单实例锁 + 超时 + fail-closed」解耦。
- 交付物（供 refactor-notes 承载）：
  1. `MigrateDatabaseVersioned(db)` 显式版本化入口；
  2. `schema_migrations` 版本表 + 有序迁移列表（0001~0004）；
  3. MySQL `GET_LOCK` 单实例锁 + 超时；SQLite 测试简化路径；
  4. 备份 fail-closed 前置（DDL/DropTable 版本）；
  5. 保留 `InitTestDB` 一键全量语义；
  6. 密码/种子渠道化，杜绝启动路径隐式 DDL。
- **禁止**：修改任何现有业务表结构、任何业务常量/字段、删除 `AutoMigrate`/`InitTestDB` 对外签名、使现有任意测试因缺少种子/索引而失败。

> 本清单只做核对与建议，未改动 `packages/server/internal/*` 任何源码，未 push。
