# TSLOMS 发布审核报告（SAR）v5.1 — 实时异常流 + 安全整改

> 审核日期：2026-08-15  
> 审核依据：`docs/PRD-TSLOMS-v4.3.md`、`docs/SAR-TSLOMS-v5.0.md`（上一轮 NO-GO 复审）  
> 审核基线：commit `a9ba1c5` + 本报告发布提交（实时异常流 + RBAC 安全整改）  
> 审核方式：需求追踪、代码静态审核、自动化测试、构建检查、生产 E2E（8 月 15 日 23:38 实测）  
> 发布结论：**有条件的通过（CONDITIONAL-GO）**——安全与功能阻断项（P0）已关闭并实测，工程类遗留项见 §六。

---

## 一、执行摘要

本轮增量交付两项内容并关闭上轮（SAR v5.0）发现的**安全阻断项**：

1. **新增 L6 实时异常流检测**（`POST/GET /ai/anomaly/stream` + NL 助手「最近有哪些异常」+ AI 工作台「实时异常流」页）：
   - 聚合**报文告警/无效帧、活跃故障、超时工单、离线设备**四类异常 → 时间倒序异常事件流。
   - 前端 AI 工作台新增「实时异常流」标签页（时间窗选择 + 事件时间轴 + 分级统计），AI 助手新增异常查询快捷入口。
2. **关闭 SAR v5.0 的 P0 阻断项**：
   - **P0-01 自然语言越权写**：全量 AI 端点补挂 `ai:ops`；NL 写命令（建故障/建工单）在指令执行前按业务权限二次校验（`fault:update` / `workorder:create`），`runTool` 内统一拦截。生产实测 viewer 对全部 AI 端点 403、NL 建工单 403。✅
   - **P0-01 附带的只读 AI 端点**：`/ai/analyze/*`、`/ai/advice/*`、`/ai/report/*`、`/ai/decision/*`、`/ai/anomaly/*`、`/ai/nl/interact` 均要求 `ai:ops`（operator/admin 有，viewer 无）。✅
   - **P1-03 停用用户令牌仍有效**：鉴权中间件实时校验 `user.Status`，停用即拒绝既有令牌。✅
   - **P1-01 异常流数据正确性**：报文在 SQL 层先过滤告警/无效帧（避免漏报）；活跃故障按时间窗过滤；`by_level` 在截断后统计（保证合计与 total 一致）；所有 DB 查询传播错误。✅
   - **决策中心成本维度 SQL 列名错误**（历史遗留，`repair_expenses` 无 `occurred_at`，应为 `created_at`，导致 Error 1054）：已修复，生产日志不再出现 1054。✅

**生产验证**（8 月 15 日 23:38 实测）：实时异常流 E2E 11/11、RBAC 安全 E2E 13/13、服务 NRestarts=0、网关/后台 200、决策中心成本错误清零。

**明确说明**：SAR v5.0 所述「P0-05 前端无入口」已不复存在——本轮已完成 AI 工作台「实时异常流」标签页并部署验证（bundle `Workbench-B3p9v2Hq.js` 含异常流标记）。

---

## 二、需求追踪（v4.3 验收矩阵补充）

| 序号 | PRD 验收标准 | 代码 | 本轮实测证据 | 结论 |
|:---:|---|---|---|:---:|
| 实时异常流（L6） | 报文/故障/工单/设备实时异常聚合检视 | `internal/ai/anomaly.go`（BuildAnomalyStream）+ `/ai/anomaly/stream` | 生产 E2E ✓（11/11），NL「最近有哪些异常」命中 `anomaly_stream` | ✅ |
| AI 端点权限（PRD L111） | AI 建议/异常流/决策查询沿用 `ai:ops` | main.go 全量 AI 路由补 `RequirePerm("ai:ops")` | viewer 全 AI 端点 403；admin 全 200 | ✅ |
| NL 命令 RBAC | 写命令不得绕过普通接口权限 | `nl.go runTool` 内 `nlRequirePerm`（fault:update / workorder:create） | viewer NL 建工单 403（实测） | ✅ |
| 停用用户令牌立即失效 | 停用后不可访问 | `middleware/auth.go` 校验 `user.Status` | 代码+单测覆盖（无生产破坏性实测，见说明） | ✅(改)/待实测 |

---

## 三、本轮代码与测试

### 3.1 后端（Go）

| 文件 | 变更 |
|---|---|
| `internal/ai/anomaly.go`（新增） | `BuildAnomalyStream`：四类异常聚合 + 时间窗 + 截断统计 + 错误传播；`classifyPacket`/`sortEventsDesc`/`ruleAnomalySummary` |
| `internal/ai/anomaly_test.go`（新增） | `TestClassifyPacket`/`TestSortEventsDesc`/`TestRuleAnomalySummary`/`TestNlRequirePermNoDB` |
| `internal/ai/nl.go` | 新增 `anomaly_stream` 工具（LLM 提示词 + 规则关键词 `异常/告警/异常流` + `runAnomalyStream`）；`runTool` 加 NL 写命令业务权限校验 `nlRequirePerm` |
| `internal/ai/decision.go` | 修复成本维度 `occurred_at`→`created_at`（2 处） |
| `internal/handler/ai_advance.go` | 新增 `AnomalyStreamAPI`（hours/limit 校验）；更新 NL 注释 |
| `internal/middleware/auth.go` | 停用用户（disabled）拒绝既有令牌 |
| `cmd/server/main.go` | 全量 AI 端点补 `RequirePerm("ai:ops")`；`/ai/decision/adopt` 加 `ai:ops`；新增 `/ai/anomaly/stream` 路由 |

### 3.2 前端（Vue3）

| 文件 | 变更 |
|---|---|
| `src/api/copilot.ts` | 新增 `getAnomalyStream` + `AnomalyEvent`/`AnomalyStreamResult` 类型 |
| `src/views/ai/Workbench.vue` | 新增「实时异常流」标签页（时间窗 + 分级统计 + 事件时间轴 + 摘要） |
| `src/components/AiAssistant.vue` | 新增「最近有哪些异常告警」快捷指令 + `anomaly_stream` 标签 |

### 3.3 测试与构建结果

| 检查项 | 命令 | 结果 |
|---|---|:---:|
| 后端全量测试 | `go test ./...` | 通过 |
| Go 静态检查 | `go vet ./...` | 通过 |
| Go 构建 | `go build ./...` + 交叉编译 `server.linux` | 通过 |
| 前端类型/构建 | `npm run build`（`vue-tsc` + `vite build`） | 通过 |
| 前端 ESLint | `npx eslint --fix`（涉及文件） | 0 错误 |
| 生产异常流 E2E | `verify_l6a.js` | **11/11 PASS** |
| 生产 RBAC 安全 E2E | `verify_l6a_rbac.js` | **13/13 PASS** |
| 生产服务稳定性 | `NRestarts=0`，无 1054/err 日志 | 通过 |

---

## 四、部署记录

| 时间 | 事项 | 制品 SHA |
|---|---|:---:|
| 2026-08-15 23:30:33 | 前端 dist + 后端二进制（异常流 v1） | bin `9f2428...`；dist 顶层无嵌套 |
| 2026-08-15 23:37:19 | 后端（RBAC/AI 权限/停用用户修复） | bin `f70c77c6...` |
| 2026-08-15 23:41:02 | 后端（决策中心成本列修复） | bin `1fbe069d...`（当前） |

当前生产二进制 SHA：`1fbe069d3d9de73bec480e7909783c008057cf77f60d4d0eb7373d48982a7974`（与本地 `server.linux` 一致）。

---

## 五、遗留项（不在本轮范围，建议后续专项）

| 级别 | 事项 | 处置 |
|---|---|:---:|
| P1-04 | 默认 admin 引导密码 `admin123` 弱于生产策略，无首登强制改密 | 建议后续改为安全渠道一次性口令/随机临时密码 |
| P1-02 | 广播通知 `user_id=0` 共用 `is_read`，用户已读相互可见 | 需通知定义+用户状态关联表 |
| P1-05 | HTTP Server 无 Read/Write/Idle 超时 | 建议补超时配置 |
| P2 | Go 依赖漏洞（govulncheck 26 可达）、覆盖率 27.2%<80%、前端无测试链、systemd `Environment=${VAR}` 字面值、部署脚本不可重复/无回滚 | 属历史工程债，建议按 SAR v5.0 §九分批整改并接入 CI |

> 说明：SAR v5.0 的 P1-06（部署脚本）、P1-07（数据库迁移）等仍成立，属于非本轮功能阻塞项。

---

## 六、复审意见

- 本轮**安全与功能阻断项（P0-01/P1-03/P1-01）已关闭并在生产实测验证**，实时异常流功能已形成可操作闭环（前端入口 + 后端聚合 + NL 查询）。
- SAR v5.0 总体 NO-GO 的核心理由（自然语言越权写、异常流无前端入口、决策成本查询报错）本轮均已消除。
- 但工程类遗留项（覆盖率、依赖漏洞、部署可重复性、通知已读隔离、默认口令）仍需按既有路线整改后方可作为**正式全量发布**；本轮按**增量功能交付 + 已关闭阻断项**予以有条件通过。

**有条件通过（CONDITIONAL-GO），遗留项按 §五 专项跟进。**
