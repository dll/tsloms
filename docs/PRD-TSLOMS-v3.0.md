# PRD-TSLOMS-v3.0 交通信号灯检测后台运维系统

**项目名称**：交通信号灯检测后台运维系统

**英文名称**：Traffic Signal Light Operation and Maintenance System

**项目简称**：TSLOMS

**文档版本**：V3.0

**更新日期**：2026-08-14

**适用场景**：城市交通信号灯设备 MQTT 通信对接、故障自动研判、维修工单流转、运维数据可视化统计

**需求依据（本版严格遵照以下两份文档）**：
1. `docs/信号灯设备通信协议第三版本.pdf`（V0.1.3，通信协议基线）
2. `docs/信号灯检测器_故障含义.docx`（故障含义与触发条件基线）

**版本说明**：V3.0 以协议 PDF 与故障含义 DOCX 为**权威依据**重写需求，明确"协议规定"与"系统实现"的对应关系，作为后续实现的唯一需求基线。

---

## 0. 协议与实现对应总览

| 协议能力（PDF） | 实现状态 | 备注 |
|-----------------|----------|------|
| 上行签到 CMD_CHECKIN（0x00） | ✅ 已实现 | 解析 + 时间同步回应 |
| 上行告警 CMD_ALARM（0x01） | ✅ 已实现 | 解析 + 故障研判（无回应） |
| 上电报告 CMD_POWER_ON（0x03） | ✅ 已实现 | 解析 + 时间同步回应 |
| 配置下发 CMD_UPDATE_CONFIG（0x20） | ⛔ 未实现 | 后续迭代 |
| 固件查询 CMD_CHECK_FW（0x30） | 🟡 仅记录 | 后续迭代完整响应 |
| 固件请求 CMD_GET_FW（0x31） | 🟡 仅记录 | 后续迭代完整响应 |
| 远程重启 CMD_REBOOT（0x7F） | ⛔ 未实现 | 后续迭代 |
| 回应标志 CMD_ACK_FLAG（0x80） | ✅ 已实现 | MakeAckCmd/IsAckFrame |
| 时间同步（userVal=epoch+UTC8） | ✅ 已实现 | BuildTimeSyncAck |
| 故障含义（DOCX）全量 errCode | ✅ 已实现 | 见 §4 故障研判 |

---

## 一、项目概述

### 1.1 背景

信号灯监控设备在检测到故障或签到周期结束时，主动经 MQTT 向后台上报二进制数据包。系统实时接收、解析、研判，自动生成维修工单并闭环流转，实现运维数字化。

### 1.2 拓扑（PDF 图1）

```
  ┌──────────┐   MQTT 3.1.1  ┌──────────────┐  MQTT 3.1.1  ┌──────────┐
  │ 信号灯    │◄─────────────►│  MQTT Broker  │◄─────────────►│  TSLOMS  │
  │ 监控设备  │  (U入 / D出)  │  (Mosquitto) │  (U入 / D出)  │  后台系统 │
  └──────────┘               └──────────────┘              └──────────┘
```

---

## 二、通信协议（严格依据 PDF V0.1.3）

### 2.1 Topic 约定（PDF §前言）

- 上行（设备→后台）：`trafficLight/{networkCode}/{stationCode}/{ledHwId}/U`
- 下行（后台→设备）：`trafficLight/{networkCode}/{stationCode}/{ledHwId}/D`
- `networkCode`（0~254，默认0）、`stationCode`（0-65534，默认0）、`ledHwId` 均为**十六进制大写 ASCII**（示例：`trafficLight/0/0/11130000/U`）。
- 系统实现：订阅 `{topicPrefix}/+/+/+/U`；时间同步回应用 `TrimSuffix("/U")+"/D"` 构造下行 Topic。

### 2.2 命令帧 CMD_FRAME（PDF §3.1）

```
CMD_FRAME（16 字节固定头 + 变长 dat）：
字节  字段      长度    说明
00    token     1     魔术字 0x55
01    cmd       1     命令类型（见 2.4）
02    ver       1     协议版本 0x10
03    checksum  1     整包 uint8 累加低 8 位 == 0xFF
04-07 swVer     4     软件版本（位域编码，见 2.6）
08-09 cmdSeq    2     包序号（数据帧/命令帧独立计数）
10-11 datLen    2     dat 部分长度
12-15 userVal   4     用户自定义（时间同步epoch秒）
16+   dat       变长   EVENT_PAK / FIRMWARE_PAK / FIRMWARE_INFO_DAT
```

- 校验算法（PDF 附录）：整包所有字节按 uint8 累加，结果低 8 位**必须等于 0xFF**。
- 多字节字段一律**大端序**。

### 2.3 COMMAND 定义（PDF §3.1.1）

| 编码 | 命令 | 方向 | 说明 |
|------|------|------|------|
| 0x00 | CMD_CHECKIN | 设备→服务器 | 定时签到，表示工作正常 |
| 0x01 | CMD_ALARM | 设备→服务器 | 告警，信号灯异常 |
| 0x03 | CMD_POWER_ON | 设备→服务器 | 上电/重启完成 |
| 0x20 | CMD_UPDATE_CONFIG | 服务器→设备 | 下发配置更新 |
| 0x30 | CMD_CHECK_FW | 设备→服务器 | 查询是否有新固件 |
| 0x31 | CMD_GET_FW | 设备→服务器 | 请求固件数据 |
| 0x7F | CMD_REBOOT | 服务器→设备 | 远程重启 |
| 0x80 | CMD_ACK_FLAG | 回应标志 | bit7=1 表示回应帧 |

### 2.4 EVENT_PAK / EVENT_RECORD（PDF §3.1.2）

- cmd 为 CHECKIN/ALARM 时，dat 按 EVENT_PAK：`eventRecordNum(2) + datLen(2) + EVENT_RECORD[]`
- EVENT_RECORD（本实现按 24 字节、1 字节对齐、大端）：
  `ledHwId(4) + subHwId(4) + swVer(4) + confVer(4) + [ledState|reserved](1) + errCode(1) + current[3](6)`

> ⚠️ **协议歧义（PDF 内部不一致，需硬件确认）**：
> - PDF P8 typedef：`ledState(1) + errCode(1)`
> - PDF P10 示例：`reserved(1) + errCode(1)`
> 系统当前按 PRD 旧版采用 `ledState` 于字节 16。**正式定稿前应与设备厂商确认字节 16 语义**，若为 `reserved` 则需调整解析。

### 2.5 swVer / confVer 编码（PDF P6/P8）

- `swVer`：`bit[31:28]=major, bit[27:24]=minor, bit[23:18]=year(2000+n), bit[17:14]=month, bit[13:8]=day, bit[7:0]=build#`
- `confVer`：`0xYYMMDDnn`（16 进制，YYYY=年 MM=月 DD=日 nn=当日版本），例 `0x26030801`

### 2.6 时间同步（PDF §3.2/§3.4）

- CMD_CHECKIN / CMD_POWER_ON 收到后，后台返回：`cmd|0x80`，`userVal = 当前 epoch seconds（UTC+8×3600）`。
- CMD_ALARM 不需要回应。

### 2.7 固件相关（PDF §3.1.3-3.1.5、§3.5）

- 设备 CMD_CHECK_FW：服务器如有新固件，回 `CMD_CHECK_FW|0x80` + `FIRMWARE_INFO_DAT`（`swVer(4)+fwLen(2)+fwChecksum(1)+reserved(1)`）；无可升级版本则直接丢弃、不回应。
- 设备 CMD_GET_FW：服务器回 `CMD_GET_FW|0x80` + `FIRMWARE_PAK`（`target(1)+datLen(1)+offset(2)+dat[](1-256)`，分块传输，除末块外均满 256 字节）。
- **当前实现：仅记录 CMD_CHECK_FW/CMD_GET_FW 日志，不响应。属后续迭代。**

---

## 三、设备参数配置（PDF §4）

设备参数由 `trafficLightConf` + `mqtt.ini` + `trafficLight.ini` 配置，参数如下（与故障触发条件强相关，见 §4.2）：

| 参数 | 说明 | 默认示例 |
|------|------|----------|
| checkinMin | 签到周期（分钟） | 2 |
| gapSec | 信号灯切换间隙（秒） | 2 |
| ledMaxPeriodSecR/Y/G | 红/黄/绿最长周期（秒），超时上报 | 100/5/120 |
| powerLossSec | 断电最长等待（秒），三灯同灭超时上报断电 | 200 |
| ledDimThresholdR/Y/G | 红/黄/绿缺亮阈值（暂未启用） | 200 |
| mqttServerIp/Port/UserName/Password | MQTT 连接信息（用户/密码最长8位） | - |
| mqttTopicPrefix / networkCode / stationCode | Topic 前缀与网/站号 | trafficLight/0/0 |
| confVer | 配置版本（YYMMDDBB） | 26040400 |

> ⚠️ 系统**尚未实现后台 CMD_UPDATE_CONFIG 配置下发**（后续迭代）。当前设备参数以设备端 `trafficLightConf` 配置为准，后台仅读取上报的 `confVer`。

---

## 四、故障研判（严格依据《信号灯检测器_故障含义.docx》）

故障类型、errCode、触发条件是系统故障研判的**权威依据**，全量如下：

### 4.1 故障含义总表（DOCX 原文）

| 故障类型 | errCode | 触发条件 |
|----------|---------|----------|
| 正常 | 0 | 无错误 |
| 红灯周期全灭 | -1 | 当前处于红灯周期，但检测到所有灯全灭 |
| 黄灯周期全灭 | -2 | 当前处于黄灯周期，但检测到所有灯全灭 |
| 绿灯周期全灭 | -3 | 当前处于绿灯周期，但检测到所有灯全灭 |
| 红黄同亮 | -4 | 红灯和黄灯同时亮 |
| 红绿同亮 | -5 | 红灯和绿灯同时亮 |
| 黄绿同亮 | -6 | 黄灯和绿灯同时亮 |
| 红黄绿同亮 | -7 | 红、黄、绿三灯同时亮 |
| 红灯超时 | -8 | 红灯亮灯时间超过 `ledMaxPeriodSecR` |
| 黄灯超时 | -9 | 黄灯亮灯时间超过 `ledMaxPeriodSecY` |
| 绿灯超时 | -10 | 绿灯亮灯时间超过 `ledMaxPeriodSecG` |
| 红灯缺亮 | -11 | 预留缺亮判断，阈值 `ledDimThresholdR` |
| 黄灯缺亮 | -12 | 预留缺亮判断，阈值 `ledDimThresholdY` |
| 绿灯缺亮 | -13 | 预留缺亮判断，阈值 `ledDimThresholdG` |
| 断电 | -14 | 三个信号灯同时灭灯时间超过 `powerLossSec` |

### 4.2 系统映射（故障类型分类 + 等级）

| 故障类别 | errCode | 等级 | 自动建单 | 实现 |
|----------|---------|------|----------|------|
| 正常 | 0 | - | 否 | ✅ |
| 灯全灭 | -1/-2/-3 | critical(严重) | 是 | ✅ |
| 异常同亮 | -4/-5/-6/-7 | critical(严重) | 是 | ✅ |
| 亮灯超时 | -8/-9/-10 | normal(一般) | 否 | ✅ |
| 缺亮 | -11/-12/-13 | normal(一般) | 否 | ✅（DOCX 标注"预留"） |
| 断电 | -14 | critical(严重) | 是 | ✅ |

### 4.3 去重与更新（系统规则）

- 同一设备同一 errCode 在 **30 分钟窗口**内仅保留一条 active 故障，窗口内持续上报仅更新 `lastSeen` 与电流值。
- 超窗后：旧故障标记 `resolved`，再创建新故障。
- `critical` 等级（灯灭/同亮/断电）自动生成维修工单。

---

## 五、核心功能需求（系统实现）

### 5.1 MQTT 设备通信 ✅
paho 客户端、自动重连、QoS1、订阅 `trafficLight/+/+/+/U`；解析链路 `ParseCmdFrame → ParseEventPak → ParseEventRecord → 分发`；异常报文记 `packet_logs` 并丢弃（recover 兜底）。

### 5.2 故障研判 ✅（对应 §4）
按 errCode 全量分类、等级、去重、critical 自动建单。

### 5.3 工单运维 ✅
状态机 `pending → processing → completed | rejected`，`rejected → pending`（重新派发）；编号 `WO{yyyyMMdd}{同日自增4位}`；完成联动故障转 resolved；多条件筛选。

### 5.4 数据可视化 🟡
- ✅ 看板概览 + 故障类型饼图 + 故障趋势柱状图；后端统计接口齐备。
- 🟠 前端未接入工单状态饼图/设备故障排行（后端已有）；平均闭环时长、CSV、时间区间未实现。

### 5.5 日志与审计 ✅
报文日志落库 + `/logs/packets`（分页筛选）；操作日志 `operation_logs` + `/logs/operations`（登录/设备/工单审计）；前端日志页已对接。

### 5.6 鉴权与安全 ✅
JWT(HS256,72h)、bcrypt、角色校验（RequireOperator/RequireAdmin）、CORS 生产白名单、生产拒绝弱密钥、操作审计。

### 5.7 健康检查 ✅
`GET /api/v1/health`（公开），Nginx `/tsloms/health` 探活。

### 5.8 路口管理 ✅（新增）
- 路口维度设备统计：`GET /api/v1/intersections` 返回各路口设备总数、在线/离线、活跃故障、经纬度。
- 设备 `devices` 新增 `lat`/`lng`（经纬度，用于地图打点），设备详情可录入/编辑路口名称与坐标。
- 前端「路口管理」页：路口列表 + 按路口筛选设备 + 跳转地图大屏。

### 5.9 地图大屏 ✅（新增）
- 前端「地图大屏」路由 `/map`：基于 ECharts `geo` + 内置中国简图，按设备经纬度打点，显示在线（绿）/离线（红）状态。
- **不依赖第三方地图 AK**（百度/高德），离线可用；后续如需实景地图可在页面内接入地图 SDK。

---

## 六、非功能需求

1. **性能**：百级设备并发；**⚠️ 报文日志同步写库为潜在瓶颈，未异步化/压测**。
2. **可靠性**：原始报文落库、MQTT 自动重连、QoS1；**⚠️ 设备离线超时判定未实现**。
3. **兼容性**：严格遵循 PDF V0.1.3（token 0x55 / ver 0x10 / 大端 / 校验和 / 时间同步 / 故障表）。
4. **安全性**：JWT、角色、审计、CORS 白名单；**⚠️ MQTT Broker 未启用用户名/密码认证**。
5. **可维护性**：与 EQS 共享 MySQL/Redis（独立库/DB1），Nginx 8092 / 后端 8093 独立。

---

## 七、技术栈与部署（现状）

- **后端**：Go 1.22 / Gin / GORM / paho.mqtt.golang / Mosquitto
- **数据**：MySQL 8.0（库 tsloms）、Redis 7.0（DB 1）
- **前端**：Vue3 + Vite + Element Plus + ECharts
- **部署**：腾讯云 `129.211.223.113`，Nginx 8092（`/tsloms` 前缀统一入口）+ systemd 后端 8093
- **核心表**：devices / packet_logs / fault_records / work_orders / users / operation_logs

---

## 八、数据模型（与实现一致）

- `devices`：hw_id(唯一)、intersection、lat、lng、network_code、station_code、sw_version、conf_version、online_status、last_checkin_at、installed_at
- `packet_logs`：device_hw_id、raw_data(blob)、cmd_type、cmd_seq、parsed_result(json)、valid、received_at
- `fault_records`：device_hw_id、err_code、fault_type(lamp_off/abnormal_on/timeout/dim/power_loss/unknown)、fault_level(critical/normal)、led_state、current_r/y/g、first_seen、last_seen、status(active/resolved)、work_order_id
- `work_orders`：order_no(唯一)、fault_id、device_hw_id、status(pending/processing/completed/rejected)、assignee_id、result、closed_at
- `users`：username(唯一)、password_hash、role(admin/operator/viewer)、phone
- `operation_logs`：user_id、username、action、target、ip、detail、created_at

---

## 九、迭代路线（V3.1+）

| 优先级 | 工作项 | 依据 |
|--------|--------|------|
| P0 | 单元测试覆盖率 ≥80%（当前 40+ 例起步） | AGENTS.md |
| P0 | **协议澄清**：与硬件确认 EVENT_RECORD 字节16（ledState vs reserved） | PDF 歧义 |
| ✅ | 设备离线超时判定（签到 3 倍周期，OFFLINE_AFTER_MIN） | 已实现 |
| ✅ | 看板补全（工单饼图/故障排行/平均闭环/CSV/时间区间） | 已实现 |
| ✅ | 用户/角色管理 CRUD | 已实现 |
| ✅ | 路口管理（路口维度统计）+ 地图大屏（ECharts geo 打点） | 已实现（无地图 AK 依赖） |
| P1 | MQTT 消息异步化 + 报文日志批量写 | 性能 |
| P1 | MQTT Broker 用户/密码认证 | 安全 |
| P1 | 报文日志按月分区/归档 | 可靠性 |
| P2 | CMD_UPDATE_CONFIG 配置下发 | PDF §3.1.1/§4 |
| P2 | 固件 OTA（CMD_CHECK_FW/GET_FW 响应 + 上传校验） | PDF §3.1.3-3.5 |
| P2 | CMD_REBOOT 远程重启 | PDF |
| P3 | 移动端 APP、短信告警、多级审批、大数据分析、地图实景图层 | 扩展 |

---

## 十、文档索引

| 文档 | 路径 |
|------|------|
| 本需求文档 V3.0 | `docs/PRD-TSLOMS-v3.0.md` |
| 通信协议（权威） | `docs/信号灯设备通信协议第三版本.pdf` |
| 故障含义（权威） | `docs/信号灯检测器_故障含义.docx` |
| 软件审核报告 | `docs/SAR-TSLOMS-v1.0.md` |
| 历史需求 V2.0 | `docs/PRD-TSLOMS-v2.0.md` |
| 部署故障排查 | `docs/部署故障排查-腾讯云请求失败.md` |
