# TSLOMS 结项审核报告（SAR）v4.3 — 平台 AI 化（主动巡检 + 流程 AI 辅助 + 自然语言交互）

> 版本：v4.3 ｜ 日期：2026-08-15 ｜ 基线：commit `0c02e1c`（L1-L3/L4部分）→ `e93f928`（L4全流程）→ `xxxxxxx`（L5 自然语言交互）
> 前置：v4.0（AI 预测/诊断/生命周期）、v4.1（AI 原生增强）、v4.2（库存进销存/费用闭环 + RBAC 权限隔离）。
> 本版范围：A. AI 主动巡检（定时日报 + 异常预警 + 站内推送）；B. 流程 AI 辅助嵌入（全流程）；C. AI 自然语言交互（L5）。

> **六级 AI 化定位**：本版落地 **L3（AI 主动服务）** 全部 + **L4（流程 AI 辅助）** 全覆盖 + **L5（自然语言交互）** 已在顶部 AI 助手落地；
> 完整演进路线见 `docs/PRD-TSLOMS-v4.3.md ★ 平台 AI 化六级演进路线`，供后期按层推进（下一层 L6 自主决策）。

---

## 一、交付内容清单

### 后端（Go, packages/server）

| 文件 | 说明 |
|---|---|
| `internal/model/notification.go` | 站内通知模型 `Notification`（type/title/content/link/biz/user_id/is_read）；创建/未读数/列表辅助函数；AutoMigrate 注册 |
| `internal/service/patrol.go` | **AI 主动巡检协程**：启动即巡检 + 每日定时（env `PATROL_DAILY_HOUR/MIN` 默认 08:00）；生成日报 + 异常检测 + 定向推送 |
| `internal/service/patrol_test.go` | 巡检通知单测（创建/未读/已读/列表 + 面向全体广播） |
| `internal/handler/notification.go` | 通知接口：`GET /notifications`、`GET /notifications/unread-count`、`PUT /notifications/:id/read`、`PUT /notifications/read-all` |
| `cmd/server/main.go` | 注册 `notifications` 路由；启动 `NewPatrolService` 协程并纳入优雅停机；注册 L4 三新增 AI 辅助路由（device/建单/采购） |
| `internal/ai/copilot_extra.go` | **L4 流程 AI 辅助**：设备填写建议、建单推荐（优先级/备件/步骤/维修人）、采购合理性校验+供应商建议；均 LLM+规则兜底 |
| `internal/ai/copilot_extra_test.go` | L4 规则兜底单测（采购校验/采购合计/设备空 hw_id） |
| `internal/ai/nl.go` | **L5 自然语言交互引擎**：意图识别（query/command/fallback）+ 工具执行（故障排行/设备状态/工单统计/费用归因/报修建单/命令式建单）+ 内置知识库 RAG；LLM 识别、规则兜底 |
| `internal/ai/nl_test.go` | L5 规则识别/参数解析/咨询判断单测 |
| `internal/handler/ai_advance.go` | 新增 `SuggestDeviceCopilotAPI`/`SuggestWorkOrderCreateAPI`/`SuggestPurchaseCopilotAPI` + `NLInteractAPI`（L5 自然语言入口） |

### 前端（Vue3, packages/admin）

| 文件 | 说明 |
|---|---|
| `src/api/notification.ts` | 通知接口封装 + `NotificationItem` 类型 |
| `src/api/copilot.ts` | AI 建议接口封装（故障/工单 `FaultAdvice`/`WorkOrderAdvice`） |
| `src/components/AiCopilot.vue` | 通用 AI 辅助组件：生成建议 + 一键填入表单（含骨架/错误/空态）；支持故障/工单/设备/建单/采购多形态建议 |
| `src/views/layout/index.vue` | 顶部**通知铃铛**：未读红点（2 分钟轮询）+ 通知中心面板（标签/摘要/跳转/全部已读） |
| `src/views/fault/index.vue` | 故障处理弹窗嵌入 **AI 处置建议**（确认/派单辅助） |
| `src/views/workorder/index.vue` | 工单完成弹窗嵌入 **AI 维修小结**（一键填入维修结果）；**新建工单弹窗**嵌入 **AI 建单建议**（优先级/备件/步骤/维修人预选） |
| `src/views/device/index.vue` | 设备新建/编辑弹窗嵌入 **AI 填写建议**（依据录入字段实时生成） |
| `src/views/inventory/Purchase.vue` | 采购新建弹窗嵌入 **AI 采购校验**（数量/金额校验 + 供应商建议） |
| `src/components/AiAssistant.vue` | **L5 AI 助手**：顶部入口，对话式自然语言查询/命令，含快捷建议、结构化表格渲染、来源/工具标注 |
| `src/api/copilot.ts` | 新增 `nlInteract` + `NLAnswer` 类型（L5 后端对接） |

---

## 二、关键设计

### 1. AI 主动巡检调度
- 复用既有后台协程模式（同 `OfflineCheck`/`WorkOrderEscalator`）：`context` 取消实现优雅停机。
- 目录：
  - 启动即执行一次 `patrol()`（便于部署后立即有日报与预警）；
  - 之后每 60 秒检查一次，命中「当前时刻 == 配置时刻」时执行，避免重复。
- `patrol()` 流程：`GenerateDailyReport(0)`（AI/规则兜底）→ `BuildDailySnapshot()` → 日报通知 → `checkAlerts()`（超时工单/高风险设备/低库存缺货 → alert 通知）。

### 2. 站内通知模型
- `user_id=0` 表示面向全体；定向推送对 role ∈ {admin, operator} 且启用的用户逐条创建；无目标用户退化为全体。
- 未读计数/列表查询统一：`(user_id = ? OR user_id = 0)` 且 `is_read = false`。

### 3. 流程 AI 辅助（L4）
- `AiCopilot` 为受控组件：`loadFn` 拉取建议、`fillFn` 填入表单；LLM 失败时后端已规则兜底，前端始终可展示。
- 复用既有 `/ai/advice/fault/:id`、`/ai/advice/workorder/:id?stage=summary`；新增三个 **POST** 端点：
  - `/ai/advice/device`：依据前端提交的设备字段生成填写/配置建议 + 校验提醒；
  - `/ai/advice/workorder/create`：基于关联故障推荐优先级/备件/处理步骤/维修人（支持一键预选维修人）；
  - `/ai/advice/purchase`：采购明细合理性校验 + 供应商建议。
- 均采用「规则兜底先算 → LLM 增强合并」：规则保证结构字段（优先级/合计/步骤）始终可用，LLM 输出以「处理步骤：/预领备件：」等结构化前缀解析并合并，无法解析时保留规则值，避免 LLM 格式漂移导致空字段。

### 4. AI 自然语言交互（L5）
- 顶部 **AI 助手入口**（header 魔法棒图标）→ `AiAssistant` 对话面板：用户自然语言 → `POST /ai/nl/interact`。
- 后端 `nl.go`：先去「怎么/如何」咨询判断（直接走知识库，避免 LLM 误判建单）→ LLM 意图识别成结构化 JSON（intent/tool/params）→ `runTool` 按工具执行真实数据；LLM 失败/无密钥时规则识别兜底。
- 查询工具（只读真实数据）：`fault_rank`（路口故障排行）、`device_status`（在线/离线/签到）、`workorder_stats`（状态分布+超时）、`expense_summary`（费用归因）。
- 命令工具（真实写入）：`create_fault`（自然语言报修 → 结构化故障单，识别设备/路口/灯色/等级）、`create_workorder`（命令式建工单）；返回 `did_write`/`created_id`，操作日志按写操作记录。
- 知识库 RAG：内置运维知识库（操作流程/设备/采购/固件/报告），关键词检索直接回答咨询问题。
- 设备解析鲁棒性：`设备N` 显式模式 + hw_id 参数 + 路口名/交叉口提取多级兜底，保证「设备1」这类短 ID 也能命中。

---

## 三、测试结果

### 后端单测/集成
- `go build ./...` ✅、`go vet ./...` ✅、`gofmt` ✅
- `go test ./...`：ai / handler / middleware / model / mqtt / service 全绿（含新增 `patrol_test.go`）

### 前端质量门
- `vue-tsc --noEmit` ✅、`eslint --fix` ✅、`npm run build` ✅（1m13s）

### 生产 E2E（真实服务，129.211.223.113）
| 检查 | 结果 |
|---|---|
| `/my/permissions`（RBAC 未受影响） | ✅ 28 权限点 |
| `/notifications` 返回巡检通知 | ✅ total=2 unread=2 |
| 通知类型含 report + alert | ✅ types=alert,report（启动即已生成） |
| `/notifications/unread-count` | ✅ |
| 单条已读 / 全部已读 | ✅ |
| `/ai/advices`（Copilot 历史） | ✅ 200 |
| `/ai/advice/device`（设备填写建议） | ✅ 200，source=LLM；空 hw_id 有提示 |
| `/ai/advice/workorder/create`（建单建议） | ✅ 200，priority=P0，steps 非空 |
| `/ai/advice/purchase`（采购校验） | ✅ 200 含合计；非法明细触发校验 |
| `/ai/nl/interact`（故障排行「最近7天哪些路口故障最多」） | ✅ 200，tool=fault_rank，真实 SQL 聚合 |
| `/ai/nl/interact`（设备状态） | ✅ 200，tool=device_status |
| `/ai/nl/interact`（工单统计） | ✅ 200，tool=workorder_stats |
| `/ai/nl/interact`（费用归因） | ✅ 200，tool=expense_summary |
| `/ai/nl/interact`（知识库「怎么新建工单？」） | ✅ 200，tool=kb/intent=fallback |
| `/ai/nl/interact`（报修「报修：设备1黄灯不亮」） | ✅ 200，tool=create_fault，did_write=true 真实建故障单 |
| `/ai/nl/interact`（命令式建单「给设备1建工单」） | ✅ 200，tool=create_workorder，真实建工单 |
| `/ai/nl/interact` 空/缺 text | ✅ 400 + code=-1 |
| 服务 active / gateway / admin | ✅ active / 200 / 200 |
| RBAC 权限未受影响 | ✅ 28 权限点 |

> 部署后服务启动即完成一次巡检，自动生成 1 条日报（report）与 1 条预警（alert）通知，验证调度与推送链路真实生效。

---

## 四、部署记录

- 阶段一（L4 流程 AI 辅助，20260815211932/21542256）：dist + 后端二进制，SHA 校验 → 备份 → 替换 → 重启；
- 阶段二（L5 自然语言交互，20260815214834/215418/215834/220133）：前端 AiAssistant + 后端 NL 引擎逐步加固（设备解析兜底、咨询判断、空入参校验）；统一脚本 `tsloms_l5_deploy.sh`。
- 备份：`server.pre-l4-*`/`dist.pre-l4-*`、`server.pre-l5-*`/`dist.pre-l5-*`。
- 最终新二进制 SHA（远端）= `99b336e1…`，与本地 `server.linux` 一致。
- 因 RBAC 阶段发现「二进制真实路径为 `/opt/tsloms/packages/server/server`（app 目录内文件）」，脚本已按正确路径部署。
- `tsloms-server` active、NRestarts=0，网关/后台 200。

---

## 五、遗留与二期

- 下一层为 **L6 AI 自主决策**：实时异常流检测、AI 决策建议中心（健康评分/智能排班/备件预测/成本优化）、半自动/自动执行、多代理协作。
- L5 文字交互已上线；**语音输入**属后续增强（外部 ASR 通道）。
- 巡检仅站内推送；如需短信/邮件告警，属后续外部通道扩展。

---

*本报告基于代码静态审核 + 单元测试 + 生产 E2E 实测。性能并发指标未经压测，生产量级（百级设备）运行正常。*
