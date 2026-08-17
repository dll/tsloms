// Command licensegen —— 供应方离线授权码生成工具（仅供应方持有，不随服务器部署）
//
// 用法：
//
//	go run ./cmd/licensegen -module ai -days 365
//	go run ./cmd/licensegen -module core -days 3650
//
// 输出：一段 Ed25519 签名的授权码，粘贴给客户（超级管理员在系统内 /license/unlock 输入即可解锁对应模块）。
// 私钥内嵌本工具（仅此离线工具持有）；服务器仅存公钥验签，篡改授权码/有效期即验签失败。
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"time"
)

// supplierPrivateKeyB64 供应方 Ed25519 私钥（仅本离线工具持有，绝不随服务器部署）
const supplierPrivateKeyB64 = "QNn9Qbk-Pi2AYWhbeHl2_SAYfLBbTLYfFNYsXxCpoSH8iWnoFYNHnzBjfD2uyduZj1DL89E9TaW0-wek4D36Cw"

func main() {
	module := flag.String("module", "ai", "模块key(core/video/inventory/purchase/expense/supplier/ai/dispatch/notification/patrol)")
	days := flag.Int("days", 365, "授权有效期(天)")
	nbfDays := flag.Int("nbf", 0, "生效延迟(天, 默认立即)")
	flag.Parse()

	privBytes, err := base64.RawURLEncoding.DecodeString(supplierPrivateKeyB64)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		panic("supplier 私钥格式错误或长度不符")
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
