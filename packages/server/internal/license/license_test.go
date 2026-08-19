package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"
)

// 测试专用独立 Ed25519 密钥对（与生产密钥完全无关，仅测试签发授权码）。
// 生产和测试密钥解耦：生产验签用 supplierPublicKeyB64，测试验签用下方测试公钥。
const (
	supplierPrivateKeyB64_test = "tAmZSDUmFfBfrQCGftwmtPmxCUAND01jHM0AECFgL8yqUwxyd8yHBBFdIROuLjggqDMecdOKcV9R_CkwBNDQ2w"
	supplierPublicKeyB64_test  = "qlMMcnfMhwQRXSETri44IKgzHnHTinFfUfwpMATQ0Ns"
)

// useTestKey 让本包测试用独立测试公钥验签（生产验签逻辑不变，仅覆盖测试期验签公钥）。
func useTestKey() {
	pub, err := base64.RawURLEncoding.DecodeString(supplierPublicKeyB64_test)
	if err != nil {
		panic("测试公钥解码失败: " + err.Error())
	}
	SetPublicKeyForTest(ed25519.PublicKey(pub))
}

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
	useTestKey()
	now := time.Now()
	code := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	ok, err := VerifyUnlockCode(code, "ai", now)
	if err != nil || !ok {
		t.Fatalf("应验签通过: ok=%v err=%v", ok, err)
	}
}

func TestVerifyUnlockCode_MismatchModule(t *testing.T) {
	useTestKey()
	now := time.Now()
	code := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	if _, err := VerifyUnlockCode(code, "video", now); err == nil {
		t.Fatal("模块不匹配应报错")
	}
}

func TestVerifyUnlockCode_Expired(t *testing.T) {
	useTestKey()
	now := time.Now()
	code := signCode(t, "ai", now.Add(-48*time.Hour), now.Add(-24*time.Hour))
	ok, err := VerifyUnlockCode(code, "ai", now)
	if err == nil || ok {
		t.Fatalf("过期授权码应被拒: ok=%v err=%v", ok, err)
	}
}

func TestVerifyUnlockCode_NotYetValid(t *testing.T) {
	useTestKey()
	now := time.Now()
	code := signCode(t, "ai", now.Add(24*time.Hour), now.Add(30*24*time.Hour))
	ok, err := VerifyUnlockCode(code, "ai", now)
	if err == nil || ok {
		t.Fatalf("尚未生效授权码应被拒: ok=%v err=%v", ok, err)
	}
}

// TestVerifyUnlockCode_Tampered 负向用例：篡改载荷(改模块名)后，签名与载荷不匹配 → 验签必须失败。
func TestVerifyUnlockCode_Tampered(t *testing.T) {
	useTestKey()
	now := time.Now()
	good := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	// 解码载荷、改成「video」模块后重新编码（签名保持不变 → 与篡改后载荷不匹配）
	parts := splitOnDot(good)
	raw, _ := base64.RawURLEncoding.DecodeString(parts[1])
	var pl map[string]any
	if err := json.Unmarshal(raw, &pl); err != nil {
		t.Fatal("载荷解码失败")
	}
	pl["module"] = "video"
	tamperedRaw, _ := json.Marshal(pl)
	tampered := parts[0] + "." + base64.RawURLEncoding.EncodeToString(tamperedRaw)
	if _, err := VerifyUnlockCode(tampered, "ai", now); err == nil {
		t.Fatal("篡改 module 后签名不匹配，应被拒")
	}
}

// TestVerifyUnlockCode_TamperedSig 负向用例：只篡改签名本身(翻转一个字节)也须验签失败。
func TestVerifyUnlockCode_TamperedSig(t *testing.T) {
	useTestKey()
	now := time.Now()
	good := signCode(t, "ai", now.Add(-time.Hour), now.Add(30*24*time.Hour))
	parts := splitOnDot(good)
	sig, _ := base64.RawURLEncoding.DecodeString(parts[0])
	sig[0] ^= 0xFF // 破坏签名
	badCode := base64.RawURLEncoding.EncodeToString(sig) + "." + parts[1]
	if _, err := VerifyUnlockCode(badCode, "ai", now); err == nil {
		t.Fatal("签名被篡改应被拒")
	}
}

func splitOnDot(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return []string{s, ""}
}
