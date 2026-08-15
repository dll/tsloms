# TSLOMS 结项审核报告（SAR）v4.3 — 平台 AI 化（主动巡检 + 流程 Copilot）

> 版本：v4.3 ｜ 日期：2026-08-15 ｜ 基线：commit `0c02e1c`
> 前置：v4.0（AI 预测/诊断/生命周期）、v4.1（AI 原生增强）、v4.2（库存进销存/费用闭环 + RBAC 权限隔离）。
> 本版范围：A. AI 主动巡检（定时日报 + 异常预警 + 站内推送）；B. 流程 Copilot 嵌入。

> **六级 AI 化定位**：本版落地 **L3（AI 主动服务）** 全部 + **L4（流程 Copilot 嵌入）** 部分；
> 完整演进路线见 `docs/PRD-TSLOMS-v4.3.md ★ 平台 AI 化六级演进路线`，供后期按层推进。

---

## 一、交付内容清单

### 后端（Go, packages/server）

| 文件 | 说明 |
|---|---|
| `internal/model/notification.go` | 站内通知模型 `Notification`（type/title/content/link/biz/user_id/is_read）；创建/未读数/列表辅助函数；AutoMigrate 注册 |
| `internal/service/patrol.go` | **AI 主动巡检协程**：启动即巡检 + 每日定时（env `PATROL_DAILY_HOUR/MIN` 默认 08:00）；生成日报 + 异常检测 + 定向推送 |
| `internal/service/patrol_test.go` | 巡检通知单测（创建/未读/已读/列表 + 面向全体广播） |
| `internal/handler/notification.go` | 通知接口：`GET /notifications`、`GET /notifications/unread-count`、`PUT /notifications/:id/read`、`PUT /notifications/read-all` |
| `cmd/server/main.go` | 注册 `notifications` 路由；启动 `NewPatrolService` 协程并纳入优雅停机 |

### 前端（Vue3, packages/admin）

| 文件 | 说明 |
|---|---|
| `src/api/notification.ts` | 通知接口封装 + `NotificationItem` 类型 |
| `src/api/copilot.ts` | AI 建议接口封装（故障/工单 `FaultAdvice`/`WorkOrderAdvice`） |
| `src/components/AiCopilot.vue` | 通用 AI 助手组件：生成建议 + 一键填入表单（含骨架/错误/空态） |
| `src/views/layout/index.vue` | 顶部**通知铃铛**：未读红点（2 分钟轮询）+ 通知中心面板（标签/摘要/跳转/全部已读） |
| `src/views/fault/index.vue` | 故障处理弹窗嵌入 **AI 处置建议**（确认/派单辅助） |
| `src/views/workorder/index.vue` | 工单完成弹窗嵌入 **AI 维修小结**（一键填入维修结果） |

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

### 3. 流程 Copilot
- `AiCopilot` 为受控组件：`loadFn` 拉取建议、`fillFn` 填入表单；LLM 失败时后端已规则兜底，前端始终可展示。
- 复用既有 `/ai/advice/fault/:id`、`/ai/advice/workorder/:id?stage=summary`，不新增 AI 链路。

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
| 服务 active / gateway / admin | ✅ active / 200 / 200 |

> 部署后服务启动即完成一次巡检，自动生成 1 条日报（report）与 1 条预警（alert）通知，验证调度与推送链路真实生效。

---

## 四、部署记录

- 方式：后端二进制 + 前端 dist，单脚本原子替换（`tsloms_ai_deploy.sh`），校验 SHA → 备份 → 替换 → 重启 → 验证。
- 备份：`server.pre-ai-20260815201157`、`dist.pre-ai-20260815201157`。
- 新二进制 SHA（远端）= `0a332d04…`，与本地 `server.linux` 一致。
- 因 RBAC 阶段发现「二进制真实路径为 `/opt/tsloms/packages/server/server`（app 目录内文件）」，本版脚本已按正确路径部署。

---

## 五、遗留与二期

- 已列入二期：C 自然语言查询（RAG）、D 知识库问答、E 实时异常流检测、F AI 决策建议中心。
- 巡检仅站内推送；如需短信/邮件告警，属后续外部通道扩展。

---

*本报告基于代码静态审核 + 单元测试 + 生产 E2E 实测。性能并发指标未经压测，生产量级（百级设备）运行正常。*
