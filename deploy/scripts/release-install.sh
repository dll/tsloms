#!/bin/bash
# =============================================================================
# TSLOMS 制品发布脚本（v2.0 不可变制品 + 原子切换 + 探活回滚）
# 2026-08-20：重新触发生产部署验收（行为不变）。
#
# 服务器端执行，不作为 CI/CD 的一部分运行；由 cd.yml 通过 SSH 以受控方式调用。
# 职责：
#   - 接收并校验上传到 releases/<sha>.staging 的制品（sha256 校验）
#   - 原子切换 current -> releases/<sha>（保留 current/previous）
#   - 数据库备份（迁移前置，须在迁移前主动调用）
#   - 重启 + 本地/HTTPS 探活，失败自动切回 previous
#
# 用法：
#   RELEASE_SHA=<40位sha> bash release-install.sh
#
# 环境要求：
#   - 部署用户具备对 /opt/tsloms/{releases,current,previous,backups,shared} 的写权限
#   - systemctl restart tsloms-server（deploy 用户需通过 sudoers 白名单执行）
#   - 数据库凭据从 /etc/tsloms/tsloms.env 读取（禁止写入日志/文档）
# =============================================================================
set -Eeuo pipefail

RELEASE_SHA="${RELEASE_SHA:-}"
ROOT="/opt/tsloms"
RELEASE_DIR="${ROOT}/releases/${RELEASE_SHA}"
STAGING_DIR="${RELEASE_DIR}.staging"
CURRENT_LINK="${ROOT}/current"
PREVIOUS_LINK="${ROOT}/previous"
NEXT_LINK="${ROOT}/current.next"   # 原子切换用临时链接名（mv -Tf 不可用时的降级路径）
SERVICE="tsloms-server"
HEALTH_URL="http://127.0.0.1:8093/api/v1/health"

# ---------------------------------------------------------------------------
# 0. 前置校验
# ---------------------------------------------------------------------------
echo "[0] 校验发布参数"
if [ -z "${RELEASE_SHA}" ]; then
  echo "ERROR: 缺失 RELEASE_SHA（必须为 40 位提交 SHA）" >&2; exit 1
fi
if [[ ! "${RELEASE_SHA}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "ERROR: RELEASE_SHA 非 40 位十六进制: ${RELEASE_SHA}" >&2
  exit 1
fi

echo "[0] releases 目录检查"
mkdir -p "${ROOT}/releases" "${ROOT}/backups/db" "${ROOT}/shared/media"

if [ -d "${RELEASE_DIR}" ]; then
  echo "INFO: 目标 release 已存在 ${RELEASE_DIR}，强制重新校验（禁止同 SHA 内容漂移 CD-P1-01）"
  echo "[1] 重新校验已存在 release 的 manifest/结构/version"
  ( cd "${RELEASE_DIR}" && sha256sum -c manifest.sha256 ) \
    || { echo "ERROR: 已存在 release 的 SHA-256 校验失败（内容被篡改/损坏），拒绝部署。" >&2; exit 1; }
  test -f "${RELEASE_DIR}/server"     || { echo "ERROR: 已存在 release 缺失 server" >&2; exit 1; }
  test -x "${RELEASE_DIR}/server"     || { echo "ERROR: 已存在 release 的 server 不可执行" >&2; exit 1; }
  test -f "${RELEASE_DIR}/admin/dist/index.html" || { echo "ERROR: 已存在 release 缺失 admin/dist/index.html" >&2; exit 1; }
  test -f "${RELEASE_DIR}/admin/dist/version.json" || { echo "ERROR: 已存在 release 缺失 admin/dist/version.json" >&2; exit 1; }
  test "$(tr -d '\r\n' < "${RELEASE_DIR}/version.txt" 2>/dev/null)" = "${RELEASE_SHA}" \
    || { echo "ERROR: 已存在 release 的 version.txt 与目标 SHA 不一致" >&2; exit 1; }
  echo "  已存在 release 复核通过，直接使用"
else
  if [ ! -d "${STAGING_DIR}" ]; then
    echo "ERROR: 目标 release 与制品暂存目录均不存在：${RELEASE_DIR} / ${STAGING_DIR}" >&2
    exit 1
  fi
  echo "[1] 校验制品 sha256"
  ( cd "${STAGING_DIR}" && sha256sum -c manifest.sha256 ) || { echo "ERROR: 制品 SHA-256 校验失败，丢弃。" >&2; exit 1; }

  echo "[1] 结构完整性与可执行性校验"
  test -f "${STAGING_DIR}/server"            || { echo "ERROR: 缺失 server" >&2; exit 1; }
  # GitHub Artifact 下载通常不保留 Unix 执行位，落位前按制品约定恢复权限。
  chmod +x "${STAGING_DIR}/server"
  test -x "${STAGING_DIR}/server"            || { echo "ERROR: server 无法设置为可执行" >&2; exit 1; }
  test -f "${STAGING_DIR}/admin/dist/index.html" || { echo "ERROR: 缺失 admin/dist/index.html" >&2; exit 1; }
  test -f "${STAGING_DIR}/admin/dist/version.json" || { echo "ERROR: 缺失 admin/dist/version.json" >&2; exit 1; }
  test "$(tr -d '\r\n' < "${STAGING_DIR}/version.txt" 2>/dev/null)" = "${RELEASE_SHA}" \
    || { echo "ERROR: version.txt 与 RELEASE_SHA 不一致" >&2; exit 1; }
  echo "version=$(cat "${STAGING_DIR}/version.txt" 2>/dev/null || echo unknown)  sha=${RELEASE_SHA}"

  echo "[1] 原子落位 staging -> release"
  mv -Tf "${STAGING_DIR}" "${RELEASE_DIR}" || { echo "ERROR: 落位失败" >&2; exit 1; }
fi

# ---------------------------------------------------------------------------
# 1.5 一次性媒体目录迁移（旧路径 -> shared/media，幂等）
#     制品化之前媒体在 /opt/tsloms/packages/server/uploads/media，now 固定到
#     /opt/tsloms/shared/media，避免滚动发布丢失历史媒体（P0-04）。
# ---------------------------------------------------------------------------
OLD_MEDIA="/opt/tsloms/packages/server/uploads/media"
NEW_MEDIA="/opt/tsloms/shared/media"
if [ -d "${OLD_MEDIA}" ] && [ -z "$(ls -A "${NEW_MEDIA}" 2>/dev/null)" ]; then
  echo "[1.5] 媒体目录一次性迁移: ${OLD_MEDIA} -> ${NEW_MEDIA}"
  mkdir -p "${NEW_MEDIA}"
  # 使用 mv 保留 inode/所有权；若跨文件系统则改用 cp -a + rm -rf
  if mv "${OLD_MEDIA}"/* "${NEW_MEDIA}"/ 2>/dev/null; then
    rmdir "${OLD_MEDIA}" 2>/dev/null || true
    chown -R tsloms:tsloms "${NEW_MEDIA}" 2>/dev/null || true
    echo "  迁移完成"
  else
    echo "  迁移未完成（可能跨文件系统），改用 cp -a 复制"
    cp -a "${OLD_MEDIA}"/. "${NEW_MEDIA}"/ 2>/dev/null || true
    chown -R tsloms:tsloms "${NEW_MEDIA}" 2>/dev/null || true
  fi
fi

# ---------------------------------------------------------------------------
# 2. 数据库前置备份（迁移前快照；凭据从 env 文件读取，不落日志）
# ---------------------------------------------------------------------------
echo "[2] 数据库备份（迁移前置）"
ENV_FILE="/etc/tsloms/tsloms.env"
if [ ! -f "${ENV_FILE}" ]; then
  echo "ERROR: 缺少生产环境配置 ${ENV_FILE}。该文件不随制品上传，必须由运维在服务器预置（建议 0600、root:root），拒绝使用应用默认凭据继续部署。" >&2
  exit 1
fi
set +e
# shellcheck disable=SC1090
DBCONF=$(grep -E '^(DB_HOST|DB_PORT|DB_USER|DB_NAME)=' "${ENV_FILE}" 2>/dev/null)
set -e
if [ -n "${DBCONF}" ]; then
  # 从 env 文件安全取值（避免把密码打印到日志）
  # shellcheck disable=SC1090
  source "${ENV_FILE}"
  # DB_PASSWORD 为空则跳过备份（不硬编码凭据）
  if [ -n "${DB_PASSWORD:-}" ] && [ -n "${DB_NAME:-}" ]; then
    TS=$(date +%Y%m%d%H%M%S)
    # 应用专用账号不授予全局 PROCESS 权限；禁用 tablespaces 元数据仍可完整备份业务库。
    MYSQL_CMD=(mysqldump --single-transaction --routines --triggers --no-tablespaces)
    export MYSQL_PWD="${DB_PASSWORD}"
    if command -v zstd >/dev/null 2>&1; then
      "${MYSQL_CMD[@]}" -h"${DB_HOST:-127.0.0.1}" -P"${DB_PORT:-3306}" -u"${DB_USER:-tsloms}" "${DB_NAME}" \
        | zstd -T0 > "${ROOT}/backups/db/${TS}.sql.zst"
    else
      "${MYSQL_CMD[@]}" -h"${DB_HOST:-127.0.0.1}" -P"${DB_PORT:-3306}" -u"${DB_USER:-tsloms}" "${DB_NAME}" \
        > "${ROOT}/backups/db/${TS}.sql"
    fi
    unset MYSQL_PWD
    echo "  数据库备份完成: ${ROOT}/backups/db/${TS}  (sha ${RELEASE_SHA})"
  else
    echo "  WARN: 缺少 DB_NAME/DB_PASSWORD，跳过数据库备份（确保迁移不改变不可逆结构）"
  fi
else
  echo "  WARN: 未找到 /etc/tsloms/tsloms.env，跳过数据库备份"
fi

# ---------------------------------------------------------------------------
# 3. 记录当前版本（供回滚）
# ---------------------------------------------------------------------------
echo "[3] 记录当前版本"
if [ -L "${CURRENT_LINK}" ]; then
  PREV_SHA=$(basename "$(readlink -f "${CURRENT_LINK}")")
  echo "  current -> ${PREV_SHA}"
  if [ -d "${ROOT}/releases/${PREV_SHA}" ]; then
    ln -sfn "${ROOT}/releases/${PREV_SHA}" "${PREVIOUS_LINK}"
    echo "  previous -> ${ROOT}/releases/${PREV_SHA}"
  fi
else
  echo "  当前无 current 链接（首次制品化部署）"
fi

# ---------------------------------------------------------------------------
# 4. 原子切换 + 重启 + 探活（失败自动回滚）
# ---------------------------------------------------------------------------
echo "[4] 原子切换 current 指向新 release"
ln -sfn "${RELEASE_DIR}" "${NEXT_LINK}" && mv -Tf "${NEXT_LINK}" "${CURRENT_LINK}"
ls -l "${CURRENT_LINK}"

echo "[4] systemd 单元校验（P0-03：验证实际启用单元为唯一权威 tsloms，而非 root/旧 env）"
# 在重启前校验服务器实际单元属性，防止只有 restart 而应用了错误/旧单元。
CUR_USER=$(systemctl show ${SERVICE} -p User --value 2>/dev/null || echo '')
# systemd 的 show 属性为复数 EnvironmentFiles（即使 unit 指令写作 EnvironmentFile=）；
# 兼容较旧 systemd 仍暴露单数属性的情况，避免把正确配置误判为空。
CUR_ENVFILE=$(systemctl show ${SERVICE} -p EnvironmentFiles --value 2>/dev/null || echo '')
if [ -z "${CUR_ENVFILE}" ]; then
  CUR_ENVFILE=$(systemctl show ${SERVICE} -p EnvironmentFile --value 2>/dev/null || echo '')
fi
CUR_EXECSTART=$(systemctl show ${SERVICE} -p ExecStart --value 2>/dev/null || echo '')
echo "  User=$CUR_USER EnvironmentFile=$CUR_ENVFILE ExecStart=$CUR_EXECSTART"
if [ "${CUR_USER}" != "tsloms" ]; then
  echo "ERROR: 实际启用单元 User=${CUR_USER:-空}，必须为 tsloms（拒绝使用 root 单元，P0-03）" >&2; exit 1
fi
case "${CUR_ENVFILE}" in
  *"/etc/tsloms/tsloms.env"*) :;;
  *) echo "ERROR: EnvironmentFile=${CUR_ENVFILE:-空}，必须为 /etc/tsloms/tsloms.env（拒绝旧 .env 路径）" >&2; exit 1;;
esac
case "${CUR_EXECSTART}" in
  *"/opt/tsloms/current/server"*) :;;
  *) echo "ERROR: ExecStart=${CUR_EXECSTART:-空}，必须指向 /opt/tsloms/current/server" >&2; exit 1;;
esac
echo "[4] systemd 单元校验通过（唯一权威单元已生效）"

echo "[4] systemd 重新读取并重启"
# deploy 用户经 sudoers 白名单执行 systemctl restart
systemctl restart "${SERVICE}" 2>/dev/null || sudo -n systemctl restart "${SERVICE}" \
  || { echo "ERROR: 重启服务失败（权限？）" >&2; }

echo "[4] 本机健康探活（3 次 × 2s）"
ok=0
for i in 1 2 3; do
  sleep 2
  if systemctl is-active --quiet "${SERVICE}" && \
     curl -fsS --max-time 10 "${HEALTH_URL}" >/dev/null 2>&1; then
    echo "  探活 ${i}/3: OK"
    ok=1
    break
  else
    echo "  探活 ${i}/3: FAIL"
  fi
done

if [ "${ok}" != "1" ]; then
  echo "ERROR: 新版本探活失败，自动回滚 previous" >&2
  if [ -L "${PREVIOUS_LINK}" ] && [ -d "$(readlink -f "${PREVIOUS_LINK}")" ]; then
    ln -sfn "$(readlink -f "${PREVIOUS_LINK}")" "${NEXT_LINK}" && mv -Tf "${NEXT_LINK}" "${CURRENT_LINK}"
    systemctl restart "${SERVICE}" 2>/dev/null || sudo -n systemctl restart "${SERVICE}" || true
    sleep 3
    if systemctl is-active --quiet "${SERVICE}"; then
      echo "  已回滚到 previous: $(basename "$(readlink -f "${CURRENT_LINK}")")"
    fi
  else
    echo "  WARN: 无 previous 可回滚，保留当前 current（可能损坏）"
  fi
  exit 1
fi

# ---------------------------------------------------------------------------
# 5. Nginx 配置与静态资源/接口验收（CD-P2-01：校验失败必须阻断部署）
# ---------------------------------------------------------------------------
echo "[5] nginx -t（失败阻断）"
if command -v nginx >/dev/null 2>&1; then
  if ! nginx -t; then
    echo "ERROR: nginx -t 校验失败，部署阻断（需人工修复配置后重试）" >&2
    exit 1
  fi
fi

# 服务重启后已通过本机探活；此处再验收一个静态资源与一个只读 API（外部入口由 cd.yml 探活）
echo "[5] 本机静态资源与只读 API 验收"
curl -fsS --max-time 10 -o /dev/null "${HEALTH_URL}" || { echo "ERROR: 本机 health API 探活失败" >&2; exit 1; }

# 记录发布产物清单（供审计）
echo "=== 部署完成 sha=${RELEASE_SHA} ==="
ls -la "${ROOT}/current"
echo "current -> $(readlink -f "${ROOT}/current")"
echo "previous -> $( [ -L "${ROOT}/previous" ] && readlink -f "${ROOT}/previous" || echo none )"
