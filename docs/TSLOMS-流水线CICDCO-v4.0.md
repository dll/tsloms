# TSLOMS 流水线（CI / CD / CO / E2E）v4.0 落地总结与实施手册

> 文档性质：v3.0 严格审核报告的继承、整改闭环、生产落地复盘与新环境实施手册
>
> 编写日期：2026-08-20
>
> 仓库基线：`main` / `1f114105812b5541b5eb0308c6c9a78aa3b6c920`
>
> 生产版本探针：文档编写时线上为 `0.1.64` / `6fb0f5eb5c378ac8c4afe0b104b760d7e62d46e4`
>
> 前置文档：[TSLOMS-流水线CICDCO-v3.md](./TSLOMS-流水线CICDCO-v3.md)
>
> 安全说明：本文只记录 Secret 名称、文件路径、控制方法和脱敏证据，不记录密码、私钥、Token、完整主机指纹等敏感值。

---

## 1. 文档目标与结论

### 1.1 为什么需要 v4.0

v3.0 完成了严格审核，指出流水线虽然具备 CI、制品化 CD、基础 CO 的主骨架，但数据库迁移、供应链、SSH 主机校验、systemd 单元一致性、E2E、深度巡检和生产外部状态仍有明显缺口。

之后项目经历了多轮真实生产流水线修复。问题并非集中在一个脚本，而是依次暴露在以下边界：

```text
代码质量
   -> 制品生成
   -> Artifact 上传/下载
   -> 制品目录定位
   -> SHA-256 清单
   -> SSH 主机信任
   -> SSH 客户端实现差异
   -> 生产环境文件
   -> systemd 单元与运行用户
   -> 数据库备份权限
   -> 发布重试幂等性
   -> 外部 E2E 凭据
   -> E2E HTTP/HTTPS 协议
```

如果只修复当前日志的最后一条错误，下一层问题会在下一轮运行中继续出现。v4.0 的目标不是简单罗列提交，而是回答以下问题：

1. TSLOMS 当前流水线到底如何运行；
2. 每个环节实际落地了什么；
3. 漫长修复过程中每个错误的真实原因是什么；
4. 为什么此前容易连续失败；
5. 怎样在一个新环境中一步到位；
6. 怎样通过前置检查防止同类问题再次发生；
7. 哪些结论已由生产证明，哪些仍需继续治理。

### 1.2 当前总体结论

截至本版本：

- CI 质量门、覆盖率门禁、前端质量门、漏洞扫描、密钥泄漏扫描、不可变制品、SBOM 清单已经落地；
- CD 已形成“目标 SHA 固定、Artifact 下载、SHA-256 校验、严格 SSH 主机校验、流式上传、原子切换、数据库备份、systemd 重启、本机探活、外部版本探针”的完整主链路；
- 生产服务器已完成 `/etc/tsloms/tsloms.env`、`User=tsloms`、唯一 systemd 单元和共享媒体目录的一次性迁移；
- 已真实验证制品落位、数据库备份、原子切换、systemd 重启、健康探活和 Nginx 配置检查；
- E2E 已拆成外部 HTTP 冒烟与服务器本机 SSH 冒烟；服务器本机 E2E 已成功；
- 外部 E2E 的“缺少凭据”“错误使用 HTTPS 访问 HTTP 8092”和“站点根地址未补 API 前缀”问题均已修复；提交 `7007897`、`1f11410` 已推送，文档编写时等待 GitHub Actions 对最新提交完成最终复验；
- CO 已包含外部健康巡检和服务器深度探针，但深度探针的 SSH 实现仍建议后续统一为与 CD/E2E 相同的原生 OpenSSH 信任链；
- 数据库已具备版本表、单实例锁、DDL 前备份和回归测试，但数据库恢复演练、异地备份和 RTO/RPO 证据仍属于持续运维工作，不应只凭代码宣称永久达标。

因此，本报告的严格判定为：

> **CI、制品化 CD 和生产服务器基线已达到可审计、可重复发布标准；E2E 最新协议修复等待 Actions 最终复验；灾备恢复、外部监控和 CO SSH 统一属于后续持续治理项。**

### 1.3 v3.0 结论在 v4.0 中的处置

| v3.0 主要结论 | v4.0 状态 | 处置说明 |
|---|---|---|
| Go CI 与覆盖率门禁达标 | 已保持 | 固定 Go 1.26.6，覆盖率门禁不低于 80% |
| 前端 CI 达标但存在体积治理缺口 | 已整改 | 增加 bundle budget，超限阻断构建 |
| Action 使用可变 tag | 已整改 | 关键 Action 固定完整 commit SHA |
| 缺少 `govulncheck`、gitleaks、SBOM | 已整改 | CI 已接入漏洞、泄漏扫描和轻量 SBOM |
| E2E 未接入流水线 | 已整改 | CD 成功后自动触发外部与本机两类 E2E |
| 已存在 release 跳过复核 | 已整改 | 已存在 release 必须重新验证 manifest、结构、版本和执行位 |
| 手动部署未严格绑定目标 CI | 已整改 | 按目标 SHA 精确查找主 CI 成功 run |
| SSH host key 未固定 | 已整改（CD/E2E） | ED25519 扫描、SHA256 指纹比对、严格 `known_hosts` |
| systemd 两套基线不一致 | 已整改并迁移生产 | 唯一单元 `tsloms-server.service`，`User=tsloms` |
| 环境文件仍在旧路径 | 已整改并迁移生产 | `/etc/tsloms/tsloms.env`，`0600 root:root` |
| 数据库迁移不可控 | 已显著整改 | `schema_migrations`、有序迁移、MySQL `GET_LOCK`、DDL 前备份 |
| CO 只检查 TCP 端口 | 已扩展 | 深度探针覆盖 DB、Redis、磁盘、备份、制品和 MQTT 回环 |
| 生产外部状态无法由仓库证明 | 部分已现场验证 | systemd、环境文件、运行用户、入口健康和版本已验证；审批/异地灾备仍需平台证据 |

---

## 2. 当前流水线总体架构

### 2.1 主链路

```text
Developer Push to main
          |
          v
+-----------------------------+
| CI                          |
| - Go quality gate           |
| - Admin quality gate        |
| - Security scanning         |
| - Coverage >= 80%           |
| - Immutable artifact        |
+-----------------------------+
          |
          | workflow_run: success
          v
+-----------------------------+
| CD                          |
| - Resolve exact commit SHA  |
| - Download exact CI artifact|
| - Verify manifest/version   |
| - Verify SSH host key       |
| - Upload to SHA staging     |
| - Atomic release switch     |
| - Backup/restart/health     |
| - Verify production version |
+-----------------------------+
          |
          | workflow_run: success
          v
+-----------------------------+
| E2E                         |
| - External HTTP probe       |
| - Local SSH probe           |
| - Optional login/read paths |
+-----------------------------+

Every 10 minutes
          |
          v
+-----------------------------+
| CO                          |
| - API/admin/MQTT port       |
| - DB/Redis/systemd/disk     |
| - backup/artifact/MQTT loop |
| - incident issue lifecycle  |
+-----------------------------+
```

### 2.2 工作流文件与职责

| 文件 | 职责 | 触发方式 |
|---|---|---|
| `.github/workflows/ci.yml` | 后端、前端、安全、覆盖率和制品构建 | `push main`、PR、手动 |
| `.github/workflows/cd.yml` | 下载目标 SHA 制品并部署生产 | 主 CI 成功后的 `workflow_run`、手动 |
| `.github/workflows/e2e.yml` | 部署成功后的外部与本机冒烟 | CD 成功后的 `workflow_run`、手动 |
| `.github/workflows/co.yml` | 周期性持续运维巡检和告警闭环 | 每 10 分钟、手动 |
| `.github/workflows/ci-admin.yml` | 前端快速反馈工作流 | 按其独立触发规则 |

### 2.3 生产目录结构

```text
/opt/tsloms/
|-- bin/
|   |-- release-install.sh
|   `-- bootstrap-artifacts.sh
|-- releases/
|   |-- <commit-sha-A>/
|   |   |-- server
|   |   |-- admin/dist/
|   |   |-- manifest.sha256
|   |   |-- version.txt
|   |   `-- SBOM files
|   `-- <commit-sha-B>/
|-- current  -> releases/<current-sha>
|-- previous -> releases/<previous-sha>
|-- shared/
|   `-- media/
`-- backups/
    `-- db/

/etc/tsloms/
`-- tsloms.env

/etc/systemd/system/
`-- tsloms-server.service
```

### 2.4 生产唯一权威基线

生产服务必须同时满足：

```text
systemd service : tsloms-server.service
User            : tsloms
Group           : tsloms
WorkingDirectory: /opt/tsloms/current
ExecStart       : /opt/tsloms/current/server
EnvironmentFile : /etc/tsloms/tsloms.env
Environment mode: 0600 root:root
Backend health  : http://127.0.0.1:8093/api/v1/health
External entry  : http://<host>:8092/tsloms/
```

旧的 root 单元与 `/opt/tsloms/packages/server/.env` 只作为迁移来源和历史参考，不再是生产权威配置。

---

## 3. CI 落地详情

### 3.1 后端质量门

后端质量门按以下顺序运行：

1. 完整检出仓库历史，满足 gitleaks 历史扫描要求；
2. 安装并严格验证 Go 1.26.6；
3. 下载 Go Module 依赖；
4. `gofmt -l .` 必须无输出；
5. `go vet ./...`；
6. `govulncheck ./...`，发现可达漏洞直接阻断；
7. gitleaks 扫描 Git 历史和当前内容；
8. `go build ./...`；
9. `go test ./... -count=1`；
10. 生成合并覆盖率 profile；
11. 使用项目自有 `cmd/coveragecheck` 做浮点比较，覆盖率低于 80% 阻断；
12. 无论成功失败均尽量上传覆盖率报告。

关键经验：覆盖率门禁不能只依赖 shell 截断或包级简单平均。TSLOMS 使用统一 profile 和专用比较程序，避免 `80.9` 被错误截断、重复 block 被错误累计等问题。

### 3.2 前端质量门

前端质量门包括：

1. 固定 `.node-version`；
2. `npm ci`，确保依赖与 lockfile 一致；
3. ESLint；
4. `vue-tsc --noEmit`；
5. Vitest 单元测试；
6. Vite 生产构建；
7. `npm audit --omit=dev --audit-level=high`；
8. bundle budget 插件检查主入口、vendor、ECharts、Cesium 和首屏总体积。

体积预算不是普通 warning，而是构建硬门禁。合理增长必须通过修改预算表并在提交中说明原因，防止依赖或页面改动使首屏长期失控。

### 3.3 安全与供应链门禁

已落地控制：

- Action 固定完整 commit SHA，降低 tag 漂移和供应链替换风险；
- `govulncheck` 检查 Go 调用链上的已知漏洞；
- `npm audit` 检查生产前端依赖高危漏洞；
- gitleaks 扫描完整 Git 历史；
- 只对明确标识的测试密钥做最小范围豁免；
- CI 生成 Go Module 列表和前端 lockfile 作为轻量 SBOM；
- SBOM、`version.txt` 与业务文件全部纳入 SHA-256 manifest；
- GitHub Token 权限保持 `contents: read` 等最小权限。

仍可继续增强：标准 CycloneDX/SPDX、SLSA provenance、制品签名、签名验证和依赖许可策略。

### 3.4 不可变制品

只有后端和前端质量门全部成功后，`package` job 才运行。

制品包含：

```text
server
admin/dist/**
version.txt
manifest.sha256
deps-go.sbom.txt
deps-npm.lock.json
```

不包含：

- 生产 `.env`；
- SSH 私钥；
- GitHub Token；
- 数据库密码；
- 服务器本地编译工具；
- 发布脚本本身。

发布脚本由 CD 从目标提交单独上传到 `/opt/tsloms/bin/release-install.sh`。这样制品保持纯业务产物，服务器端发布逻辑也与目标提交严格对应。

### 3.5 自动版本标识

前端构建生成 `admin/dist/version.json`：

```json
{
  "version": "<major>.<minor>.<build>",
  "commit": "<40位提交SHA>",
  "build": "<GitHub Run Number>",
  "built_at": "<ISO时间>"
}
```

规则：

- 主、次版本来自 `packages/admin/package.json`；
- 每次 GitHub 构建使用 `GITHUB_RUN_NUMBER` 作为构建版本；
- `commit` 用于验证生产页面到底运行了哪次提交；
- `version.json` 进入 manifest，不能在部署后被随意替换；
- CD 最后必须读取生产 `version.json` 并确认 `commit == RELEASE_SHA`。

这解决了“页面看起来没变化，到底是没修改、没部署还是缓存”的长期判断困难。

---

## 4. CD 落地详情

### 4.1 精确绑定目标提交

自动 CD 使用主 CI `workflow_run.head_sha`。

手动 CD 必须：

1. 得到明确的 40 位提交 SHA；
2. 精确查询该 SHA 对应的主 CI；
3. 只接受 `completed + success` 的运行；
4. 不使用“main 最近一次成功 CI”作为替代；
5. 找不到该 SHA 的成功主 CI 时直接失败。

这是防止“代码 A 触发部署，却下载到代码 B 的最新成功制品”的核心控制。

### 4.2 Artifact 下载和目录定位

GitHub Artifact 下载后的层级可能因 Action 版本、上传路径和历史制品不同而变化。CD 不再假设固定目录，而是以 `server` 或 `version.txt` 的实际位置识别制品根目录。

识别完成后必须验证：

- `server` 存在；
- `admin/dist/index.html` 存在；
- `manifest.sha256` 存在或仅对明确兼容的历史制品重建；
- `version.txt` 与目标 SHA 相同；
- `admin/dist/version.json.commit` 与目标 SHA 相同；
- `sha256sum -c manifest.sha256` 全部通过。

### 4.3 SSH 信任链

CD 当前只使用一套原生 OpenSSH 信任链：

```text
DEPLOY_HOST
   -> ssh-keyscan ED25519 public host key
   -> ssh-keygen calculate SHA256 fingerprint
   -> compare DEPLOY_HOST_FINGERPRINT
   -> write verified public key to known_hosts
   -> StrictHostKeyChecking=yes
   -> SSH auth preflight
   -> upload and execute release
```

必须区分两个概念：

- `DEPLOY_HOST_FINGERPRINT`：形如 `SHA256:...` 的摘要，只用于比较；
- `known_hosts`：包含主机名、算法和完整公钥的行，用于 OpenSSH 建立连接。

把 SHA256 摘要直接写进 `known_hosts` 是错误的；把完整主机公钥行填进只接受指纹摘要的 Action 参数同样可能失败。

### 4.4 流式上传

制品使用 `tar | ssh` 流式上传到：

```text
/opt/tsloms/releases/<sha>.staging
```

优势：

- 避免逐文件 SCP 造成半成品目录；
- 保持相对目录结构；
- 上传完成后才进行服务器端复核；
- 不要求服务器安装 Git、Go、Node 或 npm；
- 服务器不会执行 `git pull` 或在线构建。

### 4.5 服务器端发布事务

`release-install.sh` 主要步骤：

```text
[0] 校验 RELEASE_SHA 和目录
[1] 校验 staging 或复核已存在 release
[1.5] 一次性迁移媒体到 shared/media
[2] 检查生产 env 并备份数据库
[3] 记录 current 为 previous
[4] current.next 原子切换为新 release
[4] 校验 systemd User/EnvironmentFiles/ExecStart
[4] 重启服务并本机探活
[4] 失败则切回 previous
[5] nginx -t
[5] 本机健康接口验收
```

### 4.6 幂等与重试

正式 release 已经存在时，不再要求 `.staging` 同时存在，而是：

1. 重新执行完整 manifest 校验；
2. 验证 server 存在且可执行；
3. 验证前端 index 和 version.json；
4. 验证 `version.txt == RELEASE_SHA`；
5. 校验通过后允许继续发布。

因此，若第一次运行在“制品落位之后、服务切换之前”失败，重试仍可继续，而不是永久卡在“找不到 staging”。

### 4.7 数据库备份

发布前要求 `/etc/tsloms/tsloms.env` 存在。该文件缺失时 fail closed，禁止应用退回默认数据库密码或默认 JWT 密钥继续生产部署。

备份命令使用：

```text
mysqldump
  --single-transaction
  --routines
  --triggers
  --no-tablespaces
```

`--no-tablespaces` 的原因是应用专用数据库账号不应为了备份获得全局 `PROCESS` 权限。禁用 tablespaces 元数据仍可备份业务数据库，同时遵守最小权限原则。

### 4.8 原子切换与回滚

切换流程：

```text
current.next -> releases/<new-sha>
mv -Tf current.next current
systemctl restart tsloms-server
health probe
```

失败流程：

```text
health failed
   -> current.next -> previous target
   -> mv -Tf current.next current
   -> restart service
   -> report rollback result
   -> exit non-zero
```

### 4.9 systemd 强制校验

发布前不是只执行 restart，而是读取实际启用单元：

```text
User             == tsloms
EnvironmentFiles contains /etc/tsloms/tsloms.env
ExecStart         contains /opt/tsloms/current/server
```

注意：unit 文件中指令名是 `EnvironmentFile=`，但 `systemctl show` 的属性通常是复数 `EnvironmentFiles`。脚本优先读取复数，并对旧 systemd 做单数回退。

### 4.10 外部入口验收

CD 在服务器本机验收后，还从 GitHub Runner 检查：

- `/tsloms/health`；
- `/tsloms/admin/`；
- `/tsloms/admin/version.json`；
- `version.json.commit == RELEASE_SHA`。

这能够识别：

- Nginx 未更新；
- current 已切换但入口仍指向旧静态目录；
- CDN/缓存仍返回旧版本；
- 后端正常但前端没有部署；
- 部署目标 SHA 与页面实际版本不一致。

---

## 5. 数据库版本化迁移

### 5.1 v3 问题

v3 指出应用启动时直接执行无版本的 AutoMigrate，缺少版本表、单实例锁、DDL 前强制备份和恢复证据。这会导致多实例并发迁移、重复执行有副作用逻辑、二进制可回滚但数据库无法回滚等风险。

### 5.2 当前实现

当前已实现：

- `schema_migrations` 版本表；
- `0001` 至 `0004` 有序迁移；
- 每个版本成功后记录版本、名称、执行时间和执行者；
- MySQL 生产环境使用 `GET_LOCK`；
- 锁、迁移和 `RELEASE_LOCK` 绑定同一独占数据库连接；
- 第二实例获取锁超时后 fail closed；
- DDL 前执行 `mysqldump | zstd`；
- 生产备份目录固定为 `/opt/tsloms/backups/db`；
- 密码通过环境变量传给子进程，不写入 argv 或日志；
- 提供 SQLite 单元测试和 MySQL 并发锁集成测试。

### 5.3 仍需持续执行的工作

代码层迁移能力不等于灾备已经永久达标。仍应周期执行：

1. 最新备份完整性检查；
2. 临时库恢复演练；
3. 迁移失败注入；
4. 大表迁移耗时评估；
5. RTO/RPO 记录；
6. 异地备份和保留周期检查。

---

## 6. E2E 落地详情

### 6.1 两类 E2E

外部 E2E：

- 从 GitHub Runner 访问生产 Nginx；
- 验证真实公网/外部入口；
- 有测试凭据时覆盖验证码、登录、仪表盘和只读接口；
- 无测试凭据时至少完成公开 health 检查并给出 warning。

服务器本机 E2E：

- 使用严格 SSH 主机校验进入生产机；
- 访问 `127.0.0.1:8093`；
- 排除公网路由、安全组和入口代理的影响；
- 用于判断“应用本身是否健康”。

两者同时存在可以快速定位：

| 外部 E2E | 本机 E2E | 典型结论 |
|---|---|---|
| 成功 | 成功 | 发布和入口均正常 |
| 失败 | 成功 | Nginx、协议、DNS、安全组或外部网络问题 |
| 失败 | 失败 | 应用、数据库、Redis、配置或服务启动问题 |
| 成功 | 失败 | 检查目标或缓存异常，需要复核探针路径 |

### 6.2 E2E 凭据策略

完整登录测试使用：

```text
E2E_ADMIN_USER
E2E_ADMIN_PASS
```

它们必须位于 GitHub `production` Environment Secret 中。

未配置密码时：

- 不硬编码生产管理员密码；
- 不打印空密码或默认密码；
- 不因缺少可选测试账号而跳过全部入口检查；
- 只执行 `/health` 并输出 warning；
- E2E 结果明确标记为“健康检查模式”，不能误称完整认证链路已覆盖。

### 6.3 HTTP/HTTPS 协议策略

当前生产 8092 是 HTTP Nginx 入口。此前 E2E 默认使用：

```text
https://<host>:8092/tsloms/api/v1
```

Node fetch 因 TLS 握手失败只报告 `TypeError: fetch failed`。

当前策略：

1. 默认地址为 `http://<host>:8092/tsloms/api/v1`；
2. 如果历史 Secret 仍配置 `https://<host>:8092`，先探测 HTTPS；
3. HTTPS 失败且对应 HTTP health 成功时自动回退；
4. 对其它端口或明确外部地址不擅自改写；
5. 日志只打印协议和脱敏主机，不打印完整 URL、查询参数或凭据；
6. Node 网络错误只输出 `network-error=<类型>`。

未来正式配置 TLS 后，应把 `DEPLOY_BASE_URL` 和 `E2E_BASE_URL` 统一更新为真实 HTTPS 地址，并取消 8092 的临时兼容回退。

---

## 7. CO 持续运维落地详情

### 7.1 基础健康巡检

每 10 分钟检查：

- 外部 API health；
- 后台首页；
- MQTT 1883 TCP 端口。

失败时：

- 使用固定标题和 label 查找已有告警；
- 分页去重，避免重复创建大量 Issue；
- 无旧告警时创建生产故障 Issue。

恢复时：

- 在旧告警添加恢复时间评论；
- 自动关闭告警 Issue。

### 7.2 深度探针

服务器本机深度探针覆盖：

| 层级 | 检查 |
|---|---|
| L1 | systemd active、近 10 分钟 journal error 数 |
| L2 | MySQL `SELECT 1` |
| L3 | Redis `PING/PONG` |
| L4 | `/` 和 `/opt/tsloms` 磁盘使用率 |
| L5 | 最近数据库备份年龄 |
| L6 | `current` 符号链接及 release 目录 |
| L7 | MQTT CONNECT/PUBLISH/SUBSCRIBE 回环 |

探针不打印密码，只打印 PASS/WARN/FAIL 摘要。

### 7.3 CO 后续建议

当前深度探针仍使用 `appleboy/ssh-action`。虽然 Action 已固定 commit SHA，但建议下一步统一为 CD/E2E 已验证的原生 OpenSSH：

```text
ssh-keyscan
   -> fingerprint compare
   -> verified known_hosts
   -> native ssh
```

同时，GitHub cron 不能替代专业监控。生产还应配置腾讯云监控、Prometheus/Alertmanager 或同类告警渠道。

---

## 8. 漫长修复经历：问题、原因、解决与预防

本节按故障层级整理。提交号用于追溯，不代表每个提交都只包含单一问题。

### 8.1 制品版本不一致与页面无法确认是否更新

**现象**

- 页面样式修改后仍像旧版；
- 无法判断是代码未修改、部署失败还是浏览器缓存；
- CD 只验证入口 200，不能证明前端是目标提交。

**原因**

- 静态页面缺少构建版本探针；
- 入口 200 只能证明某个页面可访问；
- 浏览器缓存和 Nginx 静态目录可能继续返回旧制品。

**解决**

- 增加关于页面；
- 自动生成构建号、提交 SHA 和构建时间；
- 生成 `version.json`；
- CD 校验 `version.txt`、制品 `version.json` 和生产 `version.json`；
- 相关提交：`98f7c88`、`fd05b32`、`5253f2a`。

**预防**

- 每个可部署前端都必须有机器可读版本探针；
- 部署成功条件必须包含生产探针与目标 SHA 相等；
- UI 是否变化不应依赖人工肉眼判断。

### 8.2 Artifact 目录层级不一致

**现象**

- 下载后找不到 `server`、`version.txt` 或 manifest；
- 实际文件存在，但多包了一层目录。

**原因**

- 上传 path、下载 Action 和历史制品结构不完全一致；
- CD 假设制品根固定为 `./release`。

**解决**

- 搜索 `server` 或 `version.txt` 的实际目录；
- 写入 `RELEASE_ROOT`；
- 校验失败时打印有限层级的目录树；
- 相关提交：`c9d4da8`。

**预防**

- Artifact 根目录必须由标志文件定位；
- 构建 job 应提供清晰的顶层结构；
- CI 中增加“上传前清单”和 CD 中“下载后清单”。

### 8.3 历史制品缺失 manifest 或 version.json

**现象**

- 新 CD 逻辑部署旧 CI Artifact 时直接失败；
- 缺少新版本才引入的清单文件。

**原因**

- 工作流升级与历史 Artifact 生命周期交叉；
- 手动重跑 CD 可能引用整改前制品。

**解决**

- 对明确历史制品提供受控兼容路径；
- 缺失 manifest 时按实际文件重建并立即校验；
- 缺失 version.json 时按目标 SHA 生成 legacy 标识；
- 新制品必须严格携带这些文件；
- 相关提交：`a7c9665`、`e202a90`。

**预防**

- 工作流结构变化时记录兼容窗口；
- Artifact 保留期较短，避免长期兼容历史包；
- 兼容逻辑必须只补元数据，不能跳过实际内容校验。

### 8.4 SBOM 文件缺失导致上传命令失败

**现象**

- tar 上传阶段因指定文件不存在而退出；
- 某些历史制品没有 SBOM。

**原因**

- 上传文件列表固定包含后来新增的可选文件；
- 没有区分强制制品与可选审计附件。

**解决**

- `server`、`admin`、manifest、version 为必选；
- SBOM 文件存在时才加入上传数组；
- 相关提交：`b0b5776`。

**预防**

- 对文件按 required/optional 分类；
- required 缺失立即报错，optional 缺失明确 warning；
- 不使用可能被 shell 空展开的宽泛 glob。

### 8.5 隐藏文件进入 manifest，但未上传 Artifact

**现象**

- `.bundle-report.json` 被写入 `manifest.sha256`；
- GitHub Artifact 默认未上传隐藏文件；
- CD 执行 `sha256sum -c` 报文件缺失。

**原因**

- manifest 枚举了 `release/admin` 下所有文件；
- Artifact 行为与本地目录枚举规则不一致。

**解决**

- 生成 manifest 时排除隐藏文件；
- 相关提交：`8d06267`。

**预防**

- manifest 的输入集合必须与 Artifact 实际上传集合完全一致；
- 上传后在 CI 内重新下载一次自校验是更稳妥的增强方案；
- 构建诊断报告与生产制品应分开上传。

### 8.6 SSH `known_hosts` 写入错误

**现象**

```text
No ED25519 host key is known ...
Host key verification failed
```

**原因**

- 把 `SHA256:...` 指纹摘要直接写进 `known_hosts`；
- OpenSSH 需要的是完整主机公钥行。

**解决**

- `ssh-keyscan` 获取完整 ED25519 主机公钥；
- `ssh-keygen -lf -E sha256` 计算指纹；
- 摘要与 Secret 比对成功后，将扫描到的完整公钥写入 `known_hosts`；
- 相关提交：`9836bf4`。

**预防**

- 配置说明必须明确“指纹”和“known_hosts 行”不是同一种数据；
- SSH 连接前增加只读预检；
- 禁止使用 `StrictHostKeyChecking=no` 绕过。

### 8.7 原生 SSH 已通过，但 `appleboy/ssh-action` 指纹不匹配

**现象**

- 同一个 CD job 前半段原生 SSH 登录、上传成功；
- 后半段 `appleboy/ssh-action` 报 `host key fingerprint mismatch`。

**原因**

- 一条流水线混用了两套 SSH 客户端和两套指纹解析；
- Secret 格式在原生 OpenSSH 与第三方 Action 中的含义不完全一致；
- 前半段成功已经证明网络、私钥、用户和主机可用，错误只在第二套实现。

**解决**

- 移除 CD 中第二次 `appleboy/ssh-action`；
- 原子发布步骤复用前面已验证的私钥和 `known_hosts`；
- E2E SSH 同样改为原生 OpenSSH；
- 相关提交：`e73d8cc`、`4da7ac9`。

**预防**

- 一条发布链路只保留一种 SSH 信任实现；
- 上传、执行和 E2E 应复用同一套主机信任逻辑；
- 第三方 Action 即使固定 SHA，也要验证其输入格式和行为。

### 8.8 `EnvironmentFile` 与 `EnvironmentFiles` 属性误判

**现象**

- 日志显示 `EnvironmentFile=` 为空；
- 发布脚本认为 systemd 没有配置环境文件并退出。

**原因**

- unit 指令写作 `EnvironmentFile=`；
- `systemctl show` 通常暴露的属性名却是复数 `EnvironmentFiles`；
- 脚本查询了错误属性名。

**解决**

- 优先读取 `EnvironmentFiles`；
- 为空时兼容读取 `EnvironmentFile`；
- 相关提交：`51bf3c5`。

**预防**

- systemd 校验脚本要在目标发行版上实测 `systemctl show` 输出；
- 不要仅根据 unit 文件指令名推测 D-Bus 属性名；
- 关键外部命令输出应有兼容测试。

### 8.9 `/etc/tsloms/tsloms.env` 不存在

**现象**

- 发布脚本提示生产环境文件不存在；
- 数据库备份被跳过；
- systemd 新单元无法满足权威路径要求。

**原因**

- 旧生产环境仍使用 `/opt/tsloms/packages/server/.env`；
- 新设计要求敏感配置脱离 release 并固定到 `/etc/tsloms/tsloms.env`；
- 代码整改先于服务器一次性迁移。

**解决**

- 只读确认旧 `.env` 存在且权限正确；
- 不打印内容，仅检查必要 key 是否存在；
- 创建 `/etc/tsloms`；
- 复制旧配置为 `/etc/tsloms/tsloms.env`；
- 设置 `0600 root:root`；
- 发布脚本改为缺失时 fail closed；
- 相关提交：`acc6fb9`。

**预防**

- 在启用新 CD 门禁前先运行生产 bootstrap；
- 环境文件从来不应作为 Artifact 上传；
- 新环境上线清单必须把 Secret、服务器 env 和 systemd 单元列为零阶段。

### 8.10 生产机没有 `tsloms` 系统用户

**现象**

```text
Failed to determine user credentials
status=217/USER
```

**原因**

- 仓库已把 systemd 单元改为 `User=tsloms`；
- 生产机仍是历史 root 服务，没有创建该系统用户。

**解决**

- 自动回滚旧单元，保持服务 active；
- 创建无登录 shell 的系统用户和组；
- 把共享媒体目录所有权调整为 `tsloms:tsloms`；
- 重新安装唯一权威单元并重启；
- 验证 `User=tsloms`、环境文件、ExecStart 和 health。

**预防**

- systemd 单元安装前检查 `id tsloms`；
- bootstrap 脚本负责创建用户、目录、权限和环境文件；
- 配置迁移必须带回滚函数，不能直接覆盖后祈祷服务成功。

### 8.11 数据库专用账号缺少 `PROCESS` 权限

**现象**

```text
mysqldump: Access denied; you need PROCESS privilege ... tablespaces
```

**原因**

- MySQL 新版本 mysqldump 默认尝试读取 tablespace 元数据；
- 应用专用账号遵循最小权限，没有全局 `PROCESS`；
- 直接授予 PROCESS 会扩大生产账号权限。

**解决**

- 增加 `--no-tablespaces`；
- 单独用生产凭据验证完整备份命令成功；
- 不提升数据库账号权限；
- 相关提交：`031bfa1`。

**预防**

- 在上线前用“与生产相同权限”的账号运行真实备份命令；
- 数据库客户端版本升级后重新验证备份参数；
- 备份成功不能只看文件存在，还应检查管道退出码和压缩完整性。

### 8.12 release 已落位后重试仍要求 `.staging`

**现象**

- 第一次运行已执行 `staging -> release`；
- 后续步骤失败；
- 重试时 `.staging` 已不存在，脚本在进入“release 已存在”分支前退出。

**原因**

- 前置判断顺序错误；
- 幂等性只考虑“全程成功后重复执行”，没有考虑“中间状态失败后重试”。

**解决**

- 先判断正式 release 是否存在；
- 存在则完整复核并继续；
- 只有 release 不存在时才要求 staging；
- 相关提交：`031bfa1`。

**预防**

- 发布脚本必须针对每个阶段设计重入状态；
- 故障注入至少覆盖“上传后失败”“落位后失败”“切换后失败”；
- 不应通过临时放宽完整性门禁解决幂等问题。

### 8.13 E2E 未配置管理员密码

**现象**

```text
ERROR: 未提供管理员密码
Process completed with exit code 2
```

**原因**

- `E2E_ADMIN_PASS` 未配置；
- 冒烟脚本把完整登录凭据设成所有检查的硬前置；
- 即使公开 health 可用，也直接失败。

**解决**

- 无凭据时执行公开 health；
- 输出明确 warning；
- 有凭据时自动执行完整认证链路；
- 不硬编码管理员密码；
- 相关提交：`6fb0f5e`。

**预防**

- 将测试分为匿名基础门和认证增强门；
- 必选 Secret 缺失应 fail closed，可选 Secret 缺失应降级并明确标记覆盖范围；
- 生产管理员账号不应直接作为长期探针账号，建议创建只读 E2E 专用账号。

### 8.14 E2E 用 HTTPS 访问实际 HTTP 的 8092

**现象**

```text
E2E_BASE_URL: https://***:8092/tsloms/api/v1
TypeError: fetch failed
```

**原因**

- E2E 默认地址写成 HTTPS；
- 当前生产 8092 是 HTTP；
- Node fetch 在 TLS 握手阶段失败，错误信息较笼统；
- CD 和 CO 默认使用 HTTP，但 E2E 使用 HTTPS，三个工作流基线不一致。

**解决**

- E2E 默认改为 HTTP；
- 对历史 Secret 中的 `https://host:8092` 做受控探测回退；
- Node 冒烟捕获网络异常并输出脱敏类型；
- 不打印完整 URL、Token 或响应内容；
- 相关提交：`7007897`、`1f11410`。

**预防**

- `DEPLOY_BASE_URL` 应成为 CD、E2E、CO 的单一入口来源；
- 协议、端口、路径必须作为上线前连通性矩阵统一验证；
- 从 Runner 视角实际执行 curl，而不是仅凭 Nginx 文件推测。

### 8.15 GH_TOKEN 失效与 406

**现象**

- `gh auth login --web` 网络超时；
- `--with-token` 验证返回 HTTP 406；
- 本地 gh 无法查看完整 Actions 日志。

**原因**

- 本地到 GitHub Device Flow/API 的网络和代理链路不稳定；
- 旧 Token 已删除或失效；
- 406 不代表 TSLOMS GitHub Actions 内置 `GITHUB_TOKEN` 同时失效；
- Git 推送走 SSH，与本地 gh Token 是两条独立链路。

**解决**

- 暂停本地 GH_TOKEN 配置；
- 保持 Git SSH 推送；
- 通过 GitHub 网页读取 Actions 日志；
- 流水线内部继续使用 GitHub 自动注入的 `secrets.GITHUB_TOKEN`。

**预防**

- 区分本地 `gh` 登录、Git SSH 推送、Actions `GITHUB_TOKEN` 和项目 PAT；
- 不因本地 gh 失败修改生产流水线 Token；
- PAT 只授予所需范围，过期时间和用途登记；
- 不在命令行历史或截图中暴露 Token。

---

## 9. 为什么修复过程会很长

### 9.1 问题是分层暴露的

早期错误会阻止后续代码运行。例如：

- Artifact 校验失败时，不可能看到 SSH 错误；
- SSH 失败时，不可能看到 systemd 错误；
- systemd 门禁失败时，不可能看到 E2E 协议错误。

所以每修复一层，下一层才会第一次获得真实执行机会。

### 9.2 仓库状态与生产状态不同步

仓库已经定义：

- `User=tsloms`；
- `/etc/tsloms/tsloms.env`；
- 制品化 current/release 目录。

但生产仍保留：

- root 运行用户；
- 旧 `.env` 路径；
- 未创建 `tsloms` 用户。

只提交代码不会自动完成这些一次性服务器迁移。

### 9.3 多套工具对相同概念的格式要求不同

典型例子：

- OpenSSH `known_hosts` 需要完整主机公钥；
- Secret 中保存的是 SHA256 指纹；
- 第三方 SSH Action 对 fingerprint 又有自己的解析。

概念看似相同，数据格式却不同。

### 9.4 历史 Artifact 与新工作流共存

工作流频繁修改期间，可能出现：

- 新 CD 部署旧 Artifact；
- 旧制品没有新元数据；
- 手动重跑引用历史 run；
- 同一 SHA 已在服务器落位但发布未完成。

如果没有兼容边界和幂等重试设计，排查会反复绕圈。

### 9.5 日志最初缺少阶段化诊断

仅显示 `exit code 1` 时，很容易把问题归因到网络、Token、SSH 或缓存。后来增加：

- SSH 预检；
- 制品根目录打印；
- 版本探针；
- systemd 属性打印；
- network-error 类型；
- 每个发布阶段编号。

定位速度才显著提高。

---

## 10. 新环境如何一步到位

本节是 v4.0 最重要的实施顺序。不要先推 main 再边跑边补服务器。

### 10.1 阶段 0：冻结参数与单一事实源

在部署前确认并记录：

| 参数 | 单一来源 |
|---|---|
| 生产入口 | `DEPLOY_BASE_URL` |
| E2E 入口 | `E2E_BASE_URL`，未设置时继承生产入口规则 |
| 部署主机 | `DEPLOY_HOST` |
| 部署用户 | `DEPLOY_USER` |
| SSH 私钥 | `DEPLOY_SSH_KEY` |
| SSH 指纹 | `DEPLOY_HOST_FINGERPRINT` |
| 应用环境 | `/etc/tsloms/tsloms.env` |
| systemd 单元 | `deploy/systemd/tsloms-server.service` |
| 发布根目录 | `/opt/tsloms` |

协议、端口和路径必须统一，不允许 CD 用 HTTP、E2E 用 HTTPS、CO 又使用第三个地址。

### 10.2 阶段 1：生产服务器 bootstrap

先完成：

1. 创建 `tsloms` 系统用户，无登录 shell；
2. 创建部署用户并配置 SSH 公钥；
3. 创建 `/opt/tsloms/releases`、`bin`、`shared/media`、`backups/db`；
4. 设置正确 owner、group、ACL；
5. 创建 `/etc/tsloms`；
6. 安全写入 `/etc/tsloms/tsloms.env`；
7. 设置 `0600 root:root`；
8. 安装唯一 systemd 单元；
9. `daemon-reload`；
10. 配置部署用户 sudoers 白名单；
11. 验证 Nginx 入口；
12. 确认 MySQL、Redis、MQTT 客户端工具和 zstd。

如果是旧环境迁移：

- 先备份旧 systemd 单元；
- 只读检查旧 `.env` 必要 key；
- 复制到新路径，不在日志打印内容；
- 新单元失败时自动恢复旧单元；
- 创建 `tsloms` 用户后再切换服务。

### 10.3 阶段 2：SSH 信任预验证

在 GitHub Actions 之前完成：

1. 从可信渠道核对 ED25519 主机指纹；
2. 将纯 SHA256 摘要写入 `DEPLOY_HOST_FINGERPRINT`；
3. 不把完整公钥行误填为指纹；
4. 使用与 Runner 兼容的 OpenSSH 算法验证连接；
5. 检查部署用户能写 releases/bin；
6. 检查 sudoers 允许且只允许指定命令。

### 10.4 阶段 3：GitHub production Environment

至少配置：

```text
DEPLOY_HOST
DEPLOY_USER
DEPLOY_SSH_KEY
DEPLOY_SSH_PASSPHRASE      # 私钥无口令时可为空
DEPLOY_HOST_FINGERPRINT
DEPLOY_BASE_URL
E2E_BASE_URL               # 可选
E2E_ADMIN_USER             # 可选但推荐
E2E_ADMIN_PASS             # 可选但推荐
```

并配置：

- production Environment；
- main 分支限制；
- 必要时 required reviewers；
- Secret 修改审计；
- 不在 Repository Variable 中保存敏感值。

### 10.5 阶段 4：上线前矩阵测试

必须从正确视角分别验证：

| 视角 | 检查 |
|---|---|
| 开发机 | Git SSH push、生产入口 HTTP 状态 |
| GitHub Runner | 主机 22、生产入口 8092、Artifact 下载 |
| 生产机本机 | 8093 health、MySQL、Redis、MQTT |
| systemd | User、EnvironmentFiles、ExecStart |
| 数据库 | mysqldump 专用账号权限和 `--no-tablespaces` |

### 10.6 阶段 5：制品契约测试

在第一次正式部署前，检查制品必须包含：

```text
server
admin/dist/index.html
admin/dist/version.json
version.txt
manifest.sha256
```

并测试：

- Artifact 上传后再下载；
- 隐藏文件规则；
- Unix 执行位恢复；
- manifest 全量通过；
- 版本 SHA 一致。

### 10.7 阶段 6：首次发布演练

首次发布不要直接以“最终成功”为唯一目标，而要主动验证：

1. 正常发布；
2. 同 SHA 重试；
3. 已落位 release 无 staging 的重试；
4. 损坏 manifest 被拒绝；
5. 健康失败自动回滚；
6. previous 可读；
7. version.json 与目标 SHA 一致；
8. 备份文件生成且可解压检查。

### 10.8 阶段 7：E2E 和 CO

发布成功后：

- 外部 E2E 应访问与 CD 相同协议的生产入口；
- 本机 E2E 应验证应用本体；
- 无 E2E 密码时只能宣称基础健康通过；
- 完整达标应配置只读探针账号；
- CO 应至少连续运行三次；
- 故障 Issue 创建、去重、恢复关闭必须演练。

---

## 11. 发布前自动检查建议

为了减少人工错误，建议增加独立 `preflight` job，在下载/上传大制品前完成：

```text
1. Secret 非空检查（只检查长度，不打印值）
2. DEPLOY_BASE_URL 协议/端口/路径检查
3. SSH ED25519 指纹扫描与比对
4. SSH auth preflight
5. systemctl show 关键属性
6. /etc/tsloms/tsloms.env 存在和权限检查
7. id tsloms
8. releases/backups/shared 目录权限检查
9. mysqldump --no-data/--no-tablespaces 预检
10. nginx -t
11. 生产磁盘空间阈值
```

任何检查失败都应在制品上传和 current 切换之前退出。

---

## 12. 故障定位顺序

遇到流水线失败时，按层定位，不要先猜 Token 或缓存。

### 12.1 CI 失败

```text
Go version
 -> dependency download
 -> format/vet
 -> govulncheck/gitleaks
 -> build/test/coverage
 -> admin lint/test/build/audit
 -> package
```

### 12.2 CD 下载/校验失败

检查：

1. 目标 SHA；
2. CI run_id；
3. Artifact 名；
4. 实际制品根目录；
5. required 文件；
6. manifest 中是否包含未上传隐藏文件；
7. `version.txt` 和 `version.json`。

### 12.3 SSH 失败

按顺序判断：

```text
TCP 22 reachable?
 -> host key scanned?
 -> actual fingerprint equals expected?
 -> known_hosts contains full public key?
 -> private key readable?
 -> username correct?
 -> SSH preflight succeeds?
```

不要因为主机校验失败就关闭严格校验。

### 12.4 服务器脚本失败

根据阶段编号判断：

- `[0]`：SHA 或目录；
- `[1]`：manifest、结构、版本；
- `[2]`：env 或数据库备份；
- `[3]`：previous 记录；
- `[4]`：systemd、重启、health、回滚；
- `[5]`：Nginx 或入口验收。

### 12.5 E2E 失败

先比较两类 job：

- 本机成功、外部失败：入口协议、Nginx、网络、安全组；
- 两者均失败：应用或依赖；
- 日志显示密码空：Environment Secret；
- `network-error=TypeError`：优先核对协议和端口；
- HTTP 状态非 200：再看路径和鉴权。

---

## 13. 敏感信息保护规则

### 13.1 禁止打印

- `GH_TOKEN`、PAT；
- SSH 私钥；
- 数据库密码；
- JWT Secret；
- Redis/MQTT 密码；
- 登录 Token；
- 验证码提交体；
- 完整生产环境文件；
- 完整私有 URL 中的凭据或查询参数。

### 13.2 允许打印的脱敏证据

- Secret 是否存在；
- 文件权限和 owner；
- 主机由 GitHub 自动掩码后的值；
- 指纹是否匹配；
- HTTP 状态；
- 网络错误类型；
- systemd User、EnvironmentFiles 路径、ExecStart；
- commit SHA、构建号和构建时间；
- PASS/WARN/FAIL 数量。

### 13.3 传递方式

- GitHub Secret 通过环境变量注入；
- SSH 私钥写入 `$RUNNER_TEMP` 并 `chmod 600`；
- E2E 凭据不放在命令行参数；
- 远端探针通过标准输入传递；
- 数据库密码使用 `MYSQL_PWD` 临时环境变量并及时 unset；
- `/etc/tsloms/tsloms.env` 固定 `0600 root:root`。

---

## 14. 验收清单

### 14.1 CI

- [ ] Go 版本严格匹配；
- [ ] gofmt 无差异；
- [ ] go vet 通过；
- [ ] govulncheck 通过；
- [ ] gitleaks 通过；
- [ ] Go 全量测试通过；
- [ ] 覆盖率不低于 80%；
- [ ] 前端 lint/typecheck 通过；
- [ ] 前端单测通过；
- [ ] bundle budget 通过；
- [ ] npm audit 高危门禁通过；
- [ ] Artifact 上传成功。

### 14.2 CD

- [ ] 目标 SHA 为 40 位；
- [ ] CI run 与目标 SHA 一致；
- [ ] Artifact 内容完整；
- [ ] manifest 全部 OK；
- [ ] `version.txt` 一致；
- [ ] `version.json.commit` 一致；
- [ ] SSH 指纹匹配；
- [ ] SSH 预检成功；
- [ ] staging 上传成功；
- [ ] 数据库备份成功；
- [ ] systemd 三项校验通过；
- [ ] 本机 health 成功；
- [ ] Nginx 配置成功；
- [ ] 外部 API、后台和版本探针成功。

### 14.3 E2E

- [ ] 外部入口协议正确；
- [ ] 外部 health 通过；
- [ ] 本机 health 通过；
- [ ] 若已配置 E2E 凭据，登录通过；
- [ ] 仪表盘只读接口通过；
- [ ] 通知和设备只读接口通过；
- [ ] 日志无密码、Token 或完整敏感响应。

### 14.4 CO

- [ ] API 和后台巡检；
- [ ] MQTT 端口；
- [ ] systemd active；
- [ ] MySQL；
- [ ] Redis；
- [ ] 磁盘；
- [ ] 备份年龄；
- [ ] current 链接；
- [ ] MQTT 认证回环；
- [ ] 告警 Issue 创建、去重和恢复关闭。

---

## 15. 当前生产验证记录

本轮修复期间已现场验证：

```text
/etc/tsloms/tsloms.env : exists, 0600 root:root
system user             : tsloms exists, no-login service account
systemd User            : tsloms
systemd EnvironmentFiles: /etc/tsloms/tsloms.env
systemd ExecStart       : /opt/tsloms/current/server
service status          : active
local health            : passed
external health         : HTTP 200
admin entry             : HTTP 200
version probe           : reachable
database dump           : passed with --no-tablespaces
nginx -t                : passed
atomic release switch   : passed
```

文档编写时：

- 线上版本探针为 `0.1.64` / `6fb0f5e...`；
- E2E HTTP/HTTPS 与 API 基址规范化提交为 `7007897`、`1f11410`，已经推送；
- 需以该提交对应的最新 CI、CD、E2E Actions 结果作为最终“全绿”证据；
- 在最新 Actions 完成前，不把 `1f11410` 写成已由 GitHub Runner 验证成功。

---

## 16. 剩余风险与后续改进

### 16.1 P1：统一 CO 的 SSH 实现

将 `co.yml` 深度探针从第三方 SSH Action 迁移到原生 OpenSSH，复用固定指纹和 `known_hosts` 逻辑。

### 16.2 P1：配置只读 E2E 专用账号

当前无 `E2E_ADMIN_PASS` 时只能执行基础 health。建议创建最小权限、不可执行写操作的探针账号，覆盖完整认证链路。

### 16.3 P1：正式启用 HTTPS

当前 8092 使用 HTTP。正式公网生产建议：

- 配置可信 TLS 证书；
- HTTP 跳转 HTTPS；
- 更新 `DEPLOY_BASE_URL` 和 `E2E_BASE_URL`；
- 删除 8092 HTTP/HTTPS 兼容回退；
- 复测 HSTS、Cookie Secure 和反向代理头。

### 16.4 P1：备份恢复证据

需要定期保存：

- 最新备份 zstd 完整性；
- 临时库恢复成功；
- 关键表行数抽查；
- RTO/RPO；
- 异地副本状态。

### 16.5 P2：标准 SBOM、签名与 provenance

当前轻量 SBOM 和 SHA-256 已解决依赖清单与传输完整性，但仍可增强为：

- CycloneDX/SPDX；
- Cosign 签名；
- GitHub Artifact Attestation；
- SLSA provenance；
- CD 验签后才部署。

### 16.6 P2：统一前端权威检查

明确 `ci-admin.yml` 是快速反馈还是 required check，避免与主 CI 前端质量门重复、状态解释不一致。

---

## 17. 最终小结

TSLOMS 流水线从“能构建、能 SSH、能重启”逐步演进为：

```text
可复现构建
 -> 质量和安全门禁
 -> 精确 SHA 制品
 -> 完整性和版本双校验
 -> 严格 SSH 主机信任
 -> 服务器不编译
 -> staging 与不可变 release
 -> 数据库备份与版本化迁移
 -> systemd 唯一权威单元
 -> 原子切换与自动回滚
 -> 本机/外部/版本三重探活
 -> 部署后 E2E
 -> 周期性 CO 与告警闭环
```

这次漫长修复最重要的经验不是某个具体命令，而是以下原则：

1. **先建立单一事实源。** 协议、入口、systemd、环境文件和版本不能各自维护一套。
2. **仓库整改与生产迁移必须同步规划。** 改 `User=tsloms` 前要先创建用户，改 env 路径前要先迁移文件。
3. **发布脚本必须可重入。** 失败可能发生在任意中间状态，同 SHA 重试必须安全。
4. **完整性、身份和版本是三件事。** SHA-256 验证内容，SSH 指纹验证主机，version.json 验证线上版本。
5. **不要用放宽安全门禁换取“先跑通”。** 应修正数据格式、工具差异和前置状态。
6. **日志必须阶段化且脱敏。** 能定位到制品、SSH、systemd、备份或协议，同时不输出凭据。
7. **基础健康与完整业务 E2E 要分层。** 缺少可选测试账号时仍检查可用性，但不能虚报完整覆盖。
8. **一步到位依赖 preflight。** 在大文件上传和原子切换前，把 Secret、SSH、systemd、env、权限、备份和入口全部检查完。

当最新 `1f11410` 对应的 CI、CD 和两类 E2E 全部成功，并连续运行至少三轮 CO 后，可以把本轮流水线整改标记为“主链路全面达标”。灾备恢复、HTTPS、外部监控和供应链签名仍应按持续治理计划推进。

---

## 18. v3.0 审核项完整闭环矩阵

本节逐项继承 v3.0 的编号和结论，确保 v4.0 不是另起炉灶，也避免遗漏历史风险。

| v3 编号 | v3 问题 | v4 处置 | 当前结论 |
|---|---|---|---|
| CI-P1-01 | 第三方 Action 使用可变 tag | checkout、setup、artifact、gitleaks、github-script、SSH Action 均固定 commit SHA | 已关闭；仍需定期审查升级 |
| CI-P1-02 | 缺少 SBOM、签名、漏洞与泄漏纵深检查 | 已加入 govulncheck、gitleaks、npm audit、Go 模块清单、npm lock 清单并纳入 manifest | 部分关闭；标准 SBOM、签名和 provenance 待增强 |
| CI-P1-03 | E2E 脚本未接入工作流 | 增加部署后 E2E workflow，包含外部 HTTP 与本机 SSH 两个 job | 已关闭；完整认证覆盖依赖可选 E2E Secret |
| CI-P2-01 | 前端 warning 与 bundle 体积无硬门禁 | 增加 bundle budget，超限构建失败；版本化构建报告不进入生产 manifest | 体积项已关闭；普通 lint warning 应持续清零 |
| CI-P2-02 | 主 CI 与 Admin 独立工作流重复 | 尚未完全合并，主 CI 的前端门禁仍是制品发布权威依赖 | 未关闭，P2 治理项 |
| CD-P0-01 | 数据库迁移不是可控发布事务 | 增加版本表、有序迁移、同连接 GET_LOCK、DDL 前 fail-closed 备份、测试和 runbook | 代码项已关闭；恢复演练和异地备份需持续提供证据 |
| CD-P1-01 | 已存在 release 跳过完整复核 | 重试时重新校验 manifest、结构、执行位和版本；修正无 staging 的重试顺序 | 已关闭 |
| CD-P1-02 | systemd 存在 root/旧 env 双基线 | 归档旧单元；生产已迁移为 `User=tsloms` 与 `/etc/tsloms/tsloms.env` | 已关闭并有现场证据 |
| CD-P1-03 | 手动部署可能选择“最新成功 CI” | 按目标 SHA 精确查询主 CI 成功 run，不匹配直接拒绝 | 已关闭 |
| CD-P1-04 | SSH 未固定主机指纹 | CD 与 E2E 使用 ED25519 扫描、SHA256 比对、严格 known_hosts 和原生 SSH | CD/E2E 已关闭；CO 深探针仍建议统一 |
| CD-P2-01 | Nginx 校验非阻断、入口验收不足 | `nginx -t` 失败阻断；增加本机 health、外部 API/admin/version 探针 | 已关闭 |
| CO-P1-01 | MQTT 只做 TCP 端口探测 | 深度探针增加 MQTT 认证、发布、订阅和回环检查 | 已关闭，前提是服务器安装 mosquitto 客户端和配置探针凭据 |
| CO-P1-02 | 缺少 DB、Redis、主机和备份巡检 | 深度探针覆盖 MySQL、Redis、systemd、journal、磁盘、备份年龄、current 链接 | 已关闭；阈值需按生产容量持续调整 |
| CO-P2-01 | GitHub cron 不能替代专业监控 | 文档和 runbook 已明确边界 | 未关闭，需云监控或 Prometheus/Alertmanager |
| 外部控制-1 | production Environment 审批和 Secret 权限无法由仓库证明 | 已实际使用 Environment Secret 完成部署；审批策略仍需 GitHub 页面审计 | 部分验证 |
| 外部控制-2 | systemd 实际单元无法由仓库证明 | 已现场验证 User、EnvironmentFiles、ExecStart、服务 active | 已验证 |
| 外部控制-3 | 环境文件权限无法由仓库证明 | 已现场迁移并验证 `0600 root:root` | 已验证 |
| 外部控制-4 | 备份可恢复性无法由仓库证明 | 已验证备份命令和文件生成，runbook 已建立 | 部分验证；仍需定期恢复演练 |
| 外部控制-5 | 网络暴露和专业告警无法由仓库证明 | 外部入口和本机入口已验证，公网端口与值班告警仍需平台证据 | 部分验证 |

## 19. 版本记录

- v4.0（2026-08-20）：继承 v3.0 全部审核项；记录供应链、版本化迁移、systemd、SSH、Artifact、制品清单、E2E、CO 的实际落地；完整复盘多轮生产失败的现象、原因、修复和预防；新增新环境一步到位实施顺序、故障定位流程和验收清单。
- v3.0（2026-08-19）：严格审核 CI/CD/CO，确认主流程可运行但数据库迁移、供应链、E2E、MQTT 深度探针、systemd 和生产外部证据仍有缺口。
- v2.0（2026-08-18）：建立不可变制品、manifest、workflow_run CD、原子切换、回滚和基础 CO 设计。
- v1.x：初期流水线设计与审核记录，保留为历史参考。
