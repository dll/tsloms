# TSLOMS 流水线（CI / CD / CO）v1.1

> 版本：v1.1　|　适用仓库：`github.com/dll/tsloms`（**公开仓库**）　|　生产环境：腾讯云轻量应用服务器
> 本文档描述 TSLOMS 从「代码提交」到「生产部署」再到「持续运维巡检」的完整自动化链路，所有命令均可直接复制执行。

### v1.1 修订记录

- 修正：`authorized_keys` 三行密钥的真实名称与类型（`wxx_deploy` RSA + `tsloms-cd-github-actions` RSA/ed25519 两把）。
- 修正：明确仓库为公开仓库，`GIT_TERMINAL_PROMPT=0` 是因为公开仓库匿名只读即可、无需凭据。
- 补充：回滚时 dist 与 server 备份须按**同一时间戳**配对使用。
- 补充：服务器本地构建使用 Node v20.20.2，与 CI 的 Node 22 存在差异，需知悉其影响（见 §3.4）。

---

## 1. 流水线总览

```
开发本地提交 ──push──▶ GitHub main ──┬──▶ CI 前端质量门（lint + 单测 + 构建）
                                     ├──▶ CI 后端质量门（go test + 覆盖率≥80%）
                                     └──▶ CD 生产部署（SSH 触发服务器 git pull + 构建 + 重启）
                                                     │
                                                     ▼
                                            生产服务器（129.211.223.113）
                                                     │
           每 10 分钟 ◀──cron── CO 持续运维巡检（API / 前端 / MQTT 端口）
                     └──失败──▶ 自动创建 GitHub issue 告警
```

| 环节 | Workflow 文件 | 触发条件 | 作用 |
|---|---|---|---|
| CI-前端 | `.github/workflows/ci-admin.yml` | push/PR 到 `packages/admin/**` | ESLint + 类型检查 + 单测 + 生产构建 |
| CI-后端 | `.github/workflows/ci.yml` | push/PR 到 `packages/server/**` | go vet + go test + 覆盖率 ≥80% |
| CD | `.github/workflows/cd.yml` | push 到 `main`（`packages/**`、`deploy/**`、cd.yml） | 服务器备份→拉码→构建→重启→健康检查 |
| CO | `.github/workflows/co.yml` | cron `*/10 * * * *`（每 10 分钟）+ 手动 | 巡检 API/前端/MQTT，失败自动建 issue |

---

## 2. 账号与密码清单（务必妥善保管）

> ⚠️ 本文件包含生产环境全部凭据，请勿外传，请勿提交到公开仓库。

### 2.1 GitHub

| 项目 | 值 |
|---|---|
| 仓库地址（HTTPS） | `https://github.com/dll/tsloms.git` |
| 仓库地址（SSH） | `git@github.com:dll/tsloms.git` |
| 仓库可见性 | **公开**（任何人可匿名克隆/拉取，但无法推送） |
| 仓库所有者账号 | `dll` |
| 认证方式 | Personal Access Token（`gho_` 开头，scopes：`repo`、`workflow`、`gist`），已配置在 `gh` CLI（环境变量 `GH_TOKEN`） |

### 2.2 生产服务器

| 项目 | 值 |
|---|---|
| 公网 IP | `129.211.223.113` |
| 登录账号 | `root`（可直接 sudo） |
| SSH 端口 | 22 |
| 本机（Windows）SSH 私钥 | `C:\Users\ldl\.ssh\wxx_deploy.pem`（RSA，登录生产服务器，`authorized_keys` 中为 `wxx_deploy`） |
| CD 专用部署私钥（GitHub Actions 用） | `tsloms-cd-rsa`（RSA 2048，`authorized_keys` 中为 `tsloms-cd-github-actions` 的 RSA 部分） |

> ⚠️ 经验：**服务器只接受 RSA 密钥**，ed25519 密钥登录时出现 “Server accepts key 后 Permission denied”，请务必使用 RSA。`authorized_keys` 中同时存在 ed25519 与 RSA 两把 `tsloms-cd-github-actions`，实际生效的是 RSA 那把。

### 2.3 MySQL 8.0

| 项目 | 值 |
|---|---|
| 地址 | `localhost:3306` |
| 数据库名 | `tsloms` |
| 用户 | `tsloms` |
| 密码 | `Tsloms_DB_Pass_2026!` |

### 2.4 Redis 7.0

| 项目 | 值 |
|---|---|
| 地址 | `localhost:6379` |
| DB | `1`（0-15 可选） |
| 密码 | 无（空） |

### 2.5 EMQX 5.0（MQTT 消息中间件）

| 项目 | 值 |
|---|---|
| MQTT 监听端口 | `1883`（公网可达，供检测器接入） |
| WebSocket 端口 | `8083` |
| Dashboard 端口 | `18083`（仅本机 127.0.0.1） |
| Dashboard 登录账号 | `admin` |
| Dashboard 密码 | `XcIjS4QjVuDywybE5Y9z` |
| 服务端 MQTT 账号（`.env` 使用） | `tsloms` |
| 服务端 MQTT 密码 | `zYj3dV4FI89U9QiX` |

### 2.6 TSLOMS 服务端配置（`/opt/tsloms/packages/server/.env`）

| 键 | 值 |
|---|---|
| `SERVER_PORT` | `8093` |
| `APP_ENV` | `production` |
| `DB_DRIVER` | `mysql` |
| `DB_HOST` / `DB_PORT` | `localhost` / `3306` |
| `DB_USER` / `DB_PASSWORD` | `tsloms` / `Tsloms_DB_Pass_2026!` |
| `DB_NAME` | `tsloms` |
| `REDIS_ADDR` | `localhost:6379` |
| `REDIS_PASS` | （空） |
| `REDIS_DB` | `1` |
| `JWT_SECRET` | `Xk9Tsloms2026SecretKey8mP3vR5wN2cL7jH` |
| `MQTT_BROKER` | `tcp://localhost:1883` |
| `MQTT_USERNAME` / `MQTT_PASSWORD` | `tsloms` / `zYj3dV4FI89U9QiX` |
| `MQTT_CLIENT_ID` | `tsloms-server` |
| `MQTT_TOPIC_PREFIX` | `trafficLight` |
| `MEDIA_URL_PREFIX` | `/tsloms/media` |
| `MEDIA_DIR` | `/opt/tsloms/packages/server/uploads/media` |
| `AI_API_KEY` | `5dc44da8d9dd4c28bf38cde316950f1e.nNIf7AXWrJXIcSyQ` |
| `AMAP_WEB_KEY` | `3751a2af824c964b5e26033bff337458` |

### 2.7 GitHub Actions Secrets

| Secret 名 | 值 / 来源 |
|---|---|
| `DEPLOY_HOST` | `129.211.223.113` |
| `DEPLOY_USER` | `root` |
| `DEPLOY_SSH_KEY` | CD 部署私钥 `tsloms-cd-rsa` 的**全文内容**（含 `-----BEGIN OPENSSH PRIVATE KEY-----` 头尾） |

---

## 3. 生产服务器部署环境

### 3.1 目录结构

```
/opt/tsloms
├── packages/
│   ├── admin/                 # 前端工程（dist 由 nginx 直接服务）
│   │   └── dist/              # 构建产物（nginx alias 指向这里）
│   └── server/                # Go 后端工程
│       ├── server             # 编译出的可执行文件
│       └── .env               # 环境变量（系统服务 EnvironmentFile 引用）
├── deploy/
│   ├── backups/               # CD 自动备份目录（dist-时间戳 / server-时间戳）
│   └── stage/                 # 上传暂存目录（当前 CD 方案不再使用）
└── docs/ 等
```

### 3.2 systemd 服务（`/etc/systemd/system/tsloms-server.service`）

```ini
[Unit]
Description=TSLOMS Server
After=network.target mysql.service redis.service mosquitto.service

[Service]
EnvironmentFile=/opt/tsloms/packages/server/.env
Type=simple
WorkingDirectory=/opt/tsloms/packages/server
ExecStart=/opt/tsloms/packages/server/server
Restart=always
RestartSec=5
KillSignal=SIGTERM
TimeoutStopSec=15

[Install]
WantedBy=multi-user.target
```

常用命令：

```bash
systemctl start tsloms-server        # 启动
systemctl restart tsloms-server      # 重启
systemctl stop tsloms-server         # 停止
systemctl status tsloms-server       # 状态
systemctl is-active tsloms-server    # 输出 active 即正常
journalctl -u tsloms-server -n 50    # 查看最近 50 行日志
```

### 3.3 nginx 反向代理（`/etc/nginx/sites-available/tsloms`，监听 8092）

| 路径 | 后端目标 |
|---|---|
| `/tsloms/admin` | 前端 `alias /opt/tsloms/packages/admin/dist/`（history 路由 try_files 兜底） |
| `/tsloms/api/` | `proxy_pass http://127.0.0.1:8093/api/` |
| `/tsloms/media/` | 静态文件 `/opt/tsloms/packages/server/uploads/media/` |
| `/tsloms/health` | `proxy_pass http://127.0.0.1:8093/api/v1/health` |
| `/` | `302 → /tsloms/admin/` |

改完配置重载：`nginx -t && systemctl reload nginx`

### 3.4 服务器构建环境（CD 依赖）

| 组件 | 版本 |
|---|---|
| Go | 1.22.2 |
| Node.js | v20.20.2（服务器本地构建前端用） |
| npm 镜像 | `registry.npmmirror.com` |
| Go 代理 | `goproxy.cn,direct` |

> ⚠️ **Node 版本差异说明**：CI 使用 Node 22（vite/cesium 等工具链要求），而 CD 在服务器本地用 Node v20.20.2 构建前端。两者构建产物在大部分场景兼容，但若出现 CI 通过、CD 构建失败或产物异常的情况，请优先排查此版本差异（例如：将服务器 Node 升级到 22，或 CD 改用 `n`/`nvm` 固定版本后构建）。

---

## 4. CI - 持续集成

### 4.1 前端质量门（`ci-admin.yml`）

触发：push / PR 涉及 `packages/admin/**`。

流程（全部在 `packages/admin` 下执行）：

1. `actions/checkout@v4` 检出代码
2. `actions/setup-node@v4` 安装 **Node 22**（启用 npm 缓存，缓存键为 `package-lock.json`）
3. 设置 npm 镜像并安装依赖：
   ```bash
   npm config set registry https://registry.npmmirror.com
   npm ci
   ```
4. ESLint + 类型检查：`npm run lint`
5. 单元测试：`npm run test`
6. 生产构建：`npm run build`

> ⚠️ 依赖版本约束（package.json 已锁定）：`vite ^6.0.0`、`vitest ^3.2.7`。不得把 vitest 升到 4.x，否则会产生嵌套 vite8/esbuild0.28 冲突导致 `npm ci` 失败。

### 4.2 后端质量门（`ci.yml`，原有）

触发：push / PR 涉及 `packages/server/**`。流程：`go mod download` → `gofmt` 检查（0 文件）→ `go vet` → `go build` → `go test` 全量 → **覆盖率门禁 ≥80%**（`make coverage-check COV_THRESHOLD=80`），任一失败则阻止合并。

### 4.3 查看方式

```bash
gh run list --repo dll/tsloms --workflow ci-admin.yml   # 前端
gh run list --repo dll/tsloms --workflow ci.yml         # 后端
```

---

## 5. CD - 持续部署（核心流程）

### 5.1 触发条件

push 到 `main`，且变更涉及 `packages/**`、`deploy/**` 或 `.github/workflows/cd.yml`。也可手动：

```bash
gh workflow run cd.yml --repo dll/tsloms
```

### 5.2 执行流程（单步 SSH，全部在服务器上完成）

```
第1步 备份当前版本
      ├─ cp -r packages/admin/dist  → deploy/backups/dist-<时间戳>
      └─ cp packages/server/server  → deploy/backups/server-<时间戳>

第2步 拉取最新代码
      └─ GIT_TERMINAL_PROMPT=0 git pull origin main
         （仓库为公开仓库，HTTPS 匿名只读即可；禁用凭据交互，
           避免 CI 非交互终端挂起）

第3步 构建服务端（linux/amd64）
      └─ cd packages/server
         CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server cmd/server/main.go

第4步 构建管理后台
      └─ cd ../admin
         npm config set registry https://registry.npmmirror.com
         npm ci
         npm run build

第5步 修正属主
      └─ chown -R tsloms:tsloms 前端 dist 与 server 二进制

第6步 重启并验证
      ├─ systemctl restart tsloms-server
      ├─ sleep 5
      └─ systemctl is-active tsloms-server   # 必须输出 active

第7步（在 GitHub Runner 上）健康检查
      ├─ curl http://129.211.223.113:8092/tsloms/health   # 返回 {"code":0,...}
      └─ curl -o /dev/null http://129.211.223.113:8092/tsloms/admin/
```

SSH 连接使用 GitHub Actions Secret `DEPLOY_SSH_KEY`（`appleboy/ssh-action@v1.0.3`，`use_insecure_cipher: true`，`command_timeout: 20m`）。

### 5.3 部署后人工验收

```bash
# 1. 生产 API 健康
curl http://129.211.223.113:8092/tsloms/health
# → {"code":0,"data":{"env":"production","service":"tsloms-server","status":"ok"},...}

# 2. 前端可访问（浏览器打开 http://129.211.223.113:8092/tsloms/admin/）

# 3. 服务器上核对版本与进程
ssh -i "C:\Users\ldl\.ssh\wxx_deploy.pem" root@129.211.223.113 \
  "cd /opt/tsloms && git log --oneline -1 && systemctl is-active tsloms-server"
```

### 5.4 回滚

备份在 `/opt/tsloms/deploy/backups/`，按时间戳恢复：

> ⚠️ 前端 `dist-<TS>` 与后端 `server-<TS>` 备份必须取**同一时间戳**配对恢复，避免前后端版本不一致。

```bash
ssh -i "C:\Users\ldl\.ssh\wxx_deploy.pem" root@129.211.223.113
# 找到要回滚的备份（dist-<TS> 与 server-<TS> 成对）
ls /opt/tsloms/deploy/backups/

# 前端回滚
rm -rf /opt/tsloms/packages/admin/dist
cp -r /opt/tsloms/deploy/backups/dist-<时间戳> /opt/tsloms/packages/admin/dist
chown -R tsloms:tsloms /opt/tsloms/packages/admin/dist

# 后端回滚（<时间戳> 与前端相同）
cp /opt/tsloms/deploy/backups/server-<时间戳> /opt/tsloms/packages/server/server
chown tsloms:tsloms /opt/tsloms/packages/server/server
systemctl restart tsloms-server
systemctl is-active tsloms-server
```

> 💡 备份会随每次 CD 持续累积，建议定期清理旧备份（如保留最近 10 个）：
> ```bash
> ls -t /opt/tsloms/deploy/backups/dist-* | tail -n +11 | xargs rm -rf
> ls -t /opt/tsloms/deploy/backups/server-* | tail -n +11 | xargs rm -rf
> ```

---

## 6. CO - 持续运维巡检

### 6.1 巡检内容（每 10 分钟，cron `*/10 * * * *`）

| 检查项 | 命令 | 失败即告警 |
|---|---|---|
| API 健康 | `curl -f http://129.211.223.113:8092/tsloms/health` | exit 1 |
| 前端可达 | `curl -f -o /dev/null http://129.211.223.113:8092/tsloms/admin/` | exit 1 |
| MQTT 端口 | `echo > /dev/tcp/129.211.223.113/1883` | exit 1 |

### 6.2 失败告警

任一检查失败 → `actions/github-script@v7` 在仓库创建 issue，标题含 `生产环境巡检失败`。**避免重复轰炸**：若已存在同标题 open issue 则不重复创建。

### 6.3 手动触发

```bash
gh workflow run co.yml --repo dll/tsloms
```

### 6.4 收到告警后的排查步骤

```bash
ssh -i "C:\Users\ldl\.ssh\wxx_deploy.pem" root@129.211.223.113
systemctl status tsloms-server          # 服务是否在跑
journalctl -u tsloms-server -n 50       # 查看报错
ss -tlnp | grep -E '8093|1883'          # 端口是否监听
# 修复后手动触发一次巡检确认
gh workflow run co.yml --repo dll/tsloms
# 关闭告警 issue 或等巡检恢复后手动 close
gh issue list --repo dll/tsloms --state open
```

---

## 7. 一键部署（手动触发全流程）

```bash
# 前置：已提交并推送代码到 main
git add -A && git commit -m "feat: xxx" && git push origin main

# 若 CD 因路径过滤未触发（如只改了 README），手动触发：
gh workflow run cd.yml --repo dll/tsloms

# 查看运行状态
gh run list --repo dll/tsloms --workflow cd.yml --limit 1
gh run watch <run-id> --repo dll/tsloms
```

---

## 8. 本地/服务器常用运维命令速查

| 用途 | 命令 |
|---|---|
| 登录服务器（本机） | `ssh -i "C:\Users\ldl\.ssh\wxx_deploy.pem" root@129.211.223.113` |
| 重启服务 | `systemctl restart tsloms-server` |
| 看服务日志 | `journalctl -u tsloms-server -n 50 -f` |
| 看 MQTT 连接数 | `emqx ctl clients list` |
| 看 MySQL 库表 | `mysql -u tsloms -p'Tsloms_DB_Pass_2026!' tsloms -e 'SHOW TABLES;'` |
| 看 nginx 配置 | `nginx -t && systemctl reload nginx` |

---

## 9. 附件：踩坑记录（Pitfalls）

> 以下均为本项目实施中真实踩过的坑，供维护时参考。

1. **Node 版本不足**：前端构建需 **Node ≥22**（cesium 依赖要求），CI 的 setup-node 必须写 `node-version: 22`，用 20 会构建失败。
2. **vitest 4 引发依赖地狱**：vitest ^4.1 要求 vite ^6/7/8，实际会拉取嵌套 vite8 + esbuild 0.28，与锁文件冲突报 `Missing: esbuild@0.28.2 from lock file`。**解法：vitest 锁 ^3.2.7 + vite ^6.0.0**。
3. **scp-action 上传不可靠**：`appleboy/scp-action` 在本项目传输 tar 包会“假成功”（步骤打勾但服务器上无文件）或长时间挂起。**解法：放弃上传制品，改为 ssh-action 触发服务器本地 git pull + 构建**（服务器自带完整构建环境）。
4. **ssh-action 的 `insecure` 参数无效**：该 action 无 `insecure` 输入，写了会 warning 但被忽略。正确跳过 host key 相关校验应使用 `use_insecure_cipher: true`（针对旧加密算法）。
5. **CI 非交互终端里 git pull 挂起**：GitHub Actions 的 SSH 会话无 TTY，git 的 https 凭据助手会静默等待输入导致卡死。**解法：`GIT_TERMINAL_PROMPT=0 git pull origin main`**。本项目仓库为公开仓库，HTTPS 匿名只读即可完成拉取，无需任何凭据。
6. **服务器拒收 ed25519 密钥**：公钥已写入 `authorized_keys` 仍 “Server accepts key → Permission denied”。**解法：CD 专用密钥必须用 RSA**（`authorized_keys` 中同名的 ed25519 条目不会生效）。
7. **nginx SPA 深层路由刷新 500**：`alias` + `try_files $uri $uri/ /index.html` 会造成重定向循环。**解法：用命名 location `@tsloms_spa` 兜底回退 index.html**（见 nginx 配置）。
