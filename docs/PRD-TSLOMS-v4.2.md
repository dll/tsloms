# PRD-TSLOMS-v4.2 交通信号灯检测后台运维系统

**项目名称**：交通信号灯检测后台运维系统

**英文名称**：Traffic Signal Light Operation and Maintenance System

**项目简称**：TSLOMS

**文档版本**：V4.2

**更新日期**：2026-08-15

**适用场景**：城市交通信号灯设备 MQTT 通信对接、故障自动研判、维修工单流转、**维修成本（费用/库存/采购/供应商）归集**、运维数据可视化统计、AI 故障预测/诊断/生命周期溯源、**AI 原生增强（库存/成本分析、运维报告、核心流程建议）**、固件 OTA 升级管理

**需求依据（权威基线）**：
1. `docs/信号灯设备通信协议第三版本.pdf`（V0.1.3，通信协议基线）
2. `docs/信号灯检测器_故障含义.docx`（故障含义与触发条件基线）
3. 本版在 V3.1 基础上补充：**工单管理补全、故障四态生命周期、视频监控增强、地图大屏增强、固件 OTA、库存进销存、费用归集、工单领料闭环**

**版本说明**：V4.2 为**正式发布版（AI 原生增强专题）**。在 V4.1 基础上新增 **AI 原生增强**：① 库存/成本智能分析（LLM 洞察 + 规则兜底）；② 各模块运维报告（日报/库存/成本/故障/工单/设备，自动生成并持久化）；③ 核心运维流程 AI 建议（故障确认/派单辅助、工单 Copilot、备件预领、维修小结）。嵌入式设计不打断人工操作，无 LLM Key 时自动规则降级。**本版作为最终发布基线。**

---

## 0. 协议与实现对应总览

| 协议能力（PDF） | 实现状态 | 备注 |
|-----------------|----------|------|
| 上行签到 CMD_CHECKIN（0x00） | ✅ 已实现 | 解析 + 时间同步回应 |
| 上行告警 CMD_ALARM（0x01） | ✅ 已实现 | 解析 + 故障研判（无回应） |
| 上电报告 CMD_POWER_ON（0x03） | ✅ 已实现 | 解析 + 时间同步回应 |
| 配置下发 CMD_UPDATE_CONFIG（0x20） | ⛔ 未实现 | 后续迭代（依赖设备端配置能力） |
| 固件查询 CMD_CHECK_FW（0x30） | ✅ 已实现 | 完整响应（有新版回位域值，无则0）+ 升级任务联动 |
| 固件请求 CMD_GET_FW（0x31） | ✅ 已实现 | 分块下发 FIRMWARE_PAK |
| 远程重启 CMD_REBOOT（0x7F） | ⛔ 未实现 | 后续迭代（依赖设备指令能力） |
| 回应标志 CMD_ACK_FLAG（0x80） | ✅ 已实现 | MakeAckCmd/IsAckFrame |
| 时间同步（userVal=epoch+UTC8） | ✅ 已实现 | BuildTimeSyncAck |
| 故障含义（DOCX）全量 errCode | ✅ 已实现 | 见 §4 故障研判 |
| FIRMWARE_INFO_DAT / FIRMWARE_PAK | ✅ 已实现 | 固件包模型 + OTA 任务 + 下发 |

> **协议状态提升（对比 V3.1）**：CMD_CHECK_FW / CMD_GET_FW 由「🟡 仅记录」提升为「✅ 完整实现」——固件管理模块（FirmwarePackage 上传/发布 + FirmwareUpgradeRecord 任务 + MQTT 查询/分块下发）已落地。CMD_UPDATE_CONFIG / CMD_REBOOT 仍为后续项。

---

## 一、项目概述

### 1.1 背景

信号灯监控设备在检测到故障或签到周期结束时，主动经 MQTT 向后台上报二进制数据包。系统实时接收、解析、研判，自动生成维修工单并闭环流转；同时归集维修过程产生的**物料领用（库存）与费用（人工/交通/耗材）**，实现「故障→工单→派单→处理→领料→费用→成本统计」的**全链路成本闭环**，支撑运维数字化与成本核算。

### 1.2 系统定位

- **核心运维闭环**：设备接入 → 故障研判 → 工单流转 → 维修处理 → 成本归集。
- **可视化与决策**：仪表盘、地图大屏、监控墙、故障预测/诊断/生命周期（AI）。
- **资产管理**：设备台账、路口管理、固件 OTA、库存进销存、供应商、采购、维修费用。
- **系统治理**：用户/角色/部门、操作审计、报文日志、AI 额度控制。

---

## 二、通信协议（严格依据 PDF V0.1.3）

### 2.1 Topic 约定（PDF §前言）

- 上行（设备→后台）：`trafficLight/{networkCode}/{stationCode}/{ledHwId}/U`
- 下行（后台→设备）：`trafficLight/{networkCode}/{stationCode}/{ledHwId}/D`
- 实现：订阅 `{topicPrefix}/+/+/+/U`；时间同步用 `TrimSuffix("/U")+"/D"` 构造下行。

### 2.2 命令帧 CMD_FRAME（PDF §3.1）

```
CMD_FRAME（16 字节固定头 + 变长 dat）：
00 token=0x55 / 01 cmd / 02 ver=0x10 / 03 checksum(累加低8位==0xFF)
04-07 swVer(位域) / 08-09 cmdSeq / 10-11 datLen / 12-15 userVal / 16+ dat
```
多字节字段一律**大端序**。

### 2.3 COMMAND 定义（PDF §3.1.1）

| 编码 | 命令 | 方向 | 说明 | 状态 |
|------|------|------|------|------|
| 0x00 | CMD_CHECKIN | 设备→服务器 | 定时签到 | ✅ |
| 0x01 | CMD_ALARM | 设备→服务器 | 告警 | ✅ |
| 0x03 | CMD_POWER_ON | 设备→服务器 | 上电/重启 | ✅ |
| 0x20 | CMD_UPDATE_CONFIG | 服务器→设备 | 配置下发 | ⛔ 后续 |
| 0x30 | CMD_CHECK_FW | 设备→服务器 | 固件查询 | ✅ 完整响应 |
| 0x31 | CMD_GET_FW | 设备→服务器 | 固件请求 | ✅ 分块下发 |
| 0x7F | CMD_REBOOT | 服务器→设备 | 远程重启 | ⛔ 后续 |
| 0x80 | CMD_ACK_FLAG | 回应标志 | bit7=1 表示回应帧 | ✅ |

### 2.4 EVENT_PAK / EVENT_RECORD（PDF §3.1.2）

- cmd 为 CHECKIN/ALARM 时 dat 按 `EVENT_PAK`（eventRecordNum(2)+datLen(2)+EVENT_RECORD[]）。
- EVENT_RECORD（24 字节、1 字节对齐、大端）：`ledHwId(4)+subHwId(4)+swVer(4)+confVer(4)+[ledState|reserved](1)+errCode(1)+current[3](6)`。

> ⚠️ **协议歧义（延续，待硬件确认）**：PDF P8 typedef 为 `ledState`，P10 示例为 `reserved`。当前按 `ledState` 解析（`data[16]=LedState, data[17]=ErrCode`）。定稿前需与厂商确认字节16语义。

### 2.5 swVer / confVer 编码（PDF P6/P8）

- `swVer` 位域 `[31:28]=major,[27:24]=minor,[23:18]=year(2000+n),[17:14]=month,[13:8]=day,[7:0]=build#`，系统可 `DecodeSwVer` 解码展示。
- `confVer` = `0xYYMMDDnn`，`DecodeConfVer` 解码。

### 2.6 时间同步（PDF §3.2/§3.4）

CHECKIN / POWER_ON 收到后回 `cmd|0x80`，`userVal = epoch 秒（UTC+8×3600）`；ALARM 无需回应。

### 2.7 固件 OTA（PDF §3.1.3-3.5）✅ 完整实现

- **CMD_CHECK_FW（0x30）**：`HandleCheckFW` 查询最新已发布固件，有新版回 `CHECK_FW|0x80 + FIRMWARE_INFO_DAT`（swVer/fwLen/fwChecksum），无则回 0；同时登记升级任务。
- **CMD_GET_FW（0x31）**：`HandleGetFW` 回 `GET_FW|0x80 + FIRMWARE_PAK`（target/datLen/offset/dat[]，分块 256 字节，末块除外）。
- **固件包管理**：`FirmwarePackage`（版本位域校验 v1.2.3、文件 bin、MD5、发布状态、上传人）+ `FirmwareUpgradeRecord`（设备升级任务：须已发布+设备存在+无重复待升级）。
- 前端固件管理页：固件包上传/发布/下线/删除 + 升级记录 + 发起升级（远端设备选择）。

---

## 三、设备参数配置（PDF §4）

参数由 `trafficLightConf` + `mqtt.ini` + `trafficLight.ini` 承载：`checkinMin`（签到周期）、`gapSec`、`ledMaxPeriodSecR/Y/G`（超时）、`powerLossSec`（断电）、`ledDimThresholdR/Y/G`（缺亮）、`mqttServerIp/Port/UserName/Password`、`mqttTopicPrefix/networkCode/stationCode`、`confVer`。

> ⚠️ 配置下发 CMD_UPDATE_CONFIG 尚未实现；当前参数以设备端为准，后台仅读取上报 `confVer`。

---

## 四、故障研判（严格依据《信号灯检测器_故障含义.docx》）

### 4.1 故障含义总表（DOCX 全量）

| 故障类型 | errCode | 触发条件 | 系统归类 | 等级 | 自动建单 |
|----------|---------|----------|----------|------|----------|
| 正常 | 0 | 无错误 | - | - | 否 |
| 红灯周期全灭 | -1 | 红灯周期全灭 | lamp_off | critical | 是 |
| 黄灯周期全灭 | -2 | 黄灯周期全灭 | lamp_off | critical | 是 |
| 绿灯周期全灭 | -3 | 绿灯周期全灭 | lamp_off | critical | 是 |
| 红黄同亮 | -4 | 红黄同亮 | abnormal_on | critical | 是 |
| 红绿同亮 | -5 | 红绿同亮 | abnormal_on | critical | 是 |
| 黄绿同亮 | -6 | 黄绿同亮 | abnormal_on | critical | 是 |
| 红黄绿同亮 | -7 | 三灯同亮 | abnormal_on | critical | 是 |
| 红灯超时 | -8 | 超 `ledMaxPeriodSecR` | timeout | normal | 否 |
| 黄灯超时 | -9 | 超 `ledMaxPeriodSecY` | timeout | normal | 否 |
| 绿灯超时 | -10 | 超 `ledMaxPeriodSecG` | timeout | normal | 否 |
| 红灯缺亮 | -11 | 预留 | dim | normal | 否 |
| 黄灯缺亮 | -12 | 预留 | dim | normal | 否 |
| 绿灯缺亮 | -13 | 预留 | dim | normal | 否 |
| 断电 | -14 | 三灯同灭超 `powerLossSec` | power_loss | critical | 是 |

**15/15 errCode 与 DOCX 一致。**

### 4.2 故障四态生命周期（本版新增）

故障状态机由「active/resolved」扩展为**四态**：`occurred（发生）→ confirmed（确认）→ dispatched（已派单）→ resolved（已解决）`。

| 状态 | 说明 | 转变 |
|------|------|------|
| occurred | 设备上报，故障刚发生/研判 | 处理人确认 → confirmed |
| confirmed | 已核实确认 | 派单 → dispatched |
| dispatched | 已派发维修工单 | 维修完成 → resolved |
| resolved | 已解决 | 终结态 |

- MQTT 上报新故障落地为 `occurred`；人工确认、派单（关联工单）、解决（联动工单 closed=completed）逐步推进状态。
- 兼容：旧 active 故障启动时统一迁移为 `occurred`（迁移脚本 `b48ca0c`）。
- 故障列表/详情、反馈、派单均按四态呈现，支持按状态筛选、排序、统计。

### 4.3 去重与更新（系统规则）

同一设备同一 errCode 30 分钟窗口内仅保留一条 active 故障；超窗旧故障标记 resolved 再建新；critical 自动建单。

---

## 五、工单管理

### 5.1 状态与流转

- 状态机：`pending（待处理）→ processing（处理中）→ completed（已完成）| rejected（已驳回）`，`rejected → pending`（重新派发）。
- 编号：`WO{yyyyMMdd}{同日自增4位}`。
- 完成联动故障转 resolved（四态闭环）。
- SLA：pending 超 24h / processing 超 48h 判超时（`overdue`）。

### 5.2 工单管理补全（V4 新增）

- **顶部状态统计卡**：全部 / 待处理 / 处理中 / 已解决（completed）/ 已关闭 + **超时数**，可点击筛选。
- **时间范围筛选**：创建时间范围（start_time/end_time）。
- **表格排序**：按 id / status / created_at 排序。
- **详情抽屉**：工单 + SLA 阶段/期限/超时 + 关联故障 + 设备 + 处理人 + 操作时间线（from operation_logs target=work-order/:id）。
- **设备异步分组下拉**：按路口/ID 搜索、在线/离线分组。
- **新建工单**（从活跃故障发起，自动带出设备、可选维修人员）、**删除工单**（仅管理员，解除故障绑定保留记录）。
- **派单**：`PUT /work-orders/:id/assign`（管理员/运维，只能指派运维/管理员，派后进入 processing）；可派单人 `GET /users/assignable`。

### 5.3 工单↔库存↔费用 成本闭环（V4 新增，核心）

- **工单领料出库**：`POST /inv/stocks/use`（见 §7.4）——领料时校验工单+物料+库存充足，扣库存、写 `type=use` 出库流水（关联工单/设备/ref=repair），**同事务自动生成耗材费用单**（关联工单/设备、金额=数量×单价）。
- **费用归集**：`RepairExpense` 支持 material（耗材）/labor（人工）/traffic（交通）/other（其它）四类，按工单/设备/类型/日期筛选统计；设备累计维修成本 TOP 排行。

---

## 六、视频监控与地图大屏

### 6.1 视频监控增强（V4 新增）

- **媒体分类**：举证(evidence)/监控(monitoring)/时间视频(timelapse)，上传(multipart)/RTSP 登记/云 URL。
- **多路口筛选**：`device_media` 增加信号灯字段（intersection/light_color/fault_desc/is_active_fault）；前端视频面板支持多路口多选、路口分组、仅看故障中。
- **手机上传举证必填**：路口名称 + 故障灯色（红/黄/绿/不确定）+ 故障现象，否则拒绝——保障派单定位能力。
- **登记校验**：media_type 白名单、URL 协议（rtsp/rtsps/http/https）校验、兼容播放地址（HLS/FLV）提示、设备存在性校验。
- 查询/删除：`GET /media`、`DELETE /media/:id`。

### 6.2 地图大屏增强（V4 新增）

- **Cesium GIS**：2D / 3D / 哥伦布三模式；底图 OSM/高德/卫星(style=6)/百度，WGS84→GCJ-02/BD-09 转换。
- **图层管理**：信号灯 / 故障 / 锁定图标 图层开关。
- **5 级缩放宽高可调**、快捷按钮、事件总线居中控制。
- **设备关注/锁定字段**：设备支持 focus/lock 标记，锁定图标在地图固着。
- 地图大屏保持单一职责，视频监控 / 问题反馈独立为菜单页。

---

## 七、库存进销存与维修费用（V4 新增模块）

### 7.1 物料档案（materials）

| 字段 | 说明 |
|------|------|
| code | 物料编码，唯一 |
| name / category / spec / unit | 名称/分类(灯泡/电源/控制器/线缆/信号机/其它)/规格/单位 |
| unit_price | 单价(元) |
| stock / threshold | 当前库存 / 库存预警阈值 |
| supplier_id | 默认供应商ID |
| device_hw_id | 绑定设备ID（**可空**，设备耗材才填；旧 DeviceMaterial 合并入口） |
| status | active / disabled |

- 旧耗材台账合并：`device_materials` → `materials`（`device_hw_id` 可空字段承载设备绑定），迁移脚本幂等+初始库存流水+设备绑定。
- 统计卡：物料数 / 低库存数 / 出入库笔数 / 库存总金额（stock×unit_price）。
- 低库存筛选：`stock <= threshold AND threshold > 0`。

### 7.2 出入库流水（material_stocks）

| 字段 | 说明 |
|------|------|
| type | in(采购入库)/use(领用出库)/return(退库)/gain(盘盈)/loss(盘亏报废)/adjust(手动调整) |
| quantity | 变动数量（出库为负） |
| price / amount | 单价 / 金额 |
| ref_type / ref_id | 关联类型(purchase/repair/adjust) / 关联单ID |
| work_order_id | 关联工单ID（领料时绑定） |
| operator / note | 操作人 / 备注 |

### 7.3 供应商（suppliers）与采购单（purchase_orders）

- **供应商**：名称/联系人/电话/地址/邮箱/状态 CRUD。
- **采购单**：单号 `PO{yyyyMMdd}{seq}`，状态 `draft/partial/completed/cancelled`，含 `items`（物料/数量/单价/小计/已入库数）。
- **入库**：`POST /purchases/:id/receive` 支持部分入库，事务内：更新已入库数 → 增物料库存 → 写 `type=in` 入库流水（ref=purchase）。全部入库则状态 completed + 记录 received_at。
- 仅草稿可删除；草稿/部分可取消；completed/cancelled 不可再入库。

### 7.4 工单领料出库（work-order material issue）✅

`POST /inv/stocks/use`（operator）：
- 请求：`material_id, quantity(>0), work_order_id, note?`
- 校验：工单存在、物料存在、库存充足。
- 事务：扣减库存 → 写 `type=use` 负数量流水（ref=repair、关联工单）→ **自动生成 `RepairExpense`（type=material，金额=数量×单价，关联工单/设备）**。
- 响应：`{stock, material_id}`；库存不足/工单不存在/物料不存在返回 code=-1。
- 前端物料页「领料」按钮（operator）：物料 + 工单下拉（异步 `getWorkOrders`）+ 数量 + 备注。

### 7.5 维修费用（repair_expenses）

| 字段 | 说明 |
|------|------|
| expense_no | 费用单号 `FE{yyyyMMdd}{seq}` |
| type | material(耗材)/labor(人工)/traffic(交通)/other(其它) |
| amount | 金额(元) |
| work_order_id / device_hw_id | 关联工单 / 设备（关联工单时校验存在并自动带出设备） |
| work_date / confirmed / operator | 发生日期 / 是否确认入账 / 经办人 |

- **统计**：总金额/笔数 + 分类型汇总 + 设备维修成本 TOP10。
- 领料自动生成的耗材费用 `confirmed=false`，人工可确认入账。

---

## 八、数据可视化

- **仪表盘**：概览 + 故障类型饼图 + 故障趋势柱状图 + 工单状态饼图 + 设备故障排行 + 平均闭环时长 + 时间区间筛选（7/30/90天）+ CSV 导出 + AI 额度用量概览。
- **库存/费用看板**：物料统计卡、库存总金额、费用分类汇总、设备维修成本 TOP。
- 统计接口：`/dashboard/*`、`/inv/materials/stats`、`/expenses/stats`。

---

## 九、AI 能力与额度控制

### 9.1 AI 故障预分析（规则引擎 + LLM 预案）

离线健康分（灯龄/历史故障/电流异常/离线次数/关联媒体反馈）→ 风险等级/预测类型/剩余寿命/置信度；地图按风险着色；LLM（智谱 GLM）生成应对预案，失败回退规则。

### 9.2 AI 故障诊断

反馈文字/图片（glm-4v 看图 + glm-4 读文）→ 诊断结论 + 成因 + 方案 + 建议备件；无图走文本模型；超额/无 LLM 回退规则降级。

### 9.3 AI 生命周期溯源

单设备全流程时间线 + LLM 画像（健康/高频故障/维修闭环/老化风险/保养建议）；失败回退规则画像。

### 9.4 AI 额度控制

`ai_config`（provider/模型/开关/每日 token 与调用上限）、`ai_usage`（每次调用流水）、`ai_predictions`（预测）。超限自动降级规则引擎；API Key 脱敏；`/ai/config`（PUT 仅管理员）、`/ai/usage`、`/ai/usage/logs`、`/ai/usage/reset`。

### 9.5 AI 原生增强（V4.2 新增专题）

为把 AI 从「点状工具」升级为「贯穿运维全流程 + 可沉淀学习」的 AI 原生能力，本版新增三大能力，全部**复用现有 LLM 网关与额度体系**，无 LLM Key 时自动**规则兜底**，纯增量不打断现有流程。

**① 库存 / 成本智能分析**
- `GET /ai/analyze/inventory`：库存健康分析（物料种类/总库存/总金额、低库存/缺货预警、滞销积压、高周转、近6月领用趋势）→ LLM 生成库存健康洞察与补货建议。
- `GET /ai/analyze/cost`：维修成本归因（成本结构按 耗材/人工/交通/其它、高成本设备 TOP、高成本故障类型、月度成本趋势、已确认/未确认）→ LLM 生成成本归因与降本建议。

**② 各模块运维报告（AI 自动生成 + 持久化）**
- `POST /ai/report/generate`：生成**运维日报**（device=day）或模块报告（module=inventory/cost/fault/workorder/device）。
- 报告固化到 `ai_reports` 表，`GET /ai/reports` 可查历史、按模块筛选、对比。
- 无 LLM 时以规则引擎生成同样的结构化要点，功能可用。

**③ 核心运维流程 AI 建议（嵌入式，不打断人工）**
- `GET /ai/advice/fault/:id`：确认/派单前生成**故障摘要 + 优先级(P0/P1/P2) + 应对预案 + 建议备件**（依据故障记录+设备历史+历史领料）。
- `GET /ai/advice/workorder/:id?stage=copilot|summary`：工单 Copilot 生成**根因预判 + 处理步骤 + 备件预领**；`stage=summary` 生成**维修小结**（结果说明/耗材用量/遗留问题）。
- 建议固化到 `ai_advices` 表，`GET /ai/advices` 查询历史。

**新增数据表**：`ai_reports`（报告）、`ai_advices`（流程建议）。

**权限**：分析/报告/建议均为只读调用（各角色可触发，受额度）；报告生成为 operator+。

**前端**：菜单「AI 分析 → AI 工作台」，聚合库存分析/成本分析/运维报告三个页签。

---

## 十、系统治理

- **用户/角色**：admin（全部，含用户管理/删除）、operator（业务写操作：建单/派单/处理/上传/登记/领料/费用/采购）、viewer（只读）。部门/组织管理 CRUD。

### 10.1 三角色权限矩阵（结项复核版）

| 操作 | admin | operator | viewer |
|------|:---:|:---:|:---:|
| 查看（设备/故障/工单/媒体/固件/物料/库存/供应商/采购/费用/日志/AI 预测） | ✅ | ✅ | ✅ |
| 设备新建/编辑、路口更名/定位 | ✅ | ✅ | ❌ |
| 设备删除、路口清空 | ✅ | ❌ | ❌ |
| 故障确认/派单、工单新建/状态流转/派单 | ✅ | ✅ | ❌ |
| 工单删除 | ✅ | ❌ | ❌ |
| 媒体上传/登记/举证、固件上传/发布/升级发起 | ✅ | ✅ | ❌ |
| 固件删除 | ✅ | ❌ | ❌ |
| 物料新建/编辑、库存调整/领料出库、供应商、采购单/入库、费用录入/确认 | ✅ | ✅ | ❌ |
| 物料/供应商/采购/费用 删除 | ✅ | ❌ | ❌ |
| 用户管理、部门管理、重置密码 | ✅ | ❌ | ❌ |
| AI 配置更新/用量重置、AI 用量日志 | ✅ | ❌ | ❌ |
| 运行 AI 预测/诊断/生命周期 | ✅ | ✅ | ✅（只读调用，受额度） |

> ✅ = 允许，❌ = 拒绝（接口层 RequireAdmin/RequireOperator 强制，前端按钮同步隐藏）。结项复核实测 10/10 通过。

### 10.2 多端应用定位（需求方裁定）

**需求方已最终确认并拍板（2026-08-15）**：本项目**仅交付「后端 + Web 管理端」**，不做独立多端应用（移动端 APP / 小程序）。

- 原始需求自 PRD v1.0 起将移动端 APP 列为「不含 / 后续迭代 P3」，且《设备协议确认清单》第 1 项「运维人员移动端 APP」为未勾选项（待定）。
- 结项复核时结合需求方裁定（2026-08-15）再次确认：**本次发布范围=后端 + Web 管理端（单端），无多端交付项。**
- 后端为纯 RESTful JSON API + JWT 无状态鉴权，**已预留多端对接接口**（登录、工单、告警、领料、费用等均可被第三方客户端复用）。若后续需要，仅需新增前端壳，无需改动后端。

> **结论：本版不实现多端（非遗漏，系需求方明确裁定），列为 P3 扩展。结项验收以「后端 + Web 管理端」为准。**

### 10.3 其他治理能力

- **强密码策略**：≥10 位 + 字母 + 数字（创建/重置时校验）。
- **鉴权**：JWT（HS256, 72h）、bcrypt、RequireOperator/RequireAdmin、CORS 生产白名单、拒绝弱密钥。
- **审计**：`operation_logs`（登录/设备/工单/费用/库存/采购等）+ `/logs/operations`；报文日志 `packet_logs` + `/logs/packets`。
- **健康检查**：`GET /api/v1/health`，Nginx `/tsloms/health` 探活。

---

## 十一、非功能需求

1. **性能**：百级设备并发；⚠️ 报文日志同步写库为潜在瓶颈，未异步化/压测（P1）。
2. **可靠性**：原始报文落库、MQTT 自动重连、QoS1；设备离线超时判定已实现。
3. **一致性**：库存扣减/流水/费用生成均在**单事务**内，保证成本闭环一致。
4. **兼容性**：严格遵循 PDF V0.1.3（token/ver/大端/校验和/时间同步/故障表/固件帧）。
5. **安全性**：JWT、角色、审计、CORS 白名单；⚠️ MQTT Broker 用户名/密码认证待开（运维配置）。
6. **可维护性**：与 EQS 共享 MySQL/Redis（独立库），Nginx 8092 / 后端 8093。

---

## 十二、技术栈与部署（现状）

- **后端**：Go 1.22 / Gin / GORM / paho.mqtt.golang / Mosquitto（CGO_ENABLED=0 纯 Go 交叉构建）
- **数据**：MySQL 8.0（tsloms 库，charset=utf8mb4）、Redis 7.0（DB1）
- **前端**：Vue3 + Vite + Element Plus + ECharts + Cesium（地图）
- **AI**：智谱 GLM（glm-4-flash 文本 / glm-4v 多模态），key 复用 WXX 项目
- **部署**：腾讯云 `129.211.223.113`，Nginx 8092（`/tsloms` 前缀，前端 `packages/admin/dist` alias）+ systemd 后端 8093；部署用 tarball+scp 原子发布（服务器 git-over-HTTPS 不稳定）

---

## 十三、数据模型（与实现一致）

- 协议/设备域：`devices`、`packet_logs`、`fault_records`（四态）、`device_media`
- 工单域：`work_orders`、`operation_logs`、`feedbacks`
- **库存域（V4）**：`materials`、`material_stocks`、`suppliers`、`purchase_orders`、`purchase_order_items`
- **费用域（V4）**：`repair_expenses`
- **固件域（V4）**：`firmware_packages`、`firmware_upgrade_records`
- AI 域：`ai_config`、`ai_usage`、`ai_predictions`、`ai_reports`（V4.2 报告）、`ai_advices`（V4.2 流程建议）
- 组织域：`users`、`departments`

---

## 十四、结项验收基线（V4 里程碑）

| # | 验收项 | 标准 |
|---|--------|------|
| 1 | 协议合规 | PDF V0.1.3 全部核心帧解析/校验/时间同步对齐；CHECK_FW/GET_FW 完整响应 |
| 2 | 故障研判 | 15 errCode 与 DOCX 全量一致，四态生命周期闭环 |
| 3 | 工单闭环 | pending→processing→completed 全链路 + SLA 超时 + 详情/统计/筛选 |
| 4 | **成本闭环** | 采购→入库→领料→**自动费用归集**→成本统计，数据链路贯通、单事务一致 |
| 5 | 库存进销存 | 物料/供应商/采购/流水 CRUD + 低库存预警 + 统计 |
| 6 | 固件 OTA | 固件包管理 + 设备升级任务 + MQTT 查询/分块 |
| 7 | 视频/地图/监控墙 | 多路口、举证必填、图层/缩放/锁定 |
| 8 | AI 能力 | 预测/诊断/生命周期 + 额度控制降级 |
| 9 | **AI 原生增强**（V4.2） | 库存/成本 AI 分析 + 各模块运维报告 + 故障/工单流程 AI 建议，生产实测通过（LLM 洞察） |
| 10 | 系统治理 | 三角色、强密码、审计、离线判定 |
| 11 | 测试 | Go 全量测试绿、前端 vue-tsc/ESLint/build 通过 |
| 12 | 角色与多端 | 三角色权限矩阵实测通过；多端定位明确（Web 单端+预留对接） |

---

## 十五、迭代路线（V4.2+）

| 优先级 | 工作项 | 依据/备注 |
|--------|--------|-----------|
| ✅已做 | **AI 原生增强**：库存/成本分析、运维报告、故障/工单流程建议 | V4.2 专题 |
| P0 | 协议澄清：EVENT_RECORD 字节16（ledState vs reserved） | PDF 歧义，需硬件确认 |
| P0 | 单测覆盖率 ≥80%（当前 handler 偏低） | AGENTS.md |
| P1 | 报文日志异步化 + 批量写 / 按月分区 | 性能/可靠性 |
| P1 | MQTT Broker 用户名/密码认证 | 安全（运维配置） |
| P2 | CMD_UPDATE_CONFIG 配置下发 | PDF §4 |
| P2 | CMD_REBOOT 远程重启 | PDF |
| P3 | 移动端 APP、短信告警、多级审批、实景图层、AI 定时自动巡检日报 | 扩展 |

---

## 十六、文档索引

| 文档 | 路径 |
|------|------|
| 本需求文档 V4.2 | `docs/PRD-TSLOMS-v4.2.md` |
| 结项审核报告 V4.2 | `docs/SAR-TSLOMS-v4.2.md` |
| 用户操作手册 V1.1 | `docs/TSLOMS操作手册-v1.1.md` |
| 历史需求 V3.1 | `docs/PRD-TSLOMS-v3.1.md` |
| 通信协议（权威） | `docs/信号灯设备通信协议第三版本.pdf` |
| 故障含义（权威） | `docs/信号灯检测器_故障含义.docx` |
| 设备协议确认清单 | `docs/TSLOMS-设备协议确认清单.md` |
| 部署故障排查 | `docs/部署故障排查-腾讯云请求失败.md` |
