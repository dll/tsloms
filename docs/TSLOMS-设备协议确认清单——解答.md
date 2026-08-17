# TSLOMS 设备协议确认清单——解答

**提交方**：TSLOMS 交通信号灯检测后台运维系统 — 开发团队

**日期**：2026-08-17

**用途**：对《TSLOMS-设备协议确认清单.md》所列协议歧义、可选扩展、技术确认进行逐项解答，并回答客户提出的"后台能否连上信号灯检测器、自动抓包分析信号灯状态"问题。

**配套文档**：
- `docs/TSLOMS-设备协议确认清单.md`（原清单）
- `docs/信号灯设备通信协议第三版本.pdf`（V0.1.3）
- `docs/信号灯检测器_故障含义.docx`

---

## 一、客户问题评估（重点）

### 客户问题

> 后台能连上信号灯检测器，自动抓包分析信号灯状态？

### 我方答复（已给出）

> 理论上可以，先给数据测试，通了，再连接硬件，通了，就 OK。

### 评估结论：可以实现

该承诺在现有代码上已具备完整链路支撑，分两个阶段即可落地，**无需改动后台主体逻辑**：

| 环节 | 现状 | 代码证据 |
|------|------|----------|
| 连接检测器 | 已实现：MQTT 客户端支持 Broker 地址与账号密码认证，订阅设备上行 Topic | `mqtt/client.go` Connect / SetUsername / SetPassword；`cmd/server/main.go:78` 订阅 `trafficLight/+/+/+/U` |
| 自动抓包解析 | 已实现：校验帧（token=0x55、checksum=0xFF、大端序）→ 解析 EVENT_RECORD（ledState/errCode/current[3]） | `mqtt/parser.go` ParseCmdFrame / ParseEventPak |
| 分析信号灯状态 | 已实现：多源融合研判（固件 errCode 主信号 + 电流印证/否证 + 灯态印证），故障归档、去重、置信度 | `recognition/engine.go` NewEvaluator / Validate；`mqtt/handler.go` HandleCheckin / HandleAlarm |
| 数据落库与展示 | 已实现：电流/灯态写入 FaultRecord，设备页展示固件/配置版本与灯态 | `mqtt/handler.go:354`；`handler/device.go` sw_ver_info / conf_ver_info |
| 反向通道 | 已实现：时间同步应答、固件检查/升级应答（/U 转 /D 下行） | `mqtt/handler.go` buildDownTopic / sendTimeSyncAck / sendFWCheckAck |

### "先给数据测试，通了"——可行性

**可行，且无需真实硬件**。用任意 MQTT 客户端（`mosquitto_pub`、Node MQTT 库等）向后台订阅的上行 Topic 发布构造帧，即可全链路跑通：

1. 上报帧按协议组装：`token(0x55) + cmd(0x00 签到) + ver(0x10) + checksum(0xFF - 前 3 字节和) + swVer(4) + cmdSeq(2) + datLen(2) + userVal(4) + 事件包`。
2. 事件记录 24 字节：`ledHwId(4) + subHwId(4) + swVer(4) + confVer(4) + ledState(1) + errCode(1) + currentR/Y/G(各 2，大端)`。
3. 后台自动解析、研判、生成故障与工单，前端页面可见。

仓库内已具备协议帧构造样例可供复用：`mqtt/parser_test.go`（buildFrame）、`mqtt/handler_cov_test.go`（rec[16]=0x83 构造 ledState 帧）。

**注意**：当前仓库未提供现成的"模拟器/抓包回放工具"。为配合"先给数据测试"，建议补充一个轻量数据测试脚本（Node/Go），将厂商抓包 hex 或协议样例转为 MQTT payload 发布到上行 Topic，属辅助工具，不影响既有功能。

### "再连接硬件"——可行性

**可行，代码无需改动**。设备端按协议 Topic 结构 `trafficLight/{网络号}/{站点号}/{硬件ID}/U` 上报即可；上行 Topic 已用 MQTT 通配符 `+` 订阅（`main.go:78`），硬件 ID 自动从 Topic 提取，后台按接入设备自动识别与建账。

---

## 二、必须确认项解答

### 1.1 EVENT_RECORD 结构（字节 16 语义）

**实现现状**：已按 **ledState（灯组状态）** 分支实现。

- `mqtt/parser.go:120`：`LedState: int8(data[16])`，即 confVer 之后紧接的 1 字节解析为灯组状态。
- 取值映射：0=红灯 / 1=黄灯 / 2=绿灯 / -1=未知（`faultcode.StateR/Y/G/None`，由 `mqtt/commands.go` 转发）。
- 后续 errCode（data[17]）、current[3]（data[18:24]）与 PDF P8、P10 两处定义一致。

**解答**：在厂商最终确认前，系统已按 ledState 分支运行，灯态参与研判引擎的印证（`recognition/engine.go:307` ledCorroborates）。

**风险与可解决性**：
- 若厂商最终确认为 `reserved`，仅需忽略 data[16]（一行改动），但灯态印证证据链会失效（不影响 errCode + 电流主路径）。
- 仍建议厂商提供真实设备抓包样例各 1 份（正常 + 故障）定稿。

### 1.2 current[3] 量程确认

**实现现状**：
- 大端 uint16 已实现：`mqtt/parser.go:123-125` `binary.BigEndian.Uint16` 解析三通道电流。
- 量程 0-2048（相对值）已在结构体注释明确（`mqtt/commands.go:104`）。
- 研判引擎对电流的使用：印证"灯灭时该灯电流显著偏低（<50）"，否证"灯灭但电流正常偏高（>=200）"（`recognition/engine.go:248/271`）。

**解答**：量程与字节序已按协议实现。关于"缺亮一般导致电流值变大"的方向假设，当前研判引擎**未采用该假设**（采用"缺亮电流偏低"方向）。若厂商确认实际方向相反，仅需调整印证/否证阈值方向。

---

## 三、可选扩展项决策解答（6 项）

| # | 扩展项 | 实现情况 | 解答 |
|---|--------|----------|------|
| 1 | 移动端 APP | 未实现 | 纯 Web 管理端，`packages/` 下无移动端工程。如需落地需另行立项（技术栈 + 后端接口已可复用现有告警/工单接口） |
| 2 | 短信告警通知 | 未实现 | 现有告警为站内通知（`service/patrol.go` notifyOps）。如需短信需接入短信服务商（腾讯云 SMS 等） |
| 3 | 多级审批流程 | 部分实现 | 已实现"工单超时自动升级"（pending 超 SLA 自动转 processing，`service/workorder_escalate.go`），但非"派发/关单多层审批"流程。多层审批需按业务流程定义补充 |
| 4 | 固件 OTA 完整流程 | 已实现 | CMD_CHECK_FW(0x30)/CMD_GET_FW(0x31)/CMD_UPDATE_CONFIG(0x20)/CMD_REBOOT(0x7F) 已定义；应答与升级任务完整：固件上传（版本解析、MD5、50MB 限制、位域编码）、发布/下线、升级记录、发起升级（`mqtt/commands.go`、`mqtt/handler.go`、`handler/firmware.go`）。仅待设备端配合实测 |
| 5 | 大数据深度分析 | 已实现 | 风险预测（PredictDevice/RunRulePrediction）、库存/费用分析（AnalyzeInventory/AnalyzeCost）、故障诊断（DiagnoseFeedback）、AI 巡检日报与异常检测（`internal/ai/*`、`service/patrol.go`） |
| 6 | 设备地图可视化 | 已实现 | 路口/道路分级着色聚合接口（`handler/map_data.go` /map/crossing-data、/map/road-data），前端 Cesium 3D 大屏（路口→道路→设备三级下钻，高德/OSM 瓦片，`views/map/CesiumMap.vue`）。仅需补充设备经纬度数据 |

---

## 四、其他技术确认解答

| 项 | 清单标注现状 | 代码现状 | 解答 |
|----|-------------|----------|------|
| MQTT Broker 认证 | 未启用 | 已支持 | `mqtt/client.go` 已实现 username/password 认证；`config/config.go` 支持 `MQTT_USERNAME`/`MQTT_PASSWORD` 环境变量。接入硬件时开启 Broker 鉴权并下发设备账号即可 |
| Topic 前缀 | trafficLight | 已实现 | `config/config.go:59` 默认 `trafficLight`，与协议一致，可环境变量覆盖 |
| swVer/confVer 位域编码展示 | 已存原始值 | 已实现解码展示 | `model/device.go` DecodeSwVer（major<<28 / minor<<24 + 年月日）、DecodeConfVer（0xYYMMDDnn）；`handler/device.go` 返回 sw_ver_info/conf_ver_info，前端设备页展示固件/配置版本 |

---

## 五、落地步骤建议

### 阶段一：数据测试（无硬件）

1. 启动后台（配置 `MQTT_BROKER` 指向测试 Broker，如 EMQX/Mosquitto）。
2. 补充/使用数据测试脚本：读取厂商抓包 hex 或协议样例 → 组帧 → 发布到 `trafficLight/{网络}/{站点}/{硬件ID}/U`。
3. 验证：签到/报警帧解析正确、故障自动研判、电流/灯态入库、前端页面展示、时间同步与固件应答下发。
4. 用真实抓包数据核对解析结果与设备实际状态是否一致。

### 阶段二：硬件接入

1. 开启 Broker 鉴权，为检测器配置账号（`MQTT_USERNAME`/`MQTT_PASSWORD`）。
2. 设备按协议 Topic 上报，后台自动识别硬件 ID、建立设备档案。
3. 对端到端全流程联调：签到 → 报警 → 研判 → 工单 → OTA。

---

## 六、风险与前提

1. 设备帧必须完整合法（token=0x55、整包校验和=0xFF、24 字节事件记录），否则后台按非法帧丢弃并记日志（`mqtt/parser.go` 校验逻辑）。
2. 字节 16 语义（ledState/reserved）仍需厂商抓包定稿；当前按 ledState 实现。
3. Topic 结构需与设备端实际一致（现按协议默认 `trafficLight/网络/站点/硬件ID/U`）；若设备自定义需调整 `main.go:78` 订阅模式。
4. 厂商提供的抓包数据应先按协议核对格式，避免因格式不符导致解析/研判偏差。

---

*本解答由 TSLOMS 开发团队整理，配套文档：`TSLOMS-设备协议确认清单.md`、`SAR-TSLOMS-v2.0.md`、`PRD-TSLOMS-v3.0.md`。*