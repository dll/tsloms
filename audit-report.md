# TSLOMS 智能多源故障识别研判引擎 —— 代码评审报告（audit-report）

> **产出专员**：reviewer-audit-tsloms（代码审核专员，只读评审源码，未修改任何源码/文件）
> **日期**：2026-08-17
> **范围**：**B（后端识别引擎 + 前端可视化）**
> **评审基线**：repo 根 `packages/server`（Go 后端，重点 `internal/recognition`、`internal/caselib`、`internal/faultcode`、`internal/model`、`internal/handler`、`internal/mqtt`）+ `packages/admin`（Vue3 前端）
> **输入**：`pm-checklist.md`、`refactor-notes.md`、`qa-report.md`
> **验证**：本地 `go build ./...`（exit 0）、`go vet ./internal/...`（exit 0）、`go test ./...`（12 包全 ok）、识别引擎安全红线测试逐条 PASS。**本报告不落盘任何对产品源码的改动。**

---

## 一、评审范围与结论总评

### 1.1 评审内容

| 维度 | 评审对象 | 结论 |
|---|---|---|
| 识别引擎正确性/安全性 | `internal/recognition/engine.go`（Validate/crossValidate/分流/阈值/降级/电流交叉验证） | ✅ 核心分流与安全红线正确 |
| 案例库 | `internal/caselib/caselib.go`（SeedRecord/Train/Stats/ScoreByRules） | ✅ 基本正确，有 2 处 minor |
| 错误码基座 | `internal/faultcode/faultcode.go` + `mqtt/commands.go` 转发热 delegate | ✅ R10 语义一致 |
| 数据模型 | `model/fault.go`/`fault_evidence.go`/`fault_case.go`/AutoMigrate | ✅ 只做加法合规 |
| REST 接口 | `handler/recognition.go`、`handler/fault.go`(ListFaults 筛选)、路由注册 | ⚠️ 1 处 major 并发风险 + 若干 minor |
| MQTT 接入 | `mqtt/handler.go` processFault/createWorkOrder | ✅ 红线保持，1 处行为/注释不符 |
| 向后兼容 | `/faults*`、`/work-orders*`、前端既有调用 | ✅ 向后兼容（加字段不删不改） |
| 与 internal/ai 一致性 | `internal/ai`（diff 零改动） | ✅ R7 兜底未退化 |
| 前端可视化 | `src/api/recognition.ts`、`index.vue`、`cases.vue`、`modules/index.ts`、`fault.ts` | ✅ 契约对齐，类型/构建通过 |

### 1.2 结论总评

**✅ 可合入（建议附 1 个 major 处理建议，不阻断核心正确性验证）。**

评审核验了 pm-checklist 最高安全红线——「**绝不误过滤真故障**」：
- 全量 14 个已知 errCode × 最大矛盾证据组合，自动分流结果**只有 confirmed / pending_review，绝无 filtered**（逐条实测 + 引擎数学推导双重证实，见 §二 A2/A3）。
- 未知 errCode 提前 return 为 `pending_review`（宁待确认不高判/不误报）。
- `filtered` 仅能由人工 `POST /faults/:id/review {confirmed:false}` 显式落定，自动链路**不可能误过滤真故障**。

红线 R1–R10、接口向后兼容、前端契约、AI 兜底均无退化（QA 回归 + 本次只读复核一致）。leader 三处缺陷修复均得当（§三）。**无 critical 问题**。

唯一需要提请重点关注的 major 项是 **ReviewFault 复核「自动派单」的并发去重缺口**（§二 M1）——语义为安全关键（重复派单），建议合入前或紧跟其后修复，但不改变自动识别链路不误过滤的安全结论。

---

## 二、问题分级清单

### ◎ Critical（无）

未发现会「丢弃/误判真故障」「热路径崩溃」「安全关键违约」的 critical 问题。原 power_loss 单通道 nil panic（QA 5.1）已被 leader 修复并通过安全回归，不再构成 critical。

### ◎ Major

#### M1 (handler/recognition.go `ReviewFault` + `faultReviewWorkorder`) —— 复核确认后自动派单存在并发重复派单竞态（TOCTOU）

- **文件:行**：`packages/server/internal/handler/recognition.go`（`ReviewFault` 内复核确认真故障分支 + `faultReviewWorkorder.create()`）
- **问题描述**：
  ```go
  if req.Confirmed && fault.FaultLevel == "critical" {
      model.DB.First(&fault, id)
      if fault.WorkOrderID == nil {          // 读
          h := &faultReviewWorkorder{...}
          h.create()                          // 写
      }
  }
  ```
  `create()` 内 `NextOrderNo` + `Create(WorkOrder)` 是「读 `WorkOrderID==nil` → 新建工单」的**非原子读-写**。两个并发复核请求（双击/多端同时点「确认真故障」）都能读到 `WorkOrderID==nil`，从而**各自创建一张 pending 工单**。`fault_id` 在 `work_orders` 上仅为 `index`、**无 unique 约束**（`internal/model/workorder.go`），无法在 DB 层拦截。
- **依据**：`qa-report` 的 `TestB8_ReviewPendingCriticalDispatchOnce` 是**串行重复调用幂等**验证（二次复核时已能读到 `work_order_id`），**未覆盖并发窗口**。自动链路 `processFault`/`DispatchFault` 分别因「新建即派」/「先查存在则复用」而风险较低，但 review 路径是**对既有已落库 fault 的第二次写路径**，并发窗口真实存在。
- **影响**：对同一 critical 故障重复产生工单，破坏 R6「critical 只自动建单一次」与工单管理的资源语义。
- **修复/处理建议**：
  1. 在 `work_orders` 的 `fault_id` 上加唯一约束（`uniqueIndex`），DB 层兜底；Create 冲突时捕获并回查已有工单（幂等）。
  2. 或将「校验 WorkOrderID==nil + 建单 + 回写 work_order_id」放入**单一事务**，并用条件更新 `WHERE work_order_id IS NULL`（`UPDATE fault_records SET work_order_id=? WHERE id=? AND work_order_id IS NULL` + RowAffected 判定）做乐观并发门闩；只有拿到行的请求才建单。
  3. 至少加并发幂等回归测试（两个 goroutine 并发调 review），断言只产生 1 张工单。

#### M2 (mqtt/handler.go `processFault` 去重窗口内) —— 「证据补充后升级确认」只在注释/文档层成立，运行时未升级

- **文件:行**：`packages/server/internal/mqtt/handler.go`（dedup 窗口内更新分支）
- **问题描述**：`processFault` 每次都构造全新 `Evaluator`、注入辅助证据并 `Validate()`；但当命中 30min 去重窗口内已有故障时，**直接 `return`（只更新 last_seen/电流/灯态），丢弃本次 `judge` 结果**：
  ```go
  if now.Sub(existing.LastSeen) <= dedupWindow {
      // ...Updates(...)
      return   // ← judge（可能已升级为 confirmed）被丢弃
  }
  ```
  而 engine.go 注释与 refactor-notes/pm-checklist 声称「pending_review 可被证据补充后**升级确认**」。当前该升级**仅能由人工 `POST /faults/:id/review` 完成**，自动链路对同一 pending_review 故障后续到达的更高置信/确认信号**不会自动升级或自动派单**。
- **依据**：代码走读 + engine 测试仅覆盖单次 Validate，未覆盖「同一故障窗口内二次研判升级」路径。
- **影响**：语义分歧（非安全回归——不会漏报，只是不自动升级/不自动派单）。对一个**真实 critical 但初次研判因电流矛盾降为 pending_review** 的故障，后续正常信号到达也**不会自动派单**，只能靠人工复核，拉长响应。属产品一致性问题。
- **修复/处理建议**：若设计确为「证据补充后自动升级」，应在去重窗口内分支：当 `judge.RecognitionStatus==confirmed` 且 `existing.RecognitionStatus==pending_review` 时，升级 existing 为 confirmed、回写 confidence，并在 `critical` 时补建工单（复用 M1 的幂等门闩）。若非目标，则**修正注释/文档措辞**（改为「仅支持人工升级」），避免误导。

### ◎ Minor

#### m1 (handler/recognition.go `ListFaultEvidence`) —— 死赋值
- `if fault.LastEvaluationID != ""` 分支内的 `q = model.DB.Where("evaluation_id = ?", ...)` 是**死赋值**——随后走 `model.DB.Where(fault_id OR evaluation_id).Find(&merged)`，`q` 未被使用。
- **建议**：删除该行死赋值，或用 `q` 承载并集查询消除重复表达式。

#### m2 (caselib/caselib.go `Stats()`) —— 死变量及低质量度量
- `var confirmed, pending, filtered int64` 中 `pending` 声明后仅 `_ = pending`（死变量）。
- `confirmed_or_seed` 实际 =「status∈{seed,confirmed} 的案例总数」，与「已确认」语义略不符；`filtered_as_normal` =「judged_result==normal 的案例数」作为 filtered 近似，与真实 filtered 分流不完全等价。
- **建议**：删除死变量；`confirmed_or_seed`/`filtered_as_normal` 的命名与计算口径建议在注释中明确「近似统计」，避免误导观测者。

#### m3 (recognition 电流交叉验证) —— 魔法数字无配置/未命名
- `currentCorroborates` 的 `< 50`、`currentRefutes` 的 `>= 200`（lamp_off）、`< 50`/`>= 100`（power_loss）、`crossValidate` 的 `+0.03/0.02/0.05×min(n,3)/-0.30` 均为硬编码魔法数字。
- **建议**：提取为常量并加注释（如 `currentRefuteHigh=200`、`currentCorroborateLow=50`、`powerLossRefuteSum=100`），便于后续按工况标定。

#### m4 (data model) —— `fault_case` 无 DB 唯一约束（pm-checklist 建议而未采纳）
- pm-checklist §5.1 建议 `(input_signature, device_hw_id)` 唯一约束防重复样本。当前仅由 `caselib.SeedRecord` 做**应用层 query-then-insert** 去重。MQTT 高速/并发上报时（多 goroutine/Hotpath），两请求可能同时未命中 existing 而重复写入。
- **建议**：在 `model/fault_case.go` 加 `gorm:"uniqueIndex:idx_case_sig_hw"` 或迁移时建唯一索引；Create 命中唯一冲突时回查返回已有样本（幂等）。

#### m5 (contract) —— `reviewed_at` 前后端契约不一致（死字段）
- 前端 `src/api/fault.ts` `FaultItem` 声明 `reviewed_at`，后端 `faultView`/`faultViewWithNames` **未序列化 `reviewed_at`**。当前前端也未渲染该字段，故不报崩溃，但属「声明了拿不到」的死契约。
- **建议**：二选一——后端 `faultView` 补 `"reviewed_at": f.ReviewedAt`；或前端删除该字段声明。

#### m6 (frontend) —— `fault.ts` `FaultQuery` 未声明 `recognition_status` 参数
- `fetchData()`（index.vue）以 `Record<string,any>` 传参绕过了 TS 检查，能正常下发 `recognition_status`。但 `FaultQuery` 接口本身未含该可选字段，接口描述不完整。
- **建议**：在 `FaultQuery` 追加 `recognition_status?: string`，使契约自文档化。

#### m7 (RBAC) —— `fault:delete` 与 `fault:review` Sort 码重复（QA 5.3.1 复核）
- `internal/model/rbac.go`:106 `fault:delete` Sort 8、:108 `fault:review` Sort 8 重复。仅影响权限列表展示排序，**无功能影响**（判定用 Code 非 Sort）。
- **建议**：把 `fault:review` 置 8、`fault:delete` 置 11（或重新排 fault 模块），保持排序整洁。

#### m8 (recognition) —— `filtered` 自动分流分支为死代码（QA 5.3.2，符合安全设计，保留）
- `Validate` 中 `conf < ConfLow(0.50)` → filtered，但已知码 base≥0.92、最大降幅 -0.30 → 下限 0.62 > 0.50，故自动链路**不可达**（未知码提前 return 走 pending_review）。`filtered` 仅可能由人工 review 落定。
- **结论**：这是「自动链路绝不误过滤真故障」的安全设计使然，**属有意死代码**，建议保留并注明（现有 `TestValidate_FilteredOnlyViaManualReview` 已固化为守护用例）。若未来需支持「自动过滤」，需重新评估降级下限与安全约束，**不宜盲目扩通**。

#### m9 (QA 5.2 复核) —— abnormal_on/timeout/dim 电流交叉校验缺失
- 复测确认：`currentRefutes`/`currentCorroborates` 仅覆盖 `lamp_off`/`power_loss`；对 abnormal_on(-4..-7)/timeout(-8..-10)/dim(-11..-13) 注入矛盾电流后 `crossValidate` 因 `refutes` 返回 false，conf 维持 0.92-0.98、仍判 **confirmed**（测试输出证实）。
- **影响**：非误报风险（安全方向正确），但「多源交叉验证」在这三类故障上实际退化回「单源（仅固件 errCode）+ 印证加成」，矛盾信号无法使它们**降级到待复核**。与「固件为主、多源交叉提可信」的设计目标未完全落地。
- **建议**：扩展 `currentRefutes` 覆盖 abnormal_on（本应单色同亮却出现该色电流异常）/timeout（亮灯超时但电流缺失或异常）/dim（缺亮但对应色电流偏高矛盾）等规则；或至少保证对这三级冲突信号能降到 `pending_review`（宁复核不误判）。可排期非阻塞。

---

## 三、对 leader 三处缺陷修复的复核结论

| # | 缺陷 | leader 修复 | 复核结论 |
|---|---|---|---|
| 1 | `caselib_test.go` 编译错误：`&e` → `e` | `mustConfirmed()` 返回 `(model.FaultRecognition, *recognition.Evaluator)`，`&e` 修正为 `e`（循环/取值语义正确） | ✅ **得当**。`go test ./internal/caselib` 全过；测试对 evaluator 的引用语义正确，无遗漏取址引用。 |
| 2 | recognition 电流矛盾降级逻辑：单通道电流矛盾未降级 | 改为「按故障关联灯色判断 + anyCurrent」：`lamp_off` 按关联色通道存在即可判（`currentRefutes` 仅需关联色非 nil），并新增 `anyCurrent`/`sumCurrent` | ✅ **得当**。单通道矛盾电流现能触发灯色判定的降级（`TestValidate_ConflictingCurrentDowngradesToPending` 通过），未知码/冲突信号正确落 `pending_review`。 |
| 3 | QA 5.1 major：power_loss 单通道电流 nil 指针 panic | `currentRefutes`/`currentCorroborates` 的 `power_loss` 分支改用 `sumCurrent(ev)` 逐通道判空、缺失按 0 计，不再直接解引用三指针；QA 反向用例改为正向守护 `TestValidate_PartialChannelPowerLossPanic` | ✅ **得当**。修复彻底：仅依赖「至少一通道存在」（`anyCurrent` 门闩）+「对已提供通道求和」（`sumCurrent`），单通道证明不再触发 `*nil` 解引用；守护用例实测 PASS，无新引入问题。`power_loss` 单通道高电流（R=500）现正确走「矛盾→降级待确认」而非崩溃。 |

**总体**：leader 三处修复均针对根因、语义正确、并配套正向守护测试防回归，**无需调整**。

---

## 四、代码质量观察

### 4.1 重复 / 死代码 / 结构
- `faultView` 与 `faultViewWithNames` 存在两处几乎重复的视图构建（fault.go）——后者引入预取姓名消除 N+1，前者仍逐行单查。属既有遗留，本次未加重；建议后续统一收敛到 `faultViewWithNames`（已批量预取）。
- 死代码/死赋值：`ListFaultEvidence` 的 `q` 死赋值（m1）、`caselib.Stats` 死变量 `pending`（m2）、`filtered` 自动分流分支为**有意**死代码（m8）。
- `faultcode` 抽出后 `mqtt/commands.go` 的 `LEDErr*`/`State*`/`FaultTypeFromErrCode`/`FaultLevelFromErrCode` 变为**转发热 delegate**——常量值逐一对齐，避免 recognition↔mqtt 循环依赖，设计干净（R10 语义一致，diff 只读走查通过）。

### 4.2 命名
- 包内命名整体清晰（`RuleEvidence`/`Evaluator`/`Validate`/`crossValidate`/`BuildSignature`/`SeedRecord`/`ScoreByRules`），职责单一。
- 建议：魔法数字统一命名（m3）；`Confirmed`/`RecognitionConfirmed` 易混，但已用语义化常量区分，可接受。

### 4.3 错误处理
- `processFault`/`persistEvidence` 对 DB 写失败均记录日志并继续/安全返回，未静默吞错（好）。
- **隐患**：MQTT `HandleMessage` 的 `recover()` 只是打日志，单条消息 `panic` 会被吞掉导致该条故障研判「静默丢弃」。power_loss panic 虽已修，但建议后续将 recover 与消息级「失败探针/重试」或至少结构化告警结合，避免安全关键消息静默丢失。
- `NextOrderNo` 为「count+1」非原子（无事务/锁），相邻并发工单理论上可能重号（R4 基线固有，非本次引入）；M1 修复时应一并考虑给 `order_no` unique 兜底已在基线有 `uniqueIndex`，冲突需重试。

### 4.4 并发 / DB 事务安全
- 主要并发风险集中在 M1（review 自动派单 TOCTOU）。其余写路径为同一消息顺序处理，风险可控。
- `AutoMigrate` 新增两表 + `fault_records` 加可空列，幂等；与既有 `device_media`/`feedbacks` 仅逻辑关联（`ref_media_id`/`ref_feedback_id` 未设实际 FK），符合「加表不加寄居」的宽松关联设计，可接受但建议说明外键由业务层保证。

### 4.5 安全红线复核结论（核心）
- **自动链路绝不产生 filtered**（M8/数学/测试三重证实）——安全红线**真实守住**。
- 自动链路对「已知码真实故障」最多降到 `pending_review`（保故障记录、不派单、可人工复核），**绝不丢弃**；`pending_review` 故障仍落 `fault_records` 且可通过 `recognition_status` 筛选，符合「宁等证据不漏真故障」。
- 多源证据（fault_evidence）与案例（fault_case）100% 可溯源（含被过滤批次，经 `evaluation_id` 关联），满足 pm-checklist「研判可追溯 100%」。

---

## 五、后续建议

1. **优先处理 M1**（review 自动派单并发去重）：加 unique 约束或条件更新事务 + 并发幂等测试。属本次范围最值得跟进的健壮性缺口。
2. **决定并落地 M2 语义**：「证据补充后升级确认」若要做自动升级，改 processFault 去重窗口分支；否则收敛文档/注释，避免语义误导。
3. **数据完整性**：给 `fault_case` 加 `(input_signature, device_hw_id)` 唯一约束（m4）；`NextOrderNo` 并发重号治理列入基线优化。
4. **交叉验证补强**（m9）：为 abnormal_on/timeout/dim 补充电流佐证/否证规则，使「多源交叉验证」在全部故障类型上落地；这是向 99.9999% 收敛的关键动作。
5. **契约卫生**：同步 `reviewed_at`（m5）、`FaultQuery.recognition_status`（m6）、RBAC Sort（m7）。
6. **死代码清理**：m1/m2 顺手清理；`filtered` 死分支保留并注释安全意图。
7. **覆盖率复核**：Windows 本机未聚合 `-coverprofile`，建议在干净 CI 跑 `make coverage-check`（80% 门槛）复核聚合覆盖率未回退。
8. **CesiumMap.vue(432) 类型告警**为既有历史遗留（非本次范围），建议单独排期修复并回归地图大屏。

---

*本报告由 reviewer-audit-tsloms（只读）产出，未修改任何源码/文件。仅供 leader 汇总与后续决策。*
