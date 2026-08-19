# v3 审核报告 — 7603076..5e9dfaa 流水线整改

- **审核人**: reviewer-audit-tsloms（子代理，只读评审）
- **日期**: 2026-08-19
- **范围**: 提交 `7603076`（dev: CD-P0-01 版本化迁移）→ `5e9dfaa`（CI-P1-02 私钥治理），含中间流水线整改提交
  （`ea518d3` `b164393` `a1a3a90` `5e78ac6` `282ef0b` `6b08e4f` `73aded2` `af29692` `3f943de`）
- **对照基线**: `docs/TSLOMS-流水线CICDCO-v3.md`（CD-P0 / CI-P / CO-P 要求）
- **约束遵守**: 全程只读，未修改任何源码；仅新增本报告 `audit-report-v3.md` 于仓库根；
  `git status` 复核工作区 clean（除本报告外无任何改动）；未 push。

---

## 0. 评审结论（总体）

| 项 | 结论 |
|----|------|
| **总体** | **有条件合入（CONDITIONAL REJECT/有条件通过）** |
| 达标 P0/P1 | CD-P1-01、CD-P1-02、CD-P1-04、CI-P1-01、CI-P1-02（大部分）、CI-P1-03、CI-P2-01、CO-P1-02、CO-P1-01（大部分） |
| **BLOCK** | **CD-P0-01 GET_LOCK 会话级锁在连接池上失效（无法保证单实例迁移）** |
| **HIGH** | 迁移备份目标路径落在不可变 release 目录内（`./releases/backups/db`）；mysqldump 密码明文出现在命令行；手动部署 run_id 用 `startswith("CI")`+`head -1` 可能选错 workflow |
| **MED** | `active→occurred` 由“每启动兜底”降级为“仅 0001 一次”；迁移仍在服务启动内执行、非独立 migration job（部分偏离 CD-P0-01 原文）；govulncheck 用 `@latest` 未固定 | 

**核心一句话**：版本化迁移的**结构、幂等、fail-closed 逻辑设计正确，测试覆盖充分**，但**单实例锁实现（GET_LOCK 在连接池上）是硬伤**，生产多实例并发启动时无法互斥，且该分支在 QA 报告明确未覆盖——这是本次审核判定的 BLOCK，必须先修复才能合入生产。

---

## 1. CD-P0-01 数据库版本化迁移（7603076_dev / ea518d3_QA）

### 1.1 总体评估

- `schema_migrations` 版本表：定义正确，独立于 38 张业务表，`version` 主键保证唯一。✅
- `orderedMigrations` 0001~0004：结构基座 / 活跃工单唯一索引 / 旧表合并 / 超管首建，顺序合理，注释明确“禁止改动已发布版本对应 Fn”。✅
- **38 表结构基座与重构前 `AutoMigrate` 完全一致**：逐项比对 `migrateStructureBaseline`（versioned_migrate.go）与 `AutoMigrate`（migrate.go）的模型清单，完全一致（38 张），无漏表；QA 守护用例 `TestQA_VersionedMigrate_FreshDB_All38Tables` 也逐一断言通过。✅
- 幂等性：版本表记录后不再重放，二次执行跳过已应用版本；`SeedSuperAdmin`/`SeedRBAC`/`SeedAreas` 本身幂等（存在即跳过）。QA 用例验证二次执行无副作用（超管/区划/角色数量不变）。✅
- `InitDB`/`InitTestDB` 签名语义：二者 `InitDB(cfg) (*gorm.DB,error)` / `InitTestDB() *gorm.DB` 签名未变，仅内部由 `AutoMigrate` 改为 `MigrateDatabaseVersioned`，`AutoMigrate` 函数本体仍然保留未被删除。✅ 红线程序状态机/RBAC/MQTT parser/识别引擎未被改动（QA 全包绿佐证）。✅

### 1.2 【BLOCK-1】GET_LOCK 会话级锁在连接池上失效 —— 单实例屏障不存在

**文件/行号**：
- `versioned_migrate.go:222`  `acquireMigrateLock`: `db.Raw("SELECT GET_LOCK(?, ?)", "tsloms_migrate", timeout).Scan(&ok)`
- `versioned_migrate.go:231`  `releaseMigrateLock`: `db.Exec("SELECT RELEASE_LOCK(?)", "tsloms_migrate")`
- `versioned_migrate.go:168-175` `MigrateDatabaseVersioned` 内 `acquireMigrateLock` + `defer releaseMigrateLock(db)`
- 后续 `step.Fn(db)`（0001~0003 DDL）同样走连接池。

**问题本质**：MySQL 的 `GET_LOCK`/`RELEASE_LOCK` 是**会话级（同一连接）**。GORM 的 `db.Raw`/`db.Exec`/`db.AutoMigrate` 各自从连接池任意取一条连接执行完即归还，**并不保证同一连接**。因此在当前实现下：
- `GET_LOCK` 在连接 A 上取得锁 → A 归还池；
- `0002/0003` 的 DDL（`CREATE UNIQUE INDEX`/`DropTable`）在连接 B/C 上执行，B/C 与锁无关；
- `RELEASE_LOCK` 在连接 D 上执行 → D 不持有该锁，返回 `NULL`，lock **静默泄漏在连接 A 上**；
- 更糟：连接 A 带着已持有的 `tsloms_migrate` 锁被池回收，随后被其它业务查询复用，会话结束或复用中释放，锁状态完全不可控。

**后果**：多个实例/副本同时启动时，0002（唯一索引）与 0003（DropTable）可被并发执行，直接违反 CD-P0-01“使用数据库锁/分布式锁保证单实例迁移”。虽 DDL 本身多数可重入、0002 `HasIndex` 幂等、0003 `HasTable` 幂等，但 0003 的 DropTable 与 0002 的并发清理存在竞态，且不同实例各自做备份/切换，**不具备事务性**。

**为何 QA 没发现**：`InitTestDB` 是 SQLite，`isMySQL=false` 直接跳过锁与备份分支（`versioned_migrate.go:193`），且 `InitTestDB` 单连接 `SetMaxOpenConns(1)`，即使走锁也测不出泳池问题。**QA 报告已在 §5“备注”明确 MySQL 备份/lock 分支未覆盖** —— 这正是本次 BLOCK 位于的盲区。

**修复建议（任选其一，推荐方案 2）**：
1. 用 `sqlDB.Conn(ctx)` 取一条**专用连接**并全程持有：在同一连接的 `(*sql.Conn).Raw`/`ExecContext` 上执行 GET_LOCK、全部迁移步骤、RELEASE_LOCK，最后 `defer conn.Close()`。绝不在连接池句柄（`*gorm.DB`）上做会话级锁。
2. 若用 GORM：`db.Conn() (*gorm.ConnPool)`（GORM 提供取专用连接）包裹整个迁移体，锁与所有 DDL 在同一连接上执行，出函数前 `Close()`。
3. 备选：改用 `SELECT GET_LOCK` 且整个迁移包在**同一条 `db.Transaction`** 内（事务内连接固定）——但 DDL 隐式提交会使事务语义失效，需谨慎，仍以独立连接为佳。
4. 无论哪种，**必须先补一条 MySQL 集成冒烟**：单连接下验证 GET_LOCK→迁移→RELEASE 配对、超时路径、以及“锁持有期间第二实例应被拒绝/等待”的 fail-closed 断言。

**判定**：BLOCK，不合入前必须修复单实例锁的实现并验证。

### 1.3 【HIGH-2】迁移备份目标路径写在“不可变 release 目录内”，且会被回滚/下版本清理

**文件/行号**：`versioned_migrate.go:361-366` `backupTargetDir()` 返回 `filepath.Abs(filepath.Join("releases","backups","db"))`，即相对当前工作目录解析；`../../.../backupDatabaseBeforeDDL`（L332）`MkdirAll(backupDir)` 并写入 `tsloms_<db>_<ts>.sql.zst`。

**问题**：Release-install 的服务 WorkingDirectory=`/opt/tsloms/current`（→ 某 `releases/<sha>`），因此 `filepath.Abs("releases/backups/db")` 解析为 `/opt/tsloms/current/releases/backups/db` → 实际 `/opt/tsloms/releases/<sha>/releases/backups/db`：
- 备份被放在**发布目录内部**，而非仓库/部署所用的持久备份目录 `/opt/tsloms/backups/db`（release-install.sh 步骤[2] 与 probe-deep.sh L6 探针都指向后者）。两者不一致，探针/回滚脚本找不到这份“迁移前快照”。
- 该目录随版本滚动、回滚或磁盘清理被移除/覆盖，**“迁移前备份”不持久**，违背备份用于回滚的语义。
- 在加固的 systemd 单元（`ProtectSystem=strict`，仅 `ReadWritePaths=/opt/tsloms/shared/media /opt/tsloms/current`）下，写入依赖 systemd 对 `current` 符号链接的解析目标是否列为可写；即便当前可写，也是脆弱约束。

**修复建议**：`backupTargetDir()` 改为**固定持久路径** `/opt/tsloms/backups/db`（与 release-install/probe 对齐），并在迁移前确保该目录存在且可写；备份文件应跨版本保留（按时间戳命名已具备）。

**判定**：HIGH（不破坏 fail-closed 阻断，但备份不可持久、探针/回滚找不到）。

### 1.4 【HIGH-3】mysqldump 密码明文出现在命令行/进程列表

**文件/行号**：`versioned_migrate.go:339-341`：
```go
cmdStr := fmt.Sprintf(
  "mysqldump --single-transaction --set-gtid-purged=OFF -u '%s' -p'%s' -h '%s' -P '%s' %s | zstd -q -o %s",
  creds.User, creds.Password, ...)
cmd = exec.Command("sh", "-c", cmdStr)
```
**问题**：同机其它用户/审计可在 `ps`、进程参数、审计日志中看到 DB 密码明文。release-install.sh 自己使用了 `MYSQL_PWD`+`export`（同一脚本内做法是对的），而应用内备份却回退到 `-p'<明文>'`，属安全回退。
**修复建议**：改用 `exec.Command` 参数数组（不用 `sh -c` 拼串）或导出 `MYSQL_PWD`（`cmd.Env = append(cmd.Env, "MYSQL_PWD="+password)`），避免密码入参/入日志。
**判定**：HIGH（机密泄露面）。

### 1.5 fail-closed 备份：逻辑正确，但仅在“mysql 生产 + 缺凭据”时真正 beclosed

- 缺 DB 密码 / DB_NAME 空 → `resolveBackupCreds` 返回 error → 备份失败 → 迁移被阻断（`versioned_migrate.go:194-197` "fail-closed"）。✅ 逻辑正确。
- 备份**命令执行失败或未生成文件** → error → 阻断。✅
- **但**：0002/0003 的 `NeedsBackup=true` 分支只在 `isMySQL` 下进入（L193），**缺备份凭据时在 SQLite 测试/或配置游离的 MySQL 下会被跳过**——SQLite 测试路径“安全跳过”合理（内存库无持久化）；生产 MySQL 缺凭据则 fail-closed 生效。**判定：fail-closed 覆盖正确**，唯一缺口是【HIGH-2/3】的路径/密码问题，不影响 beclosed 语义本身。✅
- 超时：GET_LOCK 有 300s timeout（L166），符合“大表迁移设超时”。但同样受【BLOCK-1】影响，超时只作用于那条游离连接。⚠️

### 1.6 幂等性复核（二次执行副作用）

- 0001 `active→occurred`：全量 `UPDATE ... WHERE status='active'`，幂等。✅
- 0002 `HasIndex` 幂等跳过。✅
- 0003 `HasTable("device_materials")` 不存在即跳过；存在则一次合并删表。✅
- 0004 `SeedSuperAdmin` 账号存在即跳过。✅
- 版本表唯一主键，重复记录由 DB 主键约束兜底。✅
- **无发现业务表重复（超管重复建、区划/角色重复）**。QA `TestQA_VersionedMigrate_IdempotentNoSideEffect` 已验证。✅

### 1.7 与重构前 AutoMigrate 的“每次启动兜底”语义差异（MED-4）

旧 `AutoMigrate` 每次启动都执行 `active→occurred`、`migrateWorkOrderActiveUnique`、`MigrateLegacyDeviceMaterials`。新实现中这些**只执行一次（按版本 gated）**。已核实：当前 MQTT/识别/处置/服务代码**不再写入 `status="active"` 的故障记录**（全部用 `FaultStatusOccurred`）；0002 唯一索引一旦建成即稳定。因此对正常升级路径无回归。**残余风险**：若曾有一台旧版本实例在 0001 之后仍向库写入 `active` 行（混合版本运行窗口），这些行不会被后续版本修正——属于升级窗口内的理论盲区。
**建议**：可作为“每启动幂等兜底”保留（不进版本表），与其它幂等种子一致；或在 runbook 注明混合版本运行期约束。
**判定**：MED（理论盲区，非阻断）。

### 1.8 仍随服务启动执行迁移，而非独立 migration job（MED-5）

CD-P0-01 原文要求“将迁移改为**显式版本化 migration job**，不在普通服务启动时隐式变更数据库”。本实现是**显式版本化**（版本表 gated，不再是无条件 AutoMigrate），满足主体精神；但仍是 `InitDB`/服务启动路径内执行，不是单独的 migration 脚本/job，也缺少迁移前的人工确认步骤。V2 遗留的 `release-install.sh` 提前备份 + 启动时版本迁移的编排尚可接受，但**在【BLOCK-1】修复前，多实例并发启动风险被放大**。
**判定**：MED，建议后续演进为独立 migration 工具并在 CI/CD 显式编排；与 BLOCK-1 修复一并处理更佳。

---

## 2. QA 报告复核（qa-report-db-migration.md）

**结论可信度：可信但需限定范围。**
- 执行范围真实（14 个含测试包全绿，`go build/vet/test` 三连通过），无造假迹象。
- 与 dev 自带用例互补、非重复的 4 个守护用例（幂等无副作用 / 版本记录字段完整 / 38 表全量 / InitTestDB 种子红线）设计合理，确实覆盖了 SQLite 路径下主要语义。
- **QA 报告自己在 §5 明确承认的盲区，恰好是本 audit 的 BLOCK 所在**：MySQL 生产路径（GET_LOCK 锁配对、mysqldump|zstd 备份 syscall、fail-closed 阻断、多实例并发）**完全未被执行/未覆盖**。因此“未发现回归”的结论**只能覆盖 SQLite/测试路径**，**不能外推到生产 MySQL 双需**。QA 报告将该盲区标注为"备注（非缺陷）"，但结合【BLOCK-1】看，它是**真实高风险未验证项**，应升级为待办。

**QA 报告遗漏的回归盲区**（本 audit 补充）：
1. GET_LOCK 连接池失效（BLOCK-1）——SQLite 无锁路径天然无法发现。
2. 备份目标目录与持久备份目录不一致（HIGH-2）——QA 未核对与 release-install/probe 的目录一致性。
3. mysqldump 密码明文入参（HIGH-3）——QA 未做安全面检查。
4. 手动部署 run_id 选错 workflow 风险（HIGH-6，见 §4）——QA 未覆盖 CD 手动 dispatch 路径。
5. QA 执行环境为 Windows/pwsh，`bash -n` 或部分 shell 脚本执行位/换行符问题可能被掩盖——建议 CD/CO 脚本在 Linux `bash -n` 与 `shellcheck` 复核。

---

## 3. CI-P1-02 license 私钥治理（5e9dfaa）

### 3.1 私钥迁出 + 环境变量读取

- `cmd/licensegen/main.go`：生产私钥常量删除，改为 `supplierPrivateKeyB64FromEnv()` 读 `TSLOMS_LICENSE_PRIVATE_KEY`，缺失即 panic（fail-close，不会用空密钥偷偷签发）。✅
- `internal/license/license.go`：`supplierPublicKeyB64`（公钥）保留内嵌（公钥非机密，验签用途），语义不变。✅
- **密钥对一致性验证（本人独立复核）**：用 Go 从旧硬编码私钥 `QNn9Qbk-...` 派生公钥 = `_Ilp6BWDR58wY3w9rsnbmY9Qy_PRPU2ltPsHpOA9-gs`，与服务器内嵌公钥**完全一致**（MATCH=true）。故环境变量注入的仍是同一把私钥，**不会导致验签失效/授权码作废**。✅

### 3.2 .gitleaks.toml 豁免范围复核

- 仅豁免两个 `_test.go` 路径 + 两个**测试专用密钥串**（`tAmZSDUmFfBfrQ...` / `qlMMcnfMhwQRX...`）。✅
- 测试私钥未经 grep 仅出现在 `_test.go`（license_test.go、handler/license_test.go），**不在任何生产代码中**。✅
- 测试密钥与已移除的生产私钥**完全不同**（明文不同），符合“只豁免测试、不放过生产”。✅
- **局限（MED/安全建议）**：被豁免的**测试私钥本身就是真实密钥**，虽与生产无关，但若测试私钥泄露也只是测试环境风险；问题在**生产私钥仍在 git 历史中**（`git log -S 'QNn9Qbk-...'` 命中 c3cd21d、291cd72）。仓库当前工作树已移除，但任何拿到仓库历史的人仍可 `git grep` 到该生产私钥。**应轮换该生产密钥**（变更 `TSLOMS_LICENSE_PRIVATE_KEY` + 更新服务器公钥并重新签发存量授权码），并建议用 `git filter-repo`/BFG 清洗历史。gitleaks action 只对**后续提交**生效，对历史无效。
- 另：gitleaks CI 用的是 `gitleaks-action@ff98106e`（固定 SHA ✅），但豁免项写死路径，若未来新增含密钥的测试文件需同步更新——属维护注意项。

### 3.3 测试补强

- `TestVerifyUnlockCode_Valid/Mismatch/Expired/NotYetValid/Tampered/TamperedSig` + handler 层 `TestLicense_RejectTamperedCode`：正/负向覆盖完整，验证篡改签名与载荷均被拒。✅
- `SetPublicKeyForTest` 只在测试调用，生产不调用，生产验签不受影响。✅
- **测试隔离注意（LOW）**：`SetPublicKeyForTest` 写包级 `cachedPublicKey`，handler 包测试一旦调用即对后续同进程测试生效、无恢复。当前所有相关测试都用测试钥，无冲突；但若未来有测试需断言“用生产公钥验签”，会受污染。建议测试结束 `t.Cleanup` 重置。

**判定**：CI-P1-02 私钥治理**达标**；生产私钥含历史残留 → 需轮换 + 清洗（MED 安全项），不阻塞当前工作树合入。

---

## 4. 流水线 CI/CD/CO 整改（b164393 / a1a3a90 / 5e78ac6 / 282ef0b / 6b08e4f / 73aded2 / af29692 / 3f943de）

### 4.1 action SHA / SSH fingerprint（CD-P1-01/04、CI-P1-01）—— ✅ 达标

- 所有第三方 action 固定完整 SHA：checkout、setup-go、setup-node、upload/download-artifact、github-script、gitleaks-action、appleboy/ssh-action。核对所用 SHA 为常见已知版本指纹。✅
- CD/CO/E2E 三处 SSH 均启用 `fingerprint`/`UserKnownHostsFile`+`StrictHostKeyChecking=yes`，移除 `StrictHostKeyChecking=no`。✅

### 4.2 release-install 已存在 release 强制复核 + systemd 单源（CD-P1-01/02）—— ✅ 达标

- 存在 release 时强制 `sha256sum -c`、结构/可执行/version 校验，失败拒绝部署（282ef0b）。✅
- 唯一权威单元 `tsloms-server.service`（User=tsloms、/etc/tsloms/tsloms.env、`current` 链接、加固沙箱）；旧 root 单元归档；重启前 `systemctl show` 校验 User/EnvironmentFile/ExecStart，任一不符 exit 1 fail-closed（282ef0b）。✅

### 4.3 【HIGH-6】手动部署 run_id 选择用 `startswith("CI")` + `head -1`，可能选中 ci-admin workflow

**文件/行号**：`.github/workflows/cd.yml` “定位 CI 制品的 run_id”步骤：
```
--jq '.workflow_runs[] | select(.name | startswith("CI")) | select(.conclusion == "success") | .id' | head -1
```
**问题**：名为 `CI - Go 质量门（...）` 的 `ci.yml` 与名为 `CI - Admin 前端质量门` 的 `ci-admin.yml` **都 startswith("CI")**。手动 `workflow_dispatch` 时 `head -1` 可能选中 ci-admin 的 run——该 run **不产出 `tsloms-release`**，随后 download-artifact 会取不到制品而失败（fail-closed，不会拉错版本），但属于**非确定性/依赖运气**，且与 v3 文档 CD-P1-03“手动部署必须先验证目标 SHA 对应的成功 CI run”不完全吻合（它确实按 SHA 查了，但未限定到产出制品的 ci.yml）。
**修复建议**：改为按工作流文件精确匹配 `select(.path == ".github/workflows/ci.yml")`（或 `.name == "CI - Go 质量门（含覆盖率门禁 ≥80%）"`），并对多个命中做去重后取 Go CI run；必要时校验该 run 的 package job 成功。
**判定**：HIGH（fail-closed 但逻辑不严谨，应修复后合入）。

### 4.4 CO 深度探针（CO-P1-01/02）—— ✅ 大部分达标

- `probe-deep.sh` 覆盖 DB/Redis/主机/systemd/journal/MQTT 认证回环/备份年龄，L2-L4 分层（CO-P1-02）。✅
- MQTT 回环改用临时文件比对 payload，localhost 归一 127.0.0.1 规避 IPv6 解析，端口解析完善（af29692）。✅
- 备份目录无文件由 FAIL 降为 WARN（首次部署容错，af29692）——**注意**：CD-P0-01 强调备份缺失应 be closed 于迁移，而这里是“巡检 WARN”，两者场景不同（一个是迁移前置 be closed，一个是部署后巡检告警），不冲突。✅
- 探针经 base64 传到服务器本机执行，凭据经 env 注入不落盘/日志。✅
- **LOW**：`getenv`/`BURL` 解析对兼容 TLS URI 与含端口 host 的健壮性建议补边界用例（af29692 已处理 localhost/IPv6 主场景）。

### 4.5 E2E / bundle-budget / runbook（CI-P1-03、CI-P2-01、CD-P0-01 runbook）—— ✅

- e2e.yml 环境级 Secret 作用域修正（73aded2）：job 级 `env`+`environment: production` 注入，修复 workflow_run 读不到环境 secret 的问题，方向正确。确认 workflow_run 由 CD 成功后触发。✅
- `smoke.js` 凭据走 env、算术验证码解算、脱敏；`smoke-local.sh` localhost 冒烟。✅
- bundle-budget.mjs（3f943de）：修正 `this` 绑定与 closeBundle 磁盘测量，避免构建失败与误判；尺寸在预算内。✅
- runbook（6b08e4f）：最小权限/sudoers 白名单/备份恢复演练/故障注入清单，补充 CD-P0-01 要求的恢复演练文档。✅

---

## 5. 逐项 P0/P1 复核结论表

| 编号 | 要求 | 提交 | 复核结论 |
|------|------|------|----------|
| **CD-P0-01** | 数据库版本化迁移 | 7603076/ea518d3 | **有条件（BLOCK-1 未达：单实例锁在连接池上失效）；结构/幂等/fail-closed 达标；备份路径与密码安全存在 HIGH-2/3；仍随启动执行（MED-5）** |
| **CD-P1-01** | 已存在 release 强制复核 | 282ef0b/b164393 | ✅ 达标（强制 re-verify manifest/version/exec） |
| **CD-P1-02** | 唯一权威 systemd 单元 | 5e78ac6/282ef0b | ✅ 达标（单源+归档+重启前 systemctl show 校验 fail-closed） |
| **CD-P1-03** | 手动部署严格 SHA↔run↔version | b164393 | ⚠️ **部分达标**：SHA 查询已按 head_sha；但 run 选择用 `startswith("CI")+head -1` 可能选中 ci-admin（HIGH-6），需精确过滤 ci.yml |
| **CD-P1-04** | SSH host fingerprint 固定 | b164393/5e78ac6 | ✅ 达标（fingerprint + StrictHostKeyChecking=yes） |
| **CD-P2-01** | nginx -t 阻断 | b164393 | ✅ 达标（release-install 中 nginx -t 失败 exit 1） |
| **CI-P1-01** | 固定 action SHA | b164393 | ✅ 达标 |
| **CI-P1-02** | SBOM/漏洞扫描/私钥治理/secret scanning | b164393/5e9dfaa | ✅ 私钥治理达标；SBOM(go/npm)+govulncheck+gitleaks 已接入；**注意**：生产私钥仍在 git 历史需轮换（安全项，非当前树阻断）；govulncheck 用 `@latest` 建议固定版本（MED） |
| **CI-P1-03** | E2E 冒烟接入 | a1a3a90/73aded2 | ✅ 达标（部署后 HTTP + SSH 本机双路径，Secret 作用域已修） |
| **CI-P2-01** | bundle budget 修复 | 3f943de/3080fa3/ab11e9f | ✅ 达标（构建不再失败，体积测量正确） |
| **CO-P1-01** | MQTT 认证回环探针 | af29692/b164393 | ✅ 达标（CONNECT/PUBLISH/SUBSCRIBE/回环比对） |
| **CO-P1-02** | DB/Redis/主机/备份巡检 | af29692/b164393 | ✅ 达标（L2-L4 分层 + 备份年龄） |

---

## 6. 风险汇总

| 级别 | 编号 | 位置 | 描述 | 建议 |
|------|------|------|------|------|
| **BLOCK** | 1 | versioned_migrate.go:222/231/168-175 | GET_LOCK/RELEASE_LOCK/DDL在连接池不同连接上执行，单实例锁失效 | 用 `sqlDB.Conn`/GORM `db.Conn()` 固定一条连接承载锁与全部迁移；补 MySQL 集成测试 |
| **HIGH** | 2 | versioned_migrate.go:361-366 | 备份目标 `./releases/backups/db` 落在不可变 release 目录内，探针/回滚找不到、跨版本不持久 | 改为固定 `/opt/tsloms/backups/db` |
| **HIGH** | 3 | versioned_migrate.go:339-341 | mysqldump 密码明文进命令行/进程列表 | 改用参数数组或 `MYSQL_PWD` |
| **HIGH** | 6 | cd.yml run_id 步骤 | `startswith("CI")`+`head -1` 可能选 ci-admin run | 精确匹配 ci.yml path/name |
| **MED** | 4 | migrate.go / versioned_migrate.go | `active→occurred` 由每启动兜底降为 0001 一次（混合版本窗口理论盲区） | 保留为每启动幂等兜底，或明确 runbook 约束 |
| **MED** | 5 | db.go InitDB | 迁移仍随服务启动执行，非独立 migration job（偏离 CD-P0-01 原文） | 后续演进独立 migration 工具；与 BLOCK-1 一并处理 |
| **MED** | 7 | 5e9dfaa | 生产密钥仍在 git 历史（c3cd21d/291cd72） | 轮换 + filter-repo 清洗 |
| **MED** | 8 | ci.yml govulncheck | `go install @latest` 未固定版本，供应链/可复现性 | 固定版本或 hash-pin |
| **LOW** | 9 | handler/license_test | `SetPublicKeyForTest` 无 t.Cleanup 恢复 | 测试结束重置 |
| **LOW** | 10 | probe-deep.sh | URI/端口解析边界可再补用例 | 维护性加固 |

---

## 7. 潜在改进（非阻断）

1. SQLite 生产/测试路径：`isMySQL` 判断依赖 `Dialector.Name()`，未来若支持其它方言（Postgres）需扩展锁/备份类型判断（当前 MySQL/SQLite 双方言足够）。
2. `envValue`/`splitLines`/`trimSpace`/`indexOf` 为手写轻量实现，语义基本正确；建议补单测覆盖（含 `# 注释`、`KEY = value` 带空格、CRLF）以防尾部空格/注释解析漂移。
3. 迁移日志打印超管一次性密码（0004）——审计 BLOCK-1 兼容，但生产日志级别下会落盘，建议确认日志输出仅本地可读或走独立安全通道。
4. `backupDatabaseBeforeDDL` 用 `CombinedOutput` 会持有整段压缩输出，大库场景内存占用大；建议 `Stdout/Stderr` 流式或仅错误捕获。

---

## 8. 合入裁定

- **不得以当前状态合入生产 v3**：CD-P0-01 的【BLOCK-1】（GET_LOCK 单实例锁失效）是硬性安全/正确性缺陷，必须在修复并补齐 MySQL 集成验证后合入。
- **修复清单（最小合入门槛）**：
  1. BLOCK-1：固定连接承载锁+迁移，补 MySQL 集成测试（锁配对、并发拒绝、超时、fail-closed）。
  2. HIGH-2：备份目标改 `/opt/tsloms/backups/db`。
  3. HIGH-3：备份密码改 `MYSQL_PWD`/参数数组。
  4. HIGH-6：cd.yml 手动部署 run_id 精确匹配 ci.yml。
  5. MED-7：生产密钥轮换 + 历史清洗（可与合入并行，但应尽快)。
- **其余项（MED/LOW）** 可作为不阻断改进项，或与本次一并修，不强制阻塞。
- 若团队接受“仅 SQLite/测试路径先合入 + MySQL 锁修复作为独立热修”，请在 CD-P0-01 release notes 中**显式标注生产 MySQL 锁未验证**，且**生产环境保持单实例/禁止多副本同时启动**作为临时缓解，直至 BLOCK-1 修复。

**红线复核**：工单/故障状态机、RBAC、MQTT parser、识别引擎、AutoMigrate/InitTestDB 签名语义——均未被改动，QA 全包绿与本人代码比对均佐证无回归。✅

---

*（本报告由 reviewer-audit-tsloms 只读生成；除 `audit-report-v3.md` 外未改动任何文件，`git status` clean，未 push。）*
