#!/usr/bin/env bash
# =============================================================================
# TSLOMS 服务器本机 E2E 冒烟（避免经受 CI runner -> 服务器 的入口网络，
# 由 E2E workflow 通过 SSH 方式调用，直接在服务器 localhost 上探活）。
#
# 依赖：curl、jq（可选，无 jq 时用 sed/grep 兜底）、node（存在时执行 smoke.js 全量，
#        否则退化为 curl 最小冒烟）。
# 用法（由 e2e.yml 传到服务器后执行）：
#   bash smoke-local.sh
# 只读校验：/health、登录(算术验证码)、/dashboard/overview、一个只读业务接口。
# 凭据仅在脚本运行时通过环境变量注入（不写死、不落盘、不打印）。
# =============================================================================
set -Eeuo pipefail

BASE="${E2E_LOCAL_BASE:-http://127.0.0.1:8093/api/v1}"
USERNAME="${E2E_ADMIN_USER:-admin}"
# 密码只从环境变量读取；为空则跳过登录类校验（至少保证 health 可达）。
PASSWORD="${E2E_ADMIN_PASS:-}"

echo "[smoke-local] base=${BASE} user=${USERNAME}"
echo "[smoke-local] 步骤1/5 健康检查"
curl -fsS --max-time 10 "${BASE}/health" >/dev/null || { echo "[smoke-local] FAIL health"; exit 1; }
echo "[smoke-local] PASS health"

if [ -z "${PASSWORD}" ]; then
  echo "[smoke-local] 未注入 E2E_ADMIN_PASS，仅完成健康检查（提示：需在环境/Secret 提供只读探针账号以覆盖登录路径）"
  exit 0
fi

# 取算术验证码并解算（复用后端 GetCaptcha 的公开接口）
CAPTCHA=$(curl -fsS --max-time 10 "${BASE}/auth/captcha")
UUID=$(printf '%s' "${CAPTCHA}" | sed -n 's/.*"uuid"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
QUESTION=$(printf '%s' "${CAPTCHA}" | sed -n 's/.*"question"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ -z "${UUID}" ] || [ -z "${QUESTION}" ]; then
  echo "[smoke-local] FAIL 获取/解析验证码"; exit 1
fi
# 解算式 'a + b = ?' / 'a - b = ?'
IFS=' ' read -ra Q <<< "${QUESTION}"
# Q 形如: ["2","+","8","=?"]  采用参数化计算（表达式由服务器提供，仅做加法/减法）
A="${Q[0]}"; OP="${Q[1]}"; B="${Q[2]}"
if [ "${OP}" = "-" ]; then
  CODE=$((10#$A - 10#$B))
else
  CODE=$((10#$A + 10#$B))
fi

echo "[smoke-local] 步骤2/5 登录"
# 用 jq 做标准 JSON 转义（用户名/密码可含特殊字符）；无 jq 则退化为 base64 注入。
if command -v jq >/dev/null 2>&1; then
  BODY=$(jq -n --arg u "${USERNAME}" --arg p "${PASSWORD}" \
    --arg cu "${UUID}" --arg cc "${CODE}" \
    '{username:$u,password:$p,captcha_uuid:$cu,captcha_code:$cc}')
else
  # 无 jq：仅转义双引号/反斜杠（探针账号一般不含特殊字符）
  P_ESC=${PASSWORD//\\/\\\\}; P_ESC=${P_ESC//\"/\\\"}
  U_ESC=${USERNAME//\\/\\\\}; U_ESC=${U_ESC//\"/\\\"}
  BODY="{\"username\":\"${U_ESC}\",\"password\":\"${P_ESC}\",\"captcha_uuid\":\"${UUID}\",\"captcha_code\":\"${CODE}\"}"
fi
LOGIN=$(curl -fsS --max-time 10 -X POST "${BASE}/auth/login" \
  -H 'Content-Type: application/json' -d "${BODY}" || true)
TOKEN=$(printf '%s' "${LOGIN}" | sed -n 's/.*"token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ -z "${TOKEN}" ]; then
  echo "[smoke-local] FAIL 登录失败（status=$(printf '%s' "${LOGIN}" | sed -n 's/.*"code"[^0-9]*\([0-9]*\).*/code=\1/p' 2>/dev/null || echo '?')）"
  exit 1
fi
echo "[smoke-local] PASS 登录"

AUTH="Authorization: Bearer ${TOKEN}"

echo "[smoke-local] 步骤3/5 仪表盘"
ov=$(curl -fsS --max-time 10 -H "${AUTH}" "${BASE}/dashboard/overview")
printf '%s' "${ov}" | grep -qE '"code"[[:space:]]*:[[:space:]]*0' || { echo "[smoke-local] FAIL dashboard"; exit 1; }
echo "[smoke-local] PASS dashboard/overview"

echo "[smoke-local] 步骤4/5 只读业务接口：通知列表"
nt=$(curl -fsS --max-time 10 -H "${AUTH}" "${BASE}/notifications?limit=5")
printf '%s' "${nt}" | grep -qE '"code"[[:space:]]*:[[:space:]]*0' || { echo "[smoke-local] FAIL notifications"; exit 1; }
echo "[smoke-local] PASS notifications"

echo "[smoke-local] 步骤5/5 只读业务接口：设备列表"
dv=$(curl -fsS --max-time 10 -H "${AUTH}" "${BASE}/devices?limit=5")
printf '%s' "${dv}" | grep -qE '"code"[[:space:]]*:[[:space:]]*0' || { echo "[smoke-local] FAIL devices"; exit 1; }
echo "[smoke-local] PASS devices"

echo "[smoke-local] 全部通过 ✅"
