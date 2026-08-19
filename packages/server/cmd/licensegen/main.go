// Command licensegen —— 供应方离线授权码生成工具（仅供应方持有，不随服务器部署）
//
// 用法：
//
//	go run ./cmd/licensegen -module ai -days 365
//	go run ./cmd/licensegen -module core -days 3650
//
// 输出：一段 Ed25519 签名的授权码，粘贴给客户（超级管理员在系统内 /license/unlock 输入即可解锁对应模块）。
// 私钥不内嵌本工具，从环境变量 TSLOMS_LICENSE_PRIVATE_KEY(base64url 编码) 读取（仅供应方离线签发时持有）；
// 服务器仅存公钥验签，篡改授权码/有效期即验签失败。
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"
)

// supplierPrivateKeyB64FromEnv 从环境变量读取供应方 Ed25519 私钥(base64url)。
// 生产私钥绝不入仓库；本离线工具运行时通过 TSLOMS_LICENSE_PRIVATE_KEY 注入。
func supplierPrivateKeyB64FromEnv() string {
	v := os.Getenv("TSLOMS_LICENSE_PRIVATE_KEY")
	if v == "" {
		panic("缺少环境变量 TSLOMS_LICENSE_PRIVATE_KEY：请先设置供应方 Ed25519 私钥(base64url) 再运行本离线签发工具")
	}
	return v
}

func main() {
	module := flag.String("module", "ai", "模块key(core/video/inventory/purchase/expense/supplier/ai/dispatch/notification/patrol)")
	days := flag.Int("days", 365, "授权有效期(天)")
	nbfDays := flag.Int("nbf", 0, "生效延迟(天, 默认立即)")
	flag.Parse()

	supplierPrivateKeyB64 := supplierPrivateKeyB64FromEnv()
	privBytes, err := base64.RawURLEncoding.DecodeString(supplierPrivateKeyB64)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		panic("TSLOMS_LICENSE_PRIVATE_KEY 私钥格式错误或长度不符（需为 ed25519 base64url 私钥）")
	}
	priv := ed25519.PrivateKey(privBytes)

	now := time.Now()
	payload := map[string]any{
		"module": *module,
		"nbf":    now.Add(time.Duration(*nbfDays) * 24 * time.Hour).Unix(),
		"exp":    now.Add(time.Duration(*days) * 24 * time.Hour).Unix(),
	}
	raw, _ := json.Marshal(payload)
	sig := ed25519.Sign(priv, raw)

	code := base64.RawURLEncoding.EncodeToString(sig) + "." + base64.RawURLEncoding.EncodeToString(raw)
	fmt.Println("模块:", *module, "| 有效期(天):", *days)
	fmt.Println("授权码:")
	fmt.Println(code)
}
