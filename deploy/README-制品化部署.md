# TSLOMS 制品化部署（v2.0）操作说明

> 适用版本：v2.0 制品化改造（不可变制品 + releases/current 原子切换）。
> 本文代替旧的"服务器本地构建"部署方式。

## 架构概览

```text
push main
  ├─ CI (.github/workflows/ci.yml)
  │    ├─ quality-gate  job  覆盖率≥80% + gofmt/vet/test
  │    └─ package      job  构建不可变制品(server+admin/dist+manifest) 上传 artifact
  │                            only on main
  └─ CD (.github/workflows/cd.yml)  workflow_run 触发，仅 CI 成功后
       ├─ 下载 CI 制品并 sha256 校验
       ├─ 上传到服务器 /opt/tsloms/releases/<sha>.staging（tar 流式，不中途落位）
       └─ 执行服务器端 release-install.sh
            ├─ 数据库迁移前备份（/opt/tsloms/backups/db/）
            ├─ 原子切换 current -> releases/<sha>
            ├─ systemctl restart + 本机探活
            └─ 失败自动切回 previous
```

服务器目录结构：

```text
/opt/tsloms/
├── releases/<commit-sha>/server
├── releases/<commit-sha>/admin/dist/
├── current -> releases/<commit-sha>      # systemd 与 nginx 均指向这里
├── previous -> releases/<prev-sha>
├── backups/db/*.sql(.zst)
├── shared/media/                         # 用户媒体（不随版本滚动删除）
└── bin/{release-install.sh, bootstrap-artifacts.sh}
```

systemd 单元 `ExecStart=/opt/tsloms/current/server`，nginx 前端/媒体分别指向
`/opt/tsloms/current/admin/dist/` 与 `/opt/tsloms/shared/media/`。

## ⚠️ 首次上线（生产机）必须完成

> 服务器不编译、不 npm install、不 git pull——只跑制品。但**第一次**需要一次性迁移现役版本。

1. **配置生产环境 GitHub Secrets（环境级，Environment=`production`）**
   - `DEPLOY_HOST`、`DEPLOY_USER`、`DEPLOY_SSH_KEY`（私钥）、`DEPLOY_SSH_PASSPHRASE`（无则留空）
   - 生产 SSH 密钥请放 **Environment `production` 的 secrets**（`workflow_run` 只认环境级 secrets，repository secrets 读不到）。
   - 在 GitHub 仓库 → Settings → Environments → `production`：设置 required reviewers / 分支保护，并把上述 secrets 放这里。

2. **（可选但推荐）先跑 bootstrap 迁移现役版本**
   - 上传 `deploy/scripts/bootstrap-artifacts.sh` 到服务器，以 root 执行一次：
     ```bash
     sudo bash /opt/tsloms/bin/bootstrap-artifacts.sh
     ```
   - 它会把当前的 `packages/server/server` 与 `packages/admin/dist` 复制成第一个 release，
     建立 `current` 符号链接，迁移媒体到 `shared/media`，并**保留原路径以便回退**。

3. **替换 systemd 单元并重载**
   ```bash
   sudo cp deploy/systemd/tsloms-server.service /etc/systemd/system/tsloms-server.service
   sudo systemctl daemon-reload
   sudo systemctl restart tsloms-server
   curl -fsS http://127.0.0.1:8093/api/v1/health
   ```

4. **替换并重载 nginx**
   - 将 `deploy/nginx/tsloms.conf`（与 https 版）覆盖生产 nginx 配置，
     确认 `current` 符号链接与 `shared/media` 路径后执行 `nginx -t && systemctl reload nginx`。

## 发布流程（日常）

- 合并到 `main` → CI 全绿 → CI 构建制品 → CD 自动部署。
- 手动部署：Actions → CD → Run workflow → 输入 SHA（可选，默认取最新成功 CI 制品）。

## 回滚

- 自动：新版本探活失败时 release-install.sh 自动切回 `previous` 并令 workflow 失败。
- 人工：确认仍能访问的前提下，把 `current` 指回上一 release 并重启：
  ```bash
  ln -sfn "$(readlink /opt/tsloms/previous)" /opt/tsloms/current.next
  mv -Tf /opt/tsloms/current.next /opt/tsloms/current
  sudo systemctl restart tsloms-server
  curl -fsS http://127.0.0.1:8093/api/v1/health
  ```
- 该脚本不删除原 `/opt/tsloms/packages`（仅作历史数据兜底）。systemd 唯一权威单元为
  `deploy/systemd/tsloms-server.service`（`User=tsloms`、`/etc/tsloms/tsloms.env`，P0-03）；
  已归档 `tsloms-server.prod-fitted.service`（root/旧 .env）**禁止**再部署到生产。详见 `deploy/systemd/README.md`。

## 备份恢复 / 故障演练 / 最小权限（P0-04）

生产运维的部署用户最小权限、SSH 禁 root、sudoers 白名单、数据库备份恢复演练（mysqldump|zstd、
恢复步骤、RTO/RPO）与故障注入演练清单，详见 **`runbook-备份恢复与故障演练.md`**。

## 数据库版本化迁移（CD-P0-01，P0 级）

服务启动不再隐藏式全量 `AutoMigrate` 改库，而通过 `MigrateDatabaseVersioned` 显式版本化迁移：

- `schema_migrations` 版本表记录已应用版本；有序迁移 `0001`(结构基座+active→occurred)、
  `0002`(uk_wo_active_scope 唯一索引)、`0003`(旧 device_materials 并入 materials 并删表)、
  `0004`(超管首建)。
- 单实例锁：MySQL 用 `GET_LOCK` 且与全部迁移/释放固定在同一条独占物理连接（`*sql.Conn`）；
  SQLite 测试走简化无锁路径。任一版本失败整体 fail-closed，不进正常服务启动。
- 含 DDL/DropTable 的版本（0002/0003）执行前强制 `MYSQL_PWD` 环境变量 + `mysqldump|zstd`
  备份到 `/opt/tsloms/backups/db`，凭据缺失即阻断。

### MySQL 集成验证（v3 §7.1 P0-01 现场证据）

在被验证的集成/部署环境设置 `TSLOMS_TEST_MYSQL_DSN`,并执行：

```bash
cd packages/server
export TSLOMS_TEST_MYSQL_DSN='user:pass@tcp(127.0.0.1:3306)/tsloms_test?charset=utf8mb4'
go test ./internal/model/ -run 'TestMigrateDatabaseVersioned_MySQLGetLockIntegration' -v
```

该用例验证 GET_LOCK 配对释放、二次实例并发被拒（fail-closed）、锁超时。随后做一次
`mysqldump|zstd` 恢复演练并记录 RTO/RPO（见 `runbook-备份恢复与故障演练.md`）。

## 注意事项

- 生产 SSH 私钥、数据库密码、JWT 等一律走 Secrets / `/etc/tsloms/tsloms.env`（0600），
  **禁止**写入仓库或日志。
- 若媒体已有数据，务必先跑 bootstrap（或确认 release-install.sh 已执行一次性迁移），
  再切换 nginx 指向 `shared/media`，否则旧媒体路径 404。
- 首次部署若失败且无 `previous` 可回滚，请先用 bootstrap 生成的 release 手工恢复。
