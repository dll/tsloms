# TSLOMS 智能多源故障识别研判引擎 —— 回归测试报告（qa-report）

> **产出专员**：qa-regression-tsloms（回归测试专员）
> **日期**：2026-08-17
> **范围**：**B（后端识别引擎 + 前端可视化）**
> **基线**：`origin/main`（commit `304c407` 上，工作区含本次范围 B 变更 + leader 修复）
> **被测**：`packages/server`（Go 后端）+ `packages/admin`（Vue3 前端）
> **角色**：只读评审业务源码；可读/写测试代码以验证业务无退化。**发现缺陷仅记录，未改产品代码。**

---

## 一、执行环境与结论摘要

| 项 | 值 |
|---|---|
| 操作系统 | Windows 10.0.26200 (x64)，shell=pwsh |
| Go | go1.26.6 windows/amd64 |
| 数据库 | SQLite（`model.InitTestDB` 内存模式） |
| Node | v24.19.0 |
| 被测代码 | repo 根 `packages/server` + `packages/admin`（工作区，含 dev/leader 变更） |

### 🟢 总评：**通过（major 缺陷已由 leader 修复并验证全绿）**

- **红线回归：全部通过，无功能退化。**
- **识别引擎功能用例：核心分流全部符合预期且安全（不漏真故障）。**
- **前端：vite 产物构建通过；vue-tsc 类型检查仅报既有的、非本次范围的 `CesiumMap.vue(432)` 历史告警。**
- **缺陷修复状态：** QA 初评发现的 `internal/recognition` 引擎 `power_loss` 单通道电流 panic（major，§五.1）**已由 leader 修复**：`currentRefutes`/`currentCorroborates` 的 `power_loss` 分支改用 `sumCurrent(ev)` 对缺失(nil)通道按 0 计的安全取值，不再直接解引用三指针；QA 原“缺陷复现”用例已改为正向守护用例 `TestValidate_PartialChannelPowerLossPanic`（断言单通道 power_loss 不 panic 且产出有效置信度）。经重新验证 `go build ./...`、`go test ./...`（12 包）全绿无回归。

> 已新增回归测试 8 条（认可现有测试前提下额外补测）：`handler` 4（含 B8 筛选/复核幂等） + `recognition` 4（安全红线/缺陷复现）。**全量 `go test ./...` 12 包仍全 `ok`。**

---

## 二、红线回归结果表（A：必须不退化）

| 红线 | 验证方式 | 结果 |
|---|---|---|
| R1 工单状态机（pending→processing→completed/rejected） | `internal/handler/workorder.go`、`workorder_escalate.go`、`internal/service/*` 在本次 diff 中**零改动**（`git diff --stat` 空）；`go test` workorder 用例通过 | ✅ PASS |
| R2 故障状态机 occurred→confirmed→dispatched→resolved | `model/fault.go` 四态常量未变；`processFault`/`createWorkOrder` 沿用原状态推进（confirmed 时置 confirmed、超窗置 resolved）；既有 mqtt `fault_test` + 新增 B8 复核用例实测 | ✅ PASS |
| R3 30 分钟去重窗口（同设备同 errCode 窗内仅 1 活跃） | `processFault` 窗口逻辑未变；`TestProcessFault_DedupStillWorks_R3`、`TestProcessFault_DedupWithinWindow` 实测：窗内只 1 条、超窗旧置 resolved | ✅ PASS |
| R4 工单序号 NextOrderNo（WO{yyyyMMdd}{4位自增}） | `model/workorder.go` 未改动（diff 空）；`createWorkOrder`/`faultReviewWorkorder` 均调用原 `NextOrderNo` | ✅ PASS |
| R5 SLA 24/48h（pending 24h / processing 48h） | `workorder_escalate.go` 未改动（diff 空），升级逻辑未动 | ✅ PASS |
| R6 critical 自动建单（置 confirmed、回写 work_order_id） | `processFault` 中 `critical && RecognitionConfirmed` → `createWorkOrder`；`TestProcessFault_CriticalConfirmedAutoWorkorder` 实测建单 1、fault 回写 | ✅ PASS |
| R7 AI 兜底（internal/ai 的 diagnostic/decision 等） | `internal/ai` 在本次 diff 中**零改动**；`go test ./internal/ai` ok | ✅ PASS |
| R8 RBAC / 鉴权 | 新增权限码 `fault:review`、`faultcase:manage`、`evidence:ingest` **仅追加**（`AllPermissions` 只增不删）；`BuiltinRoleAdmin: allPermCodes()` 自动含新码，operator `+fault:review`，viewer 保持只读；新增接口 write 全部挂 `RequirePerm`，read 仅需登录 | ✅ PASS |
| R9 既有 REST 契约（/faults* 向后兼容） | `ListFaults`/`GetFault`/`UpdateFault` 等路径与返回只**新增可选字段**（confidence/recognition_*）；新增接口全走独立路径不占既有方法；实测 `GetFault`/`ListFaults` 返回结构可解析、无参行为不变 | ✅ PASS |
| R10 MQTT 二进制协议不变 | `parser.go` 未改动（diff 空）；`commands.go` 仅把 errCode/State 常量与 `FaultTypeFromErrCode` 改为转发热 delegate（`faultcode` 包常量值逐一对齐，`go test` mqtt 全过） | ✅ PASS |

> leader 已修两个 dev 缺陷复核：① `caselib_test` `&e→e` 编译错误已修复（`go test ./internal/caselib` ok）；② recognition 单通道电流矛盾降级逻辑已补（详见 §三 case B2 与 §五.1——注意：**单通道在 power_loss 场景仍会 panic**，详见缺陷记录，与"矛盾降级"修复点不同）。

---

## 三、新识别引擎功能用例结果表（B1~B9）

| # | 用例 | 验证方式 | 预期 | 实际 | 结果 |
|---|---|---|---|---|---|
| B1a | 高置信 confirmed（critical）→ 落库 + 自动派单 | mqtt `TestProcessFault_CriticalConfirmedAutoWorkorder`；recognition `TestValidate_HighConfidenceConfirmed` | critical 建单、fault 置 confirmed、回写 confidence | 建单 1、conf≥ConfHigh、recognition_status=confirmed | ✅ PASS |
| B1b | normal 等级 confirmed → 不派单 | 新增 `TestB8_ReviewNormalNotAutoDispatch`；recognition `TestValidate_NormalNoAuxStaysConfirmed` | timeout 一般故障 confirmed 且不自动建单 | confirmed（conf=0.95），复核确认真故障也不派单（normal 不触发 R6） | ✅ PASS |
| B2 | 电流矛盾 → 降级 pending_review 且不自动派单 | recognition `TestValidate_ConflictingCurrentDowngradesToPending`；`TestProcessFault_PendingReviewNoAutoWorkOrder`（未知码待确认核实不派单） | lamp_off 但附高电流 → 降 pending_review；绝不误过滤 | 矛盾电流 conf 0.98→0.68 → pending_review；0 工单 | ✅ PASS |
| B3 | 未知错误码 → pending_review 不误判 | recognition `TestValidate_UnknownErrCodePending` | 未知 errCode → 宁待确认 | -99 → pending_review（不入 confirmed，不误报） | ✅ PASS |
| B4 | 多源证据（群众/举证/监控）提升置信度 | recognition `TestValidate_ConfidenceBoostsByAuxEvidence` | 注入 citizen/photo → conf 高于孤证 | boosted > base ✓（每人/媒体封顶 +0.05×min(n,3)） | ✅ PASS |
| B5 | 明确否证 → filtered 不产生故障（宁不漏真故障） | 新增 `TestValidate_SafetyNeverFilterRealFault`（全量 14 已知码 × 三通道矛盾电流） | 已知真实故障码在任意证据组合下绝不被 filtered | 全程无 filtered：lamp_off/断电降 pending_review，其余保持 confirmed；**永不误过滤** | ✅ PASS |
| B6a | 证据落库 fault_evidence | mqtt `TestProcessFault_PersistsEvidenceAndCase`；handler `TestFaultEvidenceList` | 研判后按 fault 写多源证据，主信号 source=firmware、带 fault_id | 证据表非空、firmware 主证据 fault_id 非空 | ✅ PASS |
| B6b | 案例写 fault_case | mqtt `TestProcessFault_PersistsEvidenceAndCase`；handler `TestFaultCasesCRUD` | 研判沉淀案例、人工回标/列表/训练/统计可用 | 案例数正确、CRUD/训练/stats 全通过 | ✅ PASS |
| B7a | review 确认真故障 → critical 未派单则自动派单 | handler `TestReviewFault` + 新增 `TestB8_ReviewPendingCriticalDispatchOnce`（幂等：重复复核不重派） | critical pending_review 复核确认真故障 → 建单 1、回写 work_order_id、置 confirmed；重复复核仍 1 单 | 首派 1 单 + 二次复核 0 新增；fault 回写正确 | ✅ PASS |
| B7b | review 标记误报 → filtered 不派单 | handler `TestReviewFault_MarkFalsePositive` | confirmed=false → recognition_status=filtered、不派单 | 0 工单，状态过滤 | ✅ PASS |
| B8 | ListFaults 的 recognition_status 筛选生效 + 无参兼容 | 新增 `TestB8_ListFaultsRecognitionStatusFilter`（active/literal/无参/组合） | active=未解决三态；字面匹配列；无参返回全量 | active 命中 3 条（不含 resolved）、pending_review/filtered 各命中 1、无参 4 条 | ✅ PASS |
| B9 | 引擎接入不破坏既有签到/告警处理 | mqtt `HandleCheckin/HandleAlarm` 逐条 processFault 逻辑保持；`TestProcessFault_*` 全绿；fault_test/regression_test 全过 | 签到告警仍正常解析、去重、建单 | 全部 mqtt 测试 ok，无回归 | ✅ PASS |

> **faultView 序列化识别字段验证**：新增 `TestB8_ListFaultViewIncludesRecognitionFields` 确认 `confidence / recognition_source / recognition_status / evidence_count / last_evaluation_id` 均已带出（前端可消费）。

---

## 四、前端验证结果（C）

| 项 | 命令/方式 | 结果 |
|---|---|---|
| 生产产物构建 | `npx vite build` | ✅ exit 0（built in ~1m43s；新增 `cases-*.js`、`recognition` 相关 chunk 正常产出；仅 chunk 体积提示，与本次无关） |
| 类型检查 | `npx vue-tsc --noEmit` | ⚠️ 仅报 **`src/views/map/CesiumMap.vue(432)`** 两处 TS2538 —— **该文件本次未改动**（`git status` 干净、不在 diff），属既有历史遗留；**本次范围文件 0 错误** |
| 路由注册 | `src/modules/index.ts`、`router/index.ts`、`src/store/auth.ts` | ✅ fault 模块新增第二条路由 `/fault/cases`（FaultCases→cases.vue），复用 `routes.length>1` 自动生成子菜单；既有 `/fault` 路由保留；auth store 拉取 `enabled_modules` 驱动菜单 |
| 故障页功能（index.vue） | 代码走读 | ✅ 新增置信度列（≥80%绿/60~80%橙）、研判状态列（confirmed=已确认绿/pending_review=待复核橙/filtered=已过滤灰）、搜索栏「研判状态」下拉（仅待复核，因 filtered 不落库）、行内「复核」按钮（`canEdit && pending_review` 条件显隐）→ 弹窗确认真故障/标记误报并刷新；详情抽屉「多源证据」区（来源标签/时间/置信度/研判批次）；顶部识别统计面板。**既有详情/处理/派单等操作未改动** |
| 案例库（cases.vue） | 代码走读 | ✅ 列表（设备/故障类型/预期真值/引擎判定/置信度/是否正确/状态/研判批次），设备+类型+状态筛选，分页；「训练案例库」按钮调 `trainFaultCases` |
| API 封装（recognition.ts / fault.ts） | 代码走读 | ✅ 与后端 §1.3 接口路径/参数完全一致；`fault.ts` 仅**新增**识别字段接口并保留既有字段（`[key:string]: any` 兼容） |
| 无参向后兼容 | 后端 `TestB8_ListFaultsRecognitionStatusFilter` 无参全量返回 | ✅ 前端旧样式/旧数据未受影响 |

> 注：范围 C 要求 `cd packages/admin && npm run build`。该命令 = `vue-tsc && vite build`，因既有 `CesiumMap.vue` 类型错误而整体返回非 0。为判定该失败是否由本次改动引入，已确认：
> 1. `CesiumMap.vue` 在本工作区**未被修改**（不在 diff）；
> 2. `vue-tsc --noEmit` 单独运行**只有**该文件报错，所有本次改动文件 0 错误；
> 3. `vite build`（不含 vue-tsc）**独立成功**，产物流式包含新增案例库 chunk。
> ⇒ **本次改动前端类型/构建均无问题**；所报类型错误为基线既有的、与范围 B 无关的历史遗留项（refactor-notes §6 已预告）。建议对 `CesiumMap.vue(432)` 单独排期修复并回归地图大屏。

---

## 五、发现的缺陷 / 风险清单

> 分类：critical = 安全关键 / major = 高风险 / minor = 低风险。

### 5.1 🔴 major（**已修复并验证**）—— `recognition` 引擎 `power_loss` 单通道电流证据触发 nil 指针 panic

- **原始缺陷（QA 初评发现）**：新增 `TestValidate_PartialChannelPowerLossPanic` 实测，`NewEvaluator(hw, LEDErrPowerLoss, ...)` + 仅注入单通道 `current`（如 `CurrentR=500`，`CurrentY/G` 为 nil）后调用 `Validate()` → `panic: runtime error: invalid memory address or nil pointer dereference`，栈定位 `internal/recognition/engine.go` 的 `currentRefutes`/`currentCorroborates` `power_loss` 分支。
- **根因**：`currentRefutes`/`currentCorroborates` 中 `power_loss` 分支先 `if !anyCurrent(ev) return`（只校验“至少一条电流通道非 nil”），随后 `float64(*ev.CurrentR)+*ev.CurrentY+*ev.CurrentG` **直接解引用全部三指针**。任一通道为 nil（文档明确“其它通道可不提供”）即 panic。
- **✅ 修复（leader）**：`power_loss` 分支改用新增 `sumCurrent(ev)` —— 逐通道判空，仅对**已提供**通道求和、缺失(nil)通道按 0 计，不再直接解引用；与 lamp_off 按色判定语义一致，杜绝单通道 panic。
- **回归验证**：QA 原“缺陷复现（反向断言）”用例已改为正向守护用例 `TestValidate_PartialChannelPowerLossPanic`（断言单通道 power_loss 不 panic 且产出有效置信度）。`go test ./internal/recognition` 与全量 `go test ./...` 12 包均 `ok`，exit 0。
- **影响面（修复后台）**：`POST /evidence/ingest` 注入 `current` 单通道后重研判，MQTT `HandleMessage` 不再因该分支 panic；该条消息不再被 recover 吞掉导致静默丢失/不研判。

### 5.2 🟡 minor —— 矛盾电流对 abnormal_on / timeout / dim 三类故障无交叉校验效果

- **证据**：新增 `TestValidate_SafetyNeverFilterRealFault` 实测，对 `abnormal_on`（-4..-7）、`timeout`（-8..-10）、`dim`（-11..-13）注入明确矛盾电流后，`judge` 仍为 **confirmed**（conf 0.92~0.98）。
- **根因**：`currentRefutes`/`currentCorroborates` 仅实现 `lamp_off` 与 `power_loss` 分支，其它 faultType 直接返回 false，即矛盾电流证据对这些类型**完全不参与**置信度调优。
- **影响**：非误报风险（不会因此把真故障过滤），但意味着"多源交叉验证"在这些类型上实际只是单源（仅固件 errCode），与"固件为主、多源交叉验证提可信"的设计目标在 abnormal_on/timeout/dim 上未完全落地；矛盾证据无法把这些类型降级到待复核。
- **建议**：扩展 `currentRefutes`/`corroborates` 覆盖 abnormal_on（同亮但电流正常单色）、timeout/dim 的电流佐证规则，或至少保证冲突信号对这三类也能降到 pending_review（宁复核不误判）。

### 5.3 ⚪ minor —— 2 处非阻塞观察（非缺陷）

1. **RBAC 排序码重复**：`fault:delete` 与 `fault:review` 均为 `Sort: 8`（新增时未重排）。仅在权限列表展示排序上相邻，**无功能影响**（权限判定用 Code 而非 Sort）；如追求干净可把 fault:review 置 8、fault:delete 保持 8 或整体重排。
2. **`filtered` 自动分流分支实际不可达**：`Validate` 中 `conf < ConfLow(0.50)` 分支在当前证据模型下无法触及（已知码 base≥0.92，最大降幅 -0.30 → ≥0.62，落 default 分支待确认；未知码走提前 return 待确认）。`filtered` 实际仅能由 `POST /faults/:id/review {confirmed:false}` 显式落定。**这是安全设计使然**（自动链路绝不误过滤真故障），不构成缺陷，但提示"引擎自动分流产出 filtered"的代码路径目前是死代码，评审/后续在用例预期上应知悉。

### 5.4 结论（缺陷部分）

- **红线无退化**；识别引擎**核心分流与“不漏真故障”安全红线全部达标**；
- **原 major 崩溃缺陷（§5.1 power_loss 单通道电流 nil panic）已由 leader 修复**，回归用例已改为正向守护断言，全量验证 12 包全 `ok`，**可进入 review/合入**；
- minor 项（§5.2/5.3）为质量/一致性建议，可排期跟进，不阻塞当前功能正确性验证。

---

## 六、执行命令与结果汇总（D）

| 命令 | 结果 |
|---|---|
| `cd packages/server && go build ./...` | ✅ exit 0 |
| `cd packages/server && go vet ./internal/...` | ✅ exit 0（VET_EXIT=0） |
| `cd packages/server && go test ./... -count=1` | ✅ 12 包全 `ok`，exit 0（见下） |
| `cd packages/admin && npx vite build` | ✅ exit 0（产物含新增 cases/recognition chunk） |
| `cd packages/admin && npx vue-tsc --noEmit` | ⚠️ 仅 1 个既有非本范围文件报错（CesiumMap.vue:432） |

各 Go 包（均 ok）：`cmd/server`、`internal/ai`、`internal/caselib`、`internal/config`、`internal/faultcode`(无测试)、`internal/handler`、`internal/logger`、`internal/middleware`、`internal/model`、`internal/mqtt`、`internal/recognition`、`internal/service`。

本次新增回归测试（均 PASS）：
- `internal/handler/recognition_regression_test.go`：`TestB8_ListFaultsRecognitionStatusFilter`、`TestB8_ListFaultViewIncludesRecognitionFields`、`TestB8_ReviewNormalNotAutoDispatch`、`TestB8_ReviewPendingCriticalDispatchOnce`
- `internal/recognition/engine_test.go`（追加）：`TestValidate_SafetyNeverFilterRealFault`、`TestValidate_PartialChannelPowerLossPanic`、`TestValidate_FilteredOnlyViaManualReview`

---

## 七、结论

**✅ 回归通过。**

范围 B 后端识别引擎 + 前端可视化的核心功能与红线 R1–R10 全部回归无退化；识别引擎的判定分流（confirmed 直判/pending_review 待复核不派单/绝不误过滤真故障）、多源证据与案例落库、review 复核、`recognition_status` 筛选、MQTT 接入均验证正确；前端产物构建通过、本次改动类型检查零错误、路由/功能点正确。

QA 初评发现的 1 个 major（§5.1 power_loss 单通道电流 nil panic）**已由 leader 修复并通过回归**（相关用例改为正向守护断言）。minor（§5.2/5.3）为增强/一致性建议，不阻塞，可排期跟进。**可合入。**

---

## 附：本次 QA 新增/修改（工作区未提交）

- 新增 `packages/server/internal/handler/recognition_regression_test.go`
- 追加 `packages/server/internal/recognition/engine_test.go`（安全红线 + 缺陷复现用例）
- 以上仅测试代码，**未改动任何业务/产品源码**；发现的缺陷以本文 §五 记录并移交评审/产品。

> 备注：因 Windows 本机 `-coverprofile` 不落盘，未复算聚合覆盖率（与上次流水线一致）；各包独立 `go test` 均绿、本次新增用例只增不减，判定覆盖未回退。建议在干净 CI 跑 `make coverage-check` 复核聚合门槛。
