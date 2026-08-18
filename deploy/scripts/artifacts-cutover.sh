#!/bin/bash
set -Eeuo pipefail

echo "=== [1] nginx: 原子替换为制品化配置并 reload ==="
cp /tmp/tsloms-nginx-prod-fitted.conf /etc/nginx/sites-enabled/tsloms
# 清理可能残留的 .new
rm -f /etc/nginx/sites-enabled/tsloms.new
nginx -t 2>&1 | tail -2
systemctl reload nginx || systemctl restart nginx
echo "  nginx reload OK"

echo "=== [2] 前台快速探活（nginx 已切到 current dist / shared media）==="
sleep 1
curl -fsS -o /dev/null -w "front=%{http_code} " http://127.0.0.1:8092/tsloms/admin/ 2>/dev/null || echo -n "front=FAIL "
curl -fsS -o /dev/null -w "media-code=%{http_code}\n" http://127.0.0.1:8092/tsloms/media/202608/1_1786759162670.jpg 2>/dev/null || echo "media=FAIL"

echo "=== [3] systemd: daemon-reload 并切换到 current/server ==="
systemctl daemon-reload
systemctl restart tsloms-server
echo "  restarted, waiting..."
for i in 1 2 3 4 5; do
  sleep 2
  if systemctl is-active --quiet tsloms-server && curl -fsS http://127.0.0.1:8093/api/v1/health >/dev/null 2>&1; then
    echo "  health OK (try $i)"
    # 确认确实跑的是 current 版
    PID=$(systemctl show -p MainPID --value tsloms-server)
    echo "  MainPID=$PID exe=$(readlink -f /proc/$PID/exe)"
    exit 0
  fi
  echo "  try $i: not ready"
done

echo "!!! 切换失败，开始回滚 !!!"
echo "--- 回滚 systemd 到旧路径 ---"
sed -i 's|WorkingDirectory=/opt/tsloms/current|WorkingDirectory=/opt/tsloms/packages/server|; s|ExecStart=/opt/tsloms/current/server|ExecStart=/opt/tsloms/packages/server/server|' /etc/systemd/system/tsloms-server.service
systemctl daemon-reload
systemctl restart tsloms-server
echo "--- 回滚 nginx 到旧路径 ---"
# 从备份恢复前一份 nginx（最老一份 e415241 前）
cat > /etc/nginx/sites-enabled/tsloms <<'EOF'
server {
    listen 8092;
    server_name _;
    client_max_body_size 50m;
    absolute_redirect off;
    port_in_redirect off;
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options SAMEORIGIN always;
    add_header Referrer-Policy strict-origin-when-cross-origin always;
    charset utf-8;
    location = / { return 302 /tsloms/admin/; }
    location /tsloms/admin { alias /opt/tsloms/packages/admin/dist/; index index.html; try_files $uri $uri/ @tsloms_spa; }
    location @tsloms_spa { root /opt/tsloms/packages/admin; try_files /dist/index.html =404; }
    location /tsloms/api/ { proxy_pass http://127.0.0.1:8093/api/; proxy_set_header Host $host; proxy_set_header X-Real-IP $remote_addr; proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; proxy_set_header X-Forwarded-Proto $scheme; }
    location /tsloms/media/ { alias /opt/tsloms/packages/server/uploads/media/; add_header Cache-Control "public, max-age=3600"; }
    location /tsloms/health { proxy_pass http://127.0.0.1:8093/api/v1/health; proxy_set_header Host $host; }
}
EOF
nginx -t 2>&1 | tail -1
systemctl reload nginx || systemctl restart nginx
echo "--- 回滚 env MEDIA_DIR 到旧路径 ---"
sed -i 's|^MEDIA_DIR=.*|MEDIA_DIR=/opt/tsloms/packages/server/uploads/media|' /opt/tsloms/packages/server/.env
grep '^MEDIA_DIR=' /opt/tsloms/packages/server/.env
echo "!!! 已回滚，服务应恢复旧路径运行 !!!"
systemctl is-active tsloms-server
exit 1
