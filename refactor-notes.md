# TSLOMS 重构改动记录（refactor-notes）

> 重构专员：dev-refactor-tsloms｜日期：2026-08-16
> 依据：项目根目录 `pm-checklist.md`（pm-tsloms 需求核对结论）
> 原则：**严格不改变业务行为**。涉及行为红线（工单状态机 / 30 分钟故障去重 / 严重故障自动建单 WO 序号 / SLA 24/48h / AI 降级兜底 / RBAC 权限）的改动均保持返回结构与流转语义不变。
>
> 验证：`go build ./...` 通过；`go test ./...` 全绿（9 个包全部 ok）。

---

## 已完成改动（P0 优先）

### P0-1（C1）：`UpdateWorkOrderStatus` 重复分支合并

- **文件**：`packages/server/internal/handler/workorder.go`
- **改动前**：`rejected → pending` 的同一 `if` 条件连续写了两遍（第一块做 `closed_at=nil` + 故障回 `confirmed`；第二块又无条件 `closed_at=nil`）。
- **改动后**：合并为单个分支，只保留「`closed_at=nil` + 故障回 `FaultStatusConfirmed`」的唯一语义。
- **理由**：消除冗余/潜在维护误改点（清单 C1）。
- **是否改动行为**：否（原第一块已含 `closed_at=nil`，第二块为冗余叠加；合并后结果一致）。

### P0-2（B1）：工单列表 N+1 查询消除

- **文件**：`packages/server/internal/handler/workorder.go`
- **改动前**：`ListWorkOrders` 对每行订单调用 `workOrderView`，内部逐行 `First` 查一次 user（page_size 最大 100 → 最多 100 次额外查询）。
- **改动后**：新增 `workOrderAssigneeNames(orders)` 用一次 `WHERE id IN (...) Select("id, username")` 批量预取处理人姓名；新增 `workOrderViewWithNames(o, names)` 用预取结果构建视图。原 `workOrderView`（单对象场景）保持不变。
- **理由**：消除 N+1，返回字段结构不变。
- **是否改动行为**：否（assignee_name 取值逻辑与缺省值一致）。

### P0-3（B1）：故障列表 N+1 查询消除

- **文件**：`packages/server/internal/handler/fault.go`
- **改动前**：`ListFaults` → `faultView` 每行查负责人 + 维修人（每行最多 2 次额外查询）。
- **改动后**：新增 `faultUserNames(faults)` 批量预取（一次 `IN` 查询聚合 owner/repairer 的 username）；新增 `faultViewWithNames` 用预取 map 构建。原 `faultView`（详情/其他单对象场景）保持不变。
- **理由**：消除 N+1。
- **是否改动行为**：否。

### P0-4（B2）：MQTT 故障去重命中的无意义写优化

- **文件**：`packages/server/internal/mqtt/handler.go`（`processFault`）
- **改动前**：去重窗口内每次均无条件 `Updates`（last_seen + current 三色 + led_state），即便电流/灯态无变化也写整行。
- **改动后**：仍始终更新 `last_seen`（保持 30 分钟窗口锚点不变）；仅当 current_r/y/g 或 led_state 相较既有记录发生变化时才附带更新这些字段，否则只写 `last_seen`。
- **理由**：减少高频上报的无意义 DB 写（清单 B2）。
- **是否改动行为**：否（`last_seen` 推进保持一致；电流/灯态有差异时仍更新——与 `fault_test.go` 的「窗口内更新电流值」断言一致）。

### P1（B3）：同帧内设备 upsert 合并

- **文件**：`packages/server/internal/mqtt/handler.go`（`HandleCheckin` / `HandleAlarm` / `HandlePowerOn`）
- **改动前**：同一帧内每条 EventRecord 都对同一 `ledHwID` 重复 `upsertDevice`（各自 `WHERE ... First`）。
- **改动后**：三个 handler 均增加 per-frame `map[uint32]bool` 去重，同一硬件 ID 只 upsert 一次；故障研判（`processFault`）仍逐条执行，去重/自动建单语义不变。
- **理由**：减少热路径 DB 往返（清单 B3）。
- **是否改动行为**：否（upsert 语义为「同 hwID 最后一条记录的值生效」，合并后结果等价）。

### P1（C2）：`frameHwID` 死代码 → `topicHwID` 溯源

- **文件**：`packages/server/internal/mqtt/handler.go`
- **改动前**：`frameHwID(frame)` 无视入参恒返回 0，`HandleCheckFW`/`HandleGetFW` 打日志 `hwId=0`，无法溯源设备（误导性死代码）。
- **改动后**：移除 `frameHwID`，新增 `topicHwID(uplinkTopic)` 从 Topic（`{prefix}/{net}/{station}/{hwid}/U`）解析硬件 ID；两个 FW handler 日志改用 `topicHwID`。
- **理由**：消除误导性死代码，改善排障溯源（清单 C2，仅日志内容变化）。
- **是否改动行为**：否（仅日志字段；无法解析时仍为 0）。同步更新 `handler_cov_test.go` 中对 `frameHwID` 的断言为 `topicHwID`。

### P2（B4）：巡检库存计数与名单合并为单次扫描

- **文件**：`packages/server/internal/service/patrol.go`
- **改动前**：`checkStockAlerts` 先 `Scan` 计数一次、再 `lowStockNames` 各自独立再查一次（每态 2 次扫描）。
- **改动后**：新增 `stockCountAndNames(low)` 一次查询取所有匹配物料名（count=len 全量、名单取 stock ASC 前 6），`checkStockAlerts` 改用之；`lowStockNames` 保留为薄封装（兼容既有测试/调用）。
- **理由**：减少重复扫描（清单 B4）。
- **是否改动行为**：否（count=全部匹配数、名单=前 6，与原语义一致）。

### P2（B5）：故障列表 `active` 状态与时间参数别名抽取

- **文件**：`packages/server/internal/handler/fault.go`
- **改动前**：`active` 兼容语义（occurred/confirmed/dispatched）与 `start_time/start_date`、`end_time/end_date` 双参数别名内联在 `ListFaults`。
- **改动后**：新增 `activeStatuses` 常量、`ParseStatusFilter(status)`、`ParseFaultTimeRange(c)` 工具函数；`ListFaults` 复用。
- **理由**：消除重复/散落兼容逻辑（清单 B5）。
- **是否改动行为**：否。

### P2（C6）：角色判定统一走常量

- **文件**：`packages/server/internal/handler/response.go`
- **改动前**：`isOperator` 用字面量 `"admin"`/`"operator"`。
- **改动后**：新增 `RoleIsOperator(role)`（使用 `model.RoleAdmin`/`model.RoleOperator` 常量），`isOperator` 委托之。
- **理由**：角色字面量统一（清单 C6）。
- **是否改动行为**：否（判定逻辑一致，不影响 viewer 只读等既有效力）。

### P1（A4）：统一日志单例

- **文件**：新增 `packages/server/internal/logger/logger.go`；修改 `mqtt/client.go`、`mqtt/handler.go`、`service/offline.go`、`service/patrol.go`、`service/workorder_escalate.go`
- **改动前**：各 Handler/Service 构造器各自 `zap.NewProduction()` 新建 logger。
- **改动后**：新增 `logger.Get()`（`sync.Once` 单例，支持 `LOG_LEVEL` 环境变量），各构造器改用它注入；保留 `*zap.Logger` 字段类型不变。
- **理由**：统一日志构造与级别配置（清单 A4）。
- **是否改动行为**：否（日志输出语义不变）。

### P2（C7）：`model/db.go` 职责拆分

- **文件**：`packages/server/internal/model/db.go`（瘦身）+ 新增 `migrate.go`、`seed.go`、`legacy_migrate.go`
- **改动前**：`db.go` 352 行混合连接管理 / `AutoMigrate` / `SeedRBAC` / `MigrateLegacyDeviceMaterials` / `SeedAdmin` / `randomStrongPassword` / `containsAny` / `hasClasses` / `InitTestDB`。
- **改动后**：`db.go` 仅保留 `DB`/`RDB` 全局句柄 + `InitDB`/`InitRedis`/`InitTestDB`；`migrate.go` 放 `AutoMigrate`；`seed.go` 放 `SeedRBAC`/`SeedAdmin`/`randomStrongPassword`/`hasClasses`；`legacy_migrate.go` 放 `MigrateLegacyDeviceMaterials`/`containsAny`。
- **理由**：单一职责（清单 C7），纯文件移动、逻辑不变。
- **是否改动行为**：否。

### 补充：`internal/logger` 单测

- **文件**：新增 `packages/server/internal/logger/logger_test.go`
- **理由**：保护覆盖门禁（清单风险 5），覆盖 `ParseLevel` 与 `Get` 单例语义。

---

## 未实施项（说明）

以下清单项本次**未改动**，理由如下，供后续排期：

- **A1（`model.DB`/`RDB` 全局句柄 DI 化）**：清单风险提示明确这是「重构最大雷区」，触达 mqtt/service/handler/ai 全链路，需分步推进。本次仅收敛日志单例（A4）与职责拆分（C7），未做跨包裸引用迁移，避免大爆炸式改动导致覆盖率回退。已在 `db.go` 注释中标注演进方向。
- **A2（后台协程 BackgroundLoop 抽象）**：三处执行时机语义各异（offline/escalator 立即+各自间隔、patrol 立即+60s 窗口判定），抽象需谨慎保持时机不变，风险收益不匹配本次 P0 优先范围。
- **A3（handler 业务逻辑下沉 service）**：涉及工单状态机/派单规则等行为红线，下沉需配套单测与回归，未纳入本次非破坏性重构。
- **A5（`cmd/server/main.go` 抽 app 装配器）**：`main.go` 311 行偏大，但 `setupRouter` 已有测试依赖其签名；本次未动以免破坏入口稳定性。
- **C3（`logPacket` 的 `valid` 参数语义）**：`valid` 在解析失败路径实际已传 `false`，区分是存在的；属可读性微调，未触及报文日志写入行为。
- **C4（`llm.go` `ImageURL` omitempty）**：核对实际代码，`contentPart.ImageURL` 已带 `json:"image_url,omitempty"`（清单描述已过时），无需改动。
- **C5（ai 包上帝文件拆分）**：`nl.go`/`decision.go`/`analyze.go` 拆分属大范围结构性重构，且需严格保证 AI 降级兜底不被旁路，未纳入本次。

---

## 行为红线核对结论

以下业务逻辑在本次重构中**均未被改动**：

| 红线 | 处置 |
|---|---|
| 工单状态机（pending→processing→completed/rejected，rejected→pending） | C1 仅合并重复分支，流转语义不变 |
| 30 分钟故障去重窗口 | B2 仅跳过无变化历史写，`last_seen` 仍始终推进，窗口锚点不变 |
| 严重故障自动建单（WO 序号 `NextOrderNo`） | 未触碰 `NextOrderNo`，B3 仅合并同帧设备 upsert |
| SLA（24h/48h 升级） | `workorder_escalate.go` 仅改 logger 构造，逻辑未动 |
| AI 降级兜底 | 未改动 `internal/ai` 业务代码 |
| RBAC 权限 | C6 仅用常量替换字面量，判定效力不变 |

---

## 验证结果

- `go build ./...`：通过（exit 0）
- `go test ./...`：全绿（1 package 列表：cmd/server、internal/ai、config、handler、logger、middleware、model、mqtt、service 均 `ok`）
- 关键回归用例覆盖：`mqtt` 的 `TestProcessFault_DedupWithinWindow`、`TestProcessFault_NewRecordAfterWindow`、`TestProcessFault_CriticalCreatesWorkOrder` 等均通过，佐证去重窗口与自动建单行为未漂移。
