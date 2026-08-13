#!/bin/bash

# TSLOMS 部署脚本 - 交通信号灯运维系统

set -e

# 配置
APP_NAME="tsloms"
DEPLOY_DIR="/opt/tsloms"
SERVICE_NAME="tsloms-server"

echo "=== ${APP_NAME} 部署脚本 ==="

# 1. 更新代码
echo "1. 更新代码..."
cd $DEPLOY_DIR
git pull origin main

# 2. 构建后端
echo "2. 构建后端..."
cd packages/server
go build -o server cmd/server/main.go

# 3. 构建管理后台
echo "3. 构建管理后台..."
cd ../admin
npm install
npm run build

# 4. 重启服务
echo "4. 重启服务..."
sudo systemctl restart $SERVICE_NAME

# 5. 验证服务
echo "5. 验证服务..."
sleep 3
if systemctl is-active --quiet $SERVICE_NAME; then
    echo "[OK] 服务启动成功"
else
    echo "[FAIL] 服务启动失败"
    exit 1
fi

echo "=== 部署完成 ==="
