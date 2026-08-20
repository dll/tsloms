#!/usr/bin/env bash
# 在生产服务器本机创建 EMQX 检测器专用账号。
# 凭据仅通过环境变量/交互输入传递，不写入命令行参数、日志或仓库。
set -euo pipefail

USER_ID="${1:-}"
if [[ -z "${USER_ID}" ]]; then
  read -r -p '检测器 MQTT 用户名（建议 <=8 位）: ' USER_ID
fi
[[ "${USER_ID}" =~ ^[A-Za-z0-9_.-]{1,64}$ ]] || { echo '用户名格式无效' >&2; exit 2; }

if [[ -z "${EMQX_API_TOKEN:-}" ]]; then
  echo '请先通过 EMQX Dashboard/API 登录获取 EMQX_API_TOKEN（仅当前 shell 有效）' >&2
  exit 2
fi
read -r -s -p '检测器 MQTT 密码（按设备固件限制，建议至少8位）: ' MQTT_DEVICE_PASSWORD; echo
read -r -s -p '再次输入密码: ' MQTT_DEVICE_PASSWORD_CONFIRM; echo
[[ "${MQTT_DEVICE_PASSWORD}" == "${MQTT_DEVICE_PASSWORD_CONFIRM}" ]] || { echo '两次密码不一致' >&2; exit 2; }
export USER_ID MQTT_DEVICE_PASSWORD

python3 - <<'PY' | curl -fsS -X POST 'http://127.0.0.1:18083/api/v5/authentication/password_based:built_in_database/users' \
  -H "Authorization: Bearer ${EMQX_API_TOKEN}" -H 'Content-Type: application/json' --data-binary @-
import json, os
print(json.dumps({'user_id': os.environ['USER_ID'], 'password': os.environ['MQTT_DEVICE_PASSWORD']}))
PY
printf '检测器账号已创建：%s（密码不会回显，请通过受控渠道交付）\n' "${USER_ID}"
unset MQTT_DEVICE_PASSWORD MQTT_DEVICE_PASSWORD_CONFIRM EMQX_API_TOKEN
