# TSLOMS 重构优化核对清单（pm-checklist）

> 核对专员：pm-tsloms（只读核对）｜ 日期：2026-08-16
> 依据：现有代码基线（`origin/main` latest `a460365`）+ 需求文档（PRD v1.0–v4.3、SAR v6.1/v6.2、TSLOMS-RP-1.0、核心功能清单）
> 性质：**本清单仅用于核对重构优化范围，未修改任何代码。** 重构必须保持既有业务行为不变。

---

## 一、项目概览

### 1.1 技术栈

| 层 | 技术 | 说明 |
|---|---|---|
| 后端 | Go 1.22+ / Gin / GORM / paho.mqtt.golang / golang-jwt / zap / redis-go | 单体 Web 服务，`packages/server` |
| 前端 | Vue3 + Vite + Element Plus + ECharts + Cesium + Pinia + axios | SPA 管理后台，`packages/admin` |
| 数据库 | MySQL 8.0（生产）/ SQLite（开发/测试双模） | GORM AutoMigrate 托管 schema |
| 缓存 | Redis 7.0（TSLOMS 用 DB1，与 EQS 隔离） | SQLite 模式降级跳过 |
| 消息 | EMQX 5.0（MQTT） | 设备上行/下行 Topic；固件/时间同步/牌告 |
| 部署 | Docker / systemd / nginx | `deploy/` 目录 |

### 1.2 模块结构

**后端 `packages/server`：**
- `cmd/server`：入口 + `setupRouter` 路由装配（main.go，311 行，偏大）
- `internal/config`：环境变量配置单例（`sync.Once` 缓存）
- `internal/model`：GORM 模型 + `DB`/`RDB` 全局句柄 + 迁移 + RBAC 种子 + 管理员种子（db.go 352 行，职责较重）
- `internal/mqtt`：MQTT 客户端、二进制协议解析（parser）、消息处理/故障研判（handler，463 行，偏大）
- `internal/handler`：REST 路由处理器（`ok/fail/paginate` 等统一响应）——**业务逻辑大量堆在此层**
- `internal/middleware`：Auth / RBAC / CORS / Logger
- `internal/service`：后台协程（OfflineCheck、PatrolService、WorkOrderEscalator）
- `internal/ai`：LLM 网关 + 规则兜底引擎（预测 engine、建议 advice、报告 reports、自然语言 nl 729 行、决策 decision 559 行、分析 analyze、异常 anomaly）

**前端 `packages/admin/src`：**
- `api/*.ts`：按域封装的 axios 请求（request.ts 统一拦截/401 跳转）
- `views/*`：按模块页面（dashboard/device/fault/workorder/inventory/ai/map/settings 等）
- `components/`：`AiAssistant.vue`（L5 全局 AI 助手）、`AiCopilot.vue`（L4 流程嵌入）
- Cesium/ECharts 为重型产物，SAR 标记为 P2 性能风险

### 1.3 业务主线（演进到 PRD v4.3 的 AI 化六级）

L1 基础 AI → L2 数据级 AI 原生（报告/建议）→ L3 主动巡检/站内通知 → L4 流程 Copilot → L5 自然语言交互 → L6 决策建议中心 + 实时异常流。

---

## 二、重构目标与范围

本次重构为**非功能性优化**（代码结构 / 性能 / 可读性 / 可维护性），**严禁改变业务行为、接口契约、数据模型与外部返回结构**。

重构范围建议集中在 4 类：

1. **结构**：消除全局可变状态依赖、拆分上帝文件、统一后台协程生命周期、迁移 handler 中溢出的业务逻辑。
2. **性能**：消除 N+1 查询、重复查询、不必要的全量扫描、减少无效 DB 写入与报文解析复制。
3. **可读性/健壮性**：删除重复代码、统一错误处理、补充可测试拆分、修正明显死代码/缺陷。
4. **安全/稳定性**：保持既有安全门禁（auth/RBAC/timeout）不动，重构中不引入权限绕过或行为漂移。

> 范围红线：**不新增业务功能**（如自动派单、多代理等属后续 PRD 二期，不在本次）；**不迁移数据模型**（时序库/事件模型属 RP-1.0 后续基建）；**不重写协议**（MQTT 二进制帧结构与设备契约不可变）。

---

## 三、逐条重构优化问题点

> 位置标注为仓库相对路径。建议按优先级 P0（风险/必改）、P1（结构重要）、P2（可读性锦上添花）推进。

### A. 结构 & 全局状态

**A1. [P1] `model.DB` / `model.RDB` 全局句柄被全包直接引用**
- 文件：`internal/model/db.go`（`var DB *gorm.DB` / `var RDB *redis.Client`）
- 问题：mqtt.Handler、service.*、handler.*、ai.* 全部直接读写 `model.DB`，形成全局可变依赖，难以隔离测试与并发控制；`InitTestDB` 每次覆盖 `DB` 与真实库冲突。
- 建议：重构为依赖注入（构造时传入 `*gorm.DB`），Handler/Service 持有 DB 成员；至少先将`service`/`ai`收敛到显式参数，避免跨包裸引用全局。

**A2. [P1] 三个后台协程重复同样的“ticker + done channel + context”骨架**
- 文件：`internal/service/offline.go`、`patrol.go`、`workorder_escalate.go`
- 问题：`Start(ctx)`/`Done()`/`runOnce`+`done chan` 结构三处复制，间隔/启动即执行逻辑各异，future 新增后台任务必然再复制。
- 建议：抽象统一的 `BackgroundLoop`（interval、startImmediate、runc func(ctx)）复用；同时**保留现有执行时机语义**（offline runOnce 立即+1min、escalator 立即+15min、patrol 立即+60s 轮询窗口），避免行为改变。

**A3. [P1] handler 层职责过重，业务逻辑与 HTTP 耦合**
- 文件：`internal/handler/workorder.go`(344)、`fault.go`(324)、`inventory.go`(335)、`purchase.go`(317)、`rbac.go`(282)、`ai.go`(400)、`firmware.go`(395)、`media.go`(317)
- 问题：工单状态机/派单规则/费用校验等业务都内联在 gin handler 中，`recordOperation`、`workOrderView`、`faultView` 等既有部分视图逻辑又散落。难以单测（需 mock gin context + DB），也难复用（MQTT 自动建工单与 handler 手动建单逻辑重复）。
- 建议：业务规则下沉到 `internal/service`（如 `WorkOrderService`），handler 只做参数解析/鉴权/响应；至少把**工单状态机流转**（pending→processing→completed/rejected）抽成纯函数以单元测试。

**A4. [P1] 日志 Logger 每处 `zap.NewProduction()` 新建**
- 文件：`mqtt/client.go`、`mqtt/handler.go`、`service/*.go`（offline/patrol/escalate）、`service` 构造器
- 问题：每个 Handler/Service 自建 logger，无统一链路 id / 级别配置 / 采样，panic 栈不易串连。
- 建议：提供 `internal/logger` 包单例（复用 `config` 的 sync.Once 模式），各构造器注入，并支持日志级别环境变量。

**A5. [P2] `cmd/server/main.go` 偏大且含启动编排细节**
- 文件：`cmd/server/main.go`（311 行）
- 问题：连接初始化、协程启动、路由装配、优雅停机、超时兜底全在一个文件；`mqttClient` 包级变量仅服务于停机。
- 建议：抽出 `app` 装配器（init db+redis+mqtt+cron → Start/Stop），main 只做编排与信号；`setupRouter` 已有测试（`main_test.go`），重构注意保持其构造签名稳定。

### B. 性能

**B1. [P0] 列表接口存在 N+1 查询**
- 文件：`handler/workorder.go` `ListWorkOrders`→`workOrderView`（每行额外查一次 user）；`handler/fault.go` `ListFaults`→`faultView`（每行查设备+负责人）；`GetWorkOrder`/`GetFault` 详情同样多次单查
- 问题：page_size 最大 100，意味最多 100×(2~3) 次额外查询，接口随 pageSize 线性放大 DB 往返。
- 建议：用一次 `IN` 批量预取（按 id 集合取 username/device），或 GORM `Preload`，在 `workOrderView`/`faultView` 外联查询；**保持返回字段结构不变**。

**B2. [P1] MQTT 热路径逐条落库导致单包多次写**
- 文件：`internal/mqtt/handler.go` `logPacket` / `upsertDevice` / `processFault` / `createWorkOrder`
- 问题：每帧：1 次 packet_log 写入 + 每条事件记录的 device 读判 + 故障查重 + 可能建单，全在消息回调内**同步串行执行**（paho 消息按订阅回调分发），高并发设备上报时会阻塞、放大 DB 写压力；故障查重命中但仍在窗口内也照常 `Updates`（恒真更新）。
- 建议：(1) 故障查重先判断 `now.Sub(existing.LastSeen)<=window` 命中时跳过无变化历史更新（仅在电流/灯态有差异时更新）；(2) 消息处理异步化（channel + worker pool）并保证**顺序性/去重语义不变**；(3) 报文日志批量/异步写库，与业务写解耦。

**B3. [P1] `processFault` 与 `createWorkOrder` 关键路径重复查询**
- 文件：`mqtt/handler.go`
- 问题：同一方法内多处 `model.DB.Where(...).First`；`HandleCheckin`/`HandleAlarm`/`HandlePowerOn` 都调用 `upsertDevice`，同一 packet 内多条 event record 会对同一 device 反复 `WHERE ... First`。
- 建议：同一帧内合并设备 upsert（一次查询缓存 map[hwID] 复用），减少热路径 DB 往返。

**B4. [P2] `patrol.lowStockNames` 与 `checkStockAlerts` 语义重复查询**
- 文件：`internal/service/patrol.go`
- 问题：`checkStockAlerts` 先 `Scan` 计数，再调 `lowStockNames` 各独立再查一次，导致同类物料的计数与名单各查一遍。
- 建议：一次查询同时取 count 与 topN 名单，减少一次扫描。

**B5. [P2] `handler/fault.go` `ListFaults` 旧字段 `active` 兼容映射每次解析**
- 文件：`handler/fault.go`
- 问题：兼容代码把“active 兼容语义”写成每次查询分支，逻辑重复且文案散落。
- 建议：抽出常量 `ActiveStatuses` 与 `ParseStatusFilter` 工具函数统一处理 start/end_time 与 start/end_date 双参数别名，避免多接口重复。

### C. 可读性 & 缺陷

**C1. [P0] `UpdateWorkOrderStatus` 存在重复分支（疑似冗余/潜在缺陷）**
- 文件：`handler/workorder.go` 第 285–299 行
- 问题：`rejected → pending` 的处理**连续写了两遍相同的 if 条件**（`req.Status==pending && wo.Status==rejected`），第一个块已做 `closed_at=nil` + 故障回`confirmed`，第二个块又无条件把 `closed_at` 置 nil（仅清空时间、多余）。同一条件重复、语义冗余，易让维护者误改。
- 建议：合并为单个分支，保留故障回“已确认”+ 清空 closed_at 的唯一语义；**重构后行为应与原逻辑一致**（注：当前第二块追加的 closed_at=nil 在常态 rejected→pending 路径下与第一块叠加，无害但应清理）。

**C2. [P1] `mqtt` `frameHwID` 恒返回 0（死代码/误导）**
- 文件：`internal/mqtt/handler.go`
- 问题：`frameHwID` 注释称“固件命令帧不含事件记录，返回 0”，`HandleCheckFW`/`HandleGetFW` 用它打日志，恒为 0，无法溯源设备；纯误导性死代码。
- 建议：要么从 topic 或既有数据补取 hwID，要么让日志去掉无意义的 hwId=0，避免误导排障。

**C3. [P1] `logPacket` 的 `valid` 参数恒为 `true`（无效分支不可达）**
- 文件：`mqtt/handler.go`
- 问题：所有调用处 `valid` 都传 `true`；解析失败路径在 `logPacket(0,...,false)`（这里 valid=false 但 deviceHwID=0 且 cmd=0 等全 0），有效/无效的区分实际没被用起来。
- 建议：明确“无效包”记录语义（保留解析失败原文日志以便排障），并统一 `valid` 判定避免误导统计。

**C4. [P2] `llm.go` 中 `ImageURL` 字段类型 `*imageURL` 在 `contentPart` 未使用 omitempty 且空转仍拼接**
- 文件：`internal/ai/llm.go` `contentPart`
- 问题：多模态 parts 对文本 part 也携带空 `ImageURL`（带 omitempty? 当前 `image_url` tag 无 `omitempty`，空指针序列化为 `"image_url":null`）。文本与图像混用时可能上送 null 字段。
- 建议：为 `ImageURL` 补 `omitempty` 或在拼装时按类型只放非空字段。

**C5. [P2] `ai` 包文件过大（上帝文件）**
- 文件：`internal/ai/nl.go`(729)、`decision.go`(559)、`analyze.go`(414)
- 问题：单文件承载大量意图/工具/规则，可读性与测试定位成本高。
- 建议：按“意图识别 / 工具执行器 / 规则解析”拆分；**不改变对外 endpoint 与降级行为**。

**C6. [P2] `handler` 中 `isOperator` 与 `middleware.RequireOperator` 语义重复且角色字面量散落**
- 文件：`handler/response.go` `isOperator`；多处 `role=="admin"||role=="operator"` 字符串比较
- 问题：角色判定在 handler 与 middleware 两套实现，字面量 `"admin"`/`"operator"` 多处编码；`requirePerm`（RBAC）与遗留 `isOperator`/`RequireOperator` 并存，权限模型有两条线，易不一致。
- 建议：统一走 RBAC（`RoleIsOperator` 帮助函数 + 常量），逐步弃用遗留 `isOperator`。**注意不改变现有效力**（保留 viewer 只读等既有判定）。

**C7. [P2] `model/db.go` 同时承担模型定义、迁移、种子、数据迁移、工具函数**
- 文件：`internal/model/db.go`（352 行）
- 问题：`AutoMigrate`、`SeedRBAC`、`MigrateLegacyDeviceMaterials`、`SeedAdmin`、`randomStrongPassword`、`containsAny` 混在同一文件。
- 建议：拆分 `migrate.go`/`seed.go`/`legacy_migrate.go`/`util.go`，逻辑不变。

---

## 四、业务核心逻辑说明（供 dev/qa 重构后回归依据）

> **重构不得改变以下业务行为。** dev 改代码、qa 做回归，都以本节为准绳。

### 4.1 MQTT 设备接入与研判链路
- 上行订阅：`{topicPrefix}/+/+/+/U`（+ 匹配网络号/站点号/硬件ID），下行把末尾 `/U` 换 `/D`。
- 二进制帧：CMD_FRAME 16 字节头 +（可选）EVENT_PAK（4 字节头 + N×24 字节 EVENT_RECORD）。字段包括 token(0x55)、cmd、ver、checksum(整包 uint8 累加==0xFF)、swVer、cmdSeq、datLen、userVal；EVENT_RECORD 含 ledHwId/subHwId/swVer/confVer/ledState/errCode/current[R|Y|G]。
- 命令分派：CmdCheckin(签到)、CmdAlarm(告警)、CmdPowerOn(上电)、CmdCheckFW(固件查询)、CmdGetFW(固件数据请求)。签到/告警内的事件记录若有故障（errCode!=0）触发 `processFault`。
- **故障去重（关键）**：同一 `deviceHwId + errCode` 且状态为 occurred/confirmed/dispatched 的故障，在**30 分钟窗口内只更新 `last_seen`+电流/灯态，不新建记录**；超窗则将旧故障置为 `resolved` 并新建。违反该窗口会出现重复故障/误算活跃数。

### 4.2 严重故障自动建单
- `processFault` 中当 `faultLevel == "critical"` 时自动 `createWorkOrder`：生成 `WO{yyyyMMdd}{4位序号}`，CREATE 工单（pending）→ 回写故障 `work_order_id` 并置 `confirmed`+`confirmed_at`。
- 手动建单（handler.CreateWorkOrder）与自动建单共用 `model.NextOrderNo`，序号唯一性依赖该函数，重构时**不得改其生成规则**。

### 4.3 工单状态机与 SLA
- 状态：pending(待处理)→processing(处理中)→completed(已完成)/rejected(已驳回)。
- **派单**：pending 工单派单→processing，故障推进 `dispatched`+记录 repairer_id/dispatched_at。
- **完成**：工单置 completed 记 closed_at，其关联故障置 `resolved`+resolved_at。
- **驳回重派**：rejected→pending 时清空 closed_at，故障回 `confirmed`（对应 C1 需合并的分支）。
- SLA：pending 超 24h / processing 超 48h。`WorkOrderEscalator` 后台把超时 pending 自动升 processing 并写“系统自动升级”result；processing 超时仅日志预警不改状态。看板用 `WorkOrderOverdueHours` 标超时。

### 4.4 设备离线判定
- OfflineCheck 每 1 分钟扫描 `online_status=true` 且 `last_checkin_at < now - OfflineAfterMin(默认6min=2min签到周期×3)` 的设备置离线。

### 4.5 AI 主动巡检与站内推送
- PatrolService 启动即执行一次，之后每 60s 轮询，仅在 `PATROL_DAILY_HOUR/MIN`（默认 08:00）分钟窗口触发每日巡检。
- 巡检=生成运维日报（AI 或规则兜底）+ 推送 report 通知；异常检测（超时工单/高风险设备/低库存/缺货）→ alert 通知。
- 推送对象：`role in (admin,operator)` 且启用；**无目标用户则退化**用 `user_id=0` 面向全体。notification_reads 承载用户级已读。

### 4.6 AI 降级兜底（核心一致性要求）
- 所有 AI 能力均有**规则兜底**：无 LLM key / AI 停用 / 额度超限时回退规则引擎（预测 `PredictDevice`、报告 `rule` 版、建议固定文案、NL 意图规则识别），**保证功能可用**。
- 额度：LLM 调用先 `checkQuota`（按日次数/Token 限额），每次调用 `record` 写 AIUsage 流水。
- 决策一键采纳：`/ai/decision/adopt` 同时要求 `ai:ops` 与 `purchase:manage`（多头权限）。

### 4.7 权限模型（重构不可破坏）
- 双线并存：新旧 RBAC（`permission/role/user_permission`，`RequirePerm`）+ 遗留 `RequireAdmin/RequireOperator/isOperator`。Auth 中间件校验 JWT(HS256)+用户存在+未停用。
- 停用用户既有 JWT 即时失效（auth.go 查库校验 status）。
- 公开端点仅 login / health / 地图瓦片代理；其余全在 `Auth` 保护下。

---

## 五、风险提示

1. **全局 DB 依赖（A1）是重构最大雷区**：`model.DB` 被 mqtt/service/handler/ai 四面引用，任何“改成注入”的改动都可能触达全链路。建议分步（先 service/ai，再 handler/mqtt），每步 `go build`+全量测试通过再继续。
2. **MQTT 总线改异步（B2）**：设备运维关乎交通安全侧，异步化若乱序或丢消息会造成故障漏判/工单漏建。必须保证同一设备/事件**顺序**与 `logPacket` 不丢；先做“仅异步化报文日志”，业务研判保持同步，验证后再扩大。
3. **状态机/故障去重是行为红线（4.1/4.3）**：C1 合并分支时务必核对 completed/resolved、rejected→pending 组合形态，建议 qa 补 `pending→processing→completed` 与 `pending→rejected→pending` 两条回归用例。
4. **AI 兜底（4.6）**：降级路径规则兜底若被重构旁路，会在无 LLM key 环境整个 AI 功能失效。任何 ai/llm 改动都要在**无 key**环境跑一遍降级冒烟。
5. **覆盖率门禁**：SAR v6.2 已达 80.5%（≥80%）。重构牵动 handler/service —— 每批改动必须保持 `go test ./...` 全绿且覆盖率**不回退**到 80% 以下，否则 CI（`Makefile coverage-check`）会拦截。
6. **CI 超时环境（SAR v6.0/6.1 记录）**：Windows 工作区并行任务多时 `go test` 曾 >180s 未收敛。重构验证建议在干净 CI/预发布跑全量，勿把本地并行超时当成失败。
7. **前端重型产物（Cesium/ECharts）**：SAR 标记 P2 低网速首屏风险；本次重构若只动后端可不动前端，若默认拆分则需回归地图大屏与 ECharts 看板渲染。
8. **不破坏既有接口契约**：`ok`/`fail` 返回结构、`{code,msg,data}` 与分页 `{list,total,page,page_size}` 为前端 `request.ts` 依赖，重构前后端契约必须字面一致。

---

## 六、附注（本次核对新增发现，非规划文本）

- 前端依赖含 `cesium@^1.144.0`、`echarts@^6.1.0` 重型库；`AiAssistant.vue`/`AiCopilot.vue` 承载 L5/L4 逻辑，体积与复杂度偏高，可作前端可读性优化候选（P2）。
- 前端 `api/*.ts` 已按域清晰拆分，是良好参照；后端 handler 若照此拆分到 `service` 可明显提升对称性。
- 上述问题点均为**结构性/性能性/可读性**判断，未改动任何代码，供 dev/qa 排期与回归参考。
