# TSLOMS 交通信号灯运维系统 操作手册

**文档版本**：V1.0
**更新日期**：2026-08-15
**适用产品**：TSLOMS（交通信号灯运维系统，Web 管理端 + 后端服务）
**配套文档**：`PRD-TSLOMS-v4.1.md`（需求）、`SAR-TSLOMS-v4.1.md`（结项审核报告）

---

## 目录

1. [简介](#1-简介)
2. [系统架构与组成](#2-系统架构与组成)
3. [安装](#3-安装)
4. [部署](#4-部署)
5. [快速上手（登录与界面）](#5-快速上手登录与界面)
6. [功能使用](#6-功能使用)
7. [角色与权限](#7-角色与权限)
8. [常见问题（FAQ）](#8-常见问题faq)
9. [附录](#9-附录)

---

## 1. 简介

### 1.1 系统概述

TSLOMS（Traffic Signal Light Operation & Maintenance System，交通信号灯运维系统）是一套面向城市交通信号灯设备的**在线运维管理平台**。系统通过 MQTT 协议实时接入信号灯检测器，实现设备在线状态监控、故障自动研判、维修工单流转、维修成本归集与数据可视化统计，并提供 AI 故障预测/诊断/生命周期溯源与固件 OTA 升级能力。

系统为 **Web 管理端（浏览器访问）+ 后端服务** 的形态，支持 Chrome / Edge 等主流浏览器访问，平板与手机浏览器亦可响应式使用（本项目不做独立移动端 APP / 小程序）。

### 1.2 主要功能模块

| 模块 | 功能说明 |
|------|----------|
| 仪表盘 | 设备/故障/工单关键指标总览、故障类型分布、趋势、设备故障排行、平均闭环时长 |
| 设备管理 | 信号灯设备增删改查、在线状态、绑定路口 |
| 路口管理 | 路口信息维护、地图点位关联 |
| 地图大屏 | 设备/路口地理可视化、实时状态标注、实景图层 |
| 视频监控 | 设备实时视频、监控墙、离线取证片段 |
| 故障管理 | 故障自动/手动登记、四态生命周期（发生→确认→处置→闭环）流转 |
| 工单管理 | 故障派单、工单流转、SLA 升级、领料出库、成本归集 |
| 固件管理 | 固件上传、版本管理、OTA 升级下发、升级记录 |
| 物料库存 | 物料档案、期初/调整、领料出库、库存流水 |
| 采购管理 | 采购单创建、到货入库（自动加库存）、取消 |
| 维修费用 | 费用登记、工单领料自动生成费用、费用确认、成本统计 |
| 供应商 | 供应商档案维护 |
| AI 分析 | 故障预测、故障诊断（含图片）、生命周期溯源、AI 额度管理 |
| 系统日志 | 操作日志、报文日志 |
| 系统设置 | 用户/部门/AI 配置等治理能力 |

### 1.3 术语说明

| 术语 | 说明 |
|------|------|
| HW ID | 设备硬件唯一标识（uint32），MQTT 话题 `trafficLight/{hw_id}/...` |
| 故障四态 | occurred（发生）→ confirmed（已确认）→ dispatched（已派单）→ closed（已闭环） |
| 工单状态 | pending（待处理）→ processing（处理中）→ completed（已完成）/ closed（已闭环） |
| 领料 | 维修工单从物料库存中出库耗材，系统自动按 数量×单价 生成耗材费用 |
| SLA | 服务等级约定，工单超时自动升级提醒 |

---

## 2. 系统架构与组成

### 2.1 架构总览

系统采用 **前端(admin) + 后端(server, Go) + 数据库(MySQL) + 缓存(Redis) + 消息(MQTT Broker) + 反向代理(Nginx)** 分层架构：

```
浏览器 (Web 管理端)
   │  https://<域名>/tsloms/admin/
   ▼
Nginx (80/443 网关, 8092 内部)
   ├─ /tsloms/admin/  → 前端静态文件 (packages/admin/dist)
   ├─ /tsloms/api/    → 后端 API (反向代理到 127.0.0.1:8093/api/)
   ├─ /tsloms/media/  → 上传的媒体文件
   └─ /tsloms/health  → 健康检查
        ▼
后端服务 (Go, tsloms-server, 端口 8093)
   ├─ MySQL (tsloms 库)  ── 业务数据
   ├─ Redis (DB 1)       ── 缓存/会话辅助
   └─ MQTT Broker (1883) ── 信号灯检测器接入
        ▼
信号灯检测器设备 (MQTT 客户端, trafficLight/{hw_id}/...)
```

### 2.2 组件清单

| 组件 | 技术/版本 | 说明 |
|------|-----------|------|
| 前端 | Vue 3 + TypeScript + Element Plus + Vite + ECharts + Cesium | `packages/admin` |
| 后端 | Go（gin + gorm），纯静态编译（CGO_ENABLED=0） | `packages/server` |
| 数据库 | MySQL 8.x | 库名 `tsloms` |
| 缓存 | Redis 6.x+ | 使用 DB 1 |
| 消息 | MQTT Broker（EMQX / Mosquitto 等） | 端口 1883 |
| 反向代理 | Nginx | 对外 80/443，内部 8092 |
| 进程管理 | systemd（tsloms-server） | Linux 部署 |

### 2.3 主要端口

| 端口 | 用途 |
|------|------|
| 8092 | Nginx 内部服务端口（承载 admin + API 代理） |
| 8093 | 后端服务监听端口 |
| 3306 | MySQL |
| 6379 | Redis |
| 1883 | MQTT Broker |

---

## 3. 安装

### 3.1 前置环境要求

| 依赖 | 版本要求 | 说明 |
|------|----------|------|
| 操作系统 | Ubuntu 20.04/22.04 x64（或同类 Linux） | 生产建议 Linux |
| Go | 1.21+ | 仅构建后端需要 |
| Node.js | 18+ / npm | 仅构建前端需要 |
| MySQL | 8.x | 运行时必选 |
| Redis | 6.x+ | 运行时必选 |
| MQTT Broker | EMQX / Mosquitto | 运行时必选（对接设备） |
| Nginx | 1.18+ | 运行时可选（建议） |

> 生产环境运行仅需 MySQL / Redis / MQTT / Nginx + 编译好的二进制与前端产物；Go 与 Node 仅在**构建**时需要。

### 3.2 源码获取

```bash
# 从代码仓库克隆
git clone <仓库地址> tsloms
cd tsloms
```

项目根目录结构：

```
tsloms/
├── docs/                 # 需求/审核/协议文档
├── deploy/               # 部署配置（nginx/systemd/docker-compose/脚本）
├── packages/
│   ├── admin/            # 前端工程（Vue3）
│   └── server/           # 后端工程（Go）
└── AGENTS.md
```

### 3.3 后端依赖与构建

```bash
cd packages/server

# 设置 Go 模块代理（国内环境建议）
export GOPROXY=https://goproxy.cn,direct

# 下载依赖
go mod download

# 本地开发构建（含 CGO 亦可）
go build -o server cmd/server/main.go

# 纯静态交叉编译（Linux 生产，CGO_ENABLED=0 避免 libc 依赖）
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server-linux cmd/server/main.go
```

### 3.4 前端依赖与构建

```bash
cd packages/admin

# 安装依赖
npm install

# 类型检查（质量门）
npx vue-tsc --noEmit

# Lint 修复（质量门）
npx eslint --fix

# 生产构建（产物在 dist/，base 为 /tsloms/admin/）
npm run build
```

> 前端构建产物 `packages/admin/dist` 即 Nginx 静态目录内容。

### 3.5 数据库初始化

后端启动时会自动完成建表与基础数据初始化（含默认管理员，仅当 `users` 表为空时创建）：

| 默认项 | 值 |
|--------|-----|
| 管理员用户名 | `admin` |
| 初始密码 | `admin123`（**首次登录后必须修改**，系统强密码策略：≥10 位 + 字母 + 数字） |

> 生产环境务必在首次登录后立即修改默认密码。

---

## 4. 部署

### 4.1 部署目录约定

生产路径统一为：

```
/opt/tsloms/
├── packages/
│   ├── server/
│   │   ├── server            # 后端二进制
│   │   └── uploads/media/    # 上传媒体（视频/图片）
│   └── admin/dist/           # 前端静态产物
├── nginx/tsloms.conf         # Nginx 站点配置
└── systemd/                  # systemd unit
```

### 4.2 systemd 服务配置

创建服务单元 `/etc/systemd/system/tsloms-server.service`（参考 `deploy/systemd/tsloms-server.service`）：

```ini
[Unit]
Description=TSLOMS Server
After=network.target mysql.service redis.service

[Service]
Type=simple
User=tsloms
Group=tsloms
WorkingDirectory=/opt/tsloms/packages/server
ExecStart=/opt/tsloms/packages/server/server
Restart=always
RestartSec=5
NoNewPrivileges=true
ProtectSystem=strict
ReadWritePaths=/opt/tsloms/packages/server

# 环境变量（按实际填写，敏感项建议从环境注入）
Environment=SERVER_PORT=8093
Environment=APP_ENV=production
Environment=DB_DRIVER=mysql
Environment=DB_HOST=localhost
Environment=DB_PORT=3306
Environment=DB_USER=tsloms
Environment=DB_PASSWORD=YOUR_DB_PASSWORD
Environment=DB_NAME=tsloms
Environment=REDIS_ADDR=localhost:6379
Environment=REDIS_PASS=
Environment=REDIS_DB=1
Environment=JWT_SECRET=YOUR_STRONG_SECRET
Environment=MQTT_BROKER=tcp://localhost:1883
Environment=MQTT_USERNAME=
Environment=MQTT_PASSWORD=
Environment=MQTT_CLIENT_ID=tsloms-server
Environment=MQTT_TOPIC_PREFIX=trafficLight
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

启动服务：

```bash
sudo systemctl daemon-reload
sudo systemctl enable tsloms-server
sudo systemctl start tsloms-server
```

### 4.3 Nginx 站点配置

参考 `deploy/nginx/tsloms.conf`。核心要点：

- 前端 `location /tsloms/admin` → `alias /opt/tsloms/packages/admin/dist`，history 路由回退 `index.html`。
- API `location /tsloms/api/` → `proxy_pass http://127.0.0.1:8093/api/`。
- 媒体 `location /tsloms/media/` → `alias .../uploads/media/`。
- 健康检查 `location /tsloms/health` → 后端 `health`。
- 路径统一带 `/tsloms` 前缀；网关（如 Caddy）应原样转发 `/tsloms`，不剥离前缀。

```bash
sudo cp deploy/nginx/tsloms.conf /etc/nginx/conf.d/tsloms.conf
sudo nginx -t && sudo systemctl reload nginx
```

### 4.4 部署验证（健康检查）

```bash
# 1. 进程健康
systemctl is-active tsloms-server        # 期望: active

# 2. 健康接口
curl -s http://127.0.0.1:8093/api/v1/health   # 期望: 200/OK
curl -s http://<公网IP>:8092/tsloms/health    # 经 Nginx 探活

# 3. 管理后台
访问 http://<公网IP>:8092/tsloms/admin/   # 期望: 登录页正常
```

### 4.5 上线/升级部署（原子替换）

后端为**单二进制**，升级采用「备份 → 替换 → 重启 → 验证」原子流程（git-over-HTTPS 不稳定时使用 tarball/scp 方式）：

```bash
# 1. 备份当前二进制
cp /opt/tsloms/packages/server/server /opt/tsloms/packages/server/server.prev

# 2. 上传新二进制并替换
scp -i ~/.ssh/wxx_deploy.pem server-linux root@<server>:/tmp/server.new
ssh -i ~/.ssh/wxx_deploy.pem root@<server> "
  install -m 0755 /tmp/server.new /opt/tsloms/packages/server/server &&
  systemctl restart tsloms-server &&
  sleep 3 &&
  systemctl is-active tsloms-server &&
  curl -sf http://127.0.0.1:8093/api/v1/health"
```

> 前端升级：将新 `dist` 内容替换到 `/opt/tsloms/packages/admin/dist` 即可（无需重启后端）。

### 4.6 常用运维命令

```bash
sudo systemctl status tsloms-server      # 查看状态
sudo systemctl restart tsloms-server     # 重启
sudo journalctl -u tsloms-server -f      # 实时日志
sudo systemctl reload nginx              # 重载 Nginx
```

---

## 5. 快速上手（登录与界面）

### 5.1 登录

1. 浏览器打开地址：`http://<服务器IP或域名>:8092/tsloms/admin/`。
2. 输入用户名与密码，点击「登录」。
3. 登录成功进入仪表盘首页。

> 默认管理员 `admin / admin123`（首次登录请务必修改密码）。登录失败/未登录访问时系统自动跳转登录页。

### 5.2 界面总览

登录后左侧为功能导航菜单，顶部为当前用户信息与退出按钮，主区域为各功能页面。主要菜单：仪表盘、设备管理、路口管理、地图大屏、视频监控、故障管理、工单管理、固件管理、物料库存、采购管理、维修费用、AI 分析、系统日志、系统设置等。

### 5.3 管理员首次配置建议

1. 修改默认密码（系统设置→用户）。
2. 新建运维人员（operator）与只读人员（viewer）账号。
3. 录入路口与设备。
4. 录入供应商与物料期初库存。
5. 配置 MQTT（确认设备正常接入上报）。

---

## 6. 功能使用

> 以下各小节涉及截图处均已按「图号+图名居中于图下」预留占位，截图由实施方补充。

### 6.1 仪表盘

展示设备总数/在线数、故障待处置数、工单数、今日闭环数等核心指标，以及故障类型分布饼图、工单状态分布、故障趋势折线图、设备故障排行、平均闭环时长等。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「仪表盘」截图)

> **图 6-1 仪表盘首页**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「仪表盘-故障类型统计」截图)

> **图 6-2 仪表盘-故障类型统计**

---

### 6.2 设备管理

- 新增设备：填写硬件 ID（HW ID，唯一）、设备名称、绑定路口、经纬度等。
- 编辑/删除设备：列表操作按钮（删除需 admin 权限）。
- 设备状态：在线/离线由 MQTT 心跳判定（阈值可配置）。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「设备管理-列表」截图)

> **图 6-3 设备管理列表**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「设备管理-新增设备」截图)

> **图 6-4 新增设备**

---

### 6.3 路口管理

维护路口名称、行政区域、地图点位；支持路口重命名、设置经纬度、清空路口（admin）。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「路口管理」截图)

> **图 6-5 路口管理**

---

### 6.4 地图大屏

- 地图展示设备/路口点位，按状态着色（在线/离线/故障）。
- 支持地图类型切换（高德/百度实景图层）、点位聚合缩放。
- 点击点位查看设备详情与快捷操作。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「地图大屏」截图)

> **图 6-6 地图大屏**

---

### 6.5 视频监控 / 监控墙

- 查看设备实时视频（RTSP 拉流）。
- 监控墙：多路视频同屏展示。
- 故障/工单过程中的举证视频上传归档。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「视频监控」截图)

> **图 6-7 视频监控**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「监控大屏」截图)

> **图 6-8 监控大屏**

---

### 6.6 故障管理

故障来源：
- **自动**：设备通过 MQTT 上报事件，系统按协议解析（15 种 errCode）自动登记，并进入四态生命周期。
- **手动**：人工登记故障。

四态生命周期流转：

```
occurred（发生）→ confirmed（已确认）→ dispatched（已派单）→ closed（已闭环）
```

操作：确认故障、派单（关联工单/人员）、处置、闭环；查看故障详情与关联设备/工单。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「故障管理-列表」截图)

> **图 6-9 故障管理列表**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「故障四态流转」截图)

> **图 6-10 故障四态生命周期**

---

### 6.7 工单管理

- 派单：由故障派单生成工单，指派给运维人员。
- 流转：pending → processing → completed/closed，SLA 超时自动升级。
- **工单领料**：在处理工单时，从物料库存中领用耗材出库；系统在**同一事务**内扣减库存、写出库流水，并按 数量×单价 **自动生成一笔耗材维修费用**（关联该工单与设备）——无需手工再录费用。
- 删除工单需 admin 权限。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「工单管理-列表」截图)

> **图 6-11 工单管理列表**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「工单详情与领料出库」截图)

> **图 6-12 工单详情与领料出库**

---

### 6.8 物料库存

- 物料档案：编码、名称、单位、单价、期初库存、绑定设备等。
- 库存调整：增减库存（入库/盘盈/盘亏），生成 `type=in/out` 流水。
- **领料出库**：工单领料专用（见 6.7），扣库存 + 自动生成费用。
- 库存流水：完整记录每一次入库/调整/领料，可追溯。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「物料库存-列表」截图)

> **图 6-13 物料库存列表**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「库存调整」截图)

> **图 6-14 库存调整**

---

### 6.9 采购管理

- 新建采购单：选择供应商、录入采购明细（物料、数量、单价）。
- 到货入库：执行「入库」后自动增加对应物料库存并生成入库流水。
- 取消采购单：未入库的单据可取消（如已部分到货则会提示限制）。
- 删除采购单需 admin 权限。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「采购管理」截图)

> **图 6-15 采购管理**

---

### 6.10 维修费用

- 费用来源：① 手工录入（type=other/manual）；② **工单领料自动生成**（type=material，见 6.7）。
- 费用确认：费用登记后需确认（confirmed）；未确认费用不计入正式成本统计（或单独标记）。
- 成本统计：按类型/TOP 设备汇总，支持对接仪表盘成本看板。
- 删除费用需 admin 权限。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「维修费用-列表」截图)

> **图 6-16 维修费用列表**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「维修费用-成本统计」截图)

> **图 6-17 维修费用成本统计**

---

### 6.11 固件管理（OTA）

- 上传固件包：填写版本号、说明，上传二进制。
- 发布：将固件标记为发布状态，供设备升级。
- 升级发起：创建升级任务（选择设备/固件），下发 OTA 升级指令。
- 升级记录：查看每台设备升级历史与结果。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「固件管理」截图)

> **图 6-18 固件管理**

---

### 6.12 AI 分析

- **故障预测**：基于历史数据预测路口/设备故障概率，生成预测地图与处置建议。
- **故障诊断**：对上报的故障/反馈（可含图片）进行智能诊断，输出原因与建议。
- **生命周期**：按设备硬件 ID 追溯全生命周期（上线、故障、维修、领料、费用）。
- **额度管理**（admin）：配置 AI 调用额度、查看用量、重置。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「AI 故障预测」截图)

> **图 6-19 AI 故障预测**

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「AI 生命周期追溯」截图)

> **图 6-20 AI 生命周期追溯**

---

### 6.13 系统日志

- 操作日志：记录登录、设备/工单/费用/库存/采购等关键操作（操作人、时间、对象、结果）。
- 报文日志：记录设备 MQTT 收发报文，用于故障排查与协议核对。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「系统日志」截图)

> **图 6-21 系统日志**

---

### 6.14 系统设置

- 用户管理（admin）：增删改查、分配角色（admin/operator/viewer）、重置密码。
- 部门管理（admin）：组织架构维护。
- AI 配置（admin）：模型地址/密钥/额度等。

<!-- ===================== 截图占位 ===================== -->

![ ](此处为占位，请插入「用户管理」截图)

> **图 6-22 用户管理-角色分配**

---

## 7. 角色与权限

系统内置三种角色，前后端双重控制（后端中间件强制 + 前端按钮隐藏）：

| 权限 | admin | operator | viewer |
|------|:---:|:---:|:---:|
| 查看（设备/故障/工单/媒体/固件/物料/库存/供应商/采购/费用/日志/AI 预测） | ✅ | ✅ | ✅ |
| 设备新建/编辑、路口更名/定位 | ✅ | ✅ | ❌ |
| 设备删除、路口清空 | ✅ | ❌ | ❌ |
| 故障确认/派单、工单新建/流转/派单 | ✅ | ✅ | ❌ |
| 工单删除 | ✅ | ❌ | ❌ |
| 媒体上传、固件上传/发布/升级 | ✅ | ✅ | ❌ |
| 固件删除 | ✅ | ❌ | ❌ |
| 物料/库存调整/领料/供应商/采购/费用录入与确认 | ✅ | ✅ | ❌ |
| 业务数据删除（物料/供应商/采购/费用） | ✅ | ❌ | ❌ |
| 用户/部门管理、重置密码 | ✅ | ❌ | ❌ |
| AI 配置更新/用量重置 | ✅ | ❌ | ❌ |

> ✅ 允许 / ❌ 拒绝。结项复核实测 10/10 用例通过。

---

## 8. 常见问题（FAQ）

### 8.1 登录相关

**Q1：登录后一直跳回登录页？**
- 原因：token 失效（默认 72h）或浏览器未启用 Cookie/本地存储。
- 处理：清除浏览器缓存后重新登录；确认服务器时间与客户端时间偏差过大（超 5 分钟）会导致 JWT 失效。

**Q2：忘记管理员密码？**
- 处理：由具备数据库权限的 DBA 在 `users` 表将该用户密码重置（使用 bcrypt 哈希），或联系系统管理员；生产建议保留可用 admin 账号。系统不提供找回，请务必记录。

**Q3：密码强度校验失败？**
- 原因：系统要求 ≥10 位且同时包含字母与数字。

### 8.2 部署与连不通

**Q4：访问后台 404 或白屏？**
- 处理：确认 Nginx 已正确 `alias` 到 `/opt/tsloms/packages/admin/dist`（不是上一级目录）；`try_files` 已配置 history 回退到 `index.html`；`nginx -t` 与 `systemctl reload nginx`。

**Q5：API 请求 404 / 502？**
- 处理：确认后端已启动（`systemctl is-active tsloms-server`）；确认 Nginx `proxy_pass` 指向 `http://127.0.0.1:8093/api/`（剥离 `/tsloms` 前缀）；`curl http://127.0.0.1:8093/api/v1/health` 是否 200。

**Q6：健康检查 `/tsloms/health` 失败？**
- 处理：确认 `location /tsloms/health` 已代理到 `http://127.0.0.1:8093/api/v1/health`；后端日志 `journalctl -u tsloms-server -f` 查看报错。

**Q7：前端渲染正常但接口报跨域（CORS）？**
- 处理：生产由同域 Nginx 反向代理，不存在跨域；若前后端分离部署，需在 CORS 中间件配置正确的生产白名单域名。

### 8.3 数据库 / 数据

**Q8：后端启动失败并报数据库连接错误？**
- 处理：检查 MySQL 用户名/密码/库名与 systemd 环境变量一致；确认 `DB_HOST/DB_PORT` 可达；首次启动需确保 `users` 表为空以触发默认 admin 初始化（或在已有库上使用原账号）。

**Q9：领料后库存对不上？**
- 处理：领料为单事务（扣库存+流水+费用），正常不会出现差额。若异常，优先查看「库存流水」与「操作日志」定位，必要时用「库存调整」修正；联系管理员核查是否有并发扣减。

**Q10：工单领料没有自动生成费用？**
- 处理：确认后端为包含自动费用修复的版本（见 SAR v4.1 修复1）。领料成功后在「维修费用」列表应出现 `type=material` 且关联工单/设备的记录；若缺失，检查后端是否已升级至最新（hash 1e894e30）。

### 8.4 MQTT 与设备

**Q11：设备一直显示离线？**
- 处理：确认设备已连上 MQTT Broker（1883）；确认话题前缀 `trafficLight/{hw_id}/...` 匹配；查看「报文日志」是否有上报；检查离线判定阈值是否合理。

**Q12：设备上报报文但未解析出故障？**
- 处理：核对报文是否严格符合协议（token/ver/checksum/大端）；确认 errCode 在已知 15 种故障含义内；查看报文日志与《设备协议确认清单》。

### 8.5 固件 OTA

**Q13：固件升级下发后设备无响应？**
- 处理：确认设备端支持 `CMD_UPDATE_CONFIG`/`CMD_REBOOT`；此项依赖设备能力，若硬件不支持则升级任务将无法完成（属已知硬件依赖项，见 SAR）。

### 8.6 性能与容量

**Q14：地图大屏加载慢？**
- 处理：首屏 Cesium 静态资源较大，已做分包优化（manualChunks）；建议 CDN 或本地缓存；确认网络带宽充足。

**Q15：报文日志量大影响性能？**
- 处理：报文日志建议异步化/分区归档（后续迭代项）；当前可通过清理历史报文或调整保留策略缓解。

---

## 9. 附录

### 9.1 默认账号与安全

| 项 | 值 | 说明 |
|----|-----|------|
| 默认管理员 | `admin / admin123` | **首次登录必须修改** |
| 密码策略 | ≥10 位 + 字母 + 数字 | 创建/重置强制 |
| JWT 有效期 | 72 小时 | 过期需重新登录 |
| 密码存储 | bcrypt | 不存明文 |

### 9.2 环境变量速查（后端）

| 变量 | 默认 | 说明 |
|------|------|------|
| SERVER_PORT | 8093 | 后端监听端口 |
| APP_ENV | development | 运行环境 |
| DB_DRIVER / DB_HOST / DB_PORT / DB_USER / DB_PASSWORD / DB_NAME | mysql/localhost/3306/tsloms/tsloms/tsloms | 数据库 |
| REDIS_ADDR / REDIS_PASS / REDIS_DB | localhost:6379 / 空 / 1 | 缓存 |
| JWT_SECRET | - | 签名密钥（生产必须强随机） |
| MQTT_BROKER / MQTT_USERNAME / MQTT_PASSWORD / MQTT_CLIENT_ID / MQTT_TOPIC_PREFIX | tcp://localhost:1883 / 空 / 空 / tsloms-server / trafficLight | 消息 |
| LOG_LEVEL | info | 日志级别 |

### 9.3 关键接口速查（API）

| 方法 | 路径 | 说明 | 权限 |
|------|------|------|------|
| POST | /api/v1/auth/login | 登录 | 公开 |
| GET | /api/v1/health | 健康检查 | 公开 |
| GET/POST | /api/v1/devices | 设备列表/新建 | 读公开/写 operator |
| POST | /api/v1/inv/stocks/use | 工单领料出库（自动生成费用） | operator |
| GET/POST/PUT | /api/v1/expenses | 费用列表/登记/更新 | 读公开/写 operator |
| GET | /api/v1/dashboard/overview | 仪表盘概览 | 读 |
| GET | /api/v1/logs/operations | 操作日志 | 读 |

> 完整接口见 PRD v4.1 / 后端路由 `cmd/server/main.go`。鉴权头：`Authorization: Bearer <token>`。

### 9.4 相关文档

| 文档 | 路径 |
|------|------|
| 需求文档 V4.1 | `docs/PRD-TSLOMS-v4.1.md` |
| 结项审核报告 V4.1 | `docs/SAR-TSLOMS-v4.1.md` |
| 本操作手册 V1.0 | `docs/TSLOMS操作手册-v1.0.md` |
| 设备协议确认清单 | `docs/TSLOMS-设备协议确认清单.md` |
| 部署故障排查 | `docs/部署故障排查-腾讯云请求失败.md` |

---

*本文档由 TSLOMS 项目团队维护。截图处按「图号+图名居中于图下」规范预留占位，请在交付前补齐实景截图。*
