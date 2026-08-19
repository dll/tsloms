# TSLOMS 生产 Runbook：最小权限 / 备份恢复 / 故障注入演练（P0-04）

> 适用范围：生产服务器（Ubuntu 22.04+，systemd + MySQL + Redis + EMQX + Nginx）。
> 目标：把「部署用户最小权限」「SSH 禁 root」「数据库备份恢复」「故障注入演练」写成可执行、
> 可重复的 runbook。**本文件不落盘真实凭据**（密码一律来自 `/etc/tsloms/tsloms.env` 或 GitHub
> Environment Secret）。

---

## 0. 角色与拓扑

| 角色 | 说明 | 权限 |
|---|---|---|
| `tsloms` | 应用运行用户（systemd），不直接登录 | 拥有 `/opt/tsloms/{current,releases,shared,backups}` |
| `deploy` | 部署/运维用户（SSH 登录），最小权限 | 仅可写发布目录 + sudoers 白名单 3 条命令 |
| `root` | 系统管理，禁止 SSH 直连，仅本机/串口 | 全量 |

> 生产单元一律 `User=tsloms`（唯一权威，见 `deploy/systemd/README.md`）。**禁止任何服务以 root 运行。**

---

## 1. SSH 安全：禁 root、固定指纹、密钥白名单

### 1.1 禁止 root 远程登录
```bash
# 本机（root，或 sudo）执行：
cat > /etc/ssh/sshd_config.d/99-tsloms-hardening.conf <<'EOF'
PermitRootLogin no
PasswordAuthentication no
PubkeyAuthentication yes
AllowUsers deploy        # 只允许 deploy 用户 SSH
EOF
systemctl restart sshd
# 验证：在另一终端测试
ssh -o BatchMode=yes root@<host>       # 必须被拒绝
ssh -o BatchMode=yes deploy@<host>     # 必须仅证书可登录（密码禁用）
```

### 1.2 固定 host fingerprint（GitHub `.github/workflows/cd.yml` 已是 `StrictHostKeyChecking=yes` + fingerprint）
```bash
# 获取并记录指纹（作为 DEPLOY_HOST_FINGERPRINT secret）：
ssh-keyscan -t ed25519 <host> | awk '{print $3}'   # 例如 SHA256:...
# 首次连接验证一次真实指纹后固化；禁止再使用 StrictHostKeyChecking=no。
```

---

## 2. 部署用户最小权限与 sudoers 白名单

### 2.1 创建 deploy 用户与目录属主
```bash
sudo useradd -m -s /bin/bash -G sudo deploy          # 追加 sudo 组但下面用白名单收敛
sudo install -d -o tsloms -g tsloms -m 0750 /opt/tsloms/{releases,shared,backups} 
sudo install -d -o tsloms -g tsloms -m 0750 /opt/tsloms/current
# deploy 可写 releases/backups（用于上传制品与备份），目录本身归 tsloms 运行
```
> 实际发布目录 `/opt/tsloms/releases` 由 CD 写入，`deploy` 需写该目录。可用 ACL 精细化：
> `sudo setfacl -m u:deploy:rwx /opt/tsloms/releases /opt/tsloms/backups`

### 2.2 sudoers 白名单（只允许 3 条，禁止任意 sudo）
```bash
# /etc/sudoers.d/tsloms-deploy
# deploy 仅能：重载/重启服务、验证配置；禁止 shell、禁止修改环境文件。
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl restart tsloms-server
deploy ALL=(root) NOPASSWD: /usr/bin/systemctl daemon-reload
deploy ALL=(root) NOPASSWD: /usr/bin/nginx -t
# 禁止：!/usr/bin/systemctl * shell，!/bin/systemctl kill，!/usr/bin/sudo *
```
```bash
sudo visudo -c                        # 校验语法
sudo -u deploy sudo -l                # 列出 deploy 可用命令，确认只有白名单项
sudo -u deploy sudo systemctl restart tsloms-server   # 应成功
sudo -u deploy sudo ls /root          # 必须被拒绝（越权命令拒绝留证）
```

### 2.3 拒绝 `tsloms` 与 `deploy` 登录 root 的捷径
```bash
# 确保 allow 里只有部署必需：
sudo -u deploy sudo -l | grep -iE 'systemctl|nginx'   # 不含 'shell' / 'all' / 通配 shell
```

---

## 3. 数据库备份 / 恢复演练（mysqldump | zstd）

> 目标 RTO：≤ 15 分钟；目标 RPO：≤ 5 分钟（或按业务接受度调整）。生产备份建议异地/加密保留 ≥ 30 天。

### 3.1 每日备份（cron/systemd timer，本机 `/opt/tsloms/backups/db`）
```bash
# /etc/systemd/system/tsloms-dbbackup.service + .timer（每小时）
cat > /etc/tsloms/backup-db.sh <<'EOF'
#!/usr/bin/env bash
set -Eeuo pipefail
# 凭据只从 0600 env 读取，不打印
set -a; . /etc/tsloms/tsloms.env 2>/dev/null; set +a
export MYSQL_PWD="${DB_PASSWORD}"
TS=$(date +%Y%m%d%H%M%S)
OUT=/opt/tsloms/backups/db/${TS}.sql.zst
mysqldump --single-transaction --routines --triggers --set-gtid-purged=OFF \
  -h"${DB_HOST:-127.0.0.1}" -P"${DB_PORT:-3306}" -u"${DB_USER:-tsloms}" "${DB_NAME:-tsloms}" \
  | zstd -T0 > "$OUT"
unset MYSQL_PWD
# 校验可用性与完整性
zstd -q -t "$OUT" || { echo "备份损坏（RPO 检查失败）" >> /opt/tsloms/backups/backup.log; exit 1; }
echo "$(date -Is) OK ${OUT} $(du -h "$OUT" | cut -f1)" >> /opt/tsloms/backups/backup.log
# 保留 30 天
find /opt/tsloms/backups/db -name '*.sql.zst' -mtime +30 -delete
EOF
chmod 700 /etc/tsloms/backup-db.sh
# 若生产目录为 tsloms，改 owner：chown tsloms:tsloms
```

### 3.2 备份年龄巡检（接入 CO，参考 deploy 监控）
```bash
# CO 深度探针 probe-deep.sh 应校验最近备份时间与可读性：
latest=$(ls -t /opt/tsloms/backups/db/*.sql.zst 2>/dev/null | head -1)
[ -z "$latest" ] && { echo "无备份"; exit 1; }
age=$(( ($(date +%s) - $(stat -c %Y "$latest")) / 3600 ))
[ "$age" -le 24 ] || { echo "最近备份已 $age 小时（超 RPO）"; exit 1; }
zstd -q -t "$latest" || { echo "最新备份不可读"; exit 1; }
```

### 3.3 恢复演练（防误伤：先恢复到**临时库**，验证后再切换）
```bash
RESTORE_DB="tsloms_restore_$$"
# 1) 创建临时库并用业务低权账号做最小权限恢复（应用账号只有 CRUD，无法建库建表需给 GRANT）
sudo mysql -e "CREATE DATABASE ${RESTORE_DB} CHARACTER SET utf8mb4;"
# 2) 解压重放（本地或由 deploy 用户以白名单方式执行）
zstd -d -c /opt/tsloms/backups/db/<latest>.sql.zst \
  | sudo mysql ${RESTORE_DB}
# 3) 业务验收：连临时库做只读 SELECT 校验行数与关键表
sudo mysql -N -e "SELECT COUNT(*) FROM ${RESTORE_DB}.devices; USE ${RESTORE_DB}; SHOW TABLES;"
# 4) 演练通过后删临时库（真实事故中则按需改库名/指向）
sudo mysql -e "DROP DATABASE ${RESTORE_DB};"
```
> **失败注入检查**：故意 `zstd -c` 一份损坏文件，`zstd -t` 必须报错、备份 age 检查必须失败——
> 证明「备份损坏能被发现」而非静默。

### 3.4 RTO / RPO 测量模板（演练后填写到运维记录）
| 指标 | 目标 | 实测（本次演练） | 日期 |
|---|---|---|---|
| RPO（数据可恢复点距今） | ≤5min |  |  |
| RTO（从发现到服务可用） | ≤15min |  |  |
| 备份可读性校验 | 100% |  |  |

---

## 4. 故障注入演练清单（P1-04）

> 前提：每次演练在非业务高峰、有回滚预案，演练后恢复基线并记录时间线。

| # | 场景 | 注入方法 | 预期系统行为 | 验收 |
|---|---|---|---|---|
| 1 | 错误制品/SHA 不符 | CD 传损坏的 `server` 或改 version.txt | `release-install.sh` sha256/version 校验失败，拒绝部署 | 部署失败且 `current` 未变 |
| 2 | 服务启动失败 | 向 current 放坏二进制 or 停 MySQL | 重启后探活失败→自动切回 previous | `systemctl status` 回退成功 |
| 3 | 健康接口失败 | `iptables -A OUTPUT -p tcp --dport 8093 -j DROP`（临时） | cd.yml/co 本机探活失败→回滚或告警 | 触发回滚/告警 issue |
| 4 | 数据库迁移失败 | 恢复一个旧库后部署新版本 | 迁移报错 fail-closed，阻止启动 | 服务不劣化，日志明确 |
| 5 | 磁盘不足 | `fallocate -l 200M /tmp/fill` 占满 `/opt/tsloms` | 备份/上传失败，CO 报磁盘告警 | 备份失败可被发现 |
| 6 | 网络中断 | 断开内网/公网某个依赖 | 入口探活失败→回滚；备份不受影响 | 回滚链路可用 |
| 7 | 备份损坏 | 用损坏内容覆盖最近备份 | backup age/可读性校验失败 | 巡检报「备份不可读」 |
| 8 | 配置漂移 | 临时把 `EnvironmentFile` 指到旧 .env | deploy 前 `systemctl cat` 校验失败 | 发布被阻断 |

**通用演练步骤模板**
```bash
# 1) 记录基线版本
readlink -f /opt/tsloms/current
# 2) 执行注入（见上表）
# 3) 观察：部署/巡检/告警链路
journalctl -u tsloms-server -n 50 --no-pager
# 4) 恢复基线 & 记录时间线
sudo systemctl restart tsloms-server
# 5) 验收：进入本节表格状态，复测 health
curl -fsS http://127.0.0.1:8093/api/v1/health
```

---

## 5. 发布前自检清单（CD 也应执行）

```bash
systemctl cat tsloms-server                       # 唯一权威单元（User=tsloms、/etc/tsloms/tsloms.env）
systemctl show tsloms-server -p User -p EnvironmentFile -p ExecStart
ls -l /etc/tsloms/tsloms.env                      # 0600 root:root
ssh -o BatchMode=yes deploy@<host> sudo -l        # 越权命令应不存在
zstd -q -t $(ls -t /opt/tsloms/backups/db/*.sql.zst | head -1)   # 最近备份可读
readlink -f /opt/tsloms/current && nginx -t
```

---

## 6. 不作死红线

- **不落盘真实凭据**：密码/私钥只存在于 `/etc/tsloms/tsloms.env`（0600）或 GitHub Environment Secret；
  日志、脚本、文档不出现明文。
- **不修改业务红线**：本 runbook 只涉及运维可执行路径，不触碰 `internal/ai`、MQTT 协议、工单/故障状态机。
- **不在生产直接执行演练命令**：先在本节「通用演练步骤」的隔离流程中执行，异常先回滚再排查。
