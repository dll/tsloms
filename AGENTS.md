# TSLOMS 项目开发规范

## 项目简介

TSLOMS（Traffic Signal Light Operation and Maintenance System）是交通信号灯检测后台运维系统，基于 MQTT 协议对接信号灯设备，实现故障自动研判、维修工单流转、运维数据可视化统计。

## 语言规范

### 文档语言
- **所有文档内容**：使用中文
- **ASCII 图表**：使用英文，等宽字体，右边绝对对齐
- **图表注解**：使用中文

### 代码语言
- **代码注释**：使用中文
- **变量命名**：使用英文（遵循 Go/JavaScript 规范）
- **函数命名**：使用英文
- **提交信息**：使用中文

### 沟通语言
- **回复用户**：使用中文
- **代码审查**：使用中文
- **文档编写**：使用中文

## 文件组织

### PRD 文档
```
docs/
├── PRD-TSLOMS-v1.0 交通信号灯检测后台运维系统.md
├── PRD-TSLOMS-v2.0.md
└── 信号灯设备通信协议第三版本.pdf
```

### 命名规则
- 版本号格式：`PRD-TSLOMS-v{主版本}.{次版本}.md`
- 保留历史版本，不删除旧版本

## 开发规范

### Git 提交
- 提交信息格式：`类型: 简短描述`
- 类型：feat/fix/docs/refactor/test/chore
- 示例：`feat: 新增设备故障研判规则引擎`

### 代码风格
- Go：遵循 golangci-lint 规范
- JavaScript/TypeScript：遵循 ESLint 规范
- Vue：遵循 Vue.js 风格指南

### 测试要求
- 单元测试覆盖率：≥80%
- 集成测试：核心业务流程必须覆盖
- E2E 测试：关键用户路径必须覆盖

## 技术栈

- **后端**：Go 1.22+ / Gin / GORM / paho.mqtt.golang
- **前端**：Vue3 + Vite + Element Plus + ECharts
- **数据库**：MySQL 8.0
- **缓存**：Redis 7.0
- **消息队列**：EMQX 5.0（MQTT）
- **部署**：Docker + systemd
