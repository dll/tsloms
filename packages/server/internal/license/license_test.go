package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// supplierPrivateKeyB64_test 测试用供应方私钥（与正式公钥配对，仅测试签发授权码）
const supplierPrivateKeyB64_test = "QNn9Qbk-Pi2AYWhbeHl2_SAYfLBbTLYfFNYsXxCpoSH8iWnoFYNHnzBjfD2uyduZj1DL89E9TaW0-wek4D36Cw"

func signCode(t *testing.T, module string, nbf, exp time.Time) string {
	t.Helper()
	privBytes, err := base64.RawURLEncoding.DecodeString(supplierPrivateKeyB64_test)
	if err != nil || len(privBytes) != ed25519.PrivateKeySize {
		t.Fatal("私钥解码失败")
	}
	priv := ed25519.PrivateKey(privBytes)
	raw, _ := json.Marshal(map[string]any{"module": module, "nbf": nbf.Unix(), "exp": exp.Unix()})
	sig := ed25519.Sign(priv, raw)
	return base64.RawURLEncoding.EncodeToString(sig) + "." + base64.RawURLEncoding.EncodeToString(raw)
}

func TestVerifyUnlockCode_Valid(t *testing.T) {
	now := time.Now()
	code := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	ok, err := VerifyUnlockCode(code, "ai", now)
	if err != nil || !ok {
		t.Fatalf("应验签通过: ok=%v err=%v", ok, err)
	}
}

func TestVerifyUnlockCode_MismatchModule(t *testing.T) {
	now := time.Now()
	code := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	if _, err := VerifyUnlockCode(code, "video", now); err == nil {
		t.Fatal("模块不匹配应报错")
	}
}

func TestVerifyUnlockCode_Expired(t *testing.T) {
	now := time.Now()
	code := signCode(t, "ai", now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	ok, err := VerifyUnlockCode(code, "ai", now)
	if err == nil || ok {
		t.Fatalf("过期授权码应被拒: ok=%v err=%v", ok, err)
	}
}

func TestVerifyUnlockCode_NotYetValid(t *testing.T) {
	now := time.Now()
	code := signCode(t, "ai", now.Add(24*time.Hour), now.Add(30*24*time.Hour))
	ok, err := VerifyUnlockCode(code, "ai", now)
	if err == nil || ok {
		t.Fatalf("尚未生效授权码应被拒: ok=%v err=%v", ok, err)
	}
}

func TestVerifyUnlockCode_Tampered(t *testing.T) {
	now := time.Now()
	good := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	// 篡改载荷（改模块名）后签名不匹配 → 拒绝
	parts := stringsSplit(good)
	rawDec, _ := base64.RawURLEncoding.DecodeString(parts[1])
	_ = rawDec
	// 构造一个被篡改的码：换 payload 的 module
	tampered := good[:len(good)-3] + "zzz"
	if _, err := VerifyUnlockCode(tampered, "ai", now); err == nil {
		t.Fatal("被篡改的授权码应被拒")
	}
}

func stringsSplit(s string) []string {
	out := []string{s[:strings.Index(s, ".")], s[strings.Index(s, ".")+1:]}
	return out
}
