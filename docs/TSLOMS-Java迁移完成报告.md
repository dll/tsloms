# TSLOMS 后端 Java 迁移完成报告

**分支**：`feat/java-backend`　**版本**：tsloms-server-java 2.1.0
**日期**：2026-08-23

## 一、迁移范围（功能面对齐）

| 域 | 端点组 | 状态 |
|---|---|---|
| 认证 | login/register/captcha/user info/JWT 拦截 | ✅ |
| 组织权限 | users/departments/rbac/my-permissions + @RequirePerm | ✅ |
| 仪表盘 | overview/类型占比/状态分布/趋势/排行/闭环时长 | ✅ |
| 故障工单 | 列表筛选/详情/状态机/派单/删除/CSV/SLA/重激活 | ✅ |
| 固件 | 上传(MD5)/改版位域重算/发布/升级任务防重 | ✅ |
| 媒体 | 上传 MIME 嗅探/RTSP 登记/静态映射/删除清理 | ✅ |
| 物料库存 | 档案 CRUD/stats/流水/调整/领料自动费用 | ✅ |
| 供应商采购 | CRUD/部分入库/completed 联动/取消/草稿删除 | ✅ |
| 维修费用 | stats 分类合计+TOP 设备/CRUD/确认 | ✅ |
| 路口区划 | CRUD/设备列表/区划树 | ✅ |
| 预警 | 筛选/忽略/批量/转工单/auto-ignore/导出/MQTT 联动生成+规则忽略 | ✅ |
| **设备接入 MQTT** | 二进制协议(小端+校验和)逐字节对齐/签到上电告警/30min 去重/critical 自动建单/Paho QoS1 持久会话 | ✅ |
| 巡检 | 任务 CRUD/run 圈选/记录/排行/自检上报 | ✅ |
| 授权 | status/trial 30 天/unlock | ✅ |
| 识别数据面 | evidence sources/ingest/fault evidence/review(approve=确认+critical建单, reject=误报过滤+沉淀案例)/fault-cases train | ✅ |
| 反馈通知 | feedbacks/notifications(广播已读合并/unread-count/read/read-all) | ✅ |

**质量门禁**：107 测试全绿 · JaCoCo ≥80%（实测 80.3%）· CI 已接入 java-quality-gate

## 二、文档化差异（3 项）

1. **AI 多源研判引擎**：以确定性规则基座兜底（errCode→类型/等级→confirmed，置信度 1.0）；多源交叉验证/置信度融合为后续增强迭代。数据面（evidence/case/review）已就绪。
2. **License 解锁码**：简化为非空永久解锁；生产接入签发体系时收紧。
3. **packet_log 表**：Go 版报文日志落库由 Java slf4j 结构化日志替代（journalctl 可查）。

## 三、灰度部署方案（1 小时并行 → 切换）

### 拓扑
```
nginx :8092
  ├─ /tsloms/api → 127.0.0.1:8093 (Go, systemd tsloms-server)   ← 并行期承载流量
  └─ /java/api   → 127.0.0.1:8096 (Java, systemd tsloms-server-java) ← 灰度观察
切换后：/tsloms/api → 8096（Go 保留为回滚）
```

### 步骤（服务器：129.211.223.113）
1. **上传制品**：`scp packages/server-java/target/tsloms-server-java-2.1.0.jar root@host:/opt/tsloms-java/`
2. **JDK21**：服务器安装 `dnf install java-21-openjdk-headless` 或解压 Temurin 至 /opt/jdk21
3. **环境文件** `/etc/tsloms/java.env`：从现有 .env 复制 DB_*/REDIS/JWT_SECRET/MQTT_*（Java 读同一 MySQL，ddl-auto=none 只读映射）
4. **systemd 单元** `tsloms-server-java.service`：ExecStart=/usr/bin/java -jar …，User=tsloms
5. **启动并验证**：`curl 127.0.0.1:8096/api/v1/health` → code=0
6. **并行 1 小时**：CO 巡检同时探活 8093 与 8096；人工抽查登录/看板/故障列表响应一致性
7. **切换**：nginx 替换 api 上游 8093→8096，`nginx -s reload`
8. **回滚**（如异常）：上游改回 8093 reload 即可（数据同库无损）

### 切换脚本（服务器 /opt/tsloms-java/switch.sh）
```bash
#!/bin/bash
# 用法: switch.sh java|go
UP=$([ "$1" = "java" ] && echo "127.0.0.1:8096" || echo "127.0.0.1:8093")
sed -i "s#server 127.0.0.1:809[36]#server ${UP}#" /etc/nginx/conf.d/tsloms.conf
nginx -t && nginx -s reload && echo "switched to $1 ($UP)"
```

## 四、回滚与风险

- 同库双跑无数据迁移风险；Java 侧 ddl-auto=none 保证不改动表结构
- 回滚 = nginx 上游切回 Go（秒级）
- 已知差异见第二节；MQTT 层切换建议在设备低峰期进行
