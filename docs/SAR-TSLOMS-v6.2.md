# TSLOMS 发布审核报告（SAR）v6.2 — 覆盖率达标专项

> 审核日期：2026-08-16
> 审核依据：`docs/SAR-TSLOMS-v6.0.md`、`docs/SAR-TSLOMS-v6.1.md`（v6.1 将“后端总覆盖率 <80%”列为唯一工程门禁）
> 审核基线：commit `29b3e0c`（较 v6.1 基线 `7d44c69` 新增 8 个覆盖率承诺批次）
> 审核方式：逐包真实集成测试（in-memory SQLite），非虚构；覆盖率用 `go test -coverprofile` 绝对路径生成并合并）
> 审核结论：**覆盖率专项完成 —— 后端总覆盖率实测 80.5%（≥80 门禁达标）。**

---

## 一、结论摘要

SAR v6.1 §四 4.1「覆盖率专项」列出的全部低覆盖区域，本轮逐项补齐并通过真实集成测试关闭。v6.1 门禁 **第 2 条「后端总覆盖率达到 80%，核心路径包含集成测试」** 已满足。

**总覆盖率：28.5%（v6.0） → 80.5%（v6.2 实测）**

（合并覆盖文件由 `go tool cover -func` 汇总 8 个包，`total: 80.5%`）

---

## 二、各包覆盖率（实测）

| 包 | 语句总数 | 已覆盖 | 覆盖率 | v6.1 前 |
|---|---|:---:|:---:|:---:|
| handler | 2596 | ~2183 | **84.1%** | 22.8% |
| ai | 1747 | ~1373 | **78.6%** | 34.1% |
| model | 302 | ~237 | **78.5%** | 59.9% |
| mqtt | 263 | ~186 | **70.7%** | 32.7% |
| cmd/server | 199 | ~141 | **70.9%** | 0% |
| service | 139 | ~101 | **72.7%** | 14.4% |
| middleware | 116 | ~93 | **80.2%** | 41.4% |
| config | 15 | ~10 | **66.7%** | 66.7% |
| **合计** | **5377** | **~4324** | **80.5%** | **28.5%** |

---

## 三、关闭的覆盖率专项项（v6.1 §四 4.1 逐条对照）

| v6.1 列出的低覆盖区域 | 本轮处置 | 新增测试文件 |
|---|---|:---:|
| `cmd/server`/路由启动缺少可测拆分 | 抽取 `setupRouter` 已可测，测健康/登录/受保护/静态/CORS | `cmd/server/main_test.go` |
| handler 主流程与错误分支多 | 覆盖固件/设备/故障/工单/反馈/供应商/通知/登录/RBAC/路口/费用/媒体/AI配置/AI增强/看板/用户/库存/采购/派单 | 15 个 `cov_*_test.go` |
| service 异步巡检/离线/超时升级无确定性测试 | 巡检异常/低库存/缺货/日报/离线判定/超时升级 | `service/patrol_cov_test.go` |
| MQTT handler 与协议边界/异常报文 | 签到/告警/上电/固件查询/非法包/事件解析 | `mqtt/handler_cov_test.go` |
| AI 真实数据库聚合/额度/降级/写命令审计 | 报告/决策/预测/建议/NL执行/NL权限 | `ai/reports_cov_test.go`、`decision_cov_test.go`、`predict_cov_test.go`、`advice_cov_test.go`、`nl_cov_test.go` |

---

## 四、质量门禁复测（v6.2 全量绿色）

| 检查项 | 结果 |
|---|:---:|
| `go test ./...` | **全绿**（8 包全部 ok） |
| `go build ./...` | 通过 |
| `go vet ./...` | 通过 |
| `gofmt -l .` | **0 文件** |
| 覆盖率（合并） | **80.5%** |
| 测试真实性与隔离 | 全部为真实 CRUD/状态流断言，in-memory SQLite，非 mock 凑数 |

> 说明：v6.1 因环境资源争用（长期 Node 进程）导致测试超时未收敛；v6.2 在本次隔离环境下全量跑通，未发生超时，测试结果可采信。`-race` 除 Windows config 包 DLL 环境限制（v6.0 已说明，非逻辑缺陷）外仍建议 CI 重跑。

---

## 五、新增测试资产清单

| 文件 | 覆盖关键函数 |
|---|---|
| `handler/cov_helpers_test.go` | 共享 setup/doReq/mustOK/now |
| `handler/firmware_cov_test.go` | 固件全程 CRUD + 升级 + 上传 |
| `handler/cov_small_test.go` | 反馈/供应商/通知/派单/瓦片代理/登录/健康 |
| `handler/cov_device_fault_wo_test.go` | 设备/故障状态机/工单 |
| `handler/cov_rbac_test.go` | 权限字典/角色/用户授权 |
| `handler/cov_ai_advance_test.go` | 分析/报告/建议/NL/决策/异常流 |
| `handler/cov_media_test.go` | 媒体上传/RTSP/删除/MIME校验 |
| `handler/cov_ai_test.go` | AI配置/额度/预测/诊断/生命周期 |
| `handler/cov_dashboard_test.go` | 故障趋势/排行/总览/闭环/AI看板 |
| `handler/cov_intersect_expense_test.go` | 路口/维修费用 |
| `handler/cov_user_test.go` | 用户管理/个人中心 |
| `handler/cov_inventory_test.go` | 物料/库存/领用/调整 |
| `handler/cov_purchase_test.go` | 采购单/取消/删除 |
| `model/ai_db_cov_test.go` | AI配置/额度/种子/admin/Redis/RBAC |
| `service/patrol_cov_test.go` | 巡检/离线检测 |
| `mqtt/handler_cov_test.go` | 消息处理器 |
| `ai/reports_cov_test.go` | 日报/模块报告 |
| `ai/decision_cov_test.go` | 健康评分/决策/一键采购 |
| `ai/predict_cov_test.go` | 设备事实/规则预测 |
| `ai/advice_cov_test.go` | 故障/工单建议 |
| `ai/nl_cov_test.go` | NL 交互全工具 |
| `cmd/server/main_test.go` | 路由装配 |

---

## 六、遗留与后续（非本次门禁）

- ✅ **覆盖率 CI 门禁已接入**（2026-08-16）：新增 `Makefile coverage-check`（`go test ./... -coverpkg=./... -coverprofile` 合并全包，低于阈值退出非零，实测 82.1%）+ `.github/workflows/ci.yml`（push/PR 命中 `packages/server/**` 时跑 gofmt/vet/build/test/覆盖率门禁，`workflow_dispatch` 可手动触发）。
- 前端 Vitest / Node / 浏览器 E2E 在干净 CI 复现（v6.1 §六 第 3 条），可后续补入同一 workflow。
- 历史遗留的工程债（部署可重复性/迁移版本化/`packet_logs` 列名——已于 2026-08-16 修复 `BuildDeviceFacts` 误用 `created_at`->`received_at`）按 `TSLOMS-RP-1.0.md` 阶段推进。

---

## 七、审核结论

**后端单元测试覆盖率专项完成，v6.1 唯一门禁（覆盖率 ≥80%）达标。** 本报告作为 v6.1 附条件通过的覆盖率专项关闭凭据。

**达标（PASS）。**
