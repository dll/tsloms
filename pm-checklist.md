# TSLOMS —— 第二轮新需求核对清单（pm-checklist）

> 产出专员：**pm-tsloms**（只读核对，禁止修改任何源码/文件）
> 日期：2026-08-17 ｜ 性质：**第二轮新需求**规模核对 + 「参考项目 a」深度对比
> 说明：本文件仅做需求核对、现状诊断、差距对比、范围划分、数据/接口设计与优先级排序。**未改动任何代码与文件。**
> 旧版（第一轮：智能多源故障识别研判引擎）已另存为 `pm-checklist-a.md` 保留。

---

## 〇、本轮新需求一句话（用户原话要点，逐条覆盖核对）

1. **登录认证体系改造**：手机号为用户账号，登录验证码。
2. **预警管理**：设置、级别、显示、预警等。
3. **路口属性**：行政区划（街道、社区、道路）。
4. **路灯位置地图拾取经纬度**（地图上点击/搜索定位取经纬度落库）。
5. **地图增强**：缩放级别；路口、信号灯图标；预警；故障分级着色——全部正常→整个道路/路口绿；全部红灯→道路/路口红（停电/线路造成）；红绿中间状态按故障比例（故障/全部 或 绿灯/全部）渐变「由绿→变黄→渐变到红」；从路→路口→具体故障点（红色）；可缩放定位、三维演示、视频巡检、实时监控。
6. **自动巡检**：空间区域、街道排查、随机抽检、AI 硬件数据、信号灯自检。
7. **合并指令**：「a 项目的数据结构和实现，吸取全部，特别是预警、地图、自检，完善 tsloms 本项目。」

> 结论：本轮核心增量 = **①手机号+验证码登录**、**②预警管理（设置/级别/显示/记录）**、**③路口行政区划（涉街/社/路）**、**④设备/路口经纬度地图拾取**、**⑤地图分级渐变着色（路→路口→故障点）+三维/视频/实时**、**⑥自动巡检（空间区域/街道/随机抽检/AI硬件/信号灯自检）**。以上均以「充分吸收参考项目 a 的数据结构与实现」为基础。

---

## 一、参考项目 a 深度分析结论

### 1.1 项目 a 定位与技术栈
- **系统**：交通信号灯检测后台运维系统（RuoYi 框架风格，Spring Boot + Vue2 + Element UI + 高德 AMap + ECharts），前后端分离，接口前缀 `/prod-api`，JWT(HS512)+Cookie。
- **业务闭环**：`信号灯设备 → MQTT 上报 → 后端研判引擎（红灯全灭/超时/同亮/缺亮/断电等）→ 预警记录 signal_warning → 忽略/转工单 → 维修工单`。
- 用户账号即**手机号**（`userName = 13955832695`，phonenumber 同号）。

### 1.2 核心业务模型（表结构与关系）
```
Tenant(tenantId) 1─N Dept(deptId) 1─N User(userId)   ← RuoYi 标准
Area(areaId)     1─N Crossing(pointId) 1─N Equipment(uuid)
Crossing/Equipment 1─N Warning ；1─N WarningConfig
Dictionary(sys_dict_type + sys_dict_data) 驱动所有枚举
```

**① 路口表 signal_crossing**
| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint | 主键（48…） |
| pointId | varchar | 点位编码（point202404260413） |
| pointName | varchar | 路口名（长江中路与宿州路） |
| type | varchar | 路口类型（signal_cross_type：1直角/2卡口/3~8多路口） |
| longitude/latitude | varchar | 经纬度（GCJ-02 高德系） |
| areaId/areaName | varchar | 行政区划（340103/庐阳区） |
| status | varchar | 路口状态（1维护中/2监测中/3离线/4异常/5黄闪） |
| equipments | array | 关联设备（详情接口返回，1:N） |

**② 设备表 signal_equipment**：`id(UUID)、uuid(信号机地址码，如 1114004B/LA82533848)、areaId/areaName、pointId/pointName、batch(灯组1~8)、func(功能 1机动车/2倒计时机动…6闪光警告)、fx(方向 左转/直行/右转)、cx(朝向 由西向东…)、lon/lat、installDate、hearts(心跳数)、requestTime(最近上报)、versionValue(固件版本值，OTA)、lineStatus(1在线)、extraData(JSON 实时状态)、configUrl`。

**③ 预警表 signal_warning**（关键）
| 字段 | 说明 |
|---|---|
| id | UUID 主键 |
| pointId/pointName | 关联路口 |
| equipmentUuid | 关联设备 |
| content | **告警内容码**（signal_equipment_warning 字典：-1红全灭/-2黄全灭/-3绿全灭/-4红黄同亮/-5红绿同亮/-6黄绿同亮/-7红黄绿同亮/-8红超时/-9黄超时/-10绿超时/-11红缺亮/-12黄缺亮/-13绿缺亮/-14断电/0正常） |
| func | 信号灯功能方向（东向西直行/北侧左转） |
| dealState | 处理状态（1未处理/2已处理） |
| status | 工单状态（1未转/2已转） |
| contentLabel | 告警文案冗余字段 |
| createTime | 告警时间 |

> 实测 60,160 条 → 预警流水存储量级大，建议按时间/租户分区索引。

**④ 预警配置表 signal_warning_config**：`pointId(pointName)(all=全部)、equipmentUuid(设备)(all=全部)、content(忽略的告警码)、effectiveType(0永久/时间范围)、startTime/endTime、status(1生效)`。→ 用于**自动忽略**规则。

**⑤ 行政区 areas**：`id(区划码340102)、name、parentId(340100合肥)、areaType(3区县)、fullName(安徽省合肥市瑶海区)`。覆盖合肥市 15 区县。
> 说明：a 的行政区止于「区县」一级；**街道/社区/道路**为 TSLOMS 新增点，需在 a 的 areas 结构上扩级。

**⑥ 设备实时状态（extraData JSON）**：`siteState、lightState(辅灯 0红/1黄/2绿/-1未知)、upType、upTime、errorMsg(0=正常/-1=故障码)、greenSec/yellowSec/redSec(各灯剩余秒，动画用)、inter(前端倒计时)`. ← **地图灯色/故障判断的数据来源**。

### 1.3 地图与预警可视化实现思路（a 前端 mapScreen chunk 还原）
- 技术：高德 AMap；`zoom=14, center=[117.227234,31.820595], zooms=[7,20]`；viewMode 2D，pitch=10。
- **路口 Marker 图标随 `status` 映射**：`statusImgMap: {1:绿图标, 2:图标, 3/4:图标, 5:图标}`（红灯异常/黄闪/离线用不同图标）。
- 点击 Marker → 拉 `crossing/{id}` 详情 → 弹出 infoView：`行政区域/路口名称/路口状态(dict-tag)/路口类型` + 两个 Tab「设备列表(含在线状态)」+「预警列表(告警内容+处理状态+工单状态+时间)」。
- **灯色动画**（showRed/Green/YellowOpacity）：根据 `extraVO.lightState` 与 redSec/yellowSec/greenSec 每秒倒计时切换亮灯，`func=6(闪光警告)`、`lineStatus=2(离线)` 置灰度/半透明。
- 位置搜索：`AMap.PlaceSearch`（全国）→ 结果列表 → `setZoomAndCenter(16,[lng,lat])` 定位。
- 请求数据源：`/signal/crossingMap/getMapData`（全部路口坐标 + status，一次拉全量，轻量）。

### 1.4 巡检逻辑（a 的路由全集 + 统计接口还原）
- **外场巡检**：`/inspect/out/plan`（外场巡检计划）、`/inspect/out/result`（外场巡检结果）。
- **内场巡检**：机房/旋转/分控中心/公安网硬件/公安网软件 五类，各有 result 页。
- **巡检排行**：`/statistics/home/inspectRanking`（胡伟 29 次、崔建伟 8 次…）、故障排行 `/faultRanking`。
- 首页统计：`/statistics/home/baseData`（设备数/故障数/巡检数）、`/sevenData`（近 7 日趋势）。
- **本账号可见具体巡检业务实现细节较少**（该账号仅 SIGNAL_ADMIN 角色，未看到「空间区域/街道排查/随机抽检/AI 硬件/信号灯自检」页面源码），但这部分需求本质是 TSLOMS 的 **自检（自动巡检）能力增强**，a 的「内场/外场巡检 + 排行」可作为基础参照。

### 1.5 可吸收/借鉴到 TSLOMS 的合理之处（List）
1. **预警码字典体系**（-1~-14 + 0/all）：一套**稳定、可翻译、可作为过滤器**的告警内容编码，直接与识别引擎 errCode 对应。← 高价值
2. **预警 - 转工单 - 忽略闭环**：预警记录 dealState/status 双状态机 + flowWorkOrder 转单 + 单条/批量忽略 + 导出。TSLOMS 目前 fault→workorder 方向相反但可对齐。
3. **预警配置（忽略规则）**：按路口/设备/告警码/生效时段 配置自动忽略。TSLOMS **完全没有**，需新增。
4. **路口与设备多级模型**：pointId + equipment(uuid) 的 1:N，且设备带相位(func/fx/cx)、区域、经纬度、灯态实时数据(extraData JSON)。TSLOMS device 目前只有 intersection 名称字符串 + lat/lng + 状态，**缺相位/灯态/区域**维度。
5. **地图全量轻接口 `/crossingMap/getMapData`**：返回所有路口坐标+状态一次到位，前端本地按 status 映射图标，避免逐设备轮询。TSLOMS 地图目前打点是逐 device。
6. **行政区划树（areas）**：通用区划表 + 字典驱动，可扩展街道/社区/道路子级。
7. **统一响应/租户隔离/软删除 + 字典系统**：TSLOMS 已有弱化版，可作为规范化参考。（不强求全盘对齐）

---

## 二、TSLOMS 现状 vs 目标差距对照表（逐需求点）

| # | 需求点 | TSLOMS 现状 | 目标（本需求） | 差距判定 |
|---|---|---|---|---|
| 1 | 手机号=账号 + 验证码登录 | `username/password` + JWT；`User.Phone` 已有字段但仅资料用；前端 `store/auth.ts useAuthStore.login(username,password)` | 手机号作为登录账号；短信/图形验证码登录；向后兼容旧账号 | **大改（P0）**：认证流程 + 登录视图 + token 语义 |
| 2 | 预警管理（设置/级别/显示/预警） | 有 `fault.ts`/FaultRecord 生命周期（occurred→resolved）+ 识别引擎；**无独立"预警(设置/级别/显示)"模块**，无忽略规则 | 新增：预警配置(signal_warning_config 式忽略规则)、预警记录列表/级别/显示、转工单、忽略、导出 | **新增（P0）** |
| 3 | 路口行政区划（街道/社区/道路） | 无（device.intersection 仅名称字符串；无 area 概念） | 路口/设备带 行政区→街道→社区→道路 层级 | **新增（P0，数据模型）** |
| 4 | 路灯位置地图拾取经纬度 | 有 `SetIntersectionLocation`（传 lat/lng），**无地图点击选取 UI** | 地图上点击/搜索定位取点回填经纬度落库（路口/设备） | **新增（P1）**，后端接口基本可复用 |
| 5 | 地图分级/渐变着色 | Cesium 打点：per-device 图标(在线/离线/故障/锁定)，有 2D/3D/底图切层、信号灯/故障/锁定图层、全览 | 路口/道路级渐进着色：全绿→按故障比例(故障/全部 或 绿灯/全部)渐变 绿→黄→红；全部红=停电/线路；点入 路口→具体故障点(红)；三维演示/视频巡检/实时监控 | **大改（P0）**：a 只到 status 图标，**比例渐变为 TSLOMS 专属增强** |
| 6 | 自动巡检（区域/街道/随机抽检/AI硬件/信号灯自检） | 有 `PatrolService`（AI 日报+异常 alert，每日 8:00，后台协程）+ offline 检查 + workorder escalate；**无"空间区域/街道排查/随机抽检/信号灯自检"任务编排** | 新增：巡检任务（区域/街道/随机抽检）、自检采集、巡检记录/排行 | **新增（P1）** |
| 7 | 三维演示/视频巡检/实时监控 | 有 Cesium 3D(SceneMode)、底图、`VideoPanel.vue`(信息亭/视频)、`MonitorWall.vue`(监控墙)、实时 MQTT 状态 | 地图内嵌三维演示、视频巡检入口、实时监控刷新 | **增强（P2）**，复用现有组件为主 |

### 关键差距定性
- **认证**：不是"表单+校验"级改动，而是**登录语义与账号体系**变更——`username` 语义改为手机号，或新增独立 `phone_login` 通道。红线：不得破坏现有 users 表既有账号与 dispatch/workorder 的 owner 关联。
- **预警 vs 故障**：现状 FaultRecord 是"故障识别-派单"链路产物；需求的"预警"更接近 a 的 `signal_warning`（设备上报异常→预警记录→忽略/转单）。**二者应并存**（故障=已确认需处置；预警=前置/轻量通知），需明确边界，防止重复建单。
- **地图**：现状是**设备级**点图；需求是**路口/道路级聚合语义**（路→路口→故障点的分级下钻 + 故障比例绿黄红渐变）。需在聚合层新增"路口状态聚合计算"，Cesium 打点逻辑要重构为「道路/路口聚合 + 下钻」。

---

## 三、功能范围界定（本次实现 / 复用现有 / 明确不做）

### P0 —— 本次必须实现（最小可用闭环）
1. 认证：手机号登录（手机号=登录账号，优先）+ 验证码（短信或本地图形验证码/一次性 code，至少提供一个可落地方式）+ **保留 username/password 向后兼容**。
2. 预警管理：
   - 预警记录表 + 列表查询（路口/设备/级别/告警内容/处理状态/时间过滤）+ 详情；
   - 预警级别（级别字典）；
   - **预警配置（忽略规则）**：按路口/设备/告警类型/生效时段 → 自动忽略或降级；
   - 预警→转工单、单条/批量忽略、导出。
3. 路口行政区划数据模型（区→街道→社区→道路）+ 路口/设备挂接区划字段。
4. 地图分级渐变着色（第一版）：
   - 路口状态聚合 = f(该路口全部设备灯态/故障码比例)；
   - 全部正常→绿；全部红/断电→红；中间按 **故障/全部 或 绿灯/全部** 比例 绿→黄→红 渐变；
   - 道路级聚合（由路口聚合再上卷）→ 整条路一段一色；
   - 下钻：道路→路口→具体故障设备点（红）。
5. 位置地图拾取经纬度：地图点击/搜索取点回填到 新增路口/编辑路口/编辑设备 表单，落库。

### P1 —— 建议本次（随 P0 收尾后补充）
6. 自动巡检模块：
   - 巡检任务（空间区域/街道/随机抽检 三种模式）；
   - 信号灯自检数据采集与判定（对接 extraData/灯态/识别引擎）；
   - AI 硬件数据接入（复用 ai 包 anomaly/predict 能力）；
   - 巡检记录 + 巡检排行（参考 a `/inspectRanking`）。
7. 地图三维演示 + 视频巡检入口 + 实时监控刷新（复用 Cesium 3D、VideoPanel、MonitorWall，把入口并到地图 tab）。

### P2 —— 可后续（本轮明确不做）
8. a 项目全量功能迁移（如内场五类巡检、仓库物资、绩效、项目维护审计、未结算物资、交通工程同步等）——**不在本轮**。
9. 多租户(tenantId)/软删除(delFlag)全盘规范化——视需要，TSLOMS 已弱化可暂不引入。
10. 高德 PlaceSearch 全国地点搜索的完整 UI 增强（首版地图拾取以点击取点为核心，搜索定位可后置）。
11. 预警短信/App 推送通道（本轮仅落库+站内通知，不接真实短信网关）。

---

## 四、数据模型建议（新增/变更，命名与关系）

> 命名沿用 a 风格 + TSLOMS 现有 snake_case。全部**只做加法**，不为既有表破坏兼容。

### 4.1 用户认证相关
- **变更 `users`（User）**：
  - `username`：现有账号（保留，历史兼容）；新增 **`phone_login`（uniqueIndex, size:20）** 作为手机号登录主键，或直接规定 `username=手机号`（推荐后者以对齐 a，但需迁移数据）。**建议：新增 `PhoneVerified bool`、`PhoneToken` 一次性验证码/过期时间（或独立表，见下）。**
- **新增 `email_sms_codes`（短信/一次性验证码表）**：`phone(size:20)、code(size:8)、biz(auth/other)、expires_at、used(bool)、created_at`。首版可用于本地模拟短信。

### 4.2 行政区划（街道/社区/道路）
- **新增 `areas`（复用 a 结构 + 扩展子级）**：`id(size:32 区划码/ID)、name、parent_id、area_type(省/市/区县/街道/社区/道路/路口)、full_name、area_sort、sn`。挂 `users`(管辖中心)可选。
- **变更 `devices`（Device）**：新增 `area_id、street_id、community_id、road_name(size:64)`；已有 `intersection`(路口名)、`lat/lng`。
- **新增 `crossings`（路口表，独立建模）**——现状路口仅由 handler 聚合 devices 得出（无表）。建议**新增路口表**，否则"道路级聚合/路口属性/区划"难以稳定承载：
  - `id、point_no(点位编码)、name、type(路口类型字典)、area_id、street_id、community_id、road_name、lat、lng、status(聚合状态，落库缓存)、remark、created_at、updated_at`。
  - 与 `devices` 关系：`devices.crossing_id → crossings.id`（一对多）。

### 4.3 预警相关（核心新增）
- **新增 `warnings`（预警记录表，对齐 a signal_warning）**：
  - `id(UUID 或 bigint)、device_id、crossing_id、equipment_uuid(冗余设备码)、warning_code(int，-14~-1 对齐 errCode 字典)、warning_label、level(级别 spring 城市/轻量分级)、func(相位/功能方向)、deal_state(1未处理/2已处理)、work_order_id(可空，转单后关联)、status(1未转/2已转)、source(识别引擎/MQTT/自检/手动)、occurred_at、resolved_at、create_time`。
  - 索引：`(crossing_id, deal_state)`、`(warning_code)`、`(occurred_at desc)`。
- **新增 `warning_rules`（预警配置/忽略规则，对齐 a signal_warning_config）**：
  - `id、crossing_id(可空=all)、device_id(可空=all)、warning_code(可空=all)、level(可空)、effective_type(0永久/1时段)、start_time、end_time、action(ignore 忽略/downgrade 降级)、enabled(bool)、remark、created_at`。
- **字典新增**：`warning_level`(级别)、`warning_deal_state`、复用识别引擎 `errCode`(-1~-14) 作 `warning_code` 翻译来源。

### 4.4 巡检相关（新增）
- **新增 `patrol_tasks`**：`id、name、mode(area 空间区域/street 街道/random 随机抽检/selfcheck 信号灯自检/ai 硬件)、area_id、street_id、time_window、target_count、status(planned/running/done)、assignee_id、created_at`。
- **新增 `patrol_records`**：`id、task_id、device_id、crossing_id、patrol_type、check_result(normal/abnormal)、check_detail、selfcheck_result(JSON 灯态/自检码)、evidences(JSON)、patrol_by(巡检人)、patrol_at、lat、lng`。
- **新增 `patrol_ranking`（或查询视图）**：聚合巡检人次/异常数（对齐 a inspectRanking）。

### 4.5 地图分级聚合
- **路口状态聚合**计算可**实时查询**（不落库）或**缓存于 crossings.status**。建议首版实时聚合（路口设备数少），在 `crossings` 预留 `status`、`fault_ratio`、`green_ratio` 冗余字段便于列表/地图一次拉取（对齐 a `/getMapData` 轻量全量接口）。

### 4.6 关系总览
```
User 1─N PatrolTask 1─N PatrolRecord
Area(省→市→区→街道→社区→道路) 1─N Crossing 1─N Device
Crossing/Device 1─N Warning ；（Warning 可 1─1 WorkOrder）
Crossing/Device 1─N WarningRule（忽略规则）
User 1─1 VerifiedPhone（或 users.phone_login）
```

---

## 五、接口设计建议（新增/变更 REST，向后兼容策略）

### 5.1 认证
- 变更 `POST /auth/login`：请求体扩展为 `{login: 手机号|用户名, password?, code?, login_type: pwd|sms}`。
  - 兼容：`login_type=pwd` 时按现有 username/password 逻辑（`login` 兼容 username 与 phone_login）；
  - 新增 `login_type=sms`：`{phone, code}` 校验 `sms_codes` 表。
- 新增 `POST /auth/sms-code`（或 `/auth/code`）：请求下发（首版本地日志模拟）。
- 新增 `POST /auth/verify-phone`、`PUT /user/phone-verified`（绑定/校验）。
- **兼容策略**：旧 token/旧登录路径保持可用；`GetUserInfo` 返回体新增 `phone_login`、`phone_verified` 字段，前端做**可选**解析（不破坏现有 store）。
- 前端 `api/auth.ts` / `store/auth.ts`：login 改为多态入参；旧调用(login(username,password))保留可走 pwd 分支。

### 5.2 预警
- 新增 `GET /warnings`（分页 + 过滤：crossing/device/level/code/deal_state/status/时间范围）——对齐 a `/signal/warning/list`。
- 新增 `POST /warnings/{id}/ignore`、`POST /warnings/batch-ignore`、`GET /warnings/export`。
- 新增 `POST /warnings/{id}/to-workorder`（转工单，body 带 remark，创建 WorkOrder 并回填 work_order_id、status=已转）——复用现有 `workorder.go` 创建逻辑。
- 新增 `GET/POST/PUT/DELETE /warning-rules`（预警配置/忽略规则 CRUD）——对齐 a `/warning/config/*`。

### 5.3 路口 / 区划
- 新增 `GET/POST/PUT/DELETE /crossings`（路口 CRUD，含经纬度、区划、type）——对象化，取代 handler 内聚合。
- 新增 `GET /areas/tree?level=`（区→街道→社区→道路 树）。
- 变更 `PUT /devices/{id}`：支持更新 `crossing_id、area_id/street_id/community_id/road_name、lat/lng`（地图拾取回填）。
- 复用/扩展现有 `PUT /intersections/location` 或由 `/crossings/{id}/location` 承载经纬度拾取。

### 5.4 地图
- 新增 `GET /map/crossing-data`（全量：路口 id/name/lat/lng/status/fault_ratio/green_ratio/level 一次到位）——对齐 a `/crossingMap/getMapData`，轻量，供 Cesium 聚合着色。
- 可新增 `GET /map/road-data`（道路级聚合）若实现"道路一段一色"。
- 复用现有 `GET /devices`（全量带经纬度，供下钻到设备点）。

### 5.5 自动巡检
- 新增 `GET/POST /patrol/tasks`、`POST /patrol/tasks/{id}/run`（触发）、`GET /patrol/records`、`GET /patrol/ranking`、`POST /patrol/selfcheck`（信号灯自检手动触发一组设备）。
- 后台协程可复用 `PatrolService` 骨架扩展任务调度。

### 5.6 红线：向后兼容
- **所有新增接口独立命名空间**（如 `/warnings`、`/crossings`、`/patrol/*`、`/auth/sms-code`），不与现有 `/faults`、`/devices`、`/auth/login` 冲突。
- 认证变更只**增加** `login_type` 分支与可选返回字段，不删改 `login` 语义与 token 结构。
- 现有前端 fault.ts 解析**保持不变**；新增预警走独立页面。

---

## 六、红线清单（不得破坏既有行为）

1. **MQTT 链路**：设备心跳/状态上报/解析（`mqtt/client.go、parser.go、handler.go`）与识别（`recognition/engine.go`）**不动逻辑**；自检/预警仅**消费其产物**。
2. **识别引擎 + case 库**：`ai/*`、`recognition/*`、`caselib/*` 已稳定且含大量测试，**禁止重构语义**；预警/自检只读取 `warning_code/errCode`。
3. **工单（WorkOrder）**：现有工单创建/派单/升级（`workorder.go`、`workorder_escalate.go`）不动；新增"预警转工单"走同一创建函数或最小扩展，保证 owner/repairer/dispatch 关联不破坏。
4. **RBAC / 模块化**：模块注册（`registerModuleRoutes`、`RequireModule`、enabledModules）不破坏；新增页面若加路由，按现有模块注册机制追加，并确认核心模块恒启。
5. **用户表/角色**：`users.username`、`devices`、`faults` 既有列**不删除不不可变**；新增字段全部可空带默认（只加不改）。
6. **地图 Cesium 初始化/底图（Baidu/Gaode/OSM）**：切层与 2D/3D 逻辑保留，聚合着色以此为底座扩展，不改 Viewer 初始化与底图加载。
7. **部署**（`deploy/`）：新增环境变量（如验证码过期、模拟短信开关、巡检任务窗口）均需默认值；服务启动不因缺配置报错。
8. **既有测试**：本核对未改码；实施后必须保证现有 cov_* / regression / _test.go 全绿。
9. 禁止并行铺开：按 P0→P1 顺序串行，每步可回滚。

---

## 七、实施步骤、风险与优先级，及「最小可用路线」

### 7.1 推荐实施顺序（串行，可交付闭环）
1. **[P0-1] 数据模型先行**：新增 `warnings / warning_rules / crossings / areas / sms_codes / patrol_tasks / patrol_records` 表（migrate.go 加法迁移），`users` 加 `phone_login/phone_verified`。可与功能解耦先行落地。
2. **[P0-2] 认证改造**：`POST /auth/login` 多态 + `POST /auth/sms-code`；后端测通手机号+验证码与旧 password 双通道。
3. **[P0-3] 预警管理**：预警记录列表/详情 + 忽略/批量忽略 + 转工单 + 导出 + **预警配置（忽略规则）CRUD**。
4. **[P0-4] 路口/区划**：路口表对象化 CRUD + 区划树接口 + device 挂接区划。
5. **[P0-5] 地图分级着色**：`/map/crossing-data` 基于 P0-4 + 设备灯态算 `fault_ratio/green_ratio` → Cesium 路口/道路按比例 绿→黄→红 渐变，支持 道路→路口→故障点 下钻。
6. **[P0-6] 位置地图拾取**：路口/设备表单内嵌地图点击取点（复用 CesiumMap 交互），回填 lat/lng。
7. **[P1-7] 自动巡检**：PatrolService 扩展任务（区域/街道/随机抽检/信号灯自检/AI 硬件），巡检记录 + 排行。
8. **[P1-8] 地图三维/视频/实时增强**：三维演示入口 + VideoPanel/MonitorWall 接入 + 实时监控刷新。
9. **[P2——不做]** a 全量非核心模块迁移。

### 7.2 风险清单
| 风险 | 等级 | 缓解 |
|---|---|---|
| 认证改造破坏既有登录/工单指派人关联 | 高 | 保留 username/password 分支，phone_login 只做加法 |
| 预警与故障生命周期重叠致重复建单 | 高 | 明确边界：故障=识别确认已派单；预警=轻量记录+忽略规则；互斥来源字段 |
| 地图聚合算法/比例定义分歧（故障/全部 vs 绿灯/全部） | 中 | 需求已允许两种口径，做成可配常数；默认"故障/全部" |
| 路口无表、现由 handler 聚合 device，重构影响接口 | 中 | 新增 crossings 表并做**数据回填**，接口保持兼容（新增而非改动老聚合） |
| 短信验证码无真实网关 | 中 | 环境变量开关：真实网关 / 本地模拟（日志输出 code），保证可演示 |
| 巡检任务与现有 AI 日报协程并发冲突 | 低 | 独立调度器/独立 task 表，不抢 PatrolService 已有协程 |
| 大表 warnings 性能（a 有 6 万+） | 中 | 分页+索引+必要的时间分区 |

### 7.3 优先级汇总
| 优先级 | 内容 |
|---|---|
| **P0 必须** | 手机号+验证码登录（双通道兼容）；预警管理（记录/级别/忽略规则/转工单/导出）；路口行政区划数据模型；地图分级绿黄红渐变+下钻；位置地图拾取 |
| **P1 建议** | 自动巡检（区域/街道/随机/自检/AI硬件+排行）；三维/视频/实时监控进地图 |
| **P2 后续** | a 全量模块迁移、多租户/软删除规范化、短信/App 真实推送、PlaceSearch 完整搜索 |

### 7.4 最小可用路线（避免一次性铺太开）
> **建议最小闭环 = 仅完成 P0 的 4 件事**即可端到端验收：
> ① 手机号 + 验证码登录（含旧密码兼容）
> ② 预警记录 + 预警忽略规则 + 转工单（打通 故障/预警 → 工单）
> ③ 路口表 + 区划（区/街/社区/路）+ 路口/设备经纬度地图拾取
> ④ 地图按"故障/全部"比例对路口/道路 绿→黄→红 渐变 + 道路→路口→故障点 下钻
>
> 自动巡检、三维/视频/实时 归入 P1 二期；a 其余做 P2 取舍。先跑通 ①② 打通数据→预警→工单主链，再上地图聚合与巡检，避免大而全导致无法验收。

---

*（全文完。本核对清单由 pm-tsloms 输出，供 dev-refactor 工程师按 P0→P1 实施，QA 按红线做回归。）*
