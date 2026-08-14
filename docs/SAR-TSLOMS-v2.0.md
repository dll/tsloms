# SAR-TSLOMS-v2.0 软件审核报告

**项目名称**：交通信号灯检测后台运维系统（TSLOMS）

**审核报告版本**：V2.0（基于 PRD-TSLOMS-v3.0）

**审核日期**：2026-08-14

**审核范围**：以《信号灯设备通信协议第三版本.pdf》与《信号灯检测器_故障含义.docx》为权威基线，重点复核**协议合规性**与**故障含义实现对齐**，并复核 V1.0 报告中问题的修复情况。

**审核依据**：
- `docs/PRD-TSLOMS-v3.0.md`（需求基线）
- `docs/信号灯设备通信协议第三版本.pdf`（V0.1.3，通信协议权威）
- `docs/信号灯检测器_故障含义.docx`（故障含义权威）
- 实际代码：`packages/server` + `packages/admin`
- 生产环境：腾讯云 `129.211.223.113`（Nginx 8092 / 后端 8093）

---

## 一、总体结论

协议解析核心与故障研判**高度符合权威文档**，V1.0 报告中的 **6 项 P0 问题已全部修复**并部署验证。系统处于「可试运行、接近可交付」，仍有若干一致性与 UI 呈现问题待收尾。

| 维度 | 评级 | 说明 |
|------|------|------|
| 协议合规性 | 🟢 高 | token/ver/checksum/大端/时间同步/事件解析均对齐 PDF |
| 故障含义对齐 | 🟢 高 | 15 个 errCode 与 DOCX 全量一致（含触发条件对应） |
| 需求符合性 | 🟡 部分 | 核心完成，固件/配置下发/离线判定等未实现（已披露） |
| 功能可靠性 | 🟡 良好 | P0 缺陷已修复；残余若干 UI/一致性项 |
| 代码质量 | 🟡 良好 | 分层清晰、注释规范；缺 handler 层覆盖率 |
| 测试覆盖 | 🔶 部分 | 27 个用例起步，覆盖率约 10-41%（目标 ≥80%） |
| 数据安全 | 🟡 达标 | JWT/bcrypt/审计/CORS 白名单就位；MQTT 鉴权待开 |
| 用户体验 | 🟡 良 | 日志页已对接；故障类型中文呈现待优化 |
| 交付标准 | 🟡 未完全达标 | 见第九节清单 |

---

## 二、协议合规性审核（对照 PDF V0.1.3）

| 协议要素 | PDF 依据 | 实现 | 结论 |
|----------|----------|------|------|
| Topic 格式 `trafficLight/{net}/{sta}/{hwId}/U,D` | 前言 | 订阅 `+/+/+/U`，下行 `TrimSuffix("/U")+"/D"` | ✅ |
| token=0x55 | §3.1 表3-1 | `ParseCmdFrame` 校验 | ✅ |
| ver=0x10 | §3.1 | `CmdVer=0x10` | ✅ |
| checksum 整包 uint8 累加=0xFF | 附录 | `ParseCmdFrame` 校验 + `BuildCmdFrame` 自动计算 | ✅ |
| 大端序 | §3.1 | 全部 `binary.BigEndian` | ✅ |
| CMD_CHECKIN(0x00)/ALARM(0x01)/POWER_ON(0x03) | §3.1.1 | 常量定义 + 分发 | ✅ |
| EVENT_PAK/RECORD 解析 | §3.1.2 | 24 字节、1 字节对齐 | ✅（见歧义点） |
| 时间同步 userVal=epoch+UTC8×3600 | §3.2/3.4 | `Asia/Shanghai` + `BuildTimeSyncAck` | ✅ |
| ACK 标志 bit7=0x80 | §3.1.1 | `MakeAckCmd`/`IsAckFrame` | ✅ |
| CMD_UPDATE_CONFIG(0x20) | §3.1.1/§4 | ❌ 未实现 | 后续 |
| CMD_CHECK_FW(0x30)/GET_FW(0x31) | §3.5/3.1.3-3.5 | 🟡 仅记录，不响应 | 后续 |
| CMD_REBOOT(0x7F) | §3.1.1 | ❌ 未实现 | 后续 |
| FIRMWARE_INFO_DAT / FIRMWARE_PAK | §3.1.3-3.5 | ❌ 未实现（无固件上传/校验） | 后续 |
| swVer 位域编码 | P6 | 未解码展示（存储原始值） | 🟡 建议 |
| confVer 0xYYMMDDnn | P8 | 存储原始值 | 🟡 建议 |

**结论：核心协议解析合规性高；固件与配置下发属明确后续迭代，不构成本期交付缺陷。**

### ⚠️ 协议歧义（延续 V1，待硬件确认）
EVENT_RECORD 字节 16 在 PDF 中不一致：P8 typedef 为 `ledState`，P10 示例为 `reserved`。当前实现按 `ledState` 解析（`data[16]=LedState`，`data[17]=ErrCode`）。**正式定稿前需与设备厂商确认字节 16 语义**，若为 `reserved` 则 LED 状态显示需移除/调整。

---

## 三、故障含义对齐审核（对照 DOCX）

### 3.1 errCode 全量对照

| errCode | DOCX 故障类型 | 系统归类 | 等级 | 自动建单 | 结论 |
|---------|--------------|----------|------|----------|------|
| 0 | 正常 | - | - | 否 | ✅ |
| -1 | 红灯周期全灭 | lamp_off | critical | 是 | ✅ |
| -2 | 黄灯周期全灭 | lamp_off | critical | 是 | ✅ |
| -3 | 绿灯周期全灭 | lamp_off | critical | 是 | ✅ |
| -4 | 红黄同亮 | abnormal_on | critical | 是 | ✅ |
| -5 | 红绿同亮 | abnormal_on | critical | 是 | ✅ |
| -6 | 黄绿同亮 | abnormal_on | critical | 是 | ✅ |
| -7 | 红黄绿同亮 | abnormal_on | critical | 是 | ✅ |
| -8 | 红灯超时 | timeout | normal | 否 | ✅ |
| -9 | 黄灯超时 | timeout | normal | 否 | ✅ |
| -10 | 绿灯超时 | timeout | normal | 否 | ✅ |
| -11 | 红灯缺亮 | dim | normal | 否 | ✅（DOCX 标注预留） |
| -12 | 黄灯缺亮 | dim | normal | 否 | ✅ |
| -13 | 绿灯缺亮 | dim | normal | 否 | ✅ |
| -14 | 断电 | power_loss | critical | 是 | ✅ |

**15/15 全量一致？是。** 系统 errCode 常量、分类、等级、建单规则与 DOCX 定义完全吻合（已由 `TestFaultTypeFromErrCode`/`TestFaultLevelFromErrCode` 覆盖）。

### 3.2 触发条件对应（DOCX 触发字段 → 配置参数）
DOCX 触发条件依赖设备配置参数：`ledMaxPeriodSecR/Y/G`（超时）、`ledDimThresholdR/Y/G`（缺亮）、`powerLossSec`（断电）。这些参数在 PDF §4 明确，系统**能正确识别故障来源（errCode）**但**不参与触发判定**（判定在设备端完成，系统接收已分类的 errCode）——这一分工与协议一致（判定在线设备侧，后台做研判/工单）。

### ⚠️ UX 呈现缺口
系统 `fault_type` 存英文 slug（`lamp_off`/`abnormal_on`…），前端表格直接展示原文。**不符合 DOCX 的中文故障名**（应显示"红灯周期全灭"等）。建议前端加 erCode/故障类型中文映射，提升可读性。

---

## 四、需求符合性（对照 PRD V3）

### 4.1 已实现 ✅
MQTT 通信、协议解析、故障研判与去重、工单流转、设备台账、看板（概览/故障饼图/趋势柱状）、JWT 权限、报文/操作日志查询、健康检查。

### 4.2 未实现（后续迭代，已在 PRD V3 §9 披露）
固件 OTA、配置下发（CMD_UPDATE_CONFIG）、远程重启、设备离线超时判定、用户/角色 CRUD、看板增强（工单饼图/故障排行/平均闭环/CSV/时间区间）、MQTT 异步化。

---

## 五、V1 → V2 修复复核

| V1 问题 | V2 状态 | 验证 |
|---------|---------|------|
| 故障时间筛选参数不匹配 | ✅ 已修复 | 兼容 start_time/start_date |
| 工单 rejected 状态被改写 | ✅ 已修复 | 状态保留，测试通过 |
| 工单编号非连续 | ✅ 已修复 | NextOrderNo 同日自增 |
| CORS 生产无白名单 | ✅ 已修复 | ALLOWED_ORIGINS 白名单 |
| 系统操作日志缺失 | ✅ 已实现 | operation_logs + 查询 + 前端 |
| 报文日志无查询 API | ✅ 已实现 | /logs/packets + 前端 |
| 日志页空壳 | ✅ 已修复 | 真实数据 + 分页 |
| 零单元测试 | 🔶 部分 | 27 例（mqtt 38.5%/middleware 39.2%/model 41.2%/handler 10.3%） |

**生产实测（07:50）**：登录 200、操作日志记录 login 审计、报文日志查询 total=2、8 个核心 API code=0、后台页 200。

---

## 六、代码质量 / 测试 / 安全 / 性能

- **代码质量**：分层清晰、注释完整、统一响应体、生产隐藏内部错误；遗留：`handler` 包冗余辅助函数（未使用）、`FaultTrendStats` 冒泡排序可读性差。
- **测试**：覆盖协议解析（校验和/越界/非法帧）、故障去重/建单/等级、工单状态机（含 rejected）、JWT 鉴权、日志查询；**handler 层覆盖率低（10.3%）**，需补设备/故障/看板/鉴权接口用例。
- **安全**：JWT-HS256 防混淆、bcrypt、角色、CORS 白名单、审计、拒绝弱密钥；**MQTT Broker 未启用用户/密码认证**（待开）。
- **性能**：报文日志同步写库为潜在瓶颈、趋势全表拉取，未压测；建议 P1 异步化 + 缓存。

---

## 七、交付标准结论

**判定：接近可交付（试运行级），差 3 项即可满足内测交付。**

### 建议本期内完成（低风险收尾）
1. 🟠 故障类型/errCode **中文呈现**（前端映射，对齐 DOCX）
2. 🟠 补齐 handler 层接口测试（设备/故障/看板/鉴权），整体覆盖率向 ≥80% 收敛
3. 🟠 更新 SAR 文档索引与 PRD V3 一致性

### P1（下一迭代）
设备离线判定、看板补全、用户管理、MQTT 异步化、MQTT 鉴权、固件 OTA、配置下发、报文日志分区。

### 需外部确认
- ⚠️ **EVENT_RECORD 字节 16（ledState vs reserved）**：与硬件厂商确认。

---

## 八、测试清单（当前 27 例，全部通过）

| 包 | 用例 | 覆盖点 |
|----|------|--------|
| internal/mqtt | parser_test 11 例 | 帧解析/校验和/token/长度/datLen/事件包/分类/等级/ACK |
| internal/mqtt | fault_test 5 例 | 去重窗口/超窗重建/critical建单/normal不建单/不同errCode分离 |
| internal/handler | workorder_test 5 例 | pending→processing/rejected保留/完成联动故障/非法状态/404 |
| internal/handler | log_test 2 例 | 操作日志查询/报文日志有效筛选 |
| internal/middleware | auth_test 5 例 | 有效/无token/错签名/过期/角色 |
| internal/model | workorder_test 4 例 | 序号自增/跨日/常量 |

---

*本报告基于代码静态审核与生产实测；性能与并发指标未经压测，需补充基准测试。*
