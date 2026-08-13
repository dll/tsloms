# PRD-TSLOMS-v2.0 交通信号灯检测后台运维系统

**项目名称**：交通信号灯检测后台运维系统

**英文名称**：Traffic Signal Light Operation and Maintenance System

**项目简称**：TSLOMS

**文档版本**：V2.0

**更新日期**：2026-08-13

**适用场景**：城市交通信号灯设备 MQTT 通信对接、故障自动研判、维修工单流转、运维数据可视化统计

## 一、V2 变更摘要

V1.0 基于信号灯设备 V3 通信协议文档进行了初步设计，但在通信方式上存在重大误判：V1.0 两套方案均采用 Netty 实现 TCP/UDP 报文接收，而实际 V3 协议完全基于 MQTT 消息队列通信。V2.0 针对以下方面进行了修订：

| 变更项 | V1.0 | V2.0 |
|--------|------|------|
| 通信方式 | Netty TCP/UDP 长连接 | MQTT Broker 订阅/发布 |
| 后端语言 | Java（SpringBoot / SpringCloud） | Go 1.22+ |
| Web 框架 | SpringBoot / SpringCloud Alibaba | Gin |
| ORM | MyBatis / JPA | GORM |
| MQTT 客户端 | 无（V1.0 未考虑） | paho.mqtt.golang |
| 数据库 | MySQL | MySQL 8.0（与 EQS 项目共享实例） |
| 缓存 | Redis（仅方案二） | Redis 7.0（与 EQS 项目共享实例） |
| 时序数据 | InfluxDB（仅方案二） | MySQL 分区表（简化部署） |
| 前端 | Vue2/Vue3 | Vue3 + Vite + Element Plus |
| 部署 | 单机 / Docker 集群 | Docker + systemd（与 EQS 共享云服务器） |
| 技术方案数量 | 两套差异化方案 | 统一一套 Go 方案 |

V2.0 不再提供两套差异化方案。Go 语言在百级设备并发场景下性能充裕，单一技术方案既满足快速交付，又具备横向扩展能力，无需在轻量与高可用之间做取舍。

## 二、项目概述

### 2.1 项目简介

系统对接交通信号灯硬件设备，基于 V3 版本设备通信协议，通过 MQTT 消息队列实时接收、解析设备上报的二进制数据包，自动研判设备故障，触发维修工单下发与流转闭环。同时对故障、工单数据进行聚合统计，通过饼图、柱状图实现可视化展示，解决传统人工巡检滞后、运维数据无数字化统计的痛点。

### 2.2 核心业务流程

```
信号灯设备 → MQTT Broker → 后台订阅消息 → 二进制报文解析校验
  → 故障规则研判 → 正常数据入库存档 / 故障自动生成工单
  → 工单派发、处理、闭环 → 数据聚合统计、图表可视化展示
```

### 2.3 功能范围

**包含**：MQTT 设备通信对接、二进制报文解析校验、故障智能判别、维修工单管理、设备台账管理、固件 OTA 升级、数据统计可视化、系统权限管理

**不含**：硬件设备改造、运维人员移动端 APP（预留对接接口）

## 三、通信协议详解

### 3.1 系统架构

信号灯监控设备在检测到故障或签到周期结束时，主动向 MQTT 服务器发送数据包。后台系统通过订阅 MQTT Topic 接收设备消息，需要时也可通过 MQTT 向设备下发命令（配置更新、重启等）。

```
  ┌──────────┐         ┌──────────────┐         ┌──────────┐
  │ 信号灯    │  MQTT   │              │  MQTT   │  TSLOMS  │
  │ 监控设备  │◄───────►│  MQTT Broker │◄───────►│  后台系统  │
  └──────────┘         └──────────────┘         └──────────┘
```

MQTT 连接参数通过设备配置工具 `trafficLightConf` 写入设备，包括服务器 IP、端口、用户名、密码、Topic 前缀等。

### 3.2 命令帧格式（CMD_FRAME）

所有设备通信基于统一的二进制命令帧结构：

```c
typedef struct {
    uint8_t  token;     // 魔术字，固定 0x55
    uint8_t  cmd;       // 命令类型，见 COMMAND 定义
    uint8_t  ver;       // 协议版本号，当前 0x10 = v1.0
    uint8_t  checksum;  // 校验和，整包所有字节(uint8)相加等于 0xFF
    uint32_t swVer;     // 设备软件版本号
    uint16_t cmdSeq;    // 包序号，每发送一个命令序号加一
    uint16_t datLen;    // 数据部分长度（字节）
    uint32_t userVal;   // 用户自定义数据（用于时间同步等）
    uint8_t  dat[0];    // 变长数据，根据 cmd 类型有不同结构
} CMD_FRAME;
```

| 偏移 | 类型 | 字段 | 说明 |
|------|------|------|------|
| 0 | uint8 | token | 魔术字 0x55 |
| 1 | uint8 | cmd | 命令类型 |
| 2 | uint8 | ver | 协议版本 0x10 |
| 3 | uint8 | checksum | 校验和，整包 uint8 之和 = 0xFF |
| 4-7 | uint32 | swVer | 软件版本号（含主版本/次版本/构建日期） |
| 8-9 | uint16 | cmdSeq | 包序号 |
| 10-11 | uint16 | datLen | dat 部分长度 |
| 12-15 | uint32 | userVal | 用户自定义数据 |
| 16+ | uint8[] | dat | 变长数据内容 |

校验算法：整包所有字节按无符号数（uint8）累加，结果必须等于 0xFF。

### 3.3 命令类型定义（COMMAND）

| 编码 | 常量 | 方向 | 说明 |
|------|------|------|------|
| 0x00 | CMD_CHECKIN | 设备→服务器 | 定时签到，设备工作正常 |
| 0x01 | CMD_ALARM | 设备→服务器 | 故障报警，信号灯异常 |
| 0x03 | CMD_POWER_ON | 设备→服务器 | 上电/重启完成报告 |
| 0x20 | CMD_UPDATE_CONFIG | 服务器→设备 | 下发配置更新 |
| 0x30 | CMD_CHECK_FW | 设备→服务器 | 查询是否有新固件 |
| 0x31 | CMD_GET_FW | 设备→服务器 | 请求固件升级数据 |
| 0x7F | CMD_REBOOT | 服务器→设备 | 远程重启设备 |
| 0x80 | CMD_ACK_FLAG | 响应标志 | bit7=1 表示回应帧 |

签到（CMD_CHECKIN）和上电报告（CMD_POWER_ON）需要服务器返回时间同步信息，通过 userVal 字段返回 epoch seconds（UTC+8 时区修正）。告警（CMD_ALARM）无需回应。

### 3.4 事件数据结构（EVENT_PAK / EVENT_RECORD）

当 cmd 为 CMD_CHECKIN 或 CMD_ALARM 时，dat 部分按 EVENT_PAK 格式解释：

```c
typedef struct {
    uint16_t eventRecordNum;        // 事件记录数量
    uint16_t datLen;                // eventRecord 部分总长度
    EVENT_RECORD eventRecord[0];    // 变长事件记录数组
} EVENT_PAK;

typedef struct {
    uint32_t ledHwId;     // 设备硬件 ID（出厂唯一）
    uint32_t subHwId;     // 子灯组 ID
    uint32_t swVer;       // 固件版本号
    uint32_t confVer;     // 配置版本号（0xYYMMDDnn 格式）
    uint8_t  ledState;    // 当前灯组亮灯状态
    int8_t   errCode;     // 错误码（见下表）
    uint16_t current[3];  // 红黄绿三灯电流值（0-2048）
} EVENT_RECORD;
```

### 3.5 错误码定义（errCode）

| 常量 | 数值 | 说明 |
|------|------|------|
| LED_ERR_OK | 0 | 工作正常 |
| LED_ERR_ROFF | -1 | 红灯周期全灭 |
| LED_ERR_YOFF | -2 | 黄灯周期全灭 |
| LED_ERR_GOFF | -3 | 绿灯周期全灭 |
| LED_ERR_RYON | -4 | 红黄同亮 |
| LED_ERR_RGON | -5 | 红绿同亮 |
| LED_ERR_YGON | -6 | 黄绿同亮 |
| LED_ERR_RYGON | -7 | 红黄绿同亮 |
| LED_ERR_RON_TIMEOUT | -8 | 红灯亮灯超时 |
| LED_ERR_YON_TIMEOUT | -9 | 黄灯亮灯超时 |
| LED_ERR_GON_TIMEOUT | -10 | 绿灯亮灯超时 |
| LED_ERR_RDIM | -11 | 红灯缺亮（暂未实现） |
| LED_ERR_YDIM | -12 | 黄灯缺亮（暂未实现） |
| LED_ERR_GDIM | -13 | 绿灯缺亮（暂未实现） |
| LED_ERR_POWER_LOSS | -14 | 断电（超过设定时间阈值） |

### 3.6 灯组状态定义（LED_STATE）

| 常量 | 数值 | 说明 |
|------|------|------|
| STATE_R | 0 | 红灯状态 |
| STATE_Y | 1 | 黄灯状态 |
| STATE_G | 2 | 绿灯状态 |
| STATE_NONE | -1 | 未知状态（故障无法确定） |

### 3.7 固件升级

设备通过 CMD_CHECK_FW 查询是否有新固件，服务器返回 FIRMWARE_INFO_DAT（版本号、长度、校验和）。设备通过 CMD_GET_FW 分块请求固件数据，服务器返回 FIRMWARE_PAK（包含 offset 偏移量和 1-256 字节数据）。

### 3.8 设备配置参数

设备配置通过 MQTT 下发（CMD_UPDATE_CONFIG），主要参数包括：

| 参数 | 说明 | 示例值 |
|------|------|--------|
| checkinMin | 签到周期（分钟） | 2 |
| gapSec | 信号灯切换间隙（秒） | 2 |
| ledMaxPeriodSecR | 红灯最长周期（秒） | 100 |
| ledMaxPeriodSecY | 黄灯最长周期（秒） | 5 |
| ledMaxPeriodSecG | 绿灯最长周期（秒） | 120 |
| powerLossSec | 断电最长等待（秒） | 200 |
| mqttServerIp | MQTT 服务器 IP | - |
| mqttServerPort | MQTT 服务器端口 | - |
| mqttUserName | MQTT 用户名（最长 8 位） | - |
| mqttPassword | MQTT 密码（最长 8 位） | - |

## 四、核心功能需求

### 4.1 MQTT 设备通信模块

系统通过 paho.mqtt.golang 客户端连接 MQTT Broker，订阅设备 Topic（格式：`{topicPrefix}/{networkCode}/{stationCode}/{hwId}/U`）接收设备上报消息。

收到消息后执行以下处理链：

1. 二进制解码：按 CMD_FRAME 结构解析 token、cmd、ver、checksum、swVer、cmdSeq、datLen、userVal
2. 校验验证：token 必须为 0x55，checksum 整包累加必须等于 0xFF，不通过则记录异常报文日志并丢弃
3. 命令分发：根据 cmd 类型路由到对应处理器
4. 数据提取：从 EVENT_RECORD 中提取 ledHwId、ledState、errCode、current[3] 等核心字段
5. 时间同步：对 CMD_CHECKIN 和 CMD_POWER_ON，通过 userVal 返回当前 epoch seconds（UTC+8）

设备在线/离线判定基于 MQTT 的遗嘱消息（LWT）和签到周期超时检测。若超过 `checkinMin` 配置时间的 3 倍未收到签到包，标记设备离线。

### 4.2 故障研判模块

根据 EVENT_RECORD 中的 errCode 字段进行故障分类：

| 故障类别 | 对应 errCode | 故障等级 | 处理方式 |
|----------|-------------|----------|----------|
| 灯珠灭灯 | -1, -2, -3 | 严重 | 自动生成工单 |
| 异常同亮 | -4, -5, -6, -7 | 严重 | 自动生成工单 |
| 亮灯超时 | -8, -9, -10 | 一般 | 自动生成工单 |
| 缺亮 | -11, -12, -13 | 一般 | 记录待确认（协议暂未实现） |
| 断电 | -14 | 严重 | 自动生成工单 + 设备离线标记 |

重复故障去重规则：同一设备同一 errCode 在 30 分钟内只生成一条故障记录，后续签到中持续上报相同故障时更新故障的 lastSeen 时间戳。

### 4.3 工单运维模块

故障触发后自动生成维修工单，工单状态流转：

```
待处理 → 处理中 → 已完成
              ↘ 已驳回 → 待处理（重新派发）
```

| 字段 | 类型 | 说明 |
|------|------|------|
| 工单编号 | varchar(32) | 自动生成，格式 WO{yyyyMMdd}{seq} |
| 关联故障 | uint | 关联 fault_records 表 ID |
| 设备 ID | uint32 | 信号灯设备硬件 ID |
| 工单状态 | enum | pending / processing / completed / rejected |
| 处理人 | uint | 指派维修人员 |
| 创建时间 | datetime | 自动填充 |
| 闭环时间 | datetime | 工单完成时填充 |
| 维修结果 | text | 处理人回填的维修说明 |

支持多条件筛选（设备、状态、时间范围、故障类型）和历史工单查询。

### 4.4 数据可视化模块

**饼图**：故障类型占比、工单状态分布占比

**柱状图**：日/周/月时段故障数量趋势、各路口设备故障数量排行、工单处理效率（平均闭环时长）

支持时间维度筛选（近 7 天 / 30 天 / 自定义区间）和数据报表 CSV 导出。

### 4.5 设备配置管理模块

通过 MQTT 向设备下发配置更新命令（CMD_UPDATE_CONFIG），支持以下操作：

- 单设备配置更新：指定 hwId，下发完整配置参数
- 批量配置更新：循环调用单设备配置接口
- 配置版本管理：每次下发递增 confVer（0xYYMMDDnn 格式），设备上报 confVer 用于验证配置是否生效
- 固件 OTA 升级：上传固件文件后，响应设备的 CMD_CHECK_FW 和 CMD_GET_FW 请求

### 4.6 基础管理模块

设备台账管理（设备 ID、路口位置、安装日期、通信参数、在线状态）、用户角色权限管理（管理员/运维人员/查看人员）、系统操作日志、报文日志查询、故障日志查询。

## 五、非功能需求

1. **性能**：支持百级设备并发接入，MQTT 消息解析 + 故障研判端到端响应 < 100ms
2. **可靠性**：原始报文日志持久化存储，MQTT QoS 1 保证消息至少一次送达，网络波动时自动重连
3. **兼容性**：完全适配信号灯 V3 版本通信协议（token 0x55、ver 0x10）
4. **安全性**：JWT 登录鉴权、接口权限校验、操作日志溯源、MQTT 用户名密码认证
5. **可维护性**：与 EQS 项目共享云服务器 MySQL/Redis 实例，独立数据库/DB 索引隔离

## 六、技术方案

### 6.1 技术栈总览

| 技术层 | 选型 | 版本 | 说明 |
|--------|------|------|------|
| 后端语言 | Go | 1.22+ | 与 EQS 项目统一技术栈 |
| Web 框架 | Gin | v1.9+ | 高性能 HTTP 框架 |
| ORM | GORM | v1.30+ | 支持 MySQL/SQLite 双模式 |
| MQTT 客户端 | paho.mqtt.golang | v1.4+ | Eclipse 官方维护 |
| Redis 客户端 | go-redis/v9 | v9.5+ | 与 EQS 项目一致 |
| JWT | golang-jwt/v5 | v5.2+ | 与 EQS 项目一致 |
| 日志 | zap | v1.27+ | 与 EQS 项目一致 |
| 数据库 | MySQL | 8.0 | 与 EQS 共享实例，独立数据库 tsloms |
| 缓存 | Redis | 7.0 | 与 EQS 共享实例，使用 DB 1（EQS 使用 DB 0） |
| 前端 | Vue3 + Vite | - | Element Plus + ECharts |
| 部署 | Docker + systemd | - | 与 EQS 共享云服务器 |

### 6.2 与 EQS 项目共享基础设施

TSLOMS 与 EQS 部署在同一台腾讯云 CVM 上，共享以下基础设施：

| 资源 | EQS | TSLOMS | 隔离方式 |
|------|-----|--------|----------|
| MySQL 8.0 | 数据库 eqs | 数据库 tsloms | 独立数据库，同一实例 |
| Redis 7.0 | DB 0 | DB 1 | 独立 DB 索引 |
| Nginx | 端口 8091 | 端口 8092 | 独立 server 块 |
| Go 后端 | 端口 8090 | 端口 8093 | 独立 systemd 服务 |
| 部署目录 | /opt/eqs | /opt/tsloms | 独立目录 |
| MQTT Broker | - | 独立部署 | TSLOMS 专属（EMQX 或 Mosquitto） |

MQTT Broker 是 TSLOMS 特有的基础设施，EQS 项目不使用。可在同一台服务器上部署 EMQX（Docker 方式）或使用云厂商托管 MQTT 服务。

### 6.3 项目结构

```
tsloms/
├── docs/                           # 项目文档
│   ├── PRD-TSLOMS-v1.0*.md         # V1.0 历史版本
│   ├── PRD-TSLOMS-v2.0.md          # V2.0 当前版本
│   └── 信号灯设备通信协议第三版本.pdf
├── packages/
│   ├── server/                     # Go 后端
│   │   ├── cmd/
│   │   │   └── server/
│   │   │       └── main.go         # 程序入口
│   │   ├── internal/
│   │   │   ├── config/             # 配置管理（环境变量）
│   │   │   │   └── config.go
│   │   │   ├── handler/            # HTTP 请求处理器
│   │   │   │   ├── auth.go         # 登录鉴权
│   │   │   │   ├── device.go       # 设备管理
│   │   │   │   ├── fault.go        # 故障查询
│   │   │   │   ├── workorder.go    # 工单管理
│   │   │   │   ├── dashboard.go    # 数据看板
│   │   │   │   └── firmware.go     # 固件管理
│   │   │   ├── middleware/         # 中间件
│   │   │   │   ├── auth.go         # JWT 鉴权
│   │   │   │   ├── cors.go         # 跨域
│   │   │   │   └── logger.go       # 请求日志
│   │   │   ├── model/              # 数据模型
│   │   │   │   ├── db.go           # 数据库初始化
│   │   │   │   ├── device.go       # 设备模型
│   │   │   │   ├── packet_log.go   # 报文日志模型
│   │   │   │   ├── fault.go        # 故障记录模型
│   │   │   │   ├── workorder.go    # 工单模型
│   │   │   │   └── migrate.go      # 自动迁移
│   │   │   ├── mqtt/               # MQTT 通信层
│   │   │   │   ├── client.go       # MQTT 客户端管理
│   │   │   │   ├── parser.go       # 二进制协议解析
│   │   │   │   ├── handler.go      # 消息分发处理器
│   │   │   │   └── commands.go     # 命令帧定义与构造
│   │   │   └── service/            # 业务逻辑层
│   │   │       ├── fault.go        # 故障研判逻辑
│   │   │       └── workorder.go    # 工单生成逻辑
│   │   ├── migrations/
│   │   │   └── 001_init.sql        # 初始化 SQL
│   │   ├── Dockerfile
│   │   ├── Makefile
│   │   ├── go.mod
│   │   └── go.sum
│   └── admin/                      # Vue3 管理后台
│       ├── src/
│       │   ├── views/
│       │   │   ├── dashboard/      # 数据看板
│       │   │   ├── device/         # 设备管理
│       │   │   ├── fault/          # 故障查询
│       │   │   ├── workorder/      # 工单管理
│       │   │   ├── firmware/       # 固件管理
│       │   │   ├── log/            # 日志查询
│       │   │   └── settings/       # 系统设置
│       │   ├── router/
│       │   ├── store/
│       │   └── utils/
│       ├── package.json
│       └── vite.config.js
├── deploy/                         # 部署配置
│   ├── docker-compose.yml          # 开发环境
│   ├── docker-compose.prod.yml     # 生产环境
│   ├── nginx/
│   │   └── tsloms.conf             # Nginx 配置
│   ├── systemd/
│   │   └── tsloms-server.service   # systemd 服务
│   ├── scripts/
│   │   └── deploy.sh               # 部署脚本
│   └── .env.example                # 环境变量模板
├── .gitignore
└── AGENTS.md                       # 项目开发规范
```

### 6.4 配置管理

环境变量配置（参考 EQS 项目模式），通过 `os.Getenv` 读取，`sync.Once` 缓存：

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| SERVER_PORT | 8093 | 后端服务端口 |
| APP_ENV | development | 运行环境 |
| DB_DRIVER | mysql | 数据库驱动（mysql/sqlite） |
| DB_HOST | localhost | MySQL 主机 |
| DB_PORT | 3306 | MySQL 端口 |
| DB_USER | root | 数据库用户 |
| DB_PASSWORD | root | 数据库密码 |
| DB_NAME | tsloms | 数据库名（与 EQS 的 eqs 隔离） |
| REDIS_ADDR | localhost:6379 | Redis 地址（与 EQS 共享） |
| REDIS_PASS | | Redis 密码 |
| REDIS_DB | 1 | Redis DB 索引（EQS 使用 0） |
| JWT_SECRET | tsloms-secret-key | JWT 签名密钥 |
| MQTT_BROKER | tcp://localhost:1883 | MQTT Broker 地址 |
| MQTT_USERNAME | tsloms | MQTT 用户名 |
| MQTT_PASSWORD | | MQTT 密码 |
| MQTT_CLIENT_ID | tsloms-server | MQTT 客户端 ID |
| MQTT_TOPIC_PREFIX | trafficLight | Topic 前缀 |

## 七、核心数据模型

### 7.1 设备表（devices）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键自增 |
| hw_id | uint32 | 设备硬件 ID（出厂唯一，来自 ledHwId） |
| intersection | varchar(128) | 路口位置描述 |
| network_code | int | 网络号 |
| station_code | int | 站点号 |
| sw_version | uint32 | 固件版本号 |
| conf_version | uint32 | 配置版本号 |
| online_status | bool | 在线状态 |
| last_checkin_at | datetime | 最后签到时间 |
| installed_at | date | 安装日期 |
| created_at | datetime | 创建时间 |
| updated_at | datetime | 更新时间 |

### 7.2 报文日志表（packet_logs）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键自增 |
| device_hw_id | uint32 | 设备硬件 ID |
| raw_data | blob | 原始二进制报文 |
| cmd_type | uint8 | 命令类型 |
| cmd_seq | uint16 | 包序号 |
| parsed_result | json | 解析结果 JSON |
| valid | bool | 校验是否通过 |
| received_at | datetime | 接收时间 |

报文日志数据量大，按月分区或定期归档。

### 7.3 故障记录表（fault_records）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键自增 |
| device_hw_id | uint32 | 设备硬件 ID |
| err_code | int8 | 错误码（-1 至 -14） |
| fault_type | varchar(32) | 故障类型分类 |
| fault_level | enum | critical / normal |
| led_state | int8 | 故障时灯组状态 |
| current_r | uint16 | 红灯电流值 |
| current_y | uint16 | 黄灯电流值 |
| current_g | uint16 | 绿灯电流值 |
| first_seen | datetime | 首次出现时间 |
| last_seen | datetime | 最后出现时间 |
| status | enum | active / resolved |
| work_order_id | uint | 关联工单 ID |

### 7.4 工单表（work_orders）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键自增 |
| order_no | varchar(32) | 工单编号（WO{yyyyMMdd}{seq}） |
| fault_id | uint | 关联故障记录 ID |
| device_hw_id | uint32 | 设备硬件 ID |
| status | enum | pending / processing / completed / rejected |
| assignee_id | uint | 处理人 ID |
| result | text | 维修结果说明 |
| created_at | datetime | 创建时间 |
| closed_at | datetime | 闭环时间 |

### 7.5 用户表（users）

| 字段 | 类型 | 说明 |
|------|------|------|
| id | uint | 主键自增 |
| username | varchar(64) | 用户名 |
| password_hash | varchar(255) | 密码哈希（bcrypt） |
| role | enum | admin / operator / viewer |
| phone | varchar(20) | 手机号 |
| created_at | datetime | 创建时间 |

## 八、部署架构

### 8.1 开发环境

```yaml
# docker-compose.yml
services:
  mysql:          # MySQL 8.0（可与 EQS 共用容器）
    image: mysql:8.0
    ports: ["3306:3306"]
    environment:
      MYSQL_ROOT_PASSWORD: root
      MYSQL_DATABASE: tsloms
  redis:          # Redis 7.0（可与 EQS 共用容器）
    image: redis:7.0
    ports: ["6379:6379"]
  emqx:           # MQTT Broker
    image: emqx/emqx:5.0
    ports: ["1883:1883", "18083:18083"]
```

开发环境使用 SQLite + 本地 Redis 降级模式（参考 EQS 的 SQLite 模式），无需 Docker 即可运行。

### 8.2 生产环境

部署在与 EQS 相同的腾讯云 CVM 上：

```
                    ┌─── Nginx (8091) ── EQS 后端 (8090)
云服务器 CVM ───────┤
                    ├─── Nginx (8092) ── TSLOMS 后端 (8093)
                    ├─── MySQL 8.0 (3306)
                    │      ├─ eqs 数据库
                    │      └─ tsloms 数据库
                    ├─── Redis 7.0 (6379)
                    │      ├─ DB 0: EQS
                    │      └─ DB 1: TSLOMS
                    └─── EMQX (1883/18083) ── TSLOMS 专属
```

systemd 服务配置（参考 EQS 的 `eqs-server.service`）：

```ini
[Unit]
Description=TSLOMS Server
After=network.target mysql.service redis.service

[Service]
Type=simple
User=tsloms
Group=tsloms
WorkingDirectory=/opt/tsloms/packages/server
ExecStart=/opt/tsloms/packages/server/server
Restart=always
RestartSec=5
KillSignal=SIGTERM
TimeoutStopSec=15
Environment=SERVER_PORT=8093
Environment=APP_ENV=production
Environment=DB_DRIVER=mysql
Environment=DB_HOST=localhost
Environment=DB_PORT=3306
Environment=DB_USER=tsloms
Environment=DB_NAME=tsloms
Environment=REDIS_ADDR=localhost:6379
Environment=REDIS_DB=1
Environment=MQTT_BROKER=tcp://localhost:1883

[Install]
WantedBy=multi-user.target
```

### 8.3 Nginx 配置

```nginx
server {
    listen 8092;
    server_name _;

    client_max_body_size 50m;

    # 管理后台
    location /admin {
        alias /opt/tsloms/packages/admin/dist;
        index index.html;
        try_files $uri $uri/ /admin/index.html;
    }

    # API 代理
    location /api {
        proxy_pass http://127.0.0.1:8093;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## 九、V2.0 迭代范围

**本期实现**：MQTT 设备通信与二进制协议解析、故障自动研判与去重、工单全流程管理、设备台账管理、设备配置下发、ECharts 可视化统计、JWT 权限管理

**后续迭代**：移动端 APP、短信告警通知、多级审批流程、固件 OTA 升级完整流程、大数据深度分析、设备地图可视化
