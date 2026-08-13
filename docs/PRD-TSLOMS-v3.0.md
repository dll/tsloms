# PRD-TSLOMS-v3.0 交通信号灯检测后台运维系统

**项目名称**：交通信号灯检测后台运维系统

**英文名称**：Traffic Signal Light Operation and Maintenance System

**项目简称**：TSLOMS

**文档版本**：V3.0

**更新日期**：2026-08-14

**适用场景**：城市交通信号灯设备 MQTT 通信对接、故障自动研判、维修工单流转、运维数据可视化统计

**版本关系**：V3.0 基于 V2.0 需求基线，结合《信号灯设备通信协议第三版本（V0.1.3）》与当前已实现代码，对齐"文档-协议-实现"三者，明确已交付/未交付边界。

---

## 0. V3 变更摘要

V3.0 是"实现校订版"——在不推翻 V2 架构的前提下，依据实际代码与 PDF 协议做事实对齐，修正 V2 中与实现不符之处。

| 变更项 | V2.0 | V3.0（现状） |
|--------|------|--------------|
| 健康检查 | 无明确入口 | 新增 `GET /api/v1/health`（公开） |
| 系统操作日志 | 需求提及，无实现 | 已实现：`operation_logs` 表 + 登录/增改审计 + 查询 API |
| 报文日志 | 有落库、无查询 | 已实现 `/logs/packets` 查询 API + 前端日志页 |
| 响应体结构 | 未统一 | 已统一 `{code, msg, data}`，业务错误 `code=-1` |
| 工单驳回状态 | 状态机含 rejected | 修正：rejected 为合法存储态，重新派发为显式 pending 转换 |
| CORS 安全 | 未明确 | 生产仅放行 `ALLOWED_ORIGINS` 白名单 |
| 工单编号 | `WO{yyyyMMdd}{seq}` | 修正为同日自增序号（`NextOrderNo`） |
| 故障时间筛选 | 参数未定义 | 兼容 `start_time`/`start_date`、`end_time`/`end_date` |
| 单元测试 | 0 | 新增协议解析/故障研判/工单状态机/鉴权/日志共 27 项用例 |
| 部署路径 | 无 `/tsloms` 前缀 | Nginx 8092 采用 `/tsloms` 前缀统一入口 |

---

## 一、项目概述

### 1.1 项目简介

系统对接交通信号灯硬件设备，基于《信号灯设备通信协议第三版本（V0.1.3）》，通过 MQTT 消息队列实时接收、解析设备上报的二进制数据包，自动研判设备故障，触发维修工单下发与流转闭环；同时对故障、工单数据聚合统计并通过 ECharts 可视化。解决传统人工巡检滞后、运维数据无数字化统计的痛点。

### 1.2 核心业务流程

```
信号灯设备 → MQTT Broker(Mosquitto) → 后台订阅 trafficLight/+/+/+/U
  → 二进制报文解析校验(token/checksum/EVENT_RECORD)
  → 故障规则研判(30分钟去重) → 正常数据入库存档 / 故障自动生成工单
  → 工单派发、处理、闭环 → 数据聚合统计、图表可视化(看板)
```

### 1.3 功能范围

**已交付**：MQTT 设备通信对接、二进制报文解析校验、故障智能判别与去重、维修工单管理、设备台账管理、数据统计可视化、JWT 权限管理、系统操作日志、报文/操作日志查询、健康检查。

**未交付（后续迭代）**：设备配置下发（CMD_UPDATE_CONFIG）、固件 OTA 完整流程（CMD_CHECK_FW/CMD_GET_FW 响应）、设备离线超时自动判定、用户角色管理 CRUD、看板增强（工单状态饼图/设备故障排行/平均闭环时长/CSV 导出/时间区间筛选）、移动端 APP、短信告警、多级审批、大数据分析、地图可视化。

---

## 二、通信协议（参照《信号灯设备通信协议第三版本 V0.1.3》）

### 2.1 系统架构

信号灯监控设备在检测到故障或签到周期结束时主动向 MQTT 服务器发送数据包；后台通过订阅设备上行 Topic 接收，需要时通过下行 Topic 向设备下发命令。

```
  ┌──────────┐     MQTT      ┌──────────────┐     MQTT      ┌──────────┐
  │ 信号灯    │◄────────────►│  MQTT Broker  │◄────────────►│  TSLOMS  │
  │ 监控设备  │ (U入/D出)    │  (Mosquitto) │ (U入/D出)    │  后台系统 │
  └──────────┘               └──────────────┘              └──────────┘
```

### 2.2 Topic 约定（PDF 3）

- 上行（设备→后台）：`trafficLight/{networkCode}/{stationCode}/{ledHwId}/U`
- 下行（后台→设备）：`trafficLight/{networkCode}/{stationCode}/{ledHwId}/D`
- `networkCode`（0~254）、`stationCode`（0-65534）、`ledHwId` 均为**十六进制大写 ASCII**（如 `0/0/11130000/U`）
- 实现：后台订阅 `trafficLight/+/+/+/U`，收到签到/上电后经 `strings.TrimSuffix(…, "/U")+"/D"` 构造下行 Topic 回时间同步。

### 2.3 命令帧格式（CMD_FRAME）

```
CMD_FRAME (固定头 16 字节 + 变长 dat)：
  token[0]=0x55 | cmd[1] | ver[2]=0x10 | checksum[3] | swVer[4:8] | cmdSeq[8:10] | datLen[10:12] | userVal[12:16] | dat[16:]
```
- 校验：整包所有字节按 uint8 累加，结果低 8 位必须等于 `0xFF`。
- 大小端：多字节字段一律**大端序**（big-endian）。

### 2.4 COMMAND 定义（PDF 3.1.1）

| 编码 | 常量 | 方向 | 实现支持 |
|------|------|------|----------|
| 0x00 | CMD_CHECKIN | 设备→服务器 | ✅ 解析+时间同步回应 |
| 0x01 | CMD_ALARM | 设备→服务器 | ✅ 解析+故障研判（无回应） |
| 0x03 | CMD_POWER_ON | 设备→服务器 | ✅ 解析+时间同步回应 |
| 0x20 | CMD_UPDATE_CONFIG | 服务器→设备 | ⛔ 未实现（后续） |
| 0x30 | CMD_CHECK_FW | 设备→服务器 | 🟡 仅日志，未响应（后续） |
| 0x31 | CMD_GET_FW | 设备→服务器 | 🟡 仅日志，未响应（后续） |
| 0x7F | CMD_REBOOT | 服务器→设备 | ⛔ 未实现（后续） |
| 0x80 | CMD_ACK_FLAG | 回应标志 | ✅ `MakeAckCmd`/`IsAckFrame` |

### 2.5 EVENT_PAK / EVENT_RECORD

- 当 cmd 为 CHECKIN/ALARM 时，dat 按 EVENT_PAK 解释：`eventRecordNum(2) + datLen(2) + EVENT_RECORD[]`
- EVENT_RECORD（本实现按 24 字节 1 字节对齐）：
  `ledHwId(4) + subHwId(4) + swVer(4) + confVer(4) + ledState(1) + errCode(1) + current[3](6)`

> ⚠️ **协议澄清点（待硬件确认）**：PDF V0.1.3 的 EVENT_RECORD 定义（P8）含 `ledState`，但 CMD_CHECKIN 示例（P10）为 `reserved` 字段。本实现按 PRD V2 采用 `ledState` 置于字节 16、`errCode` 于字节 17。**需与设备厂商确认字节 16 语义**，避免状态显示错位。

### 2.6 swVer / confVer 编码（PDF P6/P8）

- `swVer`：`bit[31:28]=主版本, bit[27:24]=次版本, bit[23:18]=年份(2000+n), bit[17:14]=月, bit[13:8]=日, bit[7:0]=当日构建号`
- `confVer`：`0xYYMMDDnn`（16 进制，YY=年 MM=月 DD=日 nn=当日版本）

### 2.7 errCode 定义（PDF 表 3-3）

| errCode | 故障 | 类型 | 等级 | 建单 |
|---------|------|------|------|------|
| 0（LED_ERR_OK） | 正常 | - | - | 否 |
| -1/-2/-3（ROFF/YOFF/GOFF） | 灯周期全灭 | lamp_off | critical | 是 |
| -4/-5/-6/-7（*ON 同亮） | 异常同亮 | abnormal_on | critical | 是 |
| -8/-9/-10（*ON_TIMEOUT） | 亮灯超时 | timeout | normal | 否 |
| -11/-12/-13（*DIM） | 缺亮（暂未实现） | dim | normal | 否 |
| -14（POWER_LOSS） | 断电 | power_loss | critical | 是 |

### 2.8 时间同步（PDF 3.2/3.4）

- 后台对 CHECKIN / POWER_ON 回应：`cmd|0x80`，`userVal = 当前 epoch seconds (UTC+8×3600)`。
- 实现：`BuildTimeSyncAck` + `sendTimeSyncAck`，使用 `Asia/Shanghai` 时区。

### 2.9 固件相关（CMD_CHECK_FW/CMD_GET_FW，PDF 3.1.3-3.5）

- `FIRMWARE_INFO_DAT`：`swVer(4) + fwLen(2) + fwChecksum(1) + reserved(1)`（回应 CHECK_FW）
- `FIRMWARE_PAK`：`target(1) + datLen(1) + offset(2) + dat[](1-256)`（回应 GET_FW，分块，末块除外均 256 字节）
- **当前实现仅记录命令不响应，属于后续迭代。**

### 2.10 设备参数配置（PDF 4）

- 通过 `trafficLightConf` + `mqtt.ini` + `trafficLight.ini` 配置设备（签到周期 checkinMin、亮灯超时 ledMaxPeriodSec*、断电 powerLossSec 等）。
- 参数名与 PRD V2 §3.8 一致。**后台 CMD_UPDATE_CONFIG 下发未实现。**

---

## 三、核心功能需求（已实现）

### 3.1 MQTT 设备通信模块 ✅

- 连接：paho.mqtt.golang，30s 超时、断线自动重连、QoS1。
- 订阅：`trafficLight/+/+/+/U`（依据 `MQTT_TOPIC_PREFIX`）。
- 处理链：`ParseCmdFrame`（token/checksum/datLen 校验）→ `ParseEventPak` → `ParseEventRecord` → 分发（checkin/alarm/power_on）→ 设备 upsert / 故障研判 / 时间同步回应。
- 异常报文：解析失败记 `packet_logs` 并丢弃（不 panic，有 recover 兜底）。

### 3.2 故障研判与去重 ✅

- 依据 errCode 分类（§2.7）。
- 去重：同一设备同一 errCode 在 30 分钟窗口内仅保留一条 active 故障，更新 lastSeen 与电流；超窗则将旧故障转 resolved 再建新故障。
- 严重故障（critical）自动生成维修工单。

### 3.3 工单运维模块 ✅

- 状态机：`pending → processing → completed | rejected`；`rejected → pending`（重新派发，显式转换）。
- 字段：工单编号 `WO{yyyyMMdd}{4位序号}`（同日自增）、关联故障、设备 ID、处理人、创建/闭环时间、维修结果。
- 支持：列表多条件筛选、手动创建、状态更新（完成时联动故障转 resolved）。

### 3.4 数据可视化模块 🟡 部分

- ✅ 看板：设备/故障/工单概览卡片、故障类型占比饼图、故障趋势柱状图（近 7 天）。
- ✅ 后端接口齐备：`/dashboard/overview`、`/fault-type-stats`、`/work-order-stats`、`/fault-trend`、`/device-fault-rank`。
- 🟠 前端看板未接入：工单状态饼图、设备故障排行（后端已有）；平均闭环时长、CSV 导出、自定义时间区间未实现。

### 3.5 日志与审计 ✅

- 报文日志：`packet_logs` 表全量落库，`GET /logs/packets` 分页查询（设备/命令/有效/时间筛选）。
- 操作日志：`operation_logs` 表，登录、设备更新、工单创建/状态变更自动记录，`GET /logs/operations` 分页查询。
- 前端日志页已对接真实数据（报文/操作双 Tab + 分页）。

### 3.6 鉴权与权限 ✅

- JWT（HS256，72h），登录返回 token + 用户信息。
- 中间件：`Auth`（签名+过期+用户存在性校验）、`RequireOperator`（admin/operator）、`RequireAdmin`。
- 密码 bcrypt；生产拒绝弱密钥启动。

### 3.7 健康检查 ✅

- `GET /api/v1/health`（公开），供 Nginx `location /tsloms/health` 探活。

---

## 四、非功能需求（实现校订）

1. **性能**：百级设备并发目标；**⚠️ 报文日志同步写库为潜在瓶颈，尚未异步化/压测**。
2. **可靠性**：原始报文落库、MQTT 自动重连、QoS1；**⚠️ 设备离线超时判定未实现**（在线状态仅置 true）。
3. **兼容性**：完全适配 V0.1.3 协议（token 0x55 / ver 0x10 / 大端 / 时间同步）。
4. **安全性**：JWT 鉴权、角色校验、操作日志审计、CORS 生产白名单；**⚠️ MQTT Broker 未启用用户名/密码认证（生产 MQTT_USERNAME 为空）**。
5. **可维护性**：与 EQS 共享 MySQL/Redis（独立库/DB1 隔离），Nginx 8092/后端 8093 独立。

---

## 五、技术栈与部署（现状）

### 5.1 技术栈

| 层 | 选型 | 现状 |
|----|------|------|
| 后端 | Go 1.22 / Gin / GORM | ✅ |
| MQTT | paho.mqtt.golang / Mosquitto | ✅（PRD V2 建议 EMQX，实际用 Mosquitto，PRD 允许） |
| 数据库 | MySQL 8.0（库 tsloms） | ✅ |
| 缓存 | Redis 7.0（DB 1） | ✅ 连接已初始化（当前业务未用缓存） |
| 前端 | Vue3 + Vite + Element Plus + ECharts | ✅ |
| 部署 | systemd（后端）+ Nginx（8092） | ✅ |

### 5.2 生产部署（实际情况）

- 腾讯云 CVM `129.211.223.113`（与 EQS 同机）
- 访问入口：`http://<IP>:8092/tsloms/admin/`（Nginx 8092 直接对外，**统一 `/tsloms` 前缀**）
- Nginx `location`：`/tsloms/admin`（静态）、`/tsloms/api/`（剥离前缀代理 8093）、`/tsloms/health`、`/`→302
- 后端：systemd `tsloms-server`，8083→实际 8093，`EnvironmentFile=.env`
- 中间件：MySQL(localhost:3306)、Redis(localhost:6379)、Mosquitto(1883)

### 5.3 环境变量（新增）

| 变量 | 说明 |
|------|------|
| `ALLOWED_ORIGINS` | CORS 生产白名单（逗号分隔，可为空） |

---

## 六、核心数据模型（与实现一致）

- `devices`：hw_id（唯一）、intersection、network_code、station_code、sw_version、conf_version、online_status、last_checkin_at、installed_at
- `packet_logs`：device_hw_id、raw_data(blob)、cmd_type、cmd_seq、parsed_result(json)、valid、received_at
- `fault_records`：device_hw_id、err_code、fault_type、fault_level、led_state、current_r/y/g、first_seen、last_seen、status(active/resolved)、work_order_id
- `work_orders`：order_no（唯一）、fault_id、device_hw_id、status(pending/processing/completed/rejected)、assignee_id、result、closed_at
- `users`：username（唯一）、password_hash(bcrypt)、role(admin/operator/viewer)、phone
- `operation_logs`（新增）：user_id、username、action、target、ip、detail、created_at

---

## 七、后续迭代路线（V3.1+）

| 优先级 | 工作项 | PRD 出处 |
|--------|--------|----------|
| P0 | 单元测试覆盖率提升至 ≥80%（当前含协议/故障/工单/鉴权/日志 27 例） | AGENTS.md |
| P1 | 设备离线超时自动判定（签到 3 倍周期 / LWT） | §4.1 |
| P1 | 看板补全：工单状态饼图、设备故障排行、平均闭环时长、时间区间筛选、CSV 导出 | §4.4 |
| P1 | 用户/角色管理 CRUD | §4.6 |
| P1 | MQTT 消息处理异步化 + 报文日志批量写（性能保障） | 非功能1 |
| P1 | MQTT Broker 启用用户名/密码认证 | 非功能4 |
| P1 | 报文日志按月分区/归档 | 非功能2 |
| P2 | CMD_UPDATE_CONFIG 设备配置下发 | §4.5 |
| P2 | 固件 OTA 完整流程（CHECK_FW/GET_FW 响应 + 上传校验） | §4.5/§2.9 |
| P2 | CMD_REBOOT 远程重启 | 协议 |
| P3 | 移动端 APP、短信告警、多级审批、大数据分析、地图可视化 | §9 |

---

## 八、文档与入口速查

| 文档 | 路径 |
|------|------|
| 本需求文档 V3.0 | `docs/PRD-TSLOMS-v3.0.md` |
| 历史需求 V2.0 | `docs/PRD-TSLOMS-v2.0.md` |
| 历史需求 V1.0 | `docs/PRD-TSLOMS-v1.0*.md` |
| 软件审核报告 | `docs/SAR-TSLOMS-v1.0.md` |
| 通信协议 | `docs/信号灯设备通信协议第三版本.pdf` |
| 部署故障排查 | `docs/部署故障排查-腾讯云请求失败.md` |
