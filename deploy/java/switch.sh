#!/bin/bash
# TSLOMS 灰度切换脚本
# 用法: switch.sh java|go|rollback(=go)
TARGET="${1:-go}"
CONF="/etc/nginx/sites-enabled/tsloms"

case "$TARGET" in
  java)
    echo "[cutover] 1) 停 Go 服务释放 MQTT clientId"
    systemctl stop tsloms-server
    echo "[cutover] 2) Java 开启 MQTT"
    sed -i "s/^MQTT_ENABLED=.*/MQTT_ENABLED=true/" /etc/tsloms/java.env
    systemctl restart tsloms-server-java
    sleep 8
    echo "[cutover] 3) nginx 上游切换 8093 -> 8096"
    sed -i "s#proxy_pass http://127.0.0.1:8093/api/#proxy_pass http://127.0.0.1:8096/api/#" "$CONF"
    sed -i "s#proxy_pass http://127.0.0.1:8093/api/v1/health#proxy_pass http://127.0.0.1:8096/api/v1/health#" "$CONF"
    nginx -t && systemctl reload nginx
    sleep 3
    curl -fsS -m 8 http://127.0.0.1:8092/tsloms/health && echo "" && echo "[cutover] DONE -> Java"
    ;;
  go|rollback)
    echo "[rollback] 1) nginx 切回 Go"
    sed -i "s#proxy_pass http://127.0.0.1:8096/api/#proxy_pass http://127.0.0.1:8093/api/#" "$CONF"
    sed -i "s#proxy_pass http://127.0.0.1:8096/api/v1/health#proxy_pass http://127.0.0.1:8093/api/v1/health#" "$CONF"
    nginx -t && systemctl reload nginx
    echo "[rollback] 2) 停 Java MQTT、恢复 Go"
    sed -i "s/^MQTT_ENABLED=.*/MQTT_ENABLED=false/" /etc/tsloms/java.env
    systemctl restart tsloms-server-java
    systemctl start tsloms-server
    sleep 3
    curl -fsS -m 8 http://127.0.0.1:8092/tsloms/health && echo "" && echo "[rollback] DONE -> Go"
    ;;
  *) echo "usage: switch.sh java|go"; exit 1;;
esac
