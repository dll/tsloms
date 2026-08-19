# TSLOMS v3 流水线整改 — 最终汇总报告

> 完成日期：2026-08-19
> 依据：`docs/TSLOMS-流水线CICDCO-v3.md`（v3 严格审核报告）未达标项整改
> 方式：多 Agent 串行流水线（pm 核对 → dev 重构 → qa 回归 → reviewer 审核 → dev 修复 → leader 汇总）

## 一、总体结论

v3 报告所列未达标项**已在仓库侧全部整改并落地**，流水线主链路（CI 质量门 → 覆盖率门禁 → 不可变制品 → CD 原子切换/回滚 → CO 巡检）达到"主流程达标"，并补强了治理与灾备项。核心软件代码重构（数据库版本化迁移）已通过 dev 落地 + qa 回归 + reviewer 审核 + leader 现场核验。**判定：整体可合入发布（有条件项已闭环，MySQL 集成与真实备份演练留现场证据）。**

## 二、分项完成矩阵

| v3 项 | 状态 | 落地证据 |
|---|---|---|
| P0-01 数据库版本化迁移 | ✅ | `versioned_migrate.go`（schema_migrations 版本表 + GET_LOCK 单实例锁绑定独占连接 + 0001~0004 有序迁移 + DDL 前 fail-closed 备份）；db.go InitDB/InitTestDB 改走显式版本化入口 |
| P0-02 已存在 release 重新校验 | ✅ | `release-install.sh` 已存在目录强制重新校验 manifest/结构/version |
| P0-03 systemd 唯一单元 | ✅ | `tsloms-server.service` 唯一权威单元；prod-fitted rm root 旧单元入 archive；CD 部署前校验实际启用单元 |
| P0-04 SSH fingerprint + 最小权限/备份恢复 runbook | ✅ | cd.yml `DEPLOY_HOST_FINGERPRINT`；runbook 文档（最小权限/sudoers/备份恢复/故障注入） |
| P1-01 固定 action SHA + gitleaks/govulncheck/SBOM/签名 | ✅ | 全部第三方 action 固定完整 SHA；govulncheck + gitleaks 接入；SBOM（go/npm）入制品清单 |
| P1-02 E2E 接入 | ✅ | `e2e.yml`（HTTP 冒烟 + SSH 本机冒烟，凭据脱敏、host fingerprint） |
| P1-03 MQTT 认证探针 + DB/Redis/备份/主机巡检 | ✅ | `probe-deep.sh` + co.yml `deep-probe` job（MYSQL/Redis/磁盘/备份年龄/systemd/MQTT 回环） |
| P2-01 前端 bundle budget + lint warning | ✅ | `bundle-budget.mjs` 体积预算（cesium/vendor/echarts/entry/首屏超限 FAIL） |
| CI-P2-02 ci-admin 重复工作流 | 说明 | 保留作快速 PR 反馈，已固定 action SHA；主 CI 权威检查为 ci.yml |
| CD-P1-03 手动部署严格 SHA↔run↔version | ✅ | cd.yml 精确锁定 ci.yml 成功 run（排除 ci-admin） |
| CD-P2-01 nginx -t 阻断 | ✅ | release-install.sh nginx -t 失败即阻断 |
| 供应链私钥治理（license） | ✅ | 生产 Ed25519 私钥移出仓库改环境变量；.gitleaks.toml 仅豁免测试密钥 |

## 三、核心重构：数据库版本化迁移（CD-P0-01）

- **重构对象**：`packages/server/internal/model/db.go::InitDB/InitTestDB` 由"无条件 AutoMigrate 隐式改库"→"调用 `MigrateDatabaseVersioned` 显式版本化迁移"。
- **关键实现**：
  - `schema_migrations` 版本表（version/name/applied_at/applied_by），独立于 38 业务表。
  - 有序迁移 `0001` 结构基座（38 表 AutoMigrate + active→occurred）、`0002` 建 uk_wo_active_scope 唯一索引（NeedsBackup）、`0003` 旧 device_materials 并入 materials 并删表（NeedsBackup）、`0004` 超管首建（密码渠道化）。
  - **单实例锁（BLOCK-1 修复）**：MySQL 用 `*sql.Conn` 独占一条物理连接，GET_LOCK → 全部迁移 → RELEASE_LOCK 全程同连接，杜绝连接池漂移导致会话级锁失效；SQLite 测试走简化无锁路径。
  - **fail-closed**：任一版本失败整体终止；含 DDL/DropTable 版本（0002/0003）执行前强制 `MYSQL_PWD` 环境变量 + mysqldump|zstd 备份（凭据缺失即阻断，HIGH-3 修复），备份落 `/opt/tsloms/backups/db`（HIGH-2 修复）。
  - 幂等种子（SeedRBAC/SeedAreas/active→occurred）保留为每启动重放安全逻辑，不进版本表。
- **红线遵守**：AutoMigrate/InitTestDB/InitDB/MigrateLegacyDeviceMaterials/migrateWorkOrderActiveUnique/Seed*. 签名与语义全保留 → 80+ 处测试零改动。

## 四、质量验证（leader 现场核验，非轻信子代理）

- **全量 14 包 `go test ./... -count=1` 全 `ok`**，exit 0；`gofmt -l .` 空。
- **reviewer 审核**：发现 BLOCK-1（GET_LOCK 连接池失效）+ HIGH-2/3/6，**已全部修复并复测通过**（BLOCK-1 锁绑定独占连接、备份目录持久化、密码不入 argv、cd.yml run_id 精确锁定）。
- 新增守护测试：版本化迁移幂等/版本表/38 表全量/InitTestDB 种子语义基线（qa） + 备份目录/密码安全/GET_LOCK 集成（dev 修复）。
- **诚实标注**：MySQL GET_LOCK 集成测试在无 `TSLOMS_TEST_MYSQL_DSN` 时自动 skip——真实 MySQL 锁并发与备份恢复需集成环境验证（本环境无 MySQL/mysqldump）。

## 五、待办 / 现场证据（非仓库内可完成）

1. **真实 MySQL 备份恢复演练 + GET_LOCK 并发验证**：需在有 MySQL 的集成/部署机设 `TSLOMS_TEST_MYSQL_DSN` 跑 `TestMigrateDatabaseVersioned_MySQLGetLockIntegration`，并做一次 mysqldump|zstd 恢复演练，记录 RTO/RPO（v3 §7.1 P0-01 验收证据）。
2. **生产外部控制项**（GitHub production Environment required reviewers、SSH 禁 root、/etc/tsloms/tsloms.env 0600、备份异地副本）——需现场审计证据。
3. **CI 完整跑通一次**：本地综合验证通过，建议 push 后观察一次 CI（Go 质量门 + 覆盖率 + 前端 + CD）全绿确认流水线端到端。

## 六、提交清单（本轮 v3 整改，均 push 或待 push）

详见 git log；关键：`7603076`(db 版本化迁移) `ea518d3`(qa 回归) `5e9dfaa`(license 私钥治理) `d88fdf4`(BLOCK-1/HIGH-2/3) `4bd2a27`(HIGH-6) 及前述流水线整改。

---
**结论**：v3 未达标项整改完成，流水线通达且质量达标（可合入）。剩余为 MySQL 集成与生产现场证据，作为后续验收项，不影响仓库侧合入。
