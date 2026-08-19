# TSLOMS 流水线（CI / CD / CO）v3.0 严格审核报告

> 文档性质：基于当前仓库、GitHub Actions 实际运行记录和部署脚本的全面复核
>
> 审核日期：2026-08-19
>
> 审核基线：`main` / `f9f1dfc4d8c4151fb5a693f0660153d7d9ac1772`
>
> 适用范围：`packages/server`、`packages/admin`、`.github/workflows`、`deploy` 及本项目 CI/CD/CO 相关文档

## 1. 结论先行

### 1.1 总体判定

**当前流水线主链路已经跑通，但项目尚未达到“所有生产控制项均可证明达标”的严格生产级标准。判定为：主流程达标，治理与灾备部分有条件达标，整体不宜标记为完全达标。**

已经由实际运行证明的链路为：

```text
main push
   -> Go 质量门 + 覆盖率门禁
   -> 前端 lint / 单测 / 构建 / audit
   -> 不可变制品构建与 manifest 校验
   -> workflow_run CD（固定目标 SHA）
   -> staging 上传 / SHA-256 校验 / 原子切换
   -> systemd 重启 / 本机探活 / 外部入口探活
   -> 定时 CO 健康巡检
```

但以下事项仍阻止“完全达标”结论：

1. 数据库仍由应用启动时 `GORM AutoMigrate` 驱动，没有版本化迁移、单实例锁、超时、恢复演练和强制备份前置。
2. `release-install.sh` 对已存在的 release 直接跳过再次校验，不能证明服务器上的同名目录未被篡改。
3. CI 没有生成 SBOM、签名/证明，也没有 secret scanning、`govulncheck` 或容器漏洞门禁。
4. E2E 冒烟脚本存在但没有接入 CI、CD 后验收或 CO；CO 的 MQTT 检查只有 TCP 端口探测，没有认证和消息链路验证。
5. 生产机的 GitHub Environment 审批、Secrets、SSH host fingerprint、systemd 实际单元和备份可恢复性属于仓库外部状态，本次无法仅凭仓库证明。

### 1.2 分项达标矩阵

| 范围 | 判定 | 证据/缺口 |
|---|---|---|
| Go CI 质量门 | 达标 | Go 1.26.6、gofmt、vet、build、全量测试均通过 |
| Go 覆盖率门禁 | 达标 | GitHub Actions `32200640116` 输出总覆盖率 80.60%，阈值 80.00% |
| 前端 CI | 达标（有告警） | lint、单测、构建、`npm audit --omit=dev --audit-level=high` 通过；仍有非阻断 lint warning |
| 不可变制品 | 达标 | Linux/amd64 server、admin dist、version.txt、manifest.sha256 均已构建并校验 |
| CI→CD 关联 | 达标 | CD 由 CI 成功的 `workflow_run` 触发，按 `head_sha` checkout 和下载 artifact |
| CD 原子切换 | 达标 | staging、checksum、`current.next`、`current/previous`、重启、探活和失败回滚已实际成功 |
| CD 数据库发布事务 | 不达标 | 备份可选，未发现迁移锁、版本化迁移、恢复验证；AutoMigrate 仍在启动路径 |
| CO API/前端巡检 | 达标 | 最近运行 `32201862284` 及多次历史运行成功 |
| CO MQTT 深度巡检 | 未达标 | 仅检测 1883 TCP 是否打开，不验证账号、ACL、CONNECT、发布/订阅 |
| E2E | 未达标 | `e2e:smoke` 存在，但工作流没有调用 |
| 供应链安全 | 未达标 | action 多使用可变 tag；无 SBOM、签名、gitleaks、govulncheck 门禁 |
| 生产外部控制 | 未验证 | Environment 审批、Secrets 权限、SSH fingerprint、备份恢复和故障注入需现场证据 |

### 1.3 发布建议

当前可以继续使用已验证的 CI→CD 主链路发布，但应把数据库迁移、生产凭据/审批、制品不可变性和深度运维探针列为发布前置整改项。整改前不得在审计材料中写成“完全自动化、可逆、可证明安全的生产发布”。

## 2. 审核依据与实际验证

### 2.1 代码和配置事实

| 项目 | 当前事实 | 来源 |
|---|---|---|
| 后端模块 | `go 1.25.0`，`toolchain go1.26.6` | `packages/server/go.mod` |
| CI Go | 固定 `1.26.6`，缓存依赖路径为 `packages/server/go.sum` | `.github/workflows/ci.yml` |
| 前端 Node | `.node-version` 为 `22.14.0`；`.tool-versions` 为 `nodejs 22.14.0` | `.node-version`、`.tool-versions` |
| 后端健康接口 | 本机 `http://127.0.0.1:8093/api/v1/health` | `deploy/scripts/release-install.sh`、服务代码 |
| 外部健康接口 | `/tsloms/health`，前端 `/tsloms/admin/` | `cd.yml`、`co.yml`、Nginx 配置 |
| 制品内容 | `server`、`admin/dist`、`manifest.sha256`、`version.txt` | `.github/workflows/ci.yml` |
| 发布目录 | `/opt/tsloms/releases/<sha>`、`current`、`previous`、`shared/media` | `release-install.sh`、部署说明 |
| 数据库迁移 | 启动时调用 GORM `AutoMigrate`，并执行回填/清理/种子逻辑 | `packages/server/internal/model/db.go`、`migrate.go` |
| E2E | `packages/admin/e2e/smoke.js` 和 `npm run e2e:smoke` 存在 | `packages/admin/package.json` |
| systemd | 标准单元使用 `User=tsloms`、`/etc/tsloms/tsloms.env`；另有 prod-fitted 单元使用 root 和旧 `.env` 路径 | `deploy/systemd/*` |

### 2.2 实际运行证据

| 验证项 | 运行/结果 |
|---|---|
| 后端全量测试 | 本地 `go test ./... -count=1` 通过，所有包通过 |
| Go CI | `32200640116` 成功；gofmt、vet、build、全量测试和覆盖率均成功 |
| 覆盖率 | `total coverage: 80.60% (threshold: 80.00%)`，覆盖率 artifact 上传成功 |
| 前端质量门 | 同一 CI 运行中的 lint、单测、构建、audit 成功 |
| CD | `32200820495` 成功；上传、制品校验、原子切换、迁移备份步骤、探活和外部入口检查成功 |
| CO | `32201862284` 成功；此前多次 `CO - 生产持续运维巡检` 运行连续成功 |
| 远端基线 | `origin/main = f9f1dfc4d8c4151fb5a693f0660153d7d9ac1772` |

注：工作区存在用户/临时未提交文件（页面文件、覆盖率文件、未跟踪文档和辅助脚本）。本报告只以 Git 基线及线上工作流为准，不将这些未提交文件视为已发布变更，也没有清理或回退它们。

## 3. CI 严格审核

### 3.1 已达标项

1. **工具链固定**：CI 和制品 job 均使用 Go 1.26.6；前端使用 `.node-version`，与 Cesium 的 Node 22 要求一致。
2. **质量门完整**：后端执行 gofmt、vet、build、全量测试和浮点覆盖率门禁；前端执行 lint、类型检查、Vitest、生产构建和高危依赖审计。
3. **覆盖率比较正确**：使用 `cmd/coveragecheck` 读取合并后的 profile 并比较浮点阈值，避免整数截断和重复 profile 块造成误判。
4. **制品只在质量门之后构建**：`package.needs` 同时依赖后端和前端质量门，避免只通过一侧就发布。
5. **制品可校验**：生成 `version.txt` 和 `manifest.sha256`，CD 下载后重新检查完整性和目标 SHA。
6. **权限基线已声明**：CI 使用 `contents: read`；CO 使用 `contents: read` 和 `issues: write`。

### 3.2 未完全达标项

#### CI-P1-01：第三方 Action 使用可变 tag

`actions/checkout@v4`、`actions/setup-go@v5`、`actions/setup-node@v4`、`actions/upload-artifact@v4`、`actions/download-artifact@v4` 和 `actions/github-script@v7` 没有固定完整 commit SHA。`appleboy/ssh-action` 已固定 SHA，但整体供应链仍可能随 tag 变化。

**整改要求：**固定所有第三方 Action 的完整 commit SHA，并使用 Dependabot 或定期人工审查升级；升级须重新跑完整 CI/CD 验收。

#### CI-P1-02：缺少 SBOM、签名和依赖漏洞纵深检查

当前只有 `npm audit`，没有 `govulncheck`、secret scanning、SBOM（CycloneDX/SPDX）、制品签名或签名验证。SHA-256 可以发现传输损坏，但不能证明制品来源和发布者身份。

**整改要求：**在 package job 生成 SBOM、依赖漏洞报告和 provenance；生产 CD 至少验证签名或可信构建证明，严重漏洞阻断发布。

#### CI-P1-03：E2E 未接入质量门

`npm run e2e:smoke` 仅是仓库脚本，当前 CI 没有启动可测试环境、注入测试凭据或执行 health、登录、仪表盘、只读业务接口等关键路径。

**整改要求：**增加隔离测试环境，在合并或生产发布后执行受保护 E2E；凭据从 Secrets 注入，日志脱敏。

#### CI-P2-01：前端存在非阻断 warning，且没有 bundle budget

最近 CI 注释仍报告 Vue/JS 格式和未使用变量 warning。构建虽成功，但 Cesium 大分片和体积回归没有阈值门禁。

**整改要求：**逐步清零 warning；对主包、Cesium、首屏资源设体积预算并在 PR 中报告增量。

#### CI-P2-02：重复的 Admin 独立工作流

`ci-admin.yml` 与 `ci.yml` 都运行前端质量门。主 CI 的 package 已依赖 `ci.yml` 内的 admin job，独立工作流不会成为 package 的依赖，可能造成重复消耗或状态解释不一致。

**整改要求：**明确 `ci-admin.yml` 是快速反馈还是 required check；若保留，统一脚本和 Node 版本，并在分支保护中只选择一个权威检查。

## 4. CD 严格审核

### 4.1 已达标项

1. **依赖 CI 成功**：`cd.yml` 由 CI workflow 的 `workflow_run` 触发，并检查成功结论。
2. **目标 SHA 固定**：从 `workflow_run.head_sha` 获取 40 位小写 SHA，checkout 同一提交，artifact 的 `version.txt` 也必须匹配。
3. **服务器不构建**：CD 只下载、校验、上传和切换制品，不执行 `git pull`、`npm ci` 或 `go build`。
4. **并发受控**：`tsloms-production` concurrency 且 `cancel-in-progress: false`，避免两个发布同时改写 current/previous。
5. **传输和落位安全**：先传到 `<sha>.staging`，manifest 校验通过后再落位；当前脚本还会在制品下载丢失执行位时显式 `chmod +x server`。
6. **原子切换和回滚**：通过 `current.next` 与 `mv -Tf` 切换，保留 previous；systemd 重启后连续探活，失败切回 previous。
7. **入口验证**：服务器本机检查后，workflow 还检查 `/tsloms/health` 和 `/tsloms/admin/`。

### 4.2 阻断或高风险项

#### CD-P0-01：数据库迁移不是可逆发布事务

`release-install.sh` 会尝试在迁移前执行 `mysqldump`，但缺少 `/etc/tsloms/tsloms.env`、`DB_NAME` 或 `DB_PASSWORD` 时只打印 WARN 并继续部署。应用启动时仍会执行 GORM `AutoMigrate` 及数据回填/清理逻辑。仓库中没有版本化迁移目录、迁移版本表、跨实例锁、超时、失败补偿或恢复演练记录。

这意味着二进制可以回滚，数据库结构和数据却未必能回滚；“探活成功”也不能证明迁移安全。

**整改要求：**

- 将迁移改为显式版本化 migration job，不在普通服务启动时隐式变更数据库；
- 迁移前备份缺失或失败必须 fail closed；
- 使用数据库锁/分布式锁保证单实例迁移；
- 为大表迁移设置超时、向前兼容策略和回滚/恢复 runbook；
- 至少完成一次真实备份恢复和迁移失败演练，并保存时间线。

#### CD-P1-01：已存在 release 时跳过完整复核

脚本发现 `/opt/tsloms/releases/<sha>` 已存在时直接输出 WARN 并使用该目录，跳过 manifest、结构、version 和执行权限校验。若目录被人工修改、磁盘损坏或上一次发布留下半成品，重复部署可能运行未验证内容。

**整改要求：**已存在目录也必须重新校验 manifest、version、结构和可执行文件；校验失败应拒绝部署。更稳妥的做法是记录安装清单和签名，并禁止覆盖同 SHA 的内容。

#### CD-P1-02：生产 systemd 单元存在两套不一致基线

`deploy/systemd/tsloms-server.service` 使用 `User=tsloms`、`/etc/tsloms/tsloms.env`、强化沙箱；`tsloms-server.prod-fitted.service` 使用 `User=root`、`/opt/tsloms/packages/server/.env`。CD 只执行 `systemctl restart tsloms-server`，不会验证服务器实际启用的是哪一个单元，也不会在部署前校验 `ExecStart`、EnvironmentFile、运行用户和 `MEDIA_DIR`。

**整改要求：**保留一套唯一生产单元；部署前通过 SSH 输出并校验 `systemctl cat tsloms-server`、`systemctl show`、EnvironmentFile 权限和 current 路径；禁止生产继续使用 root 单元或旧 `.env` 路径。

#### CD-P1-03：手动部署的 artifact 选择仍依赖“最新成功 CI”

workflow_dispatch 若不提供 SHA，会使用 `github.sha`；定位 artifact 时却查询 main 上最新成功 CI。脚本随后靠 `version.txt` 检查不一致并失败，属于安全失败，但不是清晰的目标绑定逻辑。

**整改要求：**手动部署必须先验证目标 SHA 对应的成功 CI run，且 `workflow_run.head_sha`、artifact run、`version.txt` 三者严格相等；找不到对应 run 时直接给出明确错误，不查询“最新成功”作为替代。

#### CD-P1-04：SSH host key 未固定

SSH 上传使用 `StrictHostKeyChecking=no`，appleboy action 也没有传入 `fingerprint`。部署密钥虽然来自 Environment secret，但不能抵抗首次连接或 DNS/网络层中间人风险。

**整改要求：**配置并校验固定 host fingerprint，删除 `StrictHostKeyChecking=no`；部署用户使用最小权限和 sudoers 白名单，禁止 root SSH。

#### CD-P2-01：Nginx 仅 test，不自动 reload 和验证配置来源

发布脚本执行 `nginx -t` 失败时只打印 WARN，不让部署失败，也不会 reload。若 current 切换后前端或媒体 alias 不匹配，CD 的入口探活可能覆盖不到所有静态资源。

**整改要求：**在确认配置由版本化文件管理后，将 `nginx -t` 作为阻断项；按变更需要执行受限 reload，并增加一个静态资源和一个只读 API 的验收。

## 5. CO 严格审核

### 5.1 已达标项

- 每 10 分钟执行一次，可手动 dispatch。
- API 健康和后台入口均使用 `curl --fail`，失败会使 job 失败。
- 失败时按 label、分页查找并去重告警 issue。
- 恢复时添加评论并关闭同一标签下的告警。
- job 显式声明 `contents: read` 与 `issues: write`，符合最小 GitHub 权限。
- 最新运行 `32201862284` 以及此前多次定时运行成功。

### 5.2 未达标项

#### CO-P1-01：MQTT 只做 TCP 探测

`/dev/tcp/<host>/1883` 只能说明端口接受 TCP 连接，不能说明 EMQX 认证可用、ACL 正确、TLS/明文策略正确、服务能收发消息，也不能发现“端口开着但业务不可用”。

**整改要求：**准备只允许测试主题的探针账号，在隔离主题执行 CONNECT、PUBLISH、SUBSCRIBE、接收回环和断开；探针凭据只存在于 Environment secret，日志不得打印。

#### CO-P1-02：缺少数据库、Redis、主机和备份年龄巡检

当前 CO 只覆盖外部 API、前端和 MQTT TCP 端口，没有数据库连接、Redis PING、磁盘/内存、systemd 重启次数、journal 错误量、最近备份时间和备份可读性。

**整改要求：**分层增加 L2 数据依赖、L3 MQTT 业务、L4 主机/备份指标，并把结果写入脱敏摘要或外部监控系统。

#### CO-P2-01：GitHub cron 不是秒级生产监控

cron 可能延迟，且 issue 不是值班通知渠道。关键生产告警还需要腾讯云监控、Prometheus/Alertmanager 或同等系统，并明确值班责任人和升级路径。

## 6. 安全、权限和供应链审核

### 6.1 当前优点

- 仓库当前 CI/CD 文档和模板未发现应继续使用的生产明文凭据。
- CI、CD、CO 已声明基础 GitHub token 权限。
- 生产 Compose 将 MySQL、Redis、EMQX Dashboard 绑定到本机/内网，MQTT 1883 另行开放并启用认证配置。
- 标准 systemd 单元启用 `NoNewPrivileges`、`ProtectSystem=strict`、`ProtectHome`，运行用户为 `tsloms`。

### 6.2 仍需现场确认

以下项目不能由 Git 仓库单独证明，必须提交生产证据：

1. GitHub `production` Environment 是否配置 required reviewers、main 分支限制和正确 Secrets。
2. SSH host fingerprint 是否固定；部署用户是否禁止 root 登录、仅可写指定目录。
3. `/etc/tsloms/tsloms.env` 是否为 `0600`，owner 是否正确，是否与实际 systemd 单元一致。
4. 8093、MySQL、Redis、EMQX Dashboard 是否未暴露公网；1883 是否有安全组、认证和 ACL 三重限制。
5. 数据库、媒体和制品备份是否有异地副本、保留周期、加密和恢复记录。
6. GitHub audit log、Actions 日志、artifact 和服务器 shell history 是否无敏感值泄露。

## 7. 复测与验收建议

### 7.1 发布前必须完成的 P0

| 编号 | 整改 | 验收证据 |
|---|---|---|
| P0-01 | 版本化数据库迁移 + 单实例锁 + fail-closed 备份 | migration 运行日志、备份文件、恢复成功日志、失败注入记录 |
| P0-02 | 已存在 release 重新校验并禁止同 SHA 内容漂移 | 篡改 release 后重复部署被拒绝 |
| P0-03 | 统一唯一 systemd 单元和环境文件路径 | `systemctl cat/show` 脱敏输出、权限和 current 路径验收 |
| P0-04 | 固定 SSH host fingerprint、部署用户和 sudoers | SSH 连接日志、sudoers 白名单、越权命令拒绝记录 |

### 7.2 P1 质量和运维补测

| 编号 | 补测 |
|---|---|
| P1-01 | 固定所有 Action SHA，增加 gitleaks、govulncheck、SBOM 和制品签名验证 |
| P1-02 | 接入隔离 E2E，覆盖 health、登录、仪表盘、一个只读业务 API、媒体访问 |
| P1-03 | 增加 MQTT 认证探针及数据库/Redis/主机/备份巡检 |
| P1-04 | 故障注入：错误制品、服务启动失败、健康失败、迁移失败、磁盘不足、网络中断 |
| P1-05 | 完成 current/previous 回切和数据库恢复演练，记录 RTO/RPO |

### 7.3 建议的最终通过标准

连续至少 3 次 main CI、CD 和 CO 成功；覆盖率保持不低于 80%；P0 全部关闭；一次完整发布、一次自动回滚、一次人工回滚、一次数据库恢复和一次告警恢复均有可审计记录；外部生产配置逐项提供证据后，才可将总体结论改为“完全达标”。

## 8. 结论与版本记录

### 8.1 本次审核结论

**结论：部分达标。**

CI 质量门、覆盖率门禁、不可变制品、按 SHA 的 CD、原子切换、探活回滚和基础 CO 已达到可运行标准；数据库发布事务、制品重用防篡改、供应链证明、E2E、MQTT 深度探针和生产外部控制仍未达到严格生产级闭环。

### 8.2 版本记录

- v3.0（2026-08-19）：基于 `f9f1dfc`、GitHub CI/CD/CO 实际运行记录和本地后端全量测试重审；确认 CI 覆盖率 80.60%、CD 运行 32200820495、CO 运行 32201862284；新增制品重用校验、AutoMigrate/备份事务、systemd 双单元、E2E、MQTT 深度探针、Action 固定、SBOM/签名和外部生产证据项。
- v2.0（2026-08-18）：完成不可变制品、SHA-256 manifest、workflow_run CD、原子切换/回滚和基础 CO 设计。
- v1.x：历史审核版本，继续保留，不作为当前达标依据。
