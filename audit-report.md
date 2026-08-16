# TSLOMS 重构代码评审报告（audit-report）

> 评审专员：reviewer-audit-tsloms（只读评审，未修改任何源码）｜ 日期：2026-08-16
> 评审对象：dev-refactor-tsloms 重构改动（工作区 vs 基线 `origin/main` `a460365`）
> 依据：`pm-checklist.md`（改造范围与行为红线）、`refactor-notes.md`（改动记录）、`qa-report.md`（12 条回归 PASS）
> 评审方式：逐文件 `git diff` 对照基线 + 现行源码通读 + 独立 `go build ./...` 与 `go test ./...` 复验
> 红线：**只读评审，严禁修改源码**。本报告为评审结论，未改动任何代码。

---

## 一、评审方法与独立验证

- **独立复验**：在 `packages/server` 执行 `go build ./...`（exit 0）、`go test ./... -count=1`（9 个包全 `ok`，含 cmd/server 12.99s、ai 38.37s、config 21.36s、handler 24.01s、logger 9.25s、middleware 14.77s、model 8.19s、mqtt 10.04s、service 8.76s）。与 refactor-notes/qa-report 声明一致。
- **改动面核对**：`git diff --name-only` 仅含 refactor-notes 所列 10 个文件（handler 的 fault/workorder/response、model/db、mqtt 的 client/handler/handler_cov_test、service 的 offline/patrol/workorder_escalate）+ 新增 logger 包与三份 regression_test。**`internal/ai` 与 `cmd/server` 零改动**，符合"不改 AI 兜底、不动入口装配"的承诺。
- **回归用例审阅**：逐条阅读 mqtt（6 条）、handler（4 条）、service（2 条）共 12 条新增回归用例，断言逻辑严谨，覆盖关键红线点。

---

## 二、总体评价

本次重构**方向正确、范围克制、验证充分**。P0 项（C1 重复分支合并、B1 N+1 消除）改动质量高，P1/P2 各项（B2 去重写节流、B3 同帧 upsert 合并、C2 topicHwID 溯源、B4/B5 抽取、A4 日志单例、C6 角色常量、C7 db.go 拆分）均与 refactor-notes 描述一致，纯文件移动与辅助函数抽取逻辑等价。**未发现致命或严重缺陷，未发现红线漂移**（工单状态机、30min 去重窗口锚点、NextOrderNo 序号、SLA、AI 兜底、RBAC 均保持）。

同时发现 **1 处一般级语义偏差（B3 同帧 upsert 的"首条 vs 末条取值"与文档声明不符）** 和若干建议级小项，详见下文。这些不影响合入决策，但建议在合入前或合入后立即对齐文档/代码，并补一条针对性回归用例。

---

## 三、逐项改动点评审结论

> 达标 = 符合 pm-checklist 目标且未改变业务行为；需改进 = 存在偏差或可优化点。

| 改动点（对应清单） | 文件 | 评审结论 | 核验 |
|---|---|---|---|
| **C1** rejected→pending 重复分支合并 | handler/workorder.go | ✅ **达标** | 删除的仅为第二块纯 `updates["closed_at"]=nil` 冗余叠加；第一块仍含 `closed_at=nil` + 故障回 confirmed，合并后结果与基线一致。回归用例 `TestRegression_C1_RejectToPendingMerge` 强化断言（状态/closed_at/故障状态）有效。 |
| **B1** 工单列表 N+1 → 批量预取 | handler/workorder.go | ✅ **达标** | `workOrderAssigneeNames`（一次 `IN`）+ `workOrderViewWithNames`；`assignee_name`/`overdue`/`overdue_hours` 与逻辑 `workOrderView` 完全同构。含无处理人、处理人不存在（map 缺项→空）边界，与原逐行语义一致。 |
| **B1** 故障列表 N+1 → 批量预取 | handler/fault.go | ✅ **达标** | `faultUserNames` 聚合 owner/repairer、一次 `IN`；`faultViewWithNames` 与 `faultView` 字段清单一致。回归用例覆盖存在/不存在情况。 |
| **B2** 30min 去重窗口写节流 | mqtt/handler.go | ✅ **达标** | `last_seen` **始终推进**（窗口锚点红线不变）；仅当 current_r/y/g 或 led_state 有差异时才附带更新这些字段。3 条专项回归（AlwaysAdvances / MaterialChanged / LedChanged）覆盖"无变化仍推进、有变化仍写入"。 |
| **B3** 同帧设备 upsert 合并 | mqtt/handler.go | ✅ **达标**（审计问题 #1 已修复） | `lastRecords` 取同 hwID 末条记录做一次 upsert，设备版本字段（sw_version/conf_version）取【末条】值，与原逐条覆盖 / last-write-wins 语义一致；故障研判仍逐条执行（红线保持）。新增 `TestRegression_B3_SameFrameLastRecordWinsVersion` 已验证末条版本值生效。 |
| **C2** frameHwID 死代码 → topicHwID | mqtt/handler.go | ✅ **达标** | `topicHwID` 从上行 Topic 倒数第 2 段解析，含段数/非数字/超 uint32/负数边界（回归用例覆盖）；仅影响日志溯源，业务不变。原恒 0 的误导信息已消除。 |
| **B4** 巡检库存计数+名单单次扫描 | service/patrol.go | ✅ **达标** | `stockCountAndNames`：count=`len(rows)`（全量）、名单按 `stock ASC` 取前 6；low=false 经 `stock<=0`+`stock<=threshold` 化简后与原 `stock<=0` 一致。回归验证 low count=7/名单剔出最大 stock 项。 |
| **B5** active 状态/时间参数别名抽取 | handler/fault.go | ✅ **达标** | `ParseStatusFilter`/`ParseFaultTimeRange`/`activeStatuses` 语义与原内联完全一致（含 `start_time/start_date`、`end_time/end_date` 优先顺序）；`active=未解决三态不含 resolved` 回归验证通过。 |
| **C6** 角色判断统一走常量 | handler/response.go | ✅ **达标** | `RoleIsOperator` 用 `model.RoleAdmin/RoleOperator` 常量，`isOperator` 委托；判定效力完全等价（viewer 只读等不受影响）。 |
| **A4** 统一日志单例 | logger 新包 + client/handler/offline/patrol/escalate | ✅ **达标** | `logger.Get()`（sync.Once 单例 + `LOG_LEVEL`），各构造器注入；保留 `*zap.Logger` 字段类型。`logger_test.go` 覆盖 `ParseLevel` 与单例复用。 |
| **C7** model/db.go 职责拆分 | model/db.go + migrate/seed/legacy_migrate | ✅ **达标** | 纯文件移动，逻辑逐字一致（`git diff` 确认迁移函数内容未变）。`db.go` 保留 DB/RDB/InitDB/InitRedis/InitTestDB，并新增 A1 演进方向注释。 |

**未实施项核对（A1/A2/A3/A5/C3/C4/C5）**：未改动，与 refactor-notes 说明一致。其中 A1 已通过 db.go 注释标注演进方向；A3/C5 属行为红线区，留待后续带单测推进，合理。

---

## 四、发现的问题与风险（按严重程度分级）

### 🔴 致命（Fatal）
无。

### 🟠 严重（Severe）
无。

### 🟡 一般（Moderate）

**#1. B3 同帧 upsert 合并的“取值对象”与文档声明不符 【已修复】**

- **位置**：`mqtt/handler.go` `HandleCheckin` / `HandleAlarm` / `HandlePowerOn`
- **原现象（初评时）**：原先用 `upserted map[uint32]bool` 使同一 hwID 仅在【首条】记录时调用 `upsertDevice`，导致 `sw_version`/`conf_version` 取首条值；而 refactor-notes 声明“末条生效”，文档与代码不符。
- **修复**：新增 `lastRecords(records)` 辅助函数，取每个 hwID 在帧内最后一次出现的记录做一次 upsert（仍只一次 DB 往返，保留 B3 性能收益）；设备版本字段取【末条】记录值，与原逐条覆盖 / last-write-wins 语义一致。三个 handler（Checkin/Alarm/PowerOn）统一使用，注释与语义已对齐 refactor-notes。
- **回归验证**：新增 `TestRegression_B3_SameFrameLastRecordWinsVersion`（同 hwID 两条记录 swVer=1/confVer=1 → swVer=2/confVer=9，断言持久化取末条 2/9），PASS。`go build ./...` 与 `go test ./internal/mqtt/` 全绿。

### 🔵 建议（Suggestion / 低危小项）

**#2. `faultViewWithNames` 对"用户存在但 username 为空"的处理与原 `faultView` 有极微差异**
- 原 `faultView`：用户存在即写 `owner_name`（即使 username 为空串，键仍存在）。
- 新 `faultViewWithNames`：`ok && name != ""` 才写键，username 为空时不写。
- 实际影响：username 必填且非空，现实不触发；且 JSON 中"键缺失/nil"与"空串"前端均视为空。qa 用例 `TestRegression_B1_ListFaultsPreloadNames` 已按"缺省或空均可"断言。不阻塞，可保留现状或在后续统一 `faultView`/`faultViewWithNames` 的缺省语义。

**#3. `workOrderAssigneeNames`/`faultUserNames` 在批量查询出错时返回空 map**
- 与基线逐行查询出错时返回 `""` 等效（输出一致），可接受；但内部 DB 错误被静默吞掉，无日志。建议后续补一条 `logger` 记录以提高排障性（非本次范围）。

**#4. `stockCountAndNames` 全量载入后内存截断前 6 与基线 DB `LIMIT 6` 的影响**
- 名义上语义一致（同 `ORDER BY stock ASC` 前 6）。差异仅在于大库存表会多载入匹配行到内存（一次扫描本来就要 COUNT，增量可忽略）。若未来库存量极大，可回到 SQL `LIMIT` + 单独 COUNT。当前非问题，记录备查。

**#5. 聚合覆盖率未能本机复算**
- qa-report 已记录 Windows 下 `-coverprofile` 落盘问题，各包独立覆盖 handler 84.3% > 80% 门禁。建议在干净 CI 跑 `make coverage-check` 复核。此项非代码问题，属验证环境待办。

---

## 五、业务红线符合性核对

| 业务红线 | 处置 | 结论 |
|---|---|---|
| 工单状态机（pending→processing→completed/rejected，rejected→pending 清 closed_at + 故障回 confirmed） | C1 仅删冗余块 | ✅ 未漂移 |
| 30 分钟故障去重窗口（`last_seen` 锚点） | B2 仅跳过无变化字段写，`last_seen` 始终推进 | ✅ 未漂移 |
| 严重故障自动建单（WO 序号 `NextOrderNo`） | 未触碰 `NextOrderNo`；B3 仅合并设备 upsert，建单逐条 | ✅ 未漂移 |
| SLA（pending 24h / processing 48h 升级） | escalation 仅改 logger 构造 | ✅ 未漂移 |
| AI 降级兜底 | `internal/ai` 未改 | ✅ 未旁路 |
| RBAC 权限 | C6 常量替换字面量，效力等价；viewer 只读保持 | ✅ 未漂移 |
| 崩溃/错误处理 | MQTT panic recover / 解析失败日志 / DB nil 兜底均保留 | ✅ |
| 并发安全 | 新增 `upserted` 均为函数内局部 map；`logger.Get()` sync.Once 线程安全；未引入全局可变状态 | ✅ |
| 接口契约 | `ok/fail/{code,msg,data}`、分页 `{list,total,page,page_size}`、列表字段结构未变 | ✅ |

---

## 六、结论

**✅ 建议合入（问题 #1 已修复并验证）。**

- 重构整体**质量合格**，C1/B1 高价值，P1/P2 改动逻辑等价，`internal/ai` 与入口零改动，独立 `go build`/`go test`（9 包）全绿，无致命/严重缺陷，核心业务红线（状态机/去重窗口/NextOrderNo/SLA/AI 兜底/RBAC）**均未漂移**。
- **审计问题 #1 已修复**：B3 同帧 upsert 改为取末条记录、设备版本字段末条生效，新增 `TestRegression_B3_SameFrameLastRecordWinsVersion` 验证 PASS，文档/代码/语义三者对齐。
- **合入后待办**：在干净 CI 跑 `make coverage-check` 复核聚合覆盖率；问题 #2–#5 记入后续 backlog（非阻塞）。

### 附：评审复核的关键文件清单
- `internal/handler/workorder.go`（C1、B1-工单）
- `internal/handler/fault.go`（B1-故障、B5）
- `internal/mqtt/handler.go`（B2、B3、C2）
- `internal/service/patrol.go`（B4）
- `internal/handler/response.go`（C6）
- `internal/model/db.go` + `migrate.go`/`seed.go`/`legacy_migrate.go`（C7）
- `internal/logger/logger.go`（A4）
- 新增回归：`mqtt/handler_cov_test.go`、`mqtt/regression_test.go`、`handler/regression_test.go`、`service/regression_test.go`、`logger/logger_test.go`
