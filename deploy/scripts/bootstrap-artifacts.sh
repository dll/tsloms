#!/bin/bash
# =============================================================================
# TSLOMS 制品化首次引导脚本（bootstrap）
#
# 目的：把当前"服务器本地构建/直接路径部署"的在线状态平滑迁移到
#       "不可变制品 + releases/current 原子切换"结构，且不中断线上服务。
#
# 必须在生产机以 root 或具有 /opt/tsloms 写权限的用户执行一次。
# 执行顺序：
#   1. 创建 releases/current/shared 目录结构
#   2. 把现存的 server 二进制与 admin/dist 拷贝成第一个 release（不动原路径）
#   3. 建立 current -> releases/<sha> 符号链接
#   4. 迁移媒体目录到 shared/media（幂等）
#   5. 就地安装发布脚本到 /opt/tsloms/bin/release-install.sh
#   6. 打印下一步（手动替换 systemd 单元并 daemon-reload+restart）指引
#
# 说明：本脚本只做"新增目录 + 拷贝/软链"，绝不删除原 /opt/tsloms/packages
#       下的文件，确保回退路径完好。
# =============================================================================
set -Eeuo pipefail

ROOT="/opt/tsloms"
SRV_SRC="${ROOT}/packages/server/server"
DIST_SRC="${ROOT}/packages/admin/dist"
OLD_MEDIA="${ROOT}/packages/server/uploads/media"
NEW_MEDIA="${ROOT}/shared/media"
BIN="release-install.sh"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== TSLOMS 制品化引导 ==="

if [ ! -e "${SRV_SRC}" ]; then
  echo "WARN: 未找到现役 server 二进制 ${SRV_SRC}；将使用仓库脚本目录内的制品（若有）。"
fi

echo "[1] 创建目录结构"
mkdir -p "${ROOT}/releases" "${ROOT}/shared/media" "${ROOT}/bin"

# 以现有工作区为版本标识（无法取得真实 git SHA 时用时间戳兜底）
CUR_SHA="bootstrap-$(date +%Y%m%d%H%M%S)"
RELEASE="${ROOT}/releases/${CUR_SHA}"
mkdir -p "${RELEASE}/admin"

echo "[2] 拷贝现役二进制与前端为第一个 release: ${CUR_SHA}"
if [ -e "${SRV_SRC}" ]; then
  cp -a "${SRV_SRC}" "${RELEASE}/server"
  echo "  server: ${SRV_SRC} -> ${RELEASE}/server"
fi
if [ -d "${DIST_SRC}" ]; then
  cp -a "${DIST_SRC}" "${RELEASE}/admin/dist"
  echo "  admin: ${DIST_SRC} -> ${RELEASE}/admin/dist"
fi
echo "${CUR_SHA}" > "${RELEASE}/version.txt"

echo "[3] 建立 current 符号链接"
ln -sfn "${RELEASE}" "${ROOT}/current"
echo "  current -> ${RELEASE}"

echo "[4] 迁移媒体目录（幂等，旧路径保留）"
if [ -d "${OLD_MEDIA}" ] && [ -z "$(ls -A "${NEW_MEDIA}" 2>/dev/null)" ]; then
  cp -a "${OLD_MEDIA}"/. "${NEW_MEDIA}/"
  echo "  ${OLD_MEDIA} -> ${NEW_MEDIA}（复制，未删除原目录）"
else
  echo "  跳过（无旧媒体 或 shared/media 已有内容）"
fi

echo "[5] 安装发布脚本到 bin/"
if [ -f "${SCRIPT_DIR}/${BIN}" ]; then
  cp -a "${SCRIPT_DIR}/${BIN}" "${ROOT}/bin/${BIN}"
  chmod +x "${ROOT}/bin/${BIN}"
  echo "  ${ROOT}/bin/${BIN} 已就绪"
else
  echo "  WARN: 未随脚本找到 ${BIN}，请手动复制仓库 deploy/scripts/release-install.sh 到 ${ROOT}/bin/"
fi

chown -R tsloms:tsloms "${ROOT}/releases" "${ROOT}/shared" "${ROOT}/bin" 2>/dev/null || true

echo
echo "=== 引导完成 ==="
echo "目录结构："
ls -la "${ROOT}/current"
echo
echo "下一步（重要）："
echo "  1) 将仓库 deploy/systemd/tsloms-server.service 覆盖生产机 /etc/systemd/system/tsloms-server.service"
echo "  2) 执行：systemctl daemon-reload && systemctl restart tsloms-server"
echo "  3) 验证：curl -fsS http://127.0.0.1:8093/api/v1/health"
echo "  4) 确认 nginx 媒体目录已指向 ${NEW_MEDIA}（见仓库 deploy/nginx/tsloms.conf）"
echo "  回滚：本脚本未删除原 ${ROOT}/packages 下任何文件，可随时改回原 systemd 单元。"
