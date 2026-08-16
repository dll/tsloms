# TSLOMS 范围C 需求预研 —— 参考项目 `a/` 对标分析

> 产出：leader-tsloms｜日期：2026-08-17
> 目的：用户在范围B（智能故障识别引擎）基础上，要求"吸取参考项目 `a/`（aiitss.cn 信号灯运维系统）的合理之处"，重点：**预警、地图、自检**，以及手机号账号+验证码登录、路口行政区划、自动巡检等。本文档为范围C 的 pm 需求核对提供输入。

---

## 一、参考项目 `a/` 画像（摘自 a.md / aiitss-crawl-data.md / architecture-and-data.md）

- 技术栈：Vue2 + Element UI + ECharts + 高德地图；后端 Spring Boot（RuoYi 风格），前缀 `/prod-api`
- 核心业务闭环：设备 MQTT 上报 → 故障研判（红/黄/绿全灭、同亮、超时、缺亮、断电）→ 预警生成 → 转工单 → 维修
- 当前账号：SIGNAL_ADMIN（信号灯管理员）；登录账号即**手机号**（13955832695），图形验证码 + JWT

## 二、用户点名三大重点 —— 参考项目实现 → 我方需借鉴点

### 2.1 预警管理（earlyWarning + warningConfig）

参考项目实现：
- **预警列表** `GET /signal/warning/list`：路口、设备编码、功能方向、告警内容、处理状态(1未处理/2已处理)、状态、告警时间；操作：忽略（单/批）、转工单、导出
- **预警配置** `GET /signal/warning/config/list`：按「忽略路口/忽略设备/忽略预警内容 + 生效模式(永久/时间范围) + 启停」生成忽略规则，命中则不产生预警
- 数据结构：`signal_warning`（pointId/equipmentUuid/content/contentLabel/func/dealState/status/createTime）、`signal_warning_config`（pointId=all/equipmentUuid=all/content=-8/effectiveType/startTime/endTime/status）

**对标要点（需评估是否采纳）**：预警区分于故障——故障是"已确认要修"，预警是"需决策的告警事件"；预警可配置忽略规则（防止误报噪音）；预警→转工单留有备注；预警列表长时间窗口（参考项目 6 万条）。
→ 与本项目 recognition 引擎的 `pending_review`（待复核）正好衔接：**存疑/待确认的研判可先进入"预警"而非直接建故障**，由人忽略/确认/转工单。这是参考项目里"预警=决策缓冲层"的价值，值得吸收。

### 2.2 地图大屏（mapScreen）

参考项目实现：
- `GET /signal/crossingMap/getMapData`：返回全部路口点位（id/pointId/pointName/type/longitude/latitude/areaId/areaName/status），高德地图 Marker 按状态着色，左侧路口名称列表
- 地图层级：行政区(areas 树) → 路口(crossing) → 设备(equipment)

用户补充的增强需求（参考项目只有基础点位着色，**我方要做更高级的故障分级可视化**）：
- **缩放分级**：省/市 → 区/街道 → 道路 → 路口 → 具体故障点 → 三维/视频
- **故障按比例渐变着色**：整条道路/路口，全正常=绿、全故障(停电/线路)=红，中间按"故障灯数占该处灯总数比例"由绿渐变为黄→红
- 从粗到细：道路层看渐变 → 路口层看单点状态 → 故障点层定位到具体灯
- 可缩放定位、三维演示、视频巡检、实时监控（对接现有 map/CesiumMap.vue、VideoPanel.vue）

**对标要点**：参考项目提供"行政树×路口×设备 + 高德点位着色"基础；用户要求在此基础上加"故障比例渐变"与"多级下钻+三维/视频"。

### 2.3 自检 / 自动巡检（inspect）

参考项目实现（路由可见）：
- 内场巡检：机房/旋转/分控中心/公安网硬件/公安网软件（infield/*）
- 外场巡检：`/inspect/out/plan`（巡检计划）、`/inspect/out/result`（巡检结果）
- 巡检排行 `POST /statistics/home/inspectRanking`

用户要求的自动巡检方式：**空间区域、街道排查、随机抽检、AI 硬件数据、信号灯自检**。
→ "信号灯自检"：设备侧主动上报自检结果 / 服务器对设备发起自检指令（复用本项目 mqtt 的 CmdCheckFW/下发能力扩展命令）；"AI 硬件数据"：用内置 AI/规则对心跳、电流、版本做自检分析。

## 三、其它对标需求

### 3.1 手机号账号 + 验证码登录
- 参考项目：userName=手机号，图形验证码 + JWT
- 用户要求：手机号作账号；登录用**验证码**（短信验证码，非图形）→ 需评估本项目现有 auth（现为用户名+密码）改造为「手机号+短信验证码」或「手机号+密码+可选验证码」

### 3.2 路口属性扩展
- 参考项目 crossing：pointId/pointName/type/longitude/latitude/areaId/areaName/status
- 用户要求：行政区划**街道、社区、道路**层级；路灯位置**地图拾取经纬度**（前端地图点选回填）

### 3.3 行政区树（areas）
- 参考项目：areas 表（id=区划编码/name/parentId/fullName/areaType），合肥 15 区县，parentId=340100
- 用户要求：区划层级细化到 街道/社区/道路

## 四、范围C 建议模块划分（供 pm 核对细化）

1. **账号与登录**：手机号账号、短信验证码登录、现有 auth 兼容
2. **行政区划与路口属性**：areas 树扩展（区→街道→社区→道路）、crossing 增加区划字段、地图经纬度拾取
3. **预警管理**：预警视图/列表、预警配置（忽略规则/生效模式）、预警→确认/忽略/转工单，与 recognition.pending_review 衔接
4. **地图大屏**：高德/现有 Cesium 地图、路口点位着色、故障比例渐变着色、多级下钻(道路→路口→故障点)、缩放定位、三维演示、视频巡检、实时监控
5. **自动巡检**：巡检计划、空间区域/街道排查、随机抽检、AI 硬件数据自检、信号灯自检（复用 mqtt 下发）
6. （衔接）识别引擎：将 `pending_review` 分流与「预警」打通

## 五、风险与建议
- 范围C 体量大，建议按上述 6 模块拆分，优先落地**预警管理 + 地图故障分级着色 + 信号灯自检**（用户点名三大项）
- 地图底层：现有 `packages/admin/src/views/map/` 已有 Cesium/高德实现与 VideoPanel（监控）、可复用
- 登录改造需谨慎：影响既有账号，pm 需确认是否保留旧用户名/密码兼容
- 行政区数据需种子：可参考项目移植 areas 树（合肥15区县）结构
