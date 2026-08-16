# TSLOMS 回归测试报告（qa-report）

> 回归测试专员：qa-regression-tsloms｜日期：2026-08-16
> 复核对象：dev-refactor-tsloms 基于 `pm-checklist.md` 的重构改动（见 `refactor-notes.md`）
> 基线：`origin/main` latest `a460365`｜目标：验证重构**未改变既有业务行为**，无功能退化。

---

## 一、测试环境

| 项 | 值 |
|---|---|
| 操作系统 | Windows 10.0.26200 (x64) |
| Go | go1.26.6 windows/amd64 |
| 数据库 | SQLite（`model.InitTestDB` 内存模式） |
| 被测代码 | `packages/server`（工作区未提交改动 vs 基线 a460365） |
| 方式 | 静态代码比对 + 单元/接口回归测试（`go test`） |

> 注：本文档仅针对后端（本次重构只动后端）。前端未改动，不在本次回归范围。

---

## 二、执行命令与结果

| 命令 | 结果 | 说明 |
|---|---|---|
| `go build ./...` | ✅ exit 0 | 编译通过 |
| `go vet ./internal/mqtt/ ./internal/handler/ ./internal/service/` | ✅ exit 0 | 静态检查通过 |
| `go test ./... -count=1` | ✅ 9 包全 `ok`，exit 0 | 全量回归（含本次新增用例） |
| 新增回归用例 `go test -run TestRegression` | ✅ 13/13 PASS | 见第四节 |

全量包清单（均 `ok`）：`cmd/server`、`internal/ai`、`internal/config`、`internal/handler`、`internal/logger`、`internal/middleware`、`internal/model`、`internal/mqtt`、`internal/service`。

覆盖快照（`-coverpkg=./...`，本次变化最相关）：handler **84.3%**、logger 92.3%、middleware 80.2%、mqtt 73.7%、service 72.1%、model 79.3%、ai 78.6%。handler 等核心包覆盖高于 80% 门禁；本次新增用例进一步拉高 handler/mqtt/service 覆盖。

> ⚠️ 环境备注：本机 `go test -coverprofile=` 无法落盘 profile 文件、`go tool cover -func` 报 "too many arguments"（pm-checklist 风险 6 已预告的 Windows 环境问题）。因此无法在本机复算 `make coverage-check` 的**聚合**总覆盖；但各包独立覆盖已测出，且新增用例只增不减，判定**覆盖未回退**。建议在干净 CI/预发布跑 `make coverage-check` 复核聚合值。

---

## 三、行为红线静态核对结论

对照 `pm-checklist.md` 第一节“业务核心逻辑”逐条比对重构前后代码（`git diff` + 现行代码通读）：

| 红线 | 处置 | 核对结论 |
|---|---|---|
| 工单状态机（pending→processing→completed/rejected，rejected→pending） | C1 仅把两遍相同分支合并为一遍 | ✅ 合并后仍为「closed_at=nil + 故障回 confirmed」唯一语义 |
| 30 分钟故障去重窗口 | B2 仅跳过“无变化”的历史写，`last_seen` 始终推进 | ✅ 窗口锚点不变，新增用例实测验证 |
| 严重故障自动建单（WO 序号 `NextOrderNo`） | 未触碰 `NextOrderNo`；B3 仅合并同帧 upsert | ✅ `NextOrderNo` 规则未动，`createWorkOrder` 调用不变 |
| SLA（pending 24h / processing 48h） | `workorder_escalate.go` 仅改 logger 构造 | ✅ 升级逻辑未动 |
| AI 降级兜底 | `internal/ai` 业务代码未改 | ✅ 未旁路 |
| RBAC 权限 | C6 用 `RoleAdmin`/`RoleOperator` 常量替换 `"admin"/"operator"` 字面量 | ✅ `RoleIsOperator` 与原子面量判定完全等价，viewer 只读等效力不变 |

---

## 四、功能点回归用例（新增 13 条，全部 PASS）

### 4.1 MQTT 热路径（`internal/mqtt/regression_test.go`，7 条）

| 用例 | 覆盖点 | 通过 |
|---|---|---|
| `TestRegression_B2_LastSeenAlwaysAdvances` | **B2** 去重窗口内电流/灯态无变化时，`last_seen` 仍必须推进（30min 窗口锚点红线）；且不新建故障记录 | ✅ |
| `TestRegression_B2_MaterialFieldsUpdatedOnChange` | **B2** 窗口内电流有差异 → 仍写入（与改动前一致），灯态不变则保留 | ✅ |
| `TestRegression_B2_LedStateUpdatedOnChange` | **B2** 窗口内灯态有差异 → 写入 | ✅ |
| `TestRegression_B3_SameFrameUpsertMerge` | **B3** 同帧内同一 hwID 多条记录只 upsert 一次设备 | ✅ |
| `TestRegression_B3_SameFrameMergePreservesLastRecord` | **B3** 同帧合并等价语义：设备 upsert 一次、故障研判逐条（不同 errCode 各建记录）、critical 建单 | ✅ |
| `TestRegression_C2_TopicHwID` | **C2** `topicHwID` 从 `{prefix}/{net}/{station}/{hwid}/U` 提取硬件 ID；非法/段数不足/超 uint32 回退 0 | ✅ |
| `TestRegression_B3_SameFrameLastRecordWinsVersion` | **B3（audit 问题 #1 修复后）** 同帧同 hwID 两条记录 swVer/confVer 不同 → 设备持久化版本取【末条】值（2/9），证明“末条生效 / last-write-wins”语义与基线一致 | ✅ |

### 4.2 Handler（`internal/handler/regression_test.go`，4 条）

| 用例 | 覆盖点 | 通过 |
|---|---|---|
| `TestRegression_C1_RejectToPendingMerge` | **C1 状态机红线** rejected→pending：工单回 pending、`closed_at` 清空、关联故障回 confirmed（比既有 `TestWorkOrder_RejectReprocess` 多了完整语义断言） | ✅ |
| `TestRegression_B1_ListWorkOrdersPreloadNames` | **B1** 批量预取后 `assignee_name` 与逐行查询一致；含处理人、无处理人、处理人不存在（空名） | ✅ |
| `TestRegression_B1_ListFaultsPreloadNames` | **B1** 批量预取后 `owner_name`/`repairer_name` 与逐行一致；含存在/不存在情况 | ✅ |
| `TestRegression_B1_ActiveStatusFilter` | **B5 兼容红线** `status=active` 仍=occurred/confirmed/dispatched（不含 resolved）；`status=resolved` 精确匹配 | ✅ |

### 4.3 Service（`internal/service/regression_test.go`，2 条）

| 用例 | 覆盖点 | 通过 |
|---|---|---|
| `TestRegression_B4_StockCountVsTopN` | **B4 巡检库存** count=全部匹配数（7），名单=按 stock 升序前 6（剔出最大 stock 项）；low 分支区隔低库存/缺货 | ✅ |
| `TestRegression_B4_LowStockNamesThinWrapper` | **B4** `lowStockNames` 薄封装与重构前一致（名单按 stock 升序） | ✅ |

---

## 五、发现的缺陷或风险

**功能缺陷：无。** 重构未引入任何业务行为回归：

1. 未发现状态机、去重窗口、WO 序号、SLA、AI 兜底、RBAC 的红线漂移。
2. N+1 消除后 `assignee_name`/`owner_name`/`repairer_name` 语义与重构前逐行查询完全一致（含“处理人/负责人不存在时返回空”的边界）。

**风险与注意事项（非阻塞）：**

1. **聚合覆盖率未能本机复算**：Windows 环境下 `go test -coverprofile` 不落盘、`go tool cover -func` 报错。已用各包独立覆盖佐证未回退（handler 84.3% > 80%），但**建议在干净 CI 跑 `make coverage-check` 复核聚合值**（对应 pm-checklist 风险 6）。
2. **B2 写入节流的顺带影响（设计内）**：去重窗口内电流/灯态无变化时只写 `last_seen`。数据一致性上无问题（差异字段仍更新），但若外部依赖“每次上报都落全字段的时间戳”作审计，需知悉该行为。属 B2 清单本意，非回归缺陷。
3. **`internal/ai`、前端未纳入本次回归**：refactor-notes 明确未改 ai 业务，本次全量 `internal/ai` 测试通过；建议若有前端改动另行回归地图大屏/ECharts 看板。
4. **未实施项（A1/A2/A3/A5）** 详见 refactor-notes，本次未触碰，无影响。

---

## 六、结论

**✅ 回归通过。**

`go build ./...` 与 `go test ./...`（9 包全绿）通过；针对本次重构重点改动点新增的 **13 条回归用例全部 PASS**（含 B3 语义对齐新增的版本差异用例）；逐条核对 pm-checklist 行为红线，重构后业务行为与基线一致，**未发现功能退化或回归缺陷**。

建议：在干净 CI 环境执行一次 `make coverage-check` 以确认聚合覆盖率门槛（记录为后续待办，非阻塞项）。

---

## 附：本次新增回归测试文件

- `packages/server/internal/mqtt/regression_test.go`
- `packages/server/internal/handler/regression_test.go`
- `packages/server/internal/service/regression_test.go`

> （以上文件为本次 QA 新增，尚未提交；改动仍为工作区未提交状态。）
