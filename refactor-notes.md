# TSLOMS 智能多源故障识别研判引擎 —— 重构改动记录（refactor-notes）

> **产出专员**：dev-refactor-tsloms
> **日期**：2026-08-17
> **范围**：**B（前端可视化适配 + 文档）**；后端识别引擎（范围 A）为既有交付物，本记录一并汇总，供前后端对齐。
> 依据：项目根目录 `pm-checklist.md`（识别准确率 ≥ 99.9999% / 案例库 / 训练 100% 识别率目标）。
> 原则：**不改变既有 /faults* 前端调用与返回结构**（只增字段 / 新增文件，不删改既有字段绑定）；**不改后端核心逻辑**。

---

## 0. 一句话交付

后端「智能多源故障识别研判引擎」已交付并通过全绿验证（`go build` / `go vet` 通过，`go test ./...` 12 包全 ok）；本次范围 B 完成**前端可视化适配**：新增识别 API 封装、故障列表「置信度/研判状态」列与「待复核」筛选与复核交互、详情「多源证据」区、识别统计面板、案例库视图与训练入口。前端通过 `vite build`（exit 0），`vue-tsc --noEmit` 对本次改动文件 **0 错误**（仅存既有非本次范围的 CesiumMap.vue 类型告警，见 §6）。

---

## 1. 后端识别引擎（范围 A，已有，汇总）

> 后端逻辑**本次未修改**，此处仅汇总既有接口/字段清单，供前端消费与后续排期核对。

### 1.1 新增包 / 组件

| 组件 | 说明 |
|---|---|
| `internal/recognition` | 判定引擎：`Validate`（规则基座）、`crossValidate`（多源交叉验证）、`BuildSignature`（证据特征指纹）等 |
| `internal/caselib` | 案例库 `CaseRecorder`：`SeedRecord`（研判沉淀样本）、`Train`（训练骨架）、`Stats`（识别统计） |
| `internal/faultcode` | 错误码 + 分类规则基座 |

### 1.2 新增模型与表（AutoMigrate）

- **`FaultRecord` 新增字段**（仅做加法，兼容旧记录）：
  `confidence`(置信度0-1)、`recognition_source`(rule/multi-source/case)、`recognition_status`(confirmed/pending_review/filtered)、`is_false_positive`、`evidence_count`、`last_evaluation_id`、`reviewed_at`。
- **新表 `fault_evidence`**：多源证据（来源、错误码、灯态、三色电流、原始报文、媒体/反馈引用、捕获时间、单条置信度）。
- **新表 `fault_case`**：案例库（故障类型/等级、设备、输入签名 `input_signature`、证据摘要 `evidence_summary`、预期真值 `expected_result`、引擎判定 `judged_result`、判定置信度、是否正确 `is_correct`、来源研判批次 `source_evaluation_id`、状态 seed/confirmed/training/test）。

### 1.3 REST 接口清单（`internal/handler/recognition.go`）

| Method | 路径 | 说明 |
|---|---|---|
| GET | `/faults/:id/evidence` | 拉取某起故障的多源证据明细（按 fault_id / 末次研判批次并集） |
| POST | `/evidence/ingest` | 预留外部数据源证据写入（举证/反馈/监控归一化落 fault_evidence） |
| GET | `/evidence/sources` | 多源证据类型枚举：firmware/current/led_state/citizen/photo_evidence/video_monitor |
| GET | `/fault-cases` | 案例库检索/列表（page/page_size/device_hw_id/fault_type/status/is_correct） |
| POST | `/fault-cases` | 案例库新增 / 人工回标 |
| POST | `/fault-cases/train` | 触发案例库训练（骨架：向 100% 识别率收敛） |
| GET | `/recognition/stats` | 识别统计：`total_cases/accuracy/false_positive/false_negative/false_positive_rate/false_negative_rate/confirmed_or_seed/filtered_as_normal` |
| POST | `/faults/:id/review` | 待确认复核：`{confirmed:bool}` 确认真故障 / 标记误报；确认后回写高置信(0.99)，critical 且此前未派单则自动派单，并同步回标案例库 |

### 1.4 MQTT 引擎接入（`internal/mqtt/handler.go` 的 `processFault`）

- `confirmed`：落库 + critical 自动派单；
- `pending_review`：落库**但不派单**（待人工复核）；
- `filtered`：不产生故障/工单，仅记证据日志。
- `faultView` 已序列化识别字段（`confidence`、`recognition_status`、`recognition_source`、`is_false_positive`、`evidence_count`、`last_evaluation_id`、`reviewed_at`），前端可直接消费。

---

## 2. leader 修复的两个缺陷（范围 A 遗留，本次如实记录）

> 这两个缺陷由 **leader 修复**（在本次 dev 接手前已改好并验证），此处列出供回归追溯：

1. **`caselib_test` 编译错误：`&e` → `e`**
   - 文件中误将取值写为 `&e`（对循环变量取地址），导致编译失败；leader 修正为 `e` 后编译通过。

2. **`recognition` 电流矛盾降级逻辑：单通道电流矛盾未被识别**
   - 原逻辑仅对双通道/多通道电流矛盾做降级处理，单通道电流矛盾被遗漏，无法进入 `pending_review` 待复核分流；leader 补全单通道电流矛盾→降级逻辑，使矛盾信号可被判为低置信待复核而非错误确认。

---

## 3. 本次范围 B：前端改动清单（dev-refactor-tsloms，2026-08-17）

> 目录：`packages/admin/src`。沿用 Element Plus + Vue3 + 既有 `@/utils/request` 封装，**未引入任何新依赖**（依赖包中即含，未新增）。

### 3.1 新增文件

| 文件 | 内容 |
|---|---|
| `src/api/recognition.ts` | 新增识别 API 封装：`getFaultEvidence`、`ingestEvidence`、`listEvidenceSources`、`listFaultCases`、`createFaultCase`、`trainFaultCases`、`getRecognitionStats`、`reviewFault`；并含展示辅助常量与函数：`RECOGNITION_STATUSES`（confirmed/pending_review/filtered）、`recognitionStatusLabel/Tag`、`EVIDENCE_SOURCES`（firmware=固件、current=电流、led_state=灯态、citizen=群众反映、photo_evidence=手机举证、video_monitor=视频监控）、`RECOGNITION_SOURCES`（rule/multi-source/case）、`fmtConfidence`（0-1→百分比） |
| `src/views/fault/cases.vue` | **案例库视图**：`listFaultCases` 列表（设备/故障类型/预期结果/引擎判定/是否正确/置信度/状态/研判批次），支持筛选与分页，提供「训练案例库」按钮调 `trainFaultCases` |

### 3.2 变更文件

| 文件 | 改动 |
|---|---|
| `src/api/fault.ts` | 新增 `FaultItem` 接口并补充识别字段：`confidence`、`recognition_source`、`recognition_status`、`is_false_positive`、`evidence_count`、`reviewed_at`（仅做加法，兼容旧字段）；`getFaults` 返回类型收窄为 `{ list: FaultItem[]; total: number }` |
| `src/views/fault/index.vue` | 故障页可视化适配（详见 §3.3） |
| `src/modules/index.ts` | 在 fault 模块下新增第二条路由 `/fault/cases`（`FaultCases` → cases.vue），复用既有 `routes.length>1` 自动生成侧边栏子菜单机制；**不破坏既有 /fault 路由** |

### 3.3 故障页 `index.vue` 可视化细节

1. **表格新增「置信度」「研判状态」列**：
   - 置信度用 `fmtConfidence`（0-1→%），≥0.8 绿色、0.6~0.8 橙、其余默认。
   - 研判状态标签映射：`confirmed=已确认/绿`、`pending_review=待复核/橙`、`filtered=已过滤/灰`。
2. **搜索栏新增「研判状态」筛选**：提供「待复核(pending_review)」过滤项（filtered 故障后端不落库，故只提供待复核筛选；见 §5 后端限制说明）。
3. **行内「复核」操作**：对 `recognition_status==='pending_review'` 且具权限（admin/operator）的行显示「复核」按钮 → 弹窗（确认真故障 / 标记误报，调 `reviewFault(id, bool)`）；确认后自动 `fetchData()+loadStats()` 刷新。
4. **详情多源证据区**：`getFault` 详情抽屉下新增「多源证据（识别研判）」区，调 `getFaultEvidence` 渲染证据（来源标签、捕获时间、单条置信度、固件错误码/灯态/电流或原始文本、研判批次号）。
5. **顶部识别统计面板**：调 `getRecognitionStats` 展示总案例数、识别准确率、误报、漏报、已确认案例，及误报率/漏报率（准确率按 ≥100%绿 / ≥90%蓝 / 否则橙 着色）。

---

## 4. 前后端契约对齐

- 前端所有识别接口路径/参数与 §1.3 后端 `recognition.go` 完全一致；`/faults*` 既有点调（`getFaults/getFault/updateFault/dispatchFault`）与返回结构未变，仅新增字段。
- 判研状态常量 `confirmed/pending_review/filtered`、判定来源 `rule/multi-source/case`、证据来源枚举均与后端 `internal/model/fault.go`、`fault_evidence.go` 对齐。

---

## 5. 接口缺口与处置记录

1. **`GET /faults` 的 `recognition_status` 筛选（已由 leader 补实现 ✅）**：dev 开发时发现 `ListFaults` 仅按 `status(工单状态)/fault_type/fault_level/时间/设备` 过滤，未支持 `recognition_status`，若不做则前端「待复核」筛选是空操作（参数被忽略、不报错）。**leader 后续在 `internal/handler/fault.go` 的 `ListFaults` 已补上**可选参数下推：`recognition_status=active` 兼容旧语义（=未解决三态 occurred/confirmed/dispatched），其余按字面匹配 `recognition_status` 列；不带该参数时行为完全不变（向后兼容）。已 `go build` + `go test ./internal/handler/` 验证通过。文档与代码现一致。
2. **`filtered` 故障不落 `FaultRecord`**：故故障列表天然不含已过滤项，「已过滤/灰」标签主要用于同名 `fault_evidence`/`fault_case` 溯源（列表行基本不出现），符合需求（前端主要展示已落库故障的 status+confidence）。

---

## 6. 验证结果

- **前端 `vite build`**：✅ exit 0（built in ~2m46s；`cases-*.js`、`recognition-*.js` 等新增 chunk 正常产出；仅 chunk 体积提示，与本次无关）。
- **前端 `vue-tsc --noEmit`**：本次改动文件 **0 错误**；仅残留一座**既有非本次范围**的类型告警：
  `src/views/map/CesiumMap.vue(432)` —— `faultByDev` 索引类型 `undefined`（该文件本次未触碰，属历史遗留；如需根治需单独排期，且必须回归地图大屏）。
- **后端**：本次未改后端，不重复跑；沿用此前 `go build/vet` 全绿、`go test ./...` 12 包全 ok 的验证结论。

---

## 7. 未实施 / 后续建议

- 案例库「新增/人工回标」后端入口已就绪（`POST /fault-cases`），本次前端仅落地列表+训练；「手动新增人工回标样本」交互未做（留待后续，避免扩大本次改动面）。
- 案例库自动训练触发（定时/事件）未接，当前仅手动「训练」按钮（与后端 `Train` 骨架一致）。
- 识别统计面板未做历史趋势/置信度分布图（后端已返 `false_positive_rate/false_negative_rate`），可后续可视化。
- 识别引擎 → 案例库自动沉淀闭环（`SeedRecord` 已在 MQTT 接入）前端无需改动，仅展示可验证。

---

## 8. M1/M2 major 修复（reviewer 遗留项，leader 落实）

> 处理：leader-tsloms ｜ 2026-08-17 ｜ 处理 reviewer-audit-tsloms 评审遗留的 2 个 major 级问题

### M1（critical）ReviewFault 并发复核重复建单 ✅

**问题**：`ReviewFault` 复核确认真故障后自动派单存在并发 TOCTOU 竞态——读 `WorkOrderID==nil` 后非原子建单，`work_orders.fault_id` 原为普通索引（无唯一约束），并发复核/与 processFault 自动派单并发时可能重复建单。

**方案（近似“活跃工单唯一”语义，非破坏）**：
1. **部分唯一索引**（`migrate.go` 新增 `migrateWorkOrderActiveUnique`）：`CREATE UNIQUE INDEX idx_wo_fault_active ON work_orders(fault_id) WHERE status IN ('pending','processing') AND fault_id > 0`——同一故障至多一条活跃工单，允许历史工单；迁移前先清理同 fault_id 多条活跃工单（保留最新，其余置 `rejected` 并注明“系统迁移清理:重复自动派单”，非破坏、可审计）。幂等（索引已存在则跳过）。
   - **关键坑（已修复）**：部分索引必须 `AND fault_id > 0`，否则 `fault_id=0`（未关联故障的占位/测试工单）两条即触发唯一冲突，导致 `TestNextOrderNo_Sequential`、`TestDashboard_*` 等创建无 fault 工单的用例/数据写入失败。
2. **原子防重**（`model/workorder.go` 新增 `EnsureActiveWorkOrder(db, faultID, deviceHwID)`）：先 `Create`（唯一索引为 DB 层闸门）→ 冲突则复用已存在活跃单 → 条件回填 `UPDATE fault_records SET work_order_id=?, status=confirmed WHERE id=? AND work_order_id IS NULL`（应用层抢锁）。
3. **接入**：`mqtt/handler.go` 的 `createWorkOrder` 与 `handler/recognition.go` 的 `ReviewFault` 复核派单均改调用 `EnsureActiveWorkOrder`，删除旧 `faultReviewWorkorder` 直写逻辑。

### M2（critical）pending_review 自动升级缺失 ✅（方案1：实现自动升级）

**问题**：宣称“pending_review 可被证据补充后升级确认”，但自动链路实际不升级——`processFault` 去重窗口内命中 existing 后直接 `Updates(last_seen)` 并 `return`，丢弃本次 judge；critical 初次因电流矛盾降级者永不自动派单，只能人工复核。文档与行为不符。

**方案1（已实现真升级）**：`mqtt/handler.go` `processFault` 去重窗口内，新增分支：若 `existing.RecognitionStatus==pending_review && existing.WorkOrderID==nil && 本次 judge==confirmed`，则把 existing 升级为确认（回写 confidence/recognition_source/evidence_count/last_evaluation_id/last_seen/电流灯态），若 `existing.FaultLevel==critical` 则经 `EnsureActiveWorkOrder` 原子派单（只建 1 单）。绝不把已 confirmed/已派单/超窗 resolved 的故障误降级或重复派单。

### 新增测试（全 PASS）
- `internal/handler/recognition_regression_test.go`：`TestM1_ConcurrentReviewDispatchOnce`——8 个 goroutine 并发复核同一 pending_review critical 故障 → 仅 1 条活跃工单、fault 回写 work_order_id。
- `internal/mqtt/recognition_test.go`：`TestM2_PendingReviewAutoUpgradeDispatch`——预置 pending_review critical 故障 → 二次上报达 confirmed → 自动升级 + 自动派 1 单；已派单后再上报不重复派。

### 验证
`go build ./...` exit 0；`go vet ./internal/...` exit 0；`go test ./... -count=1` **12 包全 ok**（新增 M1/M2 用例 PASS，既有红线 R1–R10 无回归）。

### 改动文件
- `internal/model/migrate.go`（新增迁移函数+索引）
- `internal/model/workorder.go`（新增 `EnsureActiveWorkOrder`）
- `internal/mqtt/handler.go`（`createWorkOrder` 改造 + `processFault` M2 升级分支）
- `internal/handler/recognition.go`（`ReviewFault` 用 `EnsureActiveWorkOrder`，删 `faultReviewWorkorder`）
- 测试：`internal/handler/recognition_regression_test.go`、`internal/mqtt/recognition_test.go`
