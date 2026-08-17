# 重构改动记录（refactor-notes.md）

- **日期**：2026-08-17
- **范围**：A（后端 `packages/server`，前端零改动）
- **职责**：重构开发专员 dev-refactor-tsloms
- **依据**：`pm-checklist.md`（红线 R1–R10，条目 S1–S10）
- **目标**：实现「智能多源故障识别研判引擎」，全程向后兼容、不改变既有业务判定语义。

---

## 一、改动总览

新增 1 个规则基座包、1 个研判引擎包、1 个案例库包；扩展故障模型；改造 MQTT 故障落库通道（新增研判分流）；新增 8 组 REST 接口与 3 个权限码；仅新增不影响既有契约。

| 包/文件 | 类型 | 说明 |
|---|---|---|
| `internal/faultcode/faultcode.go` | 新增 | 错误码常量 + 故障类型/等级分类规则基座（从 mqtt 抽出） |
| `internal/recognition/engine.go` | 新增 | 多源研判引擎（规则基座→交叉验证→置信度→三层分流） |
| `internal/caselib/caselib.go` | 新增 | 案例库（沉淀/去重/训练骨架/识别统计/规则打分） |
| `internal/model/fault.go` | 修改 | `FaultRecord` 增加研判可选字段；新增识别常量与 `FaultRecognition` |
| `internal/model/fault_evidence.go` | 新增 | 多源证据表 `fault_evidence` |
| `internal/model/fault_case.go` | 新增 | 案例库表 `fault_case` |
| `internal/model/migrate.go` | 修改 | AutoMigrate 注册两张新表 |
| `internal/model/rbac.go` | 修改 | 新增 3 个权限码并纳入运维角色 |
| `internal/mqtt/handler.go` | 修改 | `processFault` 接入研判引擎并按状态分流；新增注入/落库/案例辅助函数 |
| `internal/mqtt/commands.go` | 修改 | 常量与分类函数转发到 `faultcode`（语义不变） |
| `internal/handler/recognition.go` | 新增 | 研判/证据/案例/统计/复核 REST 处理器 |
| `internal/handler/fault.go` | 修改 | `faultView*` 附带研判可选字段（缺省兼容） |
| `cmd/server/main.go` | 修改 | 注册 8 组新路由（带权限保护） |

---

## 二、核心设计：研判引擎（S1–S4）

`recognition.NewEvaluator(hwID, errCode, ledState, curR/Y/G)` 构建一次研判上下文：

1. **第 1 层 确定性规则基座**：复用既有 `faultcode.FaultTypeFromErrCode / FaultLevelFromErrCode`（R9 语义不变），`errCodeBaseConf` 给出每码基础置信度（0.92–0.98）。
2. **第 2 层 多源交叉验证**：`injectAuxEvidence` 在近 24h 窗口检索群众反映/手机举证/视频监控/设备媒体并注入；`crossValidate` 用电流与 LED 灯态做印证/否证（`currentCorroborates/Refutes`、`ledCorroborates`）。
3. **第 3 层 置信度融合与三层分流**：
   - `ConfHigh=0.90`：**confirmed**（确认，可自动派单）
   - `ConfLow=0.50`：**filtered**（误报过滤，只记证据/案例，不产生故障）
   - 中间区间 / 未知错误码：**pending_review**（待确认，不自动派单，可复核升级后派单）
   - **设计要点**：电流等证据仅做「明确否证→大幅降级」，绝不因矛盾直接被丢弃（宁可待确认，不漏真故障）。

---

## 三、数据库

- `fault_records` 新增可选列（nullable，旧数据零影响）：`confidence`、`recognition_source`、`recognition_status`、`is_false_positive`、`evidence_count`、`last_evaluation_id`、`reviewed_at`。
- 新表 `fault_evidence`（多源证据，含 evaluation_id/fault_id、来源类型、电流/灯态/错误码快照、附件/反馈引用、置信度）。
- 新表 `fault_case`（案例库：设备、输入特征签名、故障类型/等级、真值与判定、是否正确、来源研判批次、状态）。
- `AutoMigrate` 已注册两张新表；纯增量，无数据迁移风险。

---

## 四、MQTT 故障通道改造（S5）

`processFault` 改为「研判→分流」：

- **filtered**：`persistEvidence(eval,nil,…)` + `persistCase` 后直接返回（不落故障/工单）。
- **pending_review**：创建故障记录（status=occurred、recognition_status=pending_review），**不自动派单**；等待证据补充复核升级。
- **confirmed**：原逻辑（30 分钟去重窗口、critical 自动派单、`EnsureActiveWorkOrder` 原子防重）。
- 新增三个辅助函数：`injectAuxEvidence`（检索注入辅助证据）、`persistEvidence`（主信号+辅助证据归一化落库）、`persistCase`（案例沉淀）。

**红线保持验证**：
- R2/R3 30 分钟去重窗口原样保留，测试 `TestProcessFault_DedupStillWorks_R3`。
- R6 仅 confirmed 状态才自动派单，测试 `TestProcessFault_CriticalConfirmedAutoWorkOrder`。
- 工作流状态机（occurred→confirmed→dispatched→resolved）未改。
- R9 AI 回退、故障类型/等级判定语义经 `faultcode` 转发完全不变。

---

## 五、REST 接口（S6）

独立路径、向后兼容，前端零改动：

| 方法 | 路径 | 权限 | 说明 |
|---|---|---|---|
| GET | `/api/v1/faults/:id/evidence` | 登录 | 故障多源证据明细（含被过滤批次可回看） |
| POST | `/api/v1/evidence/ingest` | `evidence:ingest` | 预留外部数据源证据写入 |
| GET | `/api/v1/evidence/sources` | 登录 | 证据来源枚举 |
| GET | `/api/v1/fault-cases` | 登录 | 案例库检索/列表 |
| POST | `/api/v1/fault-cases` | `faultcase:manage` | 案例人工回标 |
| POST | `/api/v1/fault-cases/train` | `faultcase:manage` | 触发案例库训练（骨架） |
| GET | `/api/v1/recognition/stats` | 登录 | 识别正确率/误报/漏报/置信度 |
| POST | `/api/v1/faults/:id/review` | `fault:review` | 待确认复核：升级确认（critical 自动派单）或标记误报 |

新增权限码（只增不删）：`fault:review`、`faultcase:manage`、`evidence:ingest`，均已纳入 `BuiltinRoleOperator`。

---

## 六、测试

`go build ./...` ✅　`go vet ./...` ✅　`go test ./... -count=1` ✅（全部包通过）

新增测试：
- `internal/recognition/engine_test.go`：置信度计算、规则判定、多源印证提升、电流矛盾降级待确认、未知码宁待确认、一般故障保持确认、证据签名。
- `internal/caselib/caselib_test.go`：案例写库、同特征去重、误报过滤样本、训练骨架、识别统计、规则打分。
- `internal/mqtt/recognition_test.go`：研判后证据/案例落库、critical 自动工单、去重窗口保持、待确认不派单、复核升级。
- `internal/handler/recognition_test.go`：证据明细、证据注入（含非法来源/缺参校验）、来源枚举、案例增删查训统、复核确认/判误报。

---

## 七、风险与后续（P1/P2 保留）

- 真·视频监控/AI 视觉识别未在本范围实现，`injectAuxEvidence` 从已落库媒体/反馈记录检索注入（架构预留扩展点）。
- `TrainFaultCases` / `caselib.Train` 为规则打分骨架，向「100% 识别率达标」收敛，未接入外部 ML 训练框架。
- 复核自动派单复用 `EnsureActiveWorkOrder` 原子防重，保证并发下只建一条活跃工单。

---

## 10. 登录改+人事核心字段（重新修复）

> 处理：leader-tsloms ｜ 2026-08-17 ｜ 用户要求「不要手机短信验证码，改手机号作为账号 + 算术验证码」，并补齐维护人员人事字段。

### 登录改造（参考项目 a，非 SMS）
- **废弃短信验证码登录**：删除 `auth_sms.go`（SmsSender/console/devcode/phoneLogin/SendSmsCode）、`smscode.go` 模型、`/auth/sms-code` 路由、`SmsCodeTTL` 等配置。
- **新增算术验证码** `handler/captcha.go`：`GET /auth/captcha` 返回 `{uuid, question}`（如 “2 + 8 = ?”）；进程内存存储（uuid→答案/过期/校验次数）；登录带 `captcha_uuid` + `captcha_code`（答案）校验，一次性、过期、防暴力。
- **登录** `POST /auth/login`：`username`（用户名或手机号，手机号即登录账号）+ 密码 + 算术验证码；不再区分 login_type。手机号作为账号逻辑保留（`phone_login`）。
- 测试：`captcha_test.go`（Get 返回 uuid/question、错误答案拒绝、正确答案一次性通过）；旧登录用例 `TestAuth_LoginAndDisabled` 适配算术验证码。

### 人事核心字段（维护人员必要）
- `model/user.go` 新增：`work_no`(工号) / `avatar`(工作照头像) / `gender`(性别) / `id_card`(身份证号) / `address`(住址) / `education`(文化程度) / `engineer_level`(工程等级)。
- `CreateUser`：手机号**可选**，但若提供则校验 11 位格式（注册/建号即校验）；手机号自动作为 `phone_login`。
- `UpdateUser` / `UpdateMyProfile`：更新人事字段，手机号格式校验 + 同步 phone_login。
- `handler/avatar.go`：`POST /user/avatar`（工作照上传，5MB，jpg/png/webp 等，落盘 {mediaDir}/{yyyyMM}/avatar/）写回 avatar URL；`PUT /user/profile` 自助维护人事字段。
- `auth.go`：`userPayload()` 统一返回人事字段/头像/工号（Login 与 GetUserInfo 共用）。

### 前端
- `login/index.vue`：取消短信 Tab，改为单表单（账号可手机号 + 密码 + 算术题输入，点击刷新验证码）。
- `layout/index.vue`：右上角用户区改为头像（工作照）展示 + 下拉新增「个人资料」；`real_name` 优先显示。
- 新增 `views/settings/profile.vue`：上传工作照 + 编辑人事字段（姓名/手机号/工号/性别/身份证/住址/文化程度/工程等级/邮箱）。
- `api/auth.ts`：`getCaptcha` / `updateMyProfile` / `uploadMyAvatar`；`store/auth.ts`：UserInfo 补人事字段、login 接收 captcha 参数。
- `router/index.ts`：`/profile` 恒注册为 layout 子路由。

### 验证
- 后端 `go build`/`go vet` 全绿；`go test ./...`（12 包）全 ok（含新增 captcha 用例与适配后的登录用例）；gofmt 0 文件。
- 前端 `npm run build`（vue-tsc + vite）exit 0。
- 未改动红线：MQTT/识别引擎/工单状态机/去重/NextOrderNo/SLA/RBAC/既有 /faults*、/work-orders* 契约。

---

## 11. P1 自动巡检（patrol）

> 依据：pm-checklist.md P1 定义（自动巡检 + 地图增强）。本轮完成自动巡检后端；地图增强亦在 P1 范围见下。

### 后端自动巡检（dev 产出 + leader 核验）
- 新增表（AutoMigrate 加法迁移）：`patrol_tasks`、`patrol_records`（PatrolRankingItem 为查询视图，不落独立表）。
- model：`internal/model/patrol.go`：PatrolTask(name/mode area|street|random|selfcheck|ai、area_id、street_id、target_count、status、assignee_id)/PatrolRecord(task_id/device_id/crossing_id/patrol_type/check_result/check_detail/selfcheck_result(JSON)/lat/lng/patrol_by/patrol_at)/PatrolRankingItem。
- service：`internal/service/patrol_task.go` PatrolTaskService：
  - `selectTargetDevices`：area(按 district/province/city)、street(按 street_id)、random(洗牌取 target_count)、ai(仅 AI 高风险设备)、selfcheck(范围限定)。
  - `RunTask`：执行巡检→逐设备判定 normal/abnormal（有活跃故障或离线即 abnormal）→落 patrol_record → 记排行。
  - `collectSelfCheck`：信号灯自检快照（灯态/errCode/在线），`BuildRecordForSelfCheck` 判定。
  - `Ranking`：按巡检人次/异常数聚合（对齐参考项目 a inspectRanking）。
  - Task/Record CRUD、`CreateTask`/`UpdateTask`/`DeleteTask`/`ListTasks`/`ListRecords`。
- handler：`internal/handler/patrol.go`：
  - GET/POST/PUT/DELETE /patrol/tasks、GET /patrol/tasks/:id、POST /patrol/tasks/:id/run、GET /patrol/records、GET /patrol/ranking、POST /patrol/selfcheck。
  - RBAC：`patrol:manage`/`patrol:run`/`patrol:selfcheck` 仅追加（rbac.go AllPermissions + PermModulePatrol）。
- 启动接入：`cmd/server/main.go` 启动时 PatrolTaskService.Start 后台协程（与既有 AI 巡检协程隔离，独立 task 表）。
- 验证：`go build`/`go vet` 全绿；`go test ./...` 12 包全 ok（含 patrol 路由/CRUD/run/ranking/selfcheck 用例）；gofmt 0。

### 地图增强（P1 前端，留待下一步）
- 待做：三维演示、视频巡检入口、实时监控刷新入口（复用 Cesium 3D/VideoPanel/MonitorWall）；地图分级渐变着色前端（基于已部署 /map/crossing-data 的 fault_ratio/green_ratio/level 在 CesiumMap 上 绿→黄→红 着色 + 路→路口→故障点下钻）。后端 `/map/crossing-data`、`/map/road-data` 已在 P0 就绪。

---

## 12. 角色体系改造：超级管理员 / 系统管理员降级 / 模块设置

> 处理：leader-tsloms ｜ 2026-08-17 ｜ 用户规则：超管可控“模块是否可用”，系统管理员降级仅维护系统运行，其它人员为信号灯维护者。

### 角色
- 新增 `super_admin`（超级管理员）：全部权限，含 `module:manage`（模块启用/停用设置）。
- 系统管理员 `admin` **降级**：去掉 `module:manage`，保留系统运行维护与业务管理（不含模块设置）。
- `operator`（运维/信号灯维护者）、`viewer`（查看人员）不变。

### 超级管理员账号
- 内置账号 `419116` / 密码 `Osgis!!!`（**bcrypt 加密入库**，非明文），由 `SeedSuperAdmin` 幂等创建（仅当不存在时），角色 super_admin。
- 登录入口**可正常登录**（设计 A）；“不对外开放”= 模块设置能力不向普通用户开放。
- `SeedAdmin` 改为“admin 不存在才创建”，与超级管理员种子解耦。

### 模块设置（DB 持久化）
- 新增 `module_toggles` 表：可选模块运行时启用/停用（DB 优先于 env 默认）。
- `GET/PUT /modules/settings`（需 `module:manage`，仅超级管理员）：查看/开关可选模块；核心模块恒启不可关。
- `ModuleEnabled`/`EnabledModuleList` 合并 env 默认 + DB 开关。

### 测试
- `p0_superadmin_test.go`：SeedSuperAdmin 幂等创建、密码 bcrypt 可校验、super_admin 含 admin 不含 module:manage。
- 适配：`TestSeedRBAC` 角色数 3→4、`TestCreateUser_AndList` 用户数 1→2、`SeedAdmin` 语义修正。
- 已知遗留：`internal/ai` 的 `TestNlRequirePermNoDB` 存在包内测试顺序依赖（单跑 PASS，全量可能因先序测试设 DB 而断言全局 nil 失败），与本次改动无关（未触碰 internal/ai），需单独排期。

### 验证
- `go build`/`go vet` 全绿；`go test ./internal/model|handler` 全 ok；gofmt 0。

---

## 13. 授权/试用/防破解系统 + P1 地图渐变增强

> 处理：leader-tsloms ｜ 2026-08-17 ｜ 用户需求：超管控制模块/功能/产品价值，保密难破解，核心100天/可选30天试用，到期须超管授权解锁。

### 授权系统
- **internal/license/**：Ed25519 签名验签（供应方私钥离线签名、服务器仅存公钥验签）；TrialDaysCore=100、TrialDaysOptional=30；ParseUnlockCode/VerifyUnlockCode 校验 nbf/exp/签名/模块匹配/篡改。
- **internal/model/license.go**：license_state 单行表（核心激活、模块激活 map、解锁、时间回拨 last_check）。
- **internal/handler/license.go**：GET /license/status、POST /license/trial/start、POST /license/unlock（超管一键/授权码验签）——仅 module:manage（超管）。
- **cmd/licensegen**：供应方离线授权码生成工具（持私钥，go run ./cmd/licensegen -module ai -days 365）。
- **模块授权拦截**：ModuleEnabled 叠加 moduleLicenseOK——核心需核心试用/解锁，可选需该模块试用/解锁；**首次访问惰性自动开试用**；RequireModule 对超管恒放行（避免管理页死锁）；时间回拨超 1 分钟判篡改锁定。
- **前端**：/settings/license 授权管理页（状态/开始试用/一键解锁/输入授权码），仅超管可见。
- 测试：internal/license/license_test.go（验签/过期/未生效/模块不匹配/篡改）；module 测试适配惰性试用。

### P1 地图渐变增强（前端）
- CesiumMap 新增「路口分级」图层：/map/crossing-data 故障比例 → 路口彩色圆环 绿→黄→红 渐变着色（gradientColor），标注路口名；随图层开关重绘；getCrossingMapData/getRoadMapData API。
- 既有 3D/2D 场景、信号灯/故障/锁定图层、视频监控按钮已具备，本次补上分级着色核心。
- 前端 build（vue-tsc+vite）通过。

### 验证
- 后端 go test ./... 全绿（14 包含 license）；gofmt 0；前端 build exit 0。
- 待：CI + 部署 + 超管授权验收（419116 登录→开始试用→一键解锁/授权码解锁；可选模块到期锁定）。
