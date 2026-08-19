# TSLOMS 流水线（CI / CD / CO）v2.0

> 文档性质：对现有流水线的严格审核、可行性验证与改进方案  
> 审核日期：2026-08-18  
> 适用仓库：TSLOMS（`packages/admin`、`packages/server`、`deploy`）  
> 基线文件：`docs/TSLOMS-流水线CICDCO-v1.0.md`（正文标题自称 v1.1；仓库中不存在名为 `TSLOMS-流水线CICDCO-v1.1.md` 的文件）

## 1. 结论先行

### 1.1 审核结论

当前线上服务可以运行，但 v1.1 方案不能直接作为生产级自动发布规范批准执行。结论为：**有条件可行，整改完成前禁止将其视为安全、可重复、可审计的全自动 CD**。

| 范围 | 当前结论 | 说明 |
|---|---|---|
| 前端 CI | 基本可行 | `npm run lint`、`npm run test`、`npm run build` 可执行；当前有 6 个 lint warning，单元测试仅 1 个测试文件、10 个用例 |
| 后端 CI | 未达标 | `go vet`、`go build`、全量测试通过，但真实 GitHub Actions 总覆盖率为 **79.1%**，低于 80% 门禁 |
| 服务器本地构建 CD | 不可作为长期方案 | `go.mod` 要求 Go 1.25、`toolchain go1.26.6`，旧文档声称生产机 Go 1.22.2，版本冲突；生产机编译还会引入不可复现因素 |
| 当前线上可用性 | 已验证 | 2026-08-18 17:25（北京时间）只读探测：健康接口 200、后台入口 200；这只能证明当前实例可用，不能证明发布链路安全 |
| 回滚与数据安全 | 不达标 | 备份不是事务性快照，未绑定发布提交，未包含数据库迁移前置备份，失败后没有自动回滚闭环 |
| 凭据与供应链安全 | 高风险 | 基线文档包含生产密码、JWT、MQTT、AI、高德 Key、SSH 信息；Actions 使用可变 action tag，未固定提交 SHA |

### 1.2 必须先处理的 P0 项

以下任何一项未完成，都不应开放自动生产发布：

1. **立即轮换基线文档中出现过的所有凭据**，并从待提交文件中移除明文；旧文档不得原样提交到公开仓库。
2. **统一工具链版本**：生产构建必须使用 Go 1.26.6（或先把 `go.mod` 降级并重新验证），前端使用 Node 22.x；不能继续记录“服务器 Go 1.22.2”。
3. **CI 成功后才能 CD**，并部署工作流触发时的确定提交 SHA，禁止生产机 `git pull main` 获取漂移代码。
4. **采用制品发布和原子切换**，保留 current/previous 两个可启动版本，健康检查失败自动恢复 previous。
5. **把数据库迁移纳入发布事务**：迁移前备份、迁移锁、迁移失败处理和恢复演练必须有可执行记录。

## 2. 审核依据与实际项目画像

### 2.1 已核验的仓库事实

| 项目 | 实际情况 | 依据 |
|---|---|---|
| 前端 | Vue 3 + Vite 6 + Vitest 3.2.7，构建脚本为 `vue-tsc && vite build` | `packages/admin/package.json`、`package-lock.json` |
| 前端运行时 | Cesium 依赖要求 Node >=22；锁文件中亦存在 Node 22.13+ 引擎约束 | `packages/admin/package-lock.json` |
| 后端 | `go 1.25.0`，`toolchain go1.26.6` | `packages/server/go.mod` |
| 后端服务 | 默认监听 8093，健康路由 `/api/v1/health` | `packages/server/cmd/server/main.go` |
| systemd | `User=tsloms`、`Group=tsloms`，配置文件为 `/etc/tsloms/tsloms.env`，启用 `NoNewPrivileges`、`ProtectSystem` | `deploy/systemd/tsloms-server.service` |
| Nginx | 8092；`/tsloms/admin` 静态页、`/tsloms/api/` 代理、`/tsloms/health` 代理 | `deploy/nginx/tsloms.conf` |
| 本地开发编排 | `deploy/docker-compose.yml` 含 MySQL、Redis、EMQX 和 server | `deploy/docker-compose.yml` |
| 生产编排 | `deploy/docker-compose.prod.yml` 只声明基础设施，后端仍由 systemd 运行 | `deploy/docker-compose.prod.yml` |
| 数据库迁移 | 应用启动时执行 GORM `AutoMigrate`、数据清理/回填和种子初始化；没有版本化迁移目录 | `packages/server/internal/model/migrate.go`、`db.go` |
| E2E | 有 Node 原生冒烟脚本，但未接入 CI/CD | `packages/admin/e2e/smoke.js`、`package.json` |

### 2.2 可复现验证记录

以下命令在审核工作区执行，未改动业务源码：

| 验证项 | 结果 |
|---|---|
| `go vet ./...` | 通过 |
| `go build ./...` | 通过 |
| `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o <临时文件> cmd/server/main.go` | 在 Go 1.26 工具链下通过；说明命令本身可用，但不能证明旧文档所述 Go 1.22.2 生产机可构建 |
| `go test ./... -count=1` | 通过，各包测试通过 |
| `npm run lint` | 通过，0 error、6 warning |
| `npm run test` | 通过，1 个文件、10 个用例 |
| `npm run build` | 通过；Vite 报 Cesium 大分片（约 4.1 MB）警告 |
| GitHub Actions 后端覆盖率 | **失败**：总覆盖率 79.1%，门禁 80%（运行 `32090592430`） |
| GitHub Actions CD | 运行 `32115421457` 成功，但现有 CD 与 CI 是独立工作流，没有同一提交的质量门依赖 |
| GitHub Actions CO | 最近运行 `32120769257` 成功；定时任务成功不等于故障分支、告警写权限和恢复关闭已被演练 |
| 线上 `GET http://<生产地址>:8092/tsloms/health` | 200，返回 `code=0/status=ok` |
| 线上后台入口 | 200 |

> 本机为 Windows，未安装 GNU Make，因此不能直接执行 `make coverage-check`。GitHub Actions 的 Ubuntu 运行记录已经给出真实结果 79.1%，该结果优先于本机无法运行 Make 的情况。建议把覆盖率门禁改成跨平台 Go/脚本实现，避免把 Windows 开发机排除在质量验证之外。

GitHub Actions 后端运行日志还显示 `actions/setup-go` 的依赖缓存没有找到仓库根目录下的 `go.sum`，原因是模块位于 `packages/server`。应显式设置 `cache-dependency-path: packages/server/go.sum`，否则每次 CI 都可能冷启动下载依赖。

## 3. v1.1 方案的严格问题清单

### 3.1 P0：会导致错误发布或安全事故的问题

| 编号 | 问题与证据 | 风险 | v2.0 要求 |
|---|---|---|---|
| P0-01 | 文档正文包含数据库、Redis、EMQX、JWT、AI、高德和 SSH 凭据 | 公开仓库泄露后可直接接管服务或产生费用 | 凭据全部轮换；文档只保留变量名；使用 GitHub Secrets + 服务器受限 env 文件/密钥管理 |
| P0-02 | `go.mod` 为 Go 1.25/1.26.6，旧文档写生产 Go 1.22.2 | 服务器本地构建可能直接失败，或得到与 CI 不同的二进制 | CI 与制品构建固定 Go 1.26.6；生产只运行制品，不安装编译工具链 |
| P0-03 | `cd.yml` 只按 push 触发，未 `needs` CI；服务器执行 `git pull origin main` | CI 未通过也可上线；多次 push 时部署代码与工作流 SHA 不一致 | 统一流水线或 `workflow_run` 串联；checkout/部署均使用 `${{ github.sha }}` |
| P0-04 | 直接覆盖 `packages/admin/dist` 和 `packages/server/server` | 构建中途失败会留下半套前端或新旧前后端不匹配 | 发布目录按 SHA 隔离，校验通过后原子更新 `current` 符号链接 |
| P0-05 | 只复制文件备份，没有数据库备份、迁移锁和自动恢复 | 数据库结构已改变时，二进制回滚仍可能无法启动 | 迁移前 `mysqldump --single-transaction`；迁移版本化；失败自动切回并告警 |
| P0-06 | 健康检查使用公网 HTTP；SSH action 未固定 host fingerprint | 可被中间人篡改探测结果；部署密钥可能被劫持 | 生产使用 HTTPS；或在 SSH 会话中执行服务器本地探测；固定 SSH fingerprint |

### 3.2 P1：会造成不稳定、不可审计或误报的问题

| 编号 | 问题 | 改进 |
|---|---|---|
| P1-01 | 没有 `concurrency`，多个 CD 可并发写同一目录 | 增加 `group: tsloms-production`，`cancel-in-progress: false` |
| P1-02 | `appleboy/ssh-action@v1.0.3` 等使用可变 tag；`use_insecure_cipher` 会放宽加密算法，不等于 host key 校验 | 固定 action commit SHA；使用 `fingerprint`，删除不必要的 insecure cipher |
| P1-03 | 默认 `GITHUB_TOKEN` 权限未显式声明；CO 创建 issue 依赖写权限 | 在 CO job 声明 `permissions: issues: write, contents: read`；其余 job 设 `contents: read` |
| P1-04 | CO 的 `/dev/tcp` 只验证 MQTT 端口打开，不验证 EMQX 认证、订阅和消息链路 | 使用受限 MQTT 探针账号做 CONNECT/PUBLISH/SUBSCRIBE，或在服务器内部执行 `emqx ctl` |
| P1-05 | issue 去重只取默认第一页，失败时不会自动关闭恢复告警 | 使用 label + 分页查询；恢复时添加评论并关闭同一告警；保留外部告警渠道 |
| P1-06 | E2E 冒烟脚本存在但未被流水线调用 | 在临时环境或部署后受保护环境运行；账号密码只从 Secrets 注入，禁止写入命令行日志 |
| P1-07 | `AutoMigrate` 启动时执行清理、回填、建索引和种子 | 引入版本化迁移（如 golang-migrate）；至少先做迁移前备份、超时和分布式锁 |
| P1-08 | 生产 Compose 与模板存在静态不一致：Compose 使用 `DB_PASSWORD` 作为 root 密码，而 `.env.example` 单列 `DB_ROOT_PASSWORD`；Redis 在容器内 bind `127.0.0.1`，宿主端口映射后可能无法访问 | 不把该 Compose 直接用于生产；修正变量和监听地址后执行 `docker compose config`、启动、连接验收 |
| P1-09 | 服务器本地 `npm ci`、Go 下载和构建受镜像、网络、缓存影响 | CI 构建并签名制品；服务器只下载制品；必要时使用内部制品库 |
| P1-10 | 前端 lint 有 6 个 warning，Cesium 产生约 4.1 MB 分片 | 将 warning 逐步清零；用动态 import/manualChunks 拆分 Cesium，设置并监控 bundle budget |

## 4. v2.0 目标架构

推荐采用“CI 构建不可变制品，CD 只负责传输和切换，CO 负责持续探测”的单向链路：

```text
Pull Request ---> CI quality gates ---> merge main
                                      |
                                      v
                             build immutable artifacts
                                      |
                              checksum + SBOM
                                      |
                                      v
                         protected production environment
                                      |
                         upload release/<commit-sha>
                                      |
                         preflight + db backup/migration
                                      |
                    atomic switch current -> release/<sha>
                                      |
                      restart + local/HTTPS smoke checks
                           |                      |
                        success                  failure
                           |                      |
                    keep previous          switch back previous
                                      |
                                      v
                         scheduled CO + alert/recovery
```

生产目录建议调整为：

```text
/opt/tsloms/
├── releases/<commit-sha>/server
├── releases/<commit-sha>/admin/dist/
├── current -> releases/<commit-sha>
├── previous -> releases/<previous-sha>
├── backups/db/<timestamp>.sql.zst
└── shared/media/                         # 不随版本删除
```

systemd 的 `ExecStart` 指向 `/opt/tsloms/current/server`；`MEDIA_DIR` 指向 `shared/media`。发布只创建新目录、校验文件、切换符号链接和重启服务，不覆盖正在提供服务的目录。

## 5. CI（持续集成）设计

### 5.1 统一版本与触发策略

1. 在仓库增加 `.tool-versions` 或明确的 `.node-version`，Node 固定 22.x（建议 22.14+），Go 固定 1.26.6；版本必须与 `go.mod` 和锁文件一致。
2. PR：只运行受影响的前后端质量门，但公共脚本、`deploy`、工作流变更应触发完整门禁。
3. `main`：同一次 workflow 内按 `ci -> package -> deploy` 串行；不要让独立 push 工作流各自猜测最新分支。
4. 所有 job 默认：

```yaml
permissions:
  contents: read
```

5. 对生产部署增加 GitHub Environment `production`，启用 required reviewer、分支限制和部署记录。

### 5.2 后端质量门

推荐顺序：

```bash
go version                         # 必须为 1.26.6
go mod download
test -z "$(gofmt -l .)"
go vet ./...
go test ./... -count=1
go build -trimpath -ldflags "-s -w" -o dist/server ./cmd/server
go test ./... -covermode=set -coverpkg=./... -coverprofile=coverage.out
go tool cover -func coverage.out
```

覆盖率门禁必须读取 `total:` 的浮点数并比较浮点值，不能像旧 Makefile 一样依赖 `grep`、`head` 和整数截断。当前基线为 79.1%，整改目标是先补齐到 80% 以上，再把 80% 设为 required check；不得通过降低门槛伪造达标。无测试文件的 `cmd/licensegen`、`internal/faultcode` 应明确纳入或排除规则，并在脚本中固定，不允许每次解释不同。

### 5.3 前端质量门

```bash
node --version                     # 必须符合项目锁定版本
npm ci --ignore-scripts=false
npm run lint
npm run test
npm run build
npm audit --omit=dev --audit-level=high
```

构建产物应上传为带 SHA 的 artifact，同时上传 `sha256sum`、构建日志和依赖清单。`npm ci` 必须使用锁文件；镜像地址可以作为可配置的企业代理，不能把临时公共镜像写死为唯一来源。

### 5.4 安全与供应链检查

至少增加以下检查，并将高危结果设为阻断：

- secret scanning（如 gitleaks）；
- `govulncheck ./...`、`npm audit`；
- 容器构建时 Trivy 扫描；
- 生成 SBOM（CycloneDX/SPDX）并随制品保存；
- 第三方 action 固定完整 commit SHA，定期由 Dependabot/人工升级；
- 生产制品的 SHA-256 校验，必要时使用 cosign 签名。

## 6. CD（持续部署）设计

### 6.1 触发与并发

推荐使用一个可复用 workflow：CI job 成功后才执行 package/deploy；若保留独立 workflow，CD 必须由 CI 成功事件触发，并校验 `workflow_run.head_sha`。部署 job 至少包含：

```yaml
concurrency:
  group: tsloms-production
  cancel-in-progress: false

environment:
  name: production
```

生产部署脚本的第一行应记录 `RELEASE_SHA`，并拒绝空值、非 40 位 SHA 或已存在但校验不通过的制品。

### 6.2 制品发布步骤

1. CI 在 Ubuntu runner 构建 Linux/amd64 后端、前端 dist、`manifest.json` 和 SBOM。
2. 生成压缩包和 SHA-256；artifact 名称必须包含 commit SHA。
3. 通过受限部署用户上传到 `/opt/tsloms/releases/<sha>.staging`，上传完成后校验 SHA，再原子改名为 `<sha>`。
4. 服务器不执行 `git pull`、`npm ci` 或 `go build`；服务器只执行安装、迁移、切换和验证。
5. SSH 使用固定 host fingerprint、短期密钥或受限跳板；禁止 root 直接登录。部署用户只允许写 releases/shared，并通过 sudoers 执行指定的 systemctl/nginx 命令。

### 6.3 发布前置检查

```bash
set -Eeuo pipefail
test -x "/opt/tsloms/releases/${RELEASE_SHA}/server"
test -f "/opt/tsloms/releases/${RELEASE_SHA}/admin/dist/index.html"
sha256sum -c "/opt/tsloms/releases/${RELEASE_SHA}/manifest.sha256"
systemctl is-active --quiet tsloms-server
nginx -t
```

数据库发布前必须执行（凭据从受限 env 文件读取，不出现在日志）：

```bash
mkdir -p "/opt/tsloms/backups/db/${RELEASE_TS}"
mysqldump --single-transaction --routines --triggers tsloms | zstd -T0 > \
  "/opt/tsloms/backups/db/${RELEASE_TS}/tsloms.sql.zst"
```

迁移工具需要具备：版本表、单实例锁、超时、幂等、失败退出；大表迁移必须先在预生产演练。当前项目的 GORM `AutoMigrate` 在改造完成前只能作为过渡措施，不能宣称具备可逆迁移能力。

### 6.4 原子切换、探活与自动回滚

发布顺序应为：

1. 安装并校验新 release；
2. 执行数据库迁移（成功才继续）；
3. `ln -sfn releases/<sha> current.next && mv -Tf current.next current`；
4. `systemctl restart tsloms-server`；
5. 在服务器本机访问 `http://127.0.0.1:8093/api/v1/health`，再从 HTTPS 入口验证 `/tsloms/health`、后台 index 和一个只读 API；
6. 连续探活 3 次、间隔 2 秒，且 `systemctl is-active` 为 active；
7. 任一失败：记录 journal、切回 `previous`、重启、再次探活，并让 workflow 失败、创建告警。

前端和后端必须使用同一 `RELEASE_SHA`。回滚不能只恢复 dist 或只替换 server；数据库迁移若不可逆，必须使用向前兼容迁移或同时准备数据库恢复方案。

### 6.5 网络与 HTTPS

生产外部入口应使用 HTTPS 域名。若暂时只有 IP：

- GitHub runner 不应把公网 HTTP 当作唯一成功条件；至少通过 SSH 在服务器本机探活，并单独执行外部可达性检查；
- MQTT 1883 是否公网开放必须由安全组、EMQX ACL 和认证共同决定，CO 不能只测 TCP 端口；
- 8093、MySQL、Redis、EMQX Dashboard 必须只绑定内网/本机，不能因 Compose 端口映射暴露公网。

## 7. CO（持续运维）设计

### 7.1 分层探测

| 层级 | 检查 | 频率 | 失败动作 |
|---|---|---|---|
| L1 | HTTPS `/tsloms/health`、后台 index | 5～10 分钟 | 告警 |
| L2 | 登录后只读 API、数据库连接状态、Redis PING | 10 分钟 | 告警并记录脱敏响应 |
| L3 | MQTT 使用探针账号 CONNECT、订阅/发布测试主题 | 10～30 分钟 | 告警；禁止使用业务主题写入 |
| L4 | `systemctl`、磁盘、内存、备份年龄、journal 错误量 | 5～10 分钟 | 告警并触发人工排查 |
| L5 | 完整 E2E 冒烟 | 每次生产发布 + 每日 | 发布失败或创建工单 |

GitHub Actions 的 CO job 显式声明 `issues: write`，并使用标签、分页查询和稳定告警指纹去重。告警正文不得包含密码、JWT、完整 token、用户数据或数据库连接串。CO 恢复后应自动评论并关闭对应告警，保留审计记录。GitHub cron 存在延迟，不能作为秒级监控；关键生产告警应接入腾讯云监控、Prometheus/Alertmanager 或等价服务。

## 8. 凭据、权限和数据保护

### 8.1 凭据治理

基线文件曾出现生产真实凭据。处理顺序：

1. 立即轮换数据库应用账号、Redis、MQTT、EMQX Dashboard、JWT、AI、高德和 SSH 部署密钥；
2. 检查 GitHub commit、Actions 日志、artifact 和服务器 shell history 是否曾暴露；
3. 使用 GitHub Secrets/Environment Secrets、服务器 `/etc/tsloms/tsloms.env`（`root:root`、`0600`）或正式密钥管理服务；
4. 每个凭据设置负责人、用途、创建时间、轮换周期和吊销步骤；
5. 文档、日志、错误响应只显示变量名或末四位，禁止显示完整值。

### 8.2 最小权限

- GitHub CI：`contents: read`；
- CO：仅 `contents: read` + `issues: write`；
- CD：仅使用 production environment 中的部署密钥；
- 服务器：专用 `tsloms-deploy` 用户，禁止 root SSH；systemd 仍以 `tsloms` 用户运行；
- MySQL：应用账号只拥有 `tsloms` 库权限，不使用 root；
- Redis/EMQX：启用认证，Dashboard 不对公网开放；
- MQTT：按 topic 前缀授予最小 publish/subscribe ACL。

## 9. 回滚与应急操作

### 9.1 自动回滚条件

- 新服务无法在超时内启动；
- 健康接口非 2xx 或返回结构错误；
- 后台 index、核心只读 API 或 MQTT 探针失败；
- systemd 在观察窗口内反复重启；
- 校验和、签名或迁移失败。

### 9.2 人工回滚原则

1. 先冻结新的 CD（暂停 workflow 或启用维护锁）；
2. 记录当前 SHA、previous SHA、数据库迁移版本和故障时间线；
3. 回切同一 release 的前后端制品，不从 GitHub 分支重新构建；
4. 若涉及不可逆数据库变更，按迁移 runbook 恢复或执行向前兼容修复；
5. 探活通过后再解除冻结，随后完成根因分析和复盘。

示例（变量必须由发布系统注入，禁止把真实值写入文档）：

```bash
test -L /opt/tsloms/previous
ln -sfn "$(readlink /opt/tsloms/previous)" /opt/tsloms/current.next
mv -Tf /opt/tsloms/current.next /opt/tsloms/current
sudo systemctl restart tsloms-server
curl --fail --silent --show-error --max-time 10 http://127.0.0.1:8093/api/v1/health
```

## 10. 分阶段落地计划

| 阶段 | 工作项 | 完成标准 |
|---|---|---|
| 第 0 阶段（立即） | 轮换所有已暴露凭据；暂停旧 CD 自动发布；确认生产入口 HTTPS/SSH fingerprint | 密钥轮换记录齐全，旧密钥失效 |
| 第 1 阶段 | 固定 Go/Node；修正覆盖率脚本；补测试至总覆盖率 ≥80%；清理 lint warning | CI required checks 连续 3 次成功 |
| 第 2 阶段 | CI 生成带 SHA 的 server/dist/manifest/SBOM；CD 改为制品上传 | 服务器无编译动作，制品可独立校验 |
| 第 3 阶段 | releases/current/previous 原子切换；迁移备份、锁和自动回滚 | 故障注入可在 5 分钟内恢复 |
| 第 4 阶段 | CO 分层探测、MQTT 认证探针、告警去重/恢复；E2E 接入发布后 | 发布、恢复、告警均有可审计记录 |
| 第 5 阶段 | 分支保护、环境审批、依赖扫描、SBOM、定期灾备演练 | 通过一次完整演练并形成报告 |

## 11. 上线验收清单

### 代码与构建

- [ ] `go.mod`、CI、Dockerfile、生产服务器的 Go 版本一致；
- [ ] Node 版本满足 Cesium/锁文件引擎约束；
- [ ] `gofmt`、`go vet`、全量测试、前端 lint/单测/构建全部为 required checks；
- [ ] 后端总覆盖率真实达到 80%，并有可下载报告；
- [ ] E2E 冒烟至少覆盖 health、登录、仪表盘、一个 AI 只读接口和通知列表；
- [ ] 依赖漏洞、密钥扫描、SBOM 均有留档。

### 发布与运行

- [ ] CD 只能部署 CI 已验证的 commit SHA；
- [ ] 生产 workflow 有并发锁、环境审批和最小权限；
- [ ] 制品 checksum/签名校验通过；
- [ ] 发布前数据库备份可恢复，迁移有版本、锁和超时；
- [ ] current/previous 回切演练通过；
- [ ] systemd 用户、EnvironmentFile 权限、媒体目录和 Nginx alias 均符合实际路径；
- [ ] 8093、MySQL、Redis、EMQX Dashboard 未暴露公网；MQTT 访问策略经安全组和 ACL 双重确认。

### 运维与审计

- [ ] CO 能检测 API、前端、数据库/Redis、MQTT 和主机资源；
- [ ] 告警创建、去重、恢复关闭均验证过；
- [ ] 日志不含凭据和敏感业务数据；
- [ ] 最近一次发布、回滚、备份恢复和故障演练均有时间线与责任人。

## 12. 版本变更记录

- v2.0（2026-08-18）：基于仓库实际文件、GitHub Actions 运行记录和线上只读探测重写；移除所有明文凭据；增加 P0/P1 问题分级、不可变制品发布、原子回滚、迁移治理、CO 分层探测、供应链安全和验收标准。
- v1.1（基线正文，文件名实际为 v1.0）：描述了 push 后 CI、SSH 本地构建 CD、定时 CO，但存在版本命名、工具链、权限、凭据、并发、回滚和可重复性问题。原文件按历史版本保留，提交前必须脱敏。
