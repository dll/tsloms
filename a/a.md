# 交通信号灯运维管理系统（aiitss.cn）网站设计报告

> 分析对象：https://www.aiitss.cn/（运维管理系统）
> 分析账号：13955832695（信号灯管理员 / SIGNAL_ADMIN）
> 分析日期：2026-08-16
> 数据来源：真实页面渲染 + 真实 API 响应
> 详细原始数据见：`aiitss-crawl-data.md`

---

## 一、系统概述

该系统为**交通信号灯检测后台运维系统**，面向城市道路信号灯的在线监测与故障运维管理。系统基于 MQTT 对接信号灯设备，实现设备状态实时上报、故障自动研判（红灯/黄灯/绿灯异常、同亮、超时、缺亮、断电等）、预警推送、维修工单流转与运维数据可视化统计。

| 项目 | 内容 |
|---|---|
| 系统名称 | 运维管理系统 |
| 版权 | Copyright © 2018-2022 znkj |
| 前端 | Vue2 + Element UI + ECharts + 高德地图 |
| 后端 | Spring Boot（RuoYi 框架风格），接口前缀 `/prod-api` |
| 认证 | JWT（HS512），Cookie 保存 `Admin-Token`，支持"记住密码" |
| 部署 | 前后端分离，静态资源 `/static/js/*.js` 动态 chunk 按需加载 |
| 当前用户 | 谢东亮，角色：信号灯管理员（SIGNAL_ADMIN），部门：安徽省通信产业服务有限公司 |

---

## 二、系统架构

### 2.1 总体架构图

```
+------------------+        +--------------------+        +------------------+
|   浏览器前端      |  HTTP  |      网关/入口       |  HTTP  |      后端服务     |
|  Vue2 + Element  | -----> |  www.aiitss.cn      | -----> |  /prod-api/*     |
|  UI + ECharts    |        |  /static/js(chunk)  |        |  Spring Boot     |
|  高德地图 AMap   |        +--------------------+        +--------+---------+
+------------------+                                           |
        ^                                                      | MQTT/HTTP
        |  WebSocket?/轮询                                     v
        |                                          +--------------------+
        |                                          |  信号灯设备        |
        +------(设备状态数据流)-------------------|  (心跳/状态/告警)   |
                                                   +--------------------+
```

### 2.2 分层设计

| 层次 | 技术/组件 | 说明 |
|---|---|---|
| 表现层 | Vue2 + Element UI | 单页应用（SPA），路由懒加载，动态 chunk |
| 数据可视化 | ECharts + 高德地图 | 首页统计图表、地图大屏点位渲染 |
| 接口层 | RESTful API（/prod-api） | 统一返回 `{code, msg, data/total/rows}` |
| 认证层 | JWT + Cookie | 登录签发 Token，接口请求携带鉴权 |
| 数据层 | MySQL（推断）+ Redis（推断） | 租户隔离字段 `tenantId`，软删除 `delFlag` |

### 2.3 前端工程结构（由路由映射推断）

```
src/
├── views/
│   ├── dashboard/          # 首页（统计卡片 + 图表）
│   ├── signalDevice/
│   │   ├── warn/
│   │   │   ├── config/index        # 预警配置
│   │   │   └── earlyWarning/index  # 预警管理
│   │   ├── device/
│   │   │   ├── device              # 设备管理
│   │   │   └── mapScreen           # 地图大屏
│   │   └── cross/crossCard         # 路口配置
│   └── system/             # 系统管理（用户/角色/字典/参数/档案）
├── router/                 # 路由（getRouters 动态注入）
└── api/                    # 接口封装
```

---

## 三、功能架构

### 3.1 功能菜单结构

```
运维管理系统
├── 首页 (/index)
│     ├── 基础点位统计（29,588）
│     ├── 本月故障数（414）
│     ├── 本月巡检数（82）
│     └── 图表：近7日数据 / 故障排行 / 巡检排行
├── 信号灯检测 (/signal)
│     ├── 预警配置 (warningConfig)
│     │     ├── 列表查询（路口/设备/预警内容/状态过滤）
│     │     ├── 新增预警配置（忽略路口/设备/预警、生效模式、生效时间）
│     │     └── 删除配置
│     ├── 预警管理 (earlyWarning)
│     │     ├── 列表查询（路口/设备/处理状态/告警内容/时间）
│     │     ├── 忽略（单条/批量）
│     │     ├── 转工单（填写备注 → 生成维修工单）
│     │     └── 导出
│     ├── 设备管理 (device)
│     │     ├── 设备列表（区域/路口/编码/备注/在线状态过滤）
│     │     ├── 新增/修改/删除设备
│     │     ├── 显示/隐藏列、选择位置
│     │     └── OTA 升级（设备固件远程升级）
│     ├── 路口配置 (cross)
│     │     ├── 路口列表（行政区/名称/类型/状态过滤）
│     │     └── 路口新增/修改/删除（含经纬度）
│     └── 地图大屏 (mapScreen)
│           └── 高德地图展示全部路口点位与实时状态
└── 个人中心 (/user/profile)
      ├── 个人信息展示（账号/姓名/部门/角色）
      ├── 基本资料修改（昵称/手机号/性别）
      └── 修改密码
```

### 3.2 核心业务流程

**信号灯故障预警与工单流转闭环：**

```
信号灯设备 ──MQTT上报──> 后端研判引擎（红灯全灭/同亮/超时/缺亮/断电）
        │
        v
   预警记录生成（signal_warning，状态: 待处理）
        │
        ├── 预警配置命中？ ── 否 ──> 忽略/不产生
        │
        v
   预警管理列表（管理员查看）
        │
        ├── 忽略（单条/批量） ──> 标记处理状态
        │
        └── 转工单（填写备注） ──> 维修工单（signal_warning/flowWorkOrder）
                                    │
                                    v
                              维修人员处理 ──> 工单状态更新
```

**OTA 升级流程：**

```
设备管理列表 ──选择设备──> OTA升级弹窗 ──下发配置──> 设备端升级 ──> 版本值更新（versionValue）
```

---

## 四、数据库/数据结构设计

系统采用**租户隔离 + 软删除**的设计规范，公共字段：

| 字段 | 说明 |
|---|---|
| id | 主键（UUID 或自增） |
| tenantId | 租户 ID（当前租户：061b3fcc817544119385947c19d04dec） |
| delFlag | 逻辑删除标记（1=正常） |
| createBy / createTime / updateBy / updateTime | 审计字段 |

### 4.1 路口表（signal_crossing）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint | 主键（如 48） |
| pointId | varchar | 点位编号（如 point202404260413） |
| pointName | varchar | 路口名称（如 长江中路与宿州路） |
| type | varchar | 路口类型（字典 signal_cross_type：1直角/2卡口/3~8多路口） |
| longitude / latitude | varchar | 经纬度（117.289022 / 31.861512） |
| areaId | varchar | 行政区编码（340103） |
| areaName | varchar | 行政区名称（庐阳区） |
| status | varchar | 路口状态（signal_crossing_status：1维护中/2监测中/3离线/4异常/5黄闪） |
| equipments | array | 关联设备列表（列表接口中为 null，地图接口含） |

### 4.2 设备表（signal_equipment）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | varchar | 主键 UUID |
| uuid | varchar | 设备唯一编码（如 1114004B / LA82533848） |
| areaId / areaName | varchar | 所属辖区 |
| pointId / pointName | varchar | 所属路口 |
| batch | varchar | 灯组（signal_equipment_batch：灯组一~八） |
| func | varchar | 功能（signal_equipment_func：机动车/倒计时/非机动车/人行横道等） |
| fx | varchar | 方向（signal_equipment_fx：左转/直行/右转/满屏/掉头等） |
| cx | varchar | 朝向（signal_equipment_cx：由西向东/由北向南等） |
| lon / lat | varchar | 经纬度 |
| installDate | date | 安装时间 |
| hearts | int | 心跳次数（实时在线心跳，如 120） |
| requestTime | datetime | 最近上报时间 |
| versionValue | varchar | 固件版本值（OTA 升级标识，如 275371777） |
| lineStatus | varchar | 在线状态（1=在线） |
| extraData | json | 扩展数据 |
| configUrl | varchar | 配置地址 |

### 4.3 预警表（signal_warning）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | varchar | 主键 UUID |
| pointId / pointName | varchar | 关联路口（如 长江东大街与东一环路） |
| equipmentUuid | varchar | 关联设备编码（如 11140075） |
| content | varchar | 告警内容编码（字典 signal_equipment_warning：-1红灯全灭/-5红绿同亮/-8~-10亮灯超时/-14断电等） |
| func | varchar | 信号灯功能方向（如 东向西直行、北侧左转） |
| dealState | varchar | 处理状态（signal_warning_deal_state：1未处理/2已处理） |
| status | varchar | 预警状态（signal_warning_status，当前账号无数据） |
| contentLabel | varchar | 告警内容文案（冗余字段） |
| createTime | datetime | 告警时间 |

**典型告警样例：**
```
红灯周期全灭  红灯亮灯超过设定时间  红绿同亮  黄绿同亮  黄灯周期全灭
```
（近 60,160 条预警记录，均为待处理/未转工单）

### 4.4 预警配置表（signal_warning_config）

| 字段 | 类型 | 说明 |
|---|---|---|
| id | bigint | 主键 |
| pointId / pointName | varchar | 忽略的路口（all=全部） |
| equipmentUuid | varchar | 忽略的设备（all=全部） |
| content | varchar | 忽略的预警内容（-8红灯超时等） |
| effectiveType | varchar | 生效模式（0=永久生效，另有"时间范围生效"） |
| startTime / endTime | datetime | 生效时间范围 |
| status | varchar | 是否生效（1=启用） |

### 4.5 用户/权限（RuoYi 标准结构）

- 用户表：userName（13955832695）、nickName（谢东亮）、phonenumber、sex、deptId、postId
- 部门：安徽省通信产业服务有限公司
- 角色：signal_admin 角色 ID 43fb8269511a4ceaaf8b8e66ad7e4b4d（信号灯管理员）
- 权限：permissions 为空（按角色鉴权，dataScope=1 数据范围）

### 4.6 行政区表（areas）

| 字段 | 说明 |
|---|---|
| id | 区划编码（340102 等） |
| name | 区名（瑶海区等） |
| parentId | 上级（340100 合肥市） |
| areaType | 3=区县级 |
| fullName | 全称（安徽省合肥市瑶海区） |

覆盖合肥市 15 个区县：瑶海、庐阳、蜀山、包河、高新、经开、新站、滨湖新区、政务、高速辖区、肥东、肥西、庐江、长丰、巢湖。

### 4.7 字典表结构

字典数据由 `sys_dict_type` + `sys_dict_data` 两张表支撑，接口按 `dictType` 返回 `dictLabel/dictValue` 列表。业务字典类型共 10 类（详见爬取数据文件）。

---

## 五、API 设计

### 5.1 统一响应格式

```json
{ "code": 200, "msg": "操作成功/查询成功", "data": ..., "total": N, "rows": [...] }
```

- `code=200` 成功；`401` 令牌失效；`500` 服务器错误
- 列表接口：`{total, rows}`
- 非列表接口：`{data}`

### 5.2 认证与系统接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /prod-api/system/user/getInfo | 当前用户信息 |
| GET | /prod-api/system/menu/getRouters | 菜单路由 |
| GET | /prod-api/system/user/profile | 个人资料 |
| GET | /prod-api/system/config/configKey/{key} | 参数配置 |
| GET | /prod-api/system/dict/data/type/{type} | 字典数据 |
| GET/POST | /prod-api/system/user、/system/role、/system/dict、/system/config | 系统管理 CRUD |
| GET | /prod-api/system/sysUserArchives、/system/userArchives-* | 用户档案 |

### 5.3 信号灯业务接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /prod-api/signal/warning/list | 预警列表（分页） |
| POST | /prod-api/signal/warning/flowWorkOrder | 转工单 |
| GET | /prod-api/signal/warning/config/list | 预警配置列表 |
| GET | /prod-api/signal/equipment/list | 设备列表 |
| POST/PUT/DELETE | /prod-api/signal/equipment、/signal/equipment/、/signal/equipment/delete/{id} | 设备增改删 |
| GET | /prod-api/signal/equipment/sendConfig | 设备配置下发 |
| GET | /prod-api/signal/crossing/crossList | 路口列表 |
| POST/PUT/DELETE | /prod-api/signal/crossing、/signal/crossing/、/signal/crossing/delete/{id} | 路口增改删 |
| GET | /prod-api/signal/crossingMap/getMapData | 地图大屏点位数据 |

### 5.4 统计接口

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /prod-api/statistics/home/baseData | 基础统计（设备数/故障数/巡检数） |
| POST | /prod-api/statistics/home/sevenData | 近 7 日趋势 |
| POST | /prod-api/statistics/home/faultRanking | 故障人员排行 |
| POST | /prod-api/statistics/home/inspectRanking | 巡检人员排行 |

### 5.5 基础数据接口

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /prod-api/maintain/api/areas、/baseData/api/areas | 行政区划 |
| GET | /prod-api/baseData/api/battalions | 大队 |
| GET | /prod-api/baseData/point、/baseData/point-set | 点位 |
| GET | /prod-api/data/dataPoint/listByCondition | 点位条件查询 |
| POST | /prod-api/common/upload | 文件上传 |
| GET | /prod-api/maintain/interface、/maintain/team-add、/maintain/team-edit | 维护模块 |

---

## 六、关键技术设计

### 6.1 前端工程化

- **路由懒加载**：`system/menu/getRouters` 动态返回路由，前端按需加载 `chunk-*.js`
- **组件化**：Element UI 表单/表格/弹窗/下拉/消息框/确认框
- **状态管理**：Vuex（推断，含用户信息、字典缓存）
- **地图可视化**：高德 AMap JS API + WebGL（marker 聚簇、动态点位）

### 6.2 数据实时性

- 设备在线状态通过**心跳机制**维持（字段 `hearts`、`requestTime`、`lineStatus`）
- 设备状态字段实时更新，前端列表轮询刷新（推断短轮询或 MQTT 推送）

### 6.3 安全设计

- JWT 令牌（HS512 签名），Cookie 存储，支持记住密码（密码 AES 加密存储于 Cookie）
- 租户数据隔离（tenantId）
- 逻辑删除（delFlag）
- 验证码登录（图形验证码，防止暴力破解）

### 6.4 多租户与权限

- 角色权限：SIGNAL_ADMIN（信号灯管理员）——仅信号灯检测模块
- 数据权限：dataScope=1（全部数据）
- 系统预留：系统管理（用户/角色/菜单/字典/参数/档案）、维护模块（接口/团队）

---

## 七、总结与启示

该系统为典型的 **RuoYi 框架 + 信号灯行业业务** 的垂直系统：

1. **业务闭环完整**：设备上报 → 故障研判 → 预警生成 → 工单流转，形成运维闭环；
2. **预警规则可配置**：支持按路口/设备/告警类型配置忽略规则，灵活适配不同运维策略；
3. **可视化完善**：首页统计看板 + 地图大屏，便于运维决策；
4. **架构标准**：前后端分离、接口统一、字典驱动、租户隔离，具备良好的可扩展性。

**对本项目（TSLOMS）的参考价值**：本系统的数据结构、字典设计、预警-工单流转模型、地图大屏实现方式，可作为 TSLOMS 交通信号灯检测后台运维系统设计的重要参考蓝本。

---

## 附录：关键数据快照

- 设备总数：145 台（点位基础数据 29,588）
- 路口总数：23 个
- 预警总数：60,160 条（均为待处理状态）
- 覆盖区域：合肥市 15 个区县
- 告警类型：14 种（全灭/同亮/超时/缺亮/断电）
- 故障排行 TOP：王春阳 5 次、王硕 2 次、凌绪泽 2 次、程俊 2 次
- 巡检排行：胡伟 29 次、崔建伟 8 次