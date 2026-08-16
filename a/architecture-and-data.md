# 信号灯检测系统 —— 架构与数据结构参考（实测采集）

> 本文件是 `a.md` 的补充原始素材，来源于：
> 1. 前端代码静态分析（路由/组件/API 定义）
> 2. 已登录 token 实测 API 返回的 JSON 数据样例

## A. 前端模块清单（Vue 路由全集，来自 app.js）

```
/login, /register, /401, /404, /redirect/:path(.*), /trafficEdit
/
/index, /bigScreen, /cockpit/one, /cockpit/two, /review
/signal             信号灯检测（本账号可见）
/signal/earlyWarning 预警管理
/signal/warningConfig 预警配置
/signal/device 设备管理
/signal/cross 路口配置
/signal/mapScreen 地图大屏
/inspect/infield/computerRoom/result 内场-机房
/inspect/infield/rotary/result      内场-旋转
/inspect/infield/subControlCenter/result 内场-分控中心
/inspect/infield/policeNetwork/hardware/result 公安网硬件
/inspect/infield/policeNetwork/software/result 公安网软件
/inspect/out/plan 外场巡检计划
/inspect/out/result 外场巡检结果
/monitor/job, /monitor/job-log 监控调度
/maintain/interface, /maintain/team-add, /maintain/team-edit 维护保养
/performance/info 绩效
/project/maintenance, /project/maintenanceAudit, /projectManagement/maintenance, /projectOld/maintenanceAudit 项目维护/审计
/storage/equip, /storage/in, /storage/out, /storage/receive 仓库物资
/uncharged/equip, /uncharged/storage/in, /uncharged/storage/out 未结算物资
/traffic/application, /trafficEdit 交通工程
/synchronization/traffic/project 交通项目同步
/system/* （user, role, dict, config, user-auth, role-auth, userArchives...）系统管理
/tool/gen* 代码生成
/baseData/point, /baseData/point-set 点位维护
```

## B. 信号灯检测模块 API（实测）

- 路口：`/signal/crossing`、`/signal/crossing/{id}`、`/signal/crossing/crossList`、`/signal/crossing/update`、`/signal/crossing/delete/{id}`
- 地图：`/signal/crossingMap/getMapData`
- 预警：`/signal/warning/list`、`/signal/warning/flowWorkOrder`
- 预警配置：`/signal/warning/config/{list,add,edit,del}`
- 设备：`/signal/equipment/list`、`/signal/equipment`(add)、`/signal/equipment/{uuid}`、`/signal/equipment/update`、`/signal/equipment/delete/{id}`、`/signal/equipment/sendConfig`
- 点位/数据点：`/signal/crossing/pointList`、`/data/dataPoint/listByCondition`
- 基础数据：`/baseData/api/areas`、`/baseData/api/battalions`、`/maintain/api/areas`

## C. 关键数据实体字段字典

### C1. 路口 Crossing
| 字段 | 类型 | 说明 |
|---|---|---|
| id | string | 主键（租户内唯一）|
| pointId | string | 点位编号 |
| pointName | string | 路口名称 |
| type | string | 类型 |
| longitude / latitude | string | 经纬度（高德 GCJ-02）|
| areaId / areaName | string | 行政区 |
| status | string | 状态码 3/4/5 |
| equipments | array | 关联设备（详情返回）|

### C2. 设备 Equipment
| 字段 | 类型 | 说明 |
|---|---|---|
| uuid | string | 信号机设备地址码 |
| areaId / areaName | string | 行政区 |
| pointId / pointName | string | 所属路口 |
| func | string | 功能/相位（如 东向西直行）|
| fx / cx | string | 方向/朝向 |
| lon / lat | string | 经纬度 |
| installDate | string | 安装日期 |
| hearts | int | 心跳 |
| requestTime | string | 最近上报时间 |
| versionValue | string | 版本值 |
| lineStatus | string | 在线状态 1=在线 |
| batch | string | 批次 |
| extraData | object | 扩展数据 |
| configUrl | string | 配置地址 |

### C3. 预警 Warning
| 字段 | 类型 | 说明 |
|---|---|---|
| pointId / pointName | string | 路口 |
| equipmentUuid | string | 设备 |
| content | string | 故障/预警内容码（如 -1）|
| func | string | 相位 |
| contentLabel | string | 描述标签 |
| dealState | string | 处理状态 |
| status | string | 状态 |

### C4. 预警配置 WarningConfig
| 字段 | 类型 | 说明 |
|---|---|---|
| pointId / pointName | string | 路口（all=全部）|
| equipmentUuid | string | 设备（all=全部）|
| content | string | 触发内容码 |
| effectiveType | string | 生效类型 |
| startTime / endTime | string | 生效时段 |
| status | string | 启用状态 |

### C5. 区域 Area
`id`(区划码, 如 340102)、`name`、`parentId`、`areaSort`、`areaType`、`fullName`、`sn`

### C6. 用户/角色/部门
- 用户：`userId, deptId, postId, userName, nickName, email, phonenumber, sex, avatar, password(bcrypt), status, authStatus, safetyTipsFlag, loginIp, loginDate, dept{...}, roles[...]`
- 角色：`roleId, roleName, roleKey, roleSort, dataScope, menuCheckStrictly, deptCheckStrictly, status, tenantType, admin`
- 部门：`deptId, parentId, ancestors, deptName, orderNum, leader, phone, email, status, deptType, areaId`

### C7. 字典 Dict
`dictCode, dictSort, dictLabel, dictValue, dictType, cssClass, listClass, isDefault, status, remark`

## D. 对象关系
```
Tenant(tenantId) 1─N Dept(deptId) 1─N User(userId)
Area(areaId) 1─N Crossing(pointId) 1─N Equipment(uuid)
Crossing / Equipment 1─N Warning
Crossing / Equipment 1─N WarningConfig
```

## E. 采集数据样例文件（_crawled/）
- `01_user_getInfo.json` 登录用户（角色 SIGNAL_ADMIN、部门 安徽省通信产业服务有限公司、租户…dec）
- `02_menu_getRouters.json` 当前用户菜单树（/signal 信号灯检测 + 5 子菜单）
- `15_dict_data_common.json` 字典（sys_job_status）
- `20_crossing_crossList.json` 路口列表（total=23，覆盖合肥各城区）
- `22_warning_list.json` 预警列表（total=60160）
- `23_equipment_list.json` 设备列表（total=145）
- `24_warnconfig_list.json` 预警配置（total=3）
- `25_map_getData.json` 地图大屏数据（23 路口坐标）
- `26_baseData_areas.json` 行政区（15 条）
- `31_crossing_detail.json` 路口详情（含关联设备，1:N）
