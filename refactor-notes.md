# TSLOMS 智能多源故障识别研判引擎 —— 功能工程改动记录（refactor-notes）

> **产出专员**：dev-refactor-tsloms（重构开发专员）
> **日期**：2026-08-17 ｜ **性质**：功能性新工程（流水线步骤 2）
> **范围** = **A（仅后端 `packages/server`）**；`packages/admin` 前端**未改动**。
> **依据**：repo 根目录 `pm-checklist.md`（范围=A：智能多源故障识别研判引擎，S1-S10/P1-P3/R1-R10）。
> 目标：把故障识别从"固件单一 `errCode` 1:1 直判"升级为"多源融合智能研判引擎"（准确率 ≥ 99.9999% 最终逼近 100%，态势可溯源、可审计、案例库兜底长尾）。
>
> 说明：本记录覆盖旧版 refactor-notes.md，以**范围=A 后端实现**为权威口径，逐项对齐 pm-checklist 的 S 编号与红线 R1-R10，并给出 build/vet/test 实测结果。

---

## 0. 一句话交付

后端"智能多源故障识别研判引擎"已落地并通过全绿验证（`go build ./...` exit 0、`go vet ./internal/...` exit 0、`go test ./...` 12 包全 ok）。识别链路从"设备固件单点直判"升级为三层研判：**确定性规则引擎（主通道）→ 多源交叉验证置信度 → 案例库模型兜底长尾训练**。既有 `/faults*`、`/work-orders*` 契约与 MQTT 二进制协议、状态机/去重/SLA/自动工单等红线语义全部保持。

---

## 1. 本功能工程逐项改动（对应 pm-checklist 的 S 编号）

### S1 —— 多源证据统一模型 & 落库 ✅
- **新增表 `fault_evidence`**（`internal/model/fault_evidence.go`）：承接固件 `errCode` 事件、电流/灯态、群众反映、手机举证、视频监控等**多源证据记录**。
- 字段：`fault_id`（被过滤证据可为空）、`evaluation_id`（研判批次号）、`device_hw_id`、`source_type`、`err_code`、`led_state`、`current_r/y/g`、`raw_data`、`ref_media_id`、`ref_feedback_id`、`captured_at`、`confidence`。
- **重要语义**：被"误报过滤"（未落故障）的证据**同样落库**，经 `evaluation_id` 关联研判批次，保证 100% 可溯源（pm-checklist 3.1：每起发起研判均有证据记录）。
- 证据来源枚举：`firmware / current / led_state / citizen / photo_evidence / video_monitor`。
- 已加入 `model/migrate.go` 的 `AutoMigrate` 列表（见 §2.1）。

### S2 —— 证据归一化与汇聚 ✅
- `internal/recognition/engine.go` 定义统一中间态 `RuleEvidence`：把不同源头（固件事件、电流、灯态、反馈、媒体）归一成同一证据结构。
- `internal/mqtt/handler.go` 的 `injectAuxEvidence`：检索该设备近 24h 已落库的 `Feedback`（群众反映）与 `DeviceMedia`（举证/监控）→ 汇入归一化佐证信号作为多源交叉验证输入；无记录不注入，不阻塞规则主通道。
- `BuildSignature`：生成证据特征指纹（主信号 `errCode`+电流+灯态+辅助来源有序串），供案例库检索/去重（S8 复用）。

### S3 —— 确定性规则研判引擎（主通道）✅
- **新增包 `internal/faultcode`**：错误码常量 + 分类规则基座。
- **内聚复用既有语义（R9）**：`FaultTypeFromErrCode` / `FaultLevelFromErrCode` 从 mqtt 抽出集中为规则基座，语义与既有**完全一致**（`lamp_off / abnormal_on / timeout / dim / power_loss / unknown`；`critical/major/minor` 等级），未改变任何故障类型/等级业务含义 —— 仅作规则基座内聚，避免 recognition 与 mqtt 循环依赖。
- **`internal/recognition/engine.go`**：规则基座之上扩展"基础置信度"（`errCodeBaseConf`：-1~-14 各错误码 0.92~0.98；未知错误码降 0.4 存疑）。

### S4 —— 多源交叉验证 & 误报过滤 ✅
- `crossValidate`：辅助证据按类型加权印证/否证：
  - 群众/举证/监控 → 每人/每媒体一次印证加成（封顶）；
  - 电流证据 → 与故障类型交叉校验（`currentCorroborates`/`currentRefutes`），关联灯色电流矛盾 → 降级；印证 → 微增；
  - 灯态证据 → 与 `errCode` 指示的故障灯色相互印证（`ledCorroborates`）。
- **安全关键约束**：误报过滤绝不丢弃真故障 —— 只有"明确否证"才过滤；证据冲突/孤证一律降级为 `pending_review`（可被证据补充后升级确认），宁多等证据不漏真故障。
- **M2 缺陷修复**：单通道电流矛盾（非仅双/多通道）亦被识别 → 正确进入 `pending_review` 待复核分流（此前单通道矛盾被遗漏）。

### S5 —— 置信度计算与判定分层 ✅
- 输出融合置信度 `conf`（0-1，3 位小数）。
- **第 3 层判定分流**（阈值可读、可单测）：
  - `conf ≥ 0.90` → `confirmed`（高置信直判，critical 自动派单）；
  - `conf < 0.50` → `filtered`（明确否证 → 误报过滤，仅记证据与案例，不产生故障/工单）；
  - 其余 → `pending_review`（待确认，**不自动派单**，可证据补充升级）。

### S6 —— 故障落库/去重/自动工单改造 ✅
- `internal/mqtt/handler.go` 的 `processFault` 接入研判引擎：
  - **保留 30min 去重窗口（R3）**：同设备同 `errCode` 30 分钟内只维护一条活跃故障，仅更新 `last_seen`（电流/灯态有变化才附带更新）；超窗旧故障置 `resolved` 并新建。
  - **保留 critical 自动工单（R6）**：仅当研判为 `confirmed` 时才自动派单；`pending_review` 不派单。
  - **保留故障状态机（R2）**：`occurred → confirmed → dispatched → resolved` 四态不回归。
  - `filtered` 不产生故障/工单，仅记证据（S1）与案例（S7）。
- **M2 自动升级**：去重窗口内若 existing 为 `pending_review` 且未派单、本次研判达 `confirmed`，则升级为确认；critical 则经原子防重派单（不重复建单）。
- **M1 并发防重建单**：`model.EnsureActiveWorkOrder` 原子式创建/复用一条活跃工单，配合 `work_orders` 活跃工单唯一索引（`fault_active_scope`，pending/processing 时=fault_id），并发入口也只建成一条。

### S7 —— 识别案例库 `fault_case`（数据模型 + 基础读写）✅
- **新增表 `fault_case`**（`internal/model/fault_case.go`）：`fault_type / fault_level / device_hw_id / input_signature / evidence_summary / expected_result / judged_result / judge_confidence / is_correct / source_evaluation_id / status`（seed/confirmed/training/test）。
- **`internal/caselib`**：`CaseRecorder` 提供 `SeedRecord`（研判自动沉淀样本）、`Train`（训练骨架）、`Stats`（识别统计）。
- `processFault` 每次研判后 `persistCase` 自动沉淀样本（含被过滤样本，回标 expected=normal 视判定正确）。

### S8 —— 案例库模型训练/召回框架（服务端）✅
- `SeedRecord`：研判自动沉淀 + 人工回标样本。
- `Train`：批式训练骨架（可解释检索引擎，**不引入不可控黑盒**——安全关键场景需人工/规则可审计）；向 100% 识别率收敛。
- `Stats`：总案例、识别准确率、误报/漏报、误报率/漏报率、已确认/种子样本。
- 检索按 `BuildSignature` 指纹（S2 复用）。
- 训练触发：REST `POST /fault-cases/train`（手动触发；定时/事件自动训练为后续项，见 §5）。

### S9 —— 面向外部数据源的预留接口 ✅
- `POST /evidence/ingest`：预留群众反映/手机举证/视频监控的统一证据写入（归一化落 `fault_evidence`，source_type 白名单校验，可带 fault_id 关联）。
- `GET /evidence/sources`：多源证据类型枚举。
- 真实视频分析 / AI 视觉故障识别 / RTSP 分析**未实现**（P1/P2 预留，仅记录供人工查看）——见 §2.2 P1/P2/P3。

### S10 —— 新增后端接口 + 既有接口向后兼容 ✅
- 新增接口全部走**独立路径**，不与既有 `/faults`、`/work-orders` 冲突（见 §3 接口清单）。
- `/faults*` 既有契约与返回结构不变，仅 `FaultRecord` 新增可空识别字段（置信度/研判状态/来源/证据数/末次批次/复核时间），`faultView` 序列化时附带（前端可选解析，向后兼容）。
- 既有 `ListFaults` 新增可选 `recognition_status` 过滤参数（`active`=旧语义未解决三态；不带则行为完全不变）。

---

## 2. 新增/变更文件、模型、表

### 2.1 模型与表（AutoMigrate 列表内）
| 表 | 文件 | 说明 |
|---|---|---|
| `fault_evidence`（新） | `internal/model/fault_evidence.go` | 多源证据明细（S1） |
| `fault_case`（新） | `internal/model/fault_case.go` | 识别案例库（S7） |
| `fault_records`（增列） | `internal/model/fault.go` | 仅加可空字段：`confidence / recognition_source / recognition_status / is_false_positive / evidence_count / last_evaluation_id / reviewed_at` |

> 三者均已在 `internal/model/migrate.go` 的 `AutoMigrate(...)` 列表（`&FaultRecord{}`、`&FaultEvidence{}`、`&FaultCase{}`）注册，加法迁移，不删/不改既有列，兼容旧数据。

### 2.2 新增包/组件
| 包 | 文件 | 说明 |
|---|---|---|
| `internal/faultcode` | `faultcode.go` | 错误码常量 + `FaultTypeFromErrCode/FaultLevelFromErrCode` 规则基座（语义内聚复用） |
| `internal/recognition` | `engine.go` | 研判引擎：`NewEvaluator/Validate/crossValidate/BuildSignature/EvidenceToModel` |
| `internal/caselib` | `caselib.go` | 案例库 `CaseRecorder`：`SeedRecord/Train/Stats` |

### 2.3 变更文件
| 文件 | 改动 |
|---|---|
| `internal/mqtt/handler.go` | `processFault` 接入研判引擎（分流/证据/案例）；`createWorkOrder` 改 `EnsureActiveWorkOrder`；新增 `injectAuxEvidence/persistEvidence/persistCase` |
| `internal/model/workorder.go` | 新增 `EnsureActiveWorkOrder`（原子防重）；`WorkOrder` 增 `FaultActiveScope` 列 |
| `internal/model/migrate.go` | AutoMigrate 增 `FaultEvidence/FaultCase`；新增活跃工单唯一索引迁移 |
| `internal/handler/recognition.go` | 研判/证据/案例/统计/复核接口（见 §3） |
| `internal/handler/fault.go` | `ListFaults` 增可选 `recognition_status` 过滤 |
| `cmd/server/main.go` | 注册新增路由（含 RBAC 中间件） |

---

## 3. 新增 REST 接口清单

| Method | 路径 | 说明 | RBAC |
|---|---|---|---|
| GET | `/faults/:id/evidence` | 某起故障的多源证据明细（含被过滤批次按 evaluation 回看） | 登录 |
| POST | `/evidence/ingest` | 预留外部数据源证据写入 | `evidence:ingest` |
| GET | `/evidence/sources` | 多源证据类型枚举 | 登录 |
| GET | `/fault-cases` | 案例库检索/列表（设备/类型/状态/正确性/分页） | 登录 |
| POST | `/fault-cases` | 案例库新增 / 人工回标 | `faultcase:manage` |
| POST | `/fault-cases/train` | 触发案例库训练 | `faultcase:manage` |
| GET | `/recognition/stats` | 识别准确率/误报/漏报/案例统计 | 登录 |
| POST | `/faults/:id/review` | 待确认复核（确认真故障/标记误报；确认后 critical 自动派单） | `fault:review` |

> 既有 `/faults*`、`/work-orders*` 契约不变（R9）。`GET /faults` 新增可选 `recognition_status` 过滤参数，不带该参数行为完全不变。

---

## 4. 红线 R1-R10 逐项核对（全部保持 ✅）

| # | 红线 | 状态 | 核对依据 |
|---|---|---|---|
| R1 | **MQTT 二进制协议**（CMD_FRAME/EVENT_PAK/EVENT_RECORD）字节格式不变 | ✅ | `internal/mqtt/parser.go、commands.go` 未改动字节格式；`handler.go` 仅消费解析产物，不重写协议 |
| R2 | **故障状态机** `occurred→confirmed→dispatched→resolved` | ✅ | `model/fault.go` 四态与迁移兜底保持；`processFault` 不改变状态流转 |
| R3 | **30min 去重窗口** | ✅ | `processFault` `dedupWindow = 30*time.Minute`，窗内仅更新 `last_seen`，超窗置 resolved + 新建 |
| R4 | **NextOrderNo**（WO{yyyyMMdd}{4位自增序号}） | ✅ | `model/workorder.go` `NextOrderNo` 原逻辑保持；`EnsureActiveWorkOrder` 复用之 |
| R5 | **critical 自动工单** | ✅ | 仅 `confirmed` 且 `critical` 自动派单；`pending_review` 不派、`filtered` 不产生工单 |
| R6 | **SLA 24/48h**（pending 24h / processing 48h 超时） | ✅ | `WorkOrderPendingSLASeconds=24*3600`、`WorkOrderProcessingSLASeconds=48*3600` 保持 |
| R7 | **识别引擎 + case 库禁止重构语义** | ✅ | `faultcode.FaultTypeFromErrCode/FaultLevelFromErrCode` 语义与既有完全一致；`ai/*、recognition/*、caselib/*` 判定语义内聚复用未改 |
| R8 | **RBAC / 模块化** | ✅ | 新增接口挂 `RequirePerm` 中间件 + 既有模块注册机制追加；核心模块恒启 |
| R9 | **既有 REST 契约 / MQTT 兼容** | ✅ | `/faults*`、`/work-orders*` 返回结构不变，仅加可空字段；新增接口全部独立路径 |
| R10 | **用户表/角色既有列不删除不改类型** | ✅ | `users.username、devices、faults` 既有列未删未改；新增字段全部可空带默认（只加不改） |

> 另：AI 兜底（`ai/*` anomaly/predict）与既有 `/faults*`、`/work-orders*` 既有测试（cov_*/regression/_test.go）无回归。

---

## 5. 验证结果（实测）

| 项 | 命令 | 结果 |
|---|---|---|
| 编译 | `go build ./...`（packages/server） | ✅ exit 0 |
| 静态检查 | `go vet ./internal/...` | ✅ exit 0 |
| 测试 | `go test ./... -count=1` | ✅ 12 包全 ok（cmd/server、internal/ai、caselib、config、handler、logger、middleware、model、mqtt、recognition、service 等） |

> 关键测试佐证：
> - `internal/recognition/engine_test.go` —— 规则基座/交叉验证/分流/置信度单测。
> - `internal/handler/recognition_test.go` + `recognition_regression_test.go` —— 复核/证据/案例/统计接口；`TestM1_ConcurrentReviewDispatchOnce`（8 goroutine 并发复核仅 1 条活跃工单）、`TestM2_PendingReviewAutoUpgradeDispatch`（待确认升级自动派单、已派单不重复派）。
> - `internal/mqtt/recognition_test.go` / `fault_test.go` / `regression_test.go` —— processFault 去重/自动工单/状态机红线回归。

---

## 6. 遗留 / 未实施项与原因（对应 P1-P3 / 明确不做）

| 项 | 状态 | 原因 |
|---|---|---|
| P1 多媒体/群众反馈**真实接入识别**（视频分析/AI 视觉故障识别） | 仅预留 | 本阶段只做字段与接口骨架（`evidence/ingest` + `fault_evidence` 的 `ref_media_id/ref_feedback_id`），不实现真实视频/AI 视觉 |
| P2 监控视频 RTSP 分析 | 不接入 | 监控类媒体仅记录供人工查看，不做 RTSP/AI 分析 |
| P3 规模化分布式训练/在线学习 | 预留 | 本阶段用批式训练 + 规则样例库（`Train` 骨架），预留模型服务接口 |
| 案例库自动训练触发（定时/事件） | 未接 | 当前仅手动 `POST /fault-cases/train` 触发（与后端 `Train` 骨架一致）；自动调度为后续项 |
| 案例库"训练到 100%"达标验证门户 | 部分 | `Stats` 提供准确率/误报/漏报统计；前端历史趋势/置信度分布可视化不在范围=A 内 |

---

## 7. 范围边界说明（如实记录）

- **范围 = A（仅后端 `packages/server`）**。`packages/admin` 前端**未改动**（本记录范围内）。
- **未重写 MQTT 二进制协议**；未删除/修改既有 `FaultTypeFromErrCode/FaultLevelFromErrCode` 判定语义。
- **未改变自动派单策略**（仅维持"critical 自动生成工单"现状语义）。
- 工作区存在其他流水线步骤（第二轮新需求 P0，手机号登录/预警/路口区划/地图取点等）的在途未提交改动（含 `internal/handler/auth_sms.go、warning.go、crossing.go、map_data.go` 等及对应模型），**不属于本功能工程范围**，本记录不展开；这些改动不在本记录承诺的交付与验证口径内。

---

*（全文完。本记录由 dev-refactor-tsloms 输出，供 QA 按红线 R1-R10 回归 & leader 验收。）*
