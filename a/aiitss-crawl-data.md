# aiitss.cn 网站爬取原始数据

> 目标：https://www.aiitss.cn/
> 登录账号：13955832695（信号灯管理员，姓名：谢东亮）
> 爬取时间：2026-08-16
> 说明：以下数据均来自真实页面渲染与真实 API 响应，为设计报告提供依据。

## 1. 系统基础信息

- 系统名称：运维管理系统（版权: Copyright © 2018-2022 znkj All Rights Reserved.）
- 前端框架：Vue2 + Element UI（el-form、el-table、el-dialog、el-select 等）
- 地图：高德地图 AMap（JS API + WebGL，`© 2026 AutoNavi - GS(2025)5996号`）
- 后端风格：RuoYi 框架（spring boot），接口前缀 `/prod-api`
- 认证方式：Cookie 存储 `Admin-Token`（JWT HS512），`rememberMe=true`，`Admin-Expires-In=720`
- 当前登录用户角色：`SIGNAL_ADMIN`（信号灯管理员）
- 所属部门：安徽省通信产业服务有限公司

## 2. 首页统计数据

接口 POST `/prod-api/statistics/home/baseData`：

```json
{"equipNum":29588,"faultNum":414,"inspectNum":82}
```

- equipNum：基础点位（设备）数量 29588
- faultNum：本月故障数 414
- inspectNum：本月巡检数 82

接口 POST `/prod-api/statistics/home/faultRanking`（故障排行）：

```json
{"data":[{"name":"王春阳","value":5},{"name":"王硕","value":2},{"name":"凌绪泽","value":2},{"name":"程俊","value":2},{"name":"庞自翔","value":1},{"name":"黄益智","value":1},{"name":"徐先然","value":1}]}
```

接口 POST `/prod-api/statistics/home/inspectRanking`（巡检排行）：

```json
{"data":[{"name":"崔建伟","value":8},{"name":"胡伟","value":29}]}
```

接口 POST `/prod-api/statistics/home/sevenData`（近 7 日数据，图表）。

## 3. 预警管理（/signal/earlyWarning）

### 3.1 列表接口

GET `/prod-api/signal/warning/list?pageNum=1&pageSize=10`

响应示例（total=60160）：

```json
{
  "total": 60160,
  "rows": [{
    "id": "b6581519a3a04202bb41da7a194c5ee7",
    "pointId": "point202404260680",
    "pointName": "长江东大街与东一环路",
    "equipmentUuid": "11140075",
    "content": "-1",
    "func": "东向西直行",
    "contentLabel": null,
    "dealState": "1",
    "status": "1",
    "createTime": "2026-08-16 14:58:57"
  }],
  "code": 200,
  "msg": "查询成功"
}
```

### 3.2 页面字段

- 搜索条件：路口名称、设备编码、处理状态、是否转工单、告警内容、告警时间
- 表格列：编号、路口设备、功能方向、告警内容、当前状态、告警时间、操作（忽略/转工单）
- 操作：批量忽略、导出、忽略、转工单

示例告警记录：

```
路口名称: 长江东大街与东一环路  设备编码: 11140075
信号灯功能：东向西直行  红灯周期全灭
处理状态: 待处理  工单状态: 未转
告警时间: 2026-08-16 14:58:57
```

### 3.3 转工单

- 弹窗标题：转工单
- 字段：备注信息（placeholder: 请输入备注信息）
- 接口：POST `/prod-api/signal/warning/flowWorkOrder`

### 3.4 忽略

- 行操作"忽略"，点击后弹确认框（el-message-box），确认后调用忽略接口。

## 4. 预警配置（/signal/warningConfig）

### 4.1 列表接口

GET `/prod-api/signal/warning/config/list?pageNum=1&pageSize=10`

响应示例（total=3）：

```json
{
  "total": 3,
  "rows": [{
    "id": "28",
    "pointId": "all",
    "pointName": "全部",
    "equipmentUuid": "all",
    "content": "-8",
    "effectiveType": "0",
    "startTime": null,
    "endTime": null,
    "status": "1",
    "createTime": "2026-07-09 16:32:49"
  }]
}
```

### 4.2 页面字段

- 搜索条件：路口名称、设备UUID、预警内容、状态
- 表格列：序号、忽略路口、忽略设备、忽略预警、生效模式、生效时间、结束时间、状态、操作（删除）
- 现有配置 3 条：
  - 全部路口/all/红灯亮灯超过设定时间/永久生效
  - 全部路口/all/黄灯亮灯超过设定时间/永久生效
  - 全部路口/all/绿灯亮灯超过设定时间/永久生效
- 操作：新增、删除

### 4.3 新增弹窗（标题：添加预警信息）

- 字段：路口（请选择忽略的路口，含"全部"选项）、设备（请选择忽略的设备）、忽略预警（下拉：所有/正常/红灯周期全灭/黄灯周期全灭/绿灯周期全灭/红黄同亮/...）、生效模式（永久生效/时间范围生效）、是否生效（状态开关）
- 生效模式为"时间范围生效"时显示：生效时间、结束时间

## 5. 设备管理（/signal/device）

### 5.1 列表接口

GET `/prod-api/signal/equipment/list?pageNum=1&pageSize=10`

响应示例（total=145）：

```json
{
  "total": 145,
  "rows": [{
    "id": "5cba968d11424d9a994e4428fb476b9a",
    "uuid": "1114004B",
    "batch": null,
    "func": null,
    "fx": null,
    "cx": null,
    "lon": null,
    "lat": null,
    "installDate": null,
    "hearts": 120,
    "requestTime": "2026-08-16 21:22:13",
    "versionValue": "275371777",
    "lineStatus": "1",
    "extraData": null,
    "configUrl": null
  }],
  "code": 200
}
```

### 5.2 页面字段

- 搜索条件：所属区域、所属路口、设备编码、备注信息、在线状态
- 表格列：编号、设备信息（设备编码/备注信息）、辖区、路口、在线状态、安装时间、信号灯功能、操作（OTA升级）
- 操作按钮：新增、修改、删除、OTA升级

### 5.3 新增弹窗（标题：新增数据）

- 字段（el-form-item）：
  - 行政区（el-select，选项：瑶海区/庐阳区/蜀山区/包河区/高新区/经开区/新站区/滨湖新区/政务区/高速辖区/肥东县/肥西县/庐江县/长丰县/巢湖市）
  - 所属路口（请输入点位名称...）
  - 设备编码（请输入设备唯一编码）
  - 备注信息
  - 信号灯功能
  - 安装时间
  - 经纬度（经度/纬度）
- 其他弹窗：显示/隐藏、选择位置、OTA升级

### 5.4 相关接口

- POST `/prod-api/signal/equipment`（新增）
- PUT `/prod-api/signal/equipment/`（修改）
- DELETE `/prod-api/signal/equipment/delete/{id}`（删除）
- GET `/prod-api/signal/equipment/sendConfig`（下发配置）

## 6. 路口配置（/signal/cross）

### 6.1 列表接口

GET `/prod-api/signal/crossing/crossList?pageNum=1&pageSize=10`

响应示例（total=23）：

```json
{
  "total": 23,
  "rows": [{
    "id": "48",
    "pointId": "point202404260413",
    "pointName": "长江中路与宿州路",
    "type": "1",
    "longitude": "117.289022",
    "latitude": "31.861512",
    "areaId": "340103",
    "areaName": "庐阳区",
    "status": "4"
  }],
  "code": 200
}
```

### 6.2 页面字段

- 搜索条件：行政区、路口名称、路口类型、状态
- 表格列：路口编号、路口名称、路口类型、行政区、经纬度、状态、操作
- 状态枚举显示：维护中(1)、监测中(2)、离线(3)、异常(4)、黄闪(5)

### 6.3 相关接口

- POST `/prod-api/signal/crossing`（新增）
- PUT `/prod-api/signal/crossing/`（修改）
- DELETE `/prod-api/signal/crossing/delete/{id}`（删除）

## 7. 地图大屏（/signal/mapScreen）

- 接口：GET `/prod-api/signal/crossingMap/getMapData`
- 返回 `data` 数组（23 个路口点位），每个点位含：id、pointId、pointName（路口名称）、type（路口类型）、longitude、latitude、areaId、areaName、status
- 地图渲染：高德地图 Marker 标记各路口，按状态着色
- 页面左侧显示全部路口名称列表

## 8. 个人中心（/user/profile）

- 接口：GET `/prod-api/system/user/getInfo`
- 展示：账号、姓名、手机号码、所属部门、所属角色、创建日期
- 基本信息字段：用户昵称、手机号码、性别（男/女）
- 支持：基本资料、修改密码
- 相关接口：GET `/prod-api/system/user/profile`（获取当前用户信息）

## 9. 字典数据（系统字典）

### 9.1 接口

GET `/prod-api/system/dict/data/type/{dictType}`

### 9.2 signal_warning_deal_state（预警处理状态）

- 1: 未处理 / 2: 已处理

### 9.3 signal_equipment_warning（设备预警内容）

- all: 所有 / 0: 正常 / -1: 红灯周期全灭 / -2: 黄灯周期全灭 / -3: 绿灯周期全灭 / -4: 红黄同亮 / -5: 红绿同亮 / -6: 黄绿同亮 / -7: 红黄绿同亮 / -8: 红灯亮灯超过设定时间 / -9: 黄灯亮灯超过设定时间 / -10: 绿灯亮灯超过设定时间 / -11: 红灯缺亮 / -12: 黄灯缺亮 / -13: 绿灯缺亮 / -14: 断电

### 9.4 signal_equipment_batch（设备灯组）

- 1~8: 灯组一~灯组八

### 9.5 signal_equipment_func（设备功能）

- 1: 机动车 / 2: 倒计时机动车 / 3: 非机动车 / 4: 人行横道（左）/ 5: 独立倒计时 / 6: 闪光警告 / 7: 人行横道（右）

### 9.6 signal_equipment_fx（设备方向）

- 1: 左转箭头 / 2: 直行箭头 / 3: 右转箭头 / 4: 满屏 / 5: 掉头 / 6: 直左 / 7: 直右

### 9.7 signal_equipment_cx（设备朝向/车型方向）

- 1: 由西向东 / 2: 由北向南 / 3: 由东向西 / 4: 由南向北 / 5: 向东南 / 7: 向西北 / 8: 向东北

### 9.8 signal_cross_type（路口类型）

- 1: 直角路口 / 2: 高速公路卡口 / 3: 三路口 / 4: 四路口 / 5: 五路口 / 6: 六路口 / 7: 七路口 / 8: 八路口

### 9.9 signal_crossing_status（路口状态）

- 1: 维护中 / 2: 监测中 / 3: 离线 / 4: 异常 / 5: 黄闪

### 9.10 signal_equipment_extra_light_state（辅灯状态）

- 0: 红灯亮 / 1: 黄灯亮 / 2: 绿灯亮 / -1: 未知

## 10. 行政区数据（areas）

接口 GET `/prod-api/maintain/api/areas` 与 GET `/prod-api/baseData/api/areas`：

- 瑶海区(340102)、庐阳区(340103)、蜀山区(340104)、包河区(340111)、高新区(340171)、经开区(340172)、新站区(340173)、滨湖新区(340174)、政务区(340175)、高速辖区(340176)、肥东县(340178)、肥西县(340179)、庐江县(340180)、长丰县(340182)、巢湖市(340183)
- 字段：id、name、parentId(340100 合肥市)、areaType(3)、fullName(如"安徽省合肥市瑶海区")

## 11. 菜单路由结构

接口 GET `/prod-api/system/menu/getRouters`：

```
Signal  (/signal, 标题: 信号灯检测, 图标: button, Layout)
├── WarningConfig  warningConfig  信号灯检测/预警配置      signalDevice/warn/config/index     图标: bug
├── EarlyWarning    earlyWarning   信号灯检测/预警管理      signalDevice/warn/earlyWarning/index 图标: client
├── Device          device         信号灯检测/设备管理      signalDevice/device/device          图标: build
├── Cross           cross          信号灯检测/路口配置      signalDevice/cross/crossCard        图标: exit-fullscreen
└── MapScreen       mapScreen      信号灯检测/地图大屏      signalDevice/device/mapScreen       图标: chart
```

另有：首页(/index)、个人中心(/user/profile)

## 12. 完整 API 清单

### 认证与系统
- GET `/prod-api/system/user/getInfo` — 获取当前用户信息（角色、权限）
- GET `/prod-api/system/menu/getRouters` — 获取菜单路由
- GET `/prod-api/system/user/profile` — 获取个人资料
- GET `/prod-api/system/config/configKey/service.specialMaintain` — 系统配置
- GET `/prod-api/system/dict/data/type/{type}` — 字典数据
- GET/PUT `/prod-api/system/user` — 用户管理
- GET/POST `/prod-api/system/role` — 角色管理
- GET/POST `/prod-api/system/dict`、`/system/dict-data`、`/system/dict/data` — 字典管理
- GET/POST `/prod-api/system/config` — 参数配置
- GET `/prod-api/system/sysUserArchives`、`/system/userArchives-info`、`/system/userArchives-edit` — 用户档案

### 信号灯业务
- GET `/prod-api/signal/warning/list` — 预警列表
- POST `/prod-api/signal/warning/flowWorkOrder` — 转工单
- GET `/prod-api/signal/warning/config/list` — 预警配置列表
- GET `/prod-api/signal/equipment/list` — 设备列表
- POST `/prod-api/signal/equipment` — 新增设备
- PUT `/prod-api/signal/equipment/` — 修改设备
- DELETE `/prod-api/signal/equipment/delete/{id}` — 删除设备
- GET `/prod-api/signal/equipment/sendConfig` — 设备配置下发
- GET `/prod-api/signal/crossing/crossList` — 路口列表
- POST `/prod-api/signal/crossing` — 新增路口
- PUT `/prod-api/signal/crossing/` — 修改路口
- DELETE `/prod-api/signal/crossing/delete/{id}` — 删除路口
- GET `/prod-api/signal/crossingMap/getMapData` — 地图大屏数据

### 统计
- POST `/prod-api/statistics/home/baseData` — 首页基础统计
- POST `/prod-api/statistics/home/sevenData` — 近7日数据
- POST `/prod-api/statistics/home/faultRanking` — 故障排行
- POST `/prod-api/statistics/home/inspectRanking` — 巡检排行

### 基础数据
- GET `/prod-api/maintain/api/areas` — 行政区划
- GET `/prod-api/baseData/api/areas` — 行政区划
- GET `/prod-api/baseData/api/battalions` — 大队数据
- GET `/prod-api/baseData/point`、`/baseData/point-set` — 点位管理
- GET `/prod-api/data/dataPoint/listByCondition` — 数据点位条件查询
- POST `/prod-api/common/upload` — 文件上传
- GET `/prod-api/maintain/interface`、`/maintain/team-add`、`/maintain/team-edit` — 维护相关