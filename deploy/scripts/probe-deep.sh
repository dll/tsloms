#!/bin/bash
# =============================================================================
# TSLOMS 深度运维探针（CO-P1-01 / CO-P1-02）
#
# 在服务器本机执行，输出结构化检查结果。任一项失败以非零退出（供 CO 判断）。
# 凭据从 /etc/tsloms/tsloms.env 读取（MQTT / MySQL / Redis），绝不打印敏感值。
#
# 检查项：
#   L1  systemd 服务 active + 最近 journal 错误量
#   L2  MySQL 连接（本机 tcp 32766 或 3306）
#   L3  Redis PING
#   L4  磁盘使用率（/ 与 /opt/tsloms）
#   L5  备份年龄（/opt/tsloms/backups/db 最近文件是否在阈值内）
#   L6  制品目录健康（current 链接可解析、release 存在）
#   L7  MQTT 认证探针（CONNECT/PUBLISH/SUBSCRIBE 回环）——需 MQTT 命令行工具或由脚本用 bash /dev/tcp 实现
# =============================================================================
set -uo pipefail

ENV_FILE="/etc/tsloms/tsloms.env"
SERVICE="tsloms-server"
ROOT="/opt/tsloms"
RC=0

log() { echo "[probe] $*"; }
note() { echo "[probe] PASS $*"; }
warn() { echo "[probe] WARN $*"; }
fail() { echo "[probe] FAIL $*"; RC=1; }

# 读取 env 值（不回显敏感内容）
getenv() { grep "^$1=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2-; }

echo "==== TSLOMS 深度探针 $(date -Is) ===="

# ---------- L1: systemd ----------
if systemctl is-active --quiet "$SERVICE"; then
  note "systemd $SERVICE active"
else
  fail "systemd $SERVICE 非 active"
fi
# 最近 10 分钟 journal error 计数
ERR_CNT=$(journalctl -u "$SERVICE" --since "10 min ago" --priority err --no-pager 2>/dev/null | wc -l)
if [ "$ERR_CNT" -le 5 ]; then
  note "journal 近10分钟 error 数=$ERR_CNT"
else
  warn "journal 近10分钟 error 数=$ERR_CNT（偏高）"
fi

# ---------- L2: MySQL ----------
DB_HOST=$(getenv DB_HOST); DB_PORT=$(getenv DB_PORT); DB_USER=$(getenv DB_USER); DB_NAME=$(getenv DB_NAME)
DB_PASSWORD=$(getenv DB_PASSWORD)
DB_HOST="${DB_HOST:-127.0.0.1}"; DB_PORT="${DB_PORT:-3306}"
if [ -n "$DB_PASSWORD" ] && command -v mysql >/dev/null 2>&1; then
  if MYSQL_PWD="$DB_PASSWORD" timeout 8 mysql -h"$DB_HOST" -P"$DB_PORT" -u"${DB_USER:-root}" -N -e "SELECT 1" "$DB_NAME" >/dev/null 2>&1; then
    note "MySQL 连接 OK"
  else
    fail "MySQL 连接失败"
  fi
else
  warn "跳过 MySQL 检查（无凭据或 mysql 客户端缺失）"
fi

# ---------- L3: Redis ----------
REDIS_ADDR=$(getenv REDIS_ADDR); REDIS_PASS=$(getenv REDIS_PASS)
REDIS_ADDR="${REDIS_ADDR:-127.0.0.1:6379}"
RHOST="${REDIS_ADDR%%:*}"; RPORT="${REDIS_ADDR##*:}"
if command -v redis-cli >/dev/null 2>&1; then
  if [ -n "$REDIS_PASS" ]; then
    R_OK=$(timeout 6 redis-cli -h "$RHOST" -p "$RPORT" -a "$REDIS_PASS" PING 2>/dev/null)
  else
    R_OK=$(timeout 6 redis-cli -h "$RHOST" -p "$RPORT" PING 2>/dev/null)
  fi
  if [ "$R_OK" = "PONG" ]; then
    note "Redis PING OK"
  else
    fail "Redis PONG 失败（got '$R_OK'）"
  fi
else
  warn "跳过 Redis 检查（redis-cli 缺失）"
fi

# ---------- L4: 磁盘 ----------
for mp in / /opt/tsloms; do
  if [ -d "$mp" ]; then
    USED=$(df -P "$mp" 2>/dev/null | awk 'NR==2{print $5}' | tr -d '%')
    if [ -n "$USED" ] && [ "$USED" -gt 90 ]; then
      fail "磁盘 $mp 使用率 ${USED}%（>90%）"
    else
      note "磁盘 $mp 使用率 ${USED:-?}%"
    fi
  fi
done

# ---------- L5: 备份年龄 ----------
BACKUP_DIR="$ROOT/backups/db"
if [ -d "$BACKUP_DIR" ]; then
  NEWEST=$(find "$BACKUP_DIR" -maxdepth 1 -type f 2>/dev/null -printf '%T@ %p\n' | sort -rn | head -1)
  if [ -n "$NEWEST" ]; then
    TS=$(echo "$NEWEST" | cut -d' ' -f1 | cut -d. -f1)
    NOW=$(date +%s)
    AGE=$((NOW - TS))
    if [ "$AGE" -gt 172800 ]; then  # 48 小时
      fail "最近数据库备份在 ${AGE} 秒前（>48h）"
    else
      note "最近数据库备份 ${AGE} 秒前"
    fi
  else
    fail "backups/db 下无备份文件"
  fi
else
  fail "备份目录 $BACKUP_DIR 不存在"
fi

# ---------- L6: 制品目录 ----------
CURRENT="$ROOT/current"
if [ -L "$CURRENT" ] && [ -d "$(readlink -f "$CURRENT")" ]; then
  note "current 指向 $(readlink -f "$CURRENT")"
else
  fail "current 符号链接异常"
fi

# ---------- L7: MQTT 认证探针 ----------
# 需 MQTT 命令行客户端（mosquitto_pub / mosquitto_sub）。凭据从 env 读取。
MQTT_BROKER=$(getenv MQTT_BROKER); MQTT_USERNAME=$(getenv MQTT_USERNAME); MQTT_PASSWORD=$(getenv MQTT_PASSWORD)
MQTT_TOPIC_PREFIX=$(getenv MQTT_TOPIC_PREFIX); MQTT_TOPIC_PREFIX="${MQTT_TOPIC_PREFIX:-trafficLight}"
PROBE_TOPIC="${MQTT_TOPIC_PREFIX}/probe/co/$$"
if command -v mosquitto_pub >/dev/null 2>&1 && command -v mosquitto_sub >/dev/null 2>&1 && [ -n "$MQTT_BROKER" ]; then
  BROKER_HOST=$(echo "$MQTT_BROKER" | sed -E 's#(tcp|ssl|mqtt|mqtts)://##')
  # 用后台 sub 订阅探针 topic，再 pub 一条，验证回环
  PAYLOAD="co-probe-$$-$(date +%s)"
  timeout 10 mosquitto_sub -h "$BROKER_HOST" -p 1883 -u "$MQTT_USERNAME" -P "$MQTT_PASSWORD" -t "$PROBE_TOPIC" -C 1 -W 8 &
  SUB_PID=$!
  sleep 1
  timeout 6 mosquitto_pub -h "$BROKER_HOST" -p 1883 -u "$MQTT_USERNAME" -P "$MQTT_PASSWORD" -t "$PROBE_TOPIC" -m "$PAYLOAD" 2>/dev/null
  RECV=$(timeout 12 wait "$SUB_PID" 2>/dev/null)
  if [ -n "$RECV" ] && [ "$RECV" = "$PAYLOAD" ]; then
    note "MQTT CONNECT/PUBLISH/SUBSCRIBE 认证回环 OK"
  else
    fail "MQTT 认证/消息链路失败（收不到探针回环）"
  fi
else
  warn "跳过 MQTT 深度探针（mosquitto 客户端缺失或未配置 broker）"
fi

echo "==== 探针结束 RC=$RC ===="
exit $RC
