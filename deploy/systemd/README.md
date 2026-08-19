# TSLOMS 生产 systemd 单元（P0-03 统一单一授权单元）

## 规则：只有一个权威单元

生产环境 **唯一权威单元** 为 `tsloms-server.service`（本目录）。

- **运行用户**：`tsloms`（普通用户），**禁止 root**。
- **环境文件**：`/etc/tsloms/tsloms.env`（`0600`，owner `root:root`）。
- **可执行/工作目录**：`/opt/tsloms/current`（制品化原子切换符号链接，服务器不编译）。
- **沙箱**：`NoNewPrivileges=true`、`ProtectSystem=strict`、`ProtectHome=true`，
  仅开放媒体与 current 写路径。

> 旧 `tsloms-server.prod-fitted.service`（`User=root`、旧 `/opt/tsloms/packages/server/.env` 路径）
> 已归档到 `archive/`，**禁止部署**。生产若仍存在该单元，需按下方“迁移”步骤清理。

## 部署 / 覆盖单元

```bash
# 以 root 执行（服务器本地；CI/CD 经 SSH + sudoers 白名单调用）
sudo cp deploy/systemd/tsloms-server.service /etc/systemd/system/tsloms-server.service
sudo chown root:root /etc/systemd/system/tsloms-server.service
sudo chmod 0644 /etc/systemd/system/tsloms-server.service
sudo systemctl daemon-reload
sudo systemctl restart tsloms-server
```

## 部署前 / 后必须校验（CD 也应执行）

```bash
# 1) 打印实际启用的单元定义（确认 User、EnvironmentFile、ExecStart）
systemctl cat tsloms-server

# 2) 结构化读取关键属性（不含密钥值）
systemctl show tsloms-server -p User -p Group -p EnvironmentFile -p ExecStart -p ProtectSystem -p NoNewPrivileges

# 3) 确认 running 进程的工作目录/可执行来自 current（与 unit 一致，避免 restart 未应用新单元）
ls -l /proc/$(systemctl show -p MainPID --value tsloms-server)/exe
readlink -f /opt/tsloms/current

# 4) 确认环境文件权限（0600 root:root）
ls -l /etc/tsloms/tsloms.env
# 期望：-rw------- 1 root root ...

# 5) 若发现 root 单元残留，执行迁移：
#   sudo cp deploy/systemd/tsloms-server.service /etc/systemd/system/
#   sudo systemctl daemon-reload && sudo systemctl restart tsloms-server
#   sudo systemctl cat tsloms-server  # 复查 User=tsloms
```

## 关键约束

- **CD 只能 `systemctl restart tsloms-server`**：因此部署前必须验证“实际启用单元”与本文件一致，
  而不是假设 restart 生效。若服务器上仍是旧单元，应先迁移再发布。
- **禁止**在 unit 内使用 `${VAR}`（systemd 不展开，字面值注入会造成密钥泄露/连接失败）；
  敏感项统一放 `/etc/tsloms/tsloms.env`，非敏感静态项用 `Environment=`。
- 迁移媒体到 `/opt/tsloms/shared/media`（见 `README-制品化部署.md` 与 `bootstrap-artifacts.sh`）。
