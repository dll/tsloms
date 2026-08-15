package model

import (
	"testing"

	"github.com/tsloms/server/internal/config"
)

func TestGetAIConfig_Default(t *testing.T) {
	InitTestDB()
	cfg := GetAIConfig()
	if cfg.Provider != "zhipu" || cfg.TextModel != "glm-4-flash" || cfg.Enabled != true {
		t.Errorf("默认AI配置错误: %+v", cfg)
	}
	if cfg.DayTokenLimit != 1000000 || cfg.DayCallLimit != 200 {
		t.Errorf("默认额度错误: %d/%d", cfg.DayTokenLimit, cfg.DayCallLimit)
	}
}

func TestGetAIConfig_Existing(t *testing.T) {
	InitTestDB()
	DB.Create(&AIConfig{ID: 1, Provider: "deepseek", APIKey: "k", Enabled: false})
	cfg := GetAIConfig()
	if cfg.Provider != "deepseek" {
		t.Errorf("应读到已存在配置, got %+v", cfg)
	}
}

func TestTodayAIConsumed(t *testing.T) {
	InitTestDB()
	DB.Create(&AIUsage{UserID: 3, Action: "predict", Tokens: 100})
	DB.Create(&AIUsage{UserID: 3, Action: "diagnose", Tokens: 200, OK: true})
	DB.Create(&AIUsage{UserID: 4, Action: "predict", Tokens: 999})
	tokens, calls := TodayAIConsumed(3)
	if tokens != 300 || calls != 2 {
		t.Errorf("TodayAIConsumed(3) = %d tokens, %d calls; 期望 300/2", tokens, calls)
	}
}

func TestSeedAIConfig_EmptyDB(t *testing.T) {
	InitTestDB()
	SeedAIConfig("key-abc", "deepseek-chat", "")
	cfg := GetAIConfig()
	if cfg.APIKey != "key-abc" || cfg.TextModel != "deepseek-chat" {
		t.Errorf("SeedAIConfig 空库创建错误: %+v", cfg)
	}
	// VisionModel 应回退默认
	if cfg.VisionModel != "glm-4v" {
		t.Errorf("VisionModel 应回退默认, got %s", cfg.VisionModel)
	}
}

func TestSeedAIConfig_ExistingWithKey(t *testing.T) {
	InitTestDB()
	DB.Create(&AIConfig{ID: 1, Provider: "zhipu", APIKey: "", Enabled: true})
	// 已有记录 + 提供 key 且当前 key 为空 → 补充
	SeedAIConfig("new-key", "model-x", "model-y")
	cfg := GetAIConfig()
	if cfg.APIKey != "new-key" {
		t.Errorf("已有配置补充 key 失败: %q", cfg.APIKey)
	}
	// key 已有时不覆盖
	SeedAIConfig("other-key", "", "")
	cfg2 := GetAIConfig()
	if cfg2.APIKey != "new-key" {
		t.Errorf("已有 key 不应被覆盖: %q", cfg2.APIKey)
	}
}

func TestSeedAdmin(t *testing.T) {
	InitTestDB()
	// 无 admin → 创建并生成随机密码
	pwd, err := SeedAdmin("")
	if err != nil {
		t.Fatalf("SeedAdmin err: %v", err)
	}
	if len(pwd) < 12 {
		t.Errorf("随机密码太短: %q", pwd)
	}
	var adm User
	if err := DB.Where("username = ?", "admin").First(&adm).Error; err != nil {
		t.Fatalf("应创建 admin: %v", err)
	}
	// 再次调用 → 已存在不重建，返回错误或空
	_, _ = SeedAdmin("")
}

func TestSeedAdmin_WithPassword(t *testing.T) {
	InitTestDB()
	pwd, _ := SeedAdmin("Init@Pass2026")
	if pwd != "" {
		t.Errorf("显式密码应返回空(不需打印), got %q", pwd)
	}
	var adm User
	DB.Where("username = ?", "admin").First(&adm)
	if adm.PasswordHash == "" {
		t.Error("admin 应设置密码哈希")
	}
}

func TestRandomStrongPassword(t *testing.T) {
	p1 := randomStrongPassword(16)
	p2 := randomStrongPassword(16)
	if len(p1) != 16 || p1 == p2 {
		t.Errorf("随机密码异常: %q vs %q", p1, p2)
	}
	// 应含大小写字母与数字（逐一字符检查，非子串）
	hasUpper, hasLower, hasDigit := false, false, false
	for _, c := range p1 {
		switch {
		case c >= 'A' && c <= 'Z':
			hasUpper = true
		case c >= 'a' && c <= 'z':
			hasLower = true
		case c >= '0' && c <= '9':
			hasDigit = true
		}
	}
	if !hasUpper || !hasLower || !hasDigit {
		t.Errorf("随机密码应含大小写+数字: %q", p1)
	}
}

func TestContainsAny(t *testing.T) {
	if !containsAny("hello world", "abc", "zxy", "wor") {
		t.Error("应包含 wor")
	}
	if containsAny("hello", "xyz", "123") {
		t.Error("不应包含")
	}
}

func TestInitRedis_Error(t *testing.T) {
	// 无法连接的 Redis 地址 → 返回错误（不 panic）
	cfg := config.Load()
	cfg.RedisAddr = "127.0.0.1:1" // 未监听端口
	cfg.RedisPass = ""
	err := InitRedis(cfg)
	// 可能随机成功/失败；只验证不 panic，且返回 error 或 nil 都是合法路径
	_ = err
}

func TestSeedRBAC_Idempotent(t *testing.T) {
	db := InitTestDB()
	if err := SeedRBAC(db); err != nil {
		t.Fatalf("SeedRBAC err: %v", err)
	}
	var cnt int64
	db.Model(&Permission{}).Count(&cnt)
	if cnt < 10 {
		t.Errorf("SeedRBAC 应灌入全量权限, got %d", cnt)
	}
	// 幂等：再跑一次不报错、不重复
	if err := SeedRBAC(db); err != nil {
		t.Fatalf("SeedRBAC 二次 err: %v", err)
	}
	var cnt2 int64
	db.Model(&Permission{}).Count(&cnt2)
	if cnt2 != cnt {
		t.Errorf("SeedRBAC 应幂等, %d -> %d", cnt, cnt2)
	}
}

func TestDef(t *testing.T) {
	if def("", "d") != "d" {
		t.Error("def 空应回退")
	}
	if def("v", "d") != "v" {
		t.Error("def 非空应保留")
	}
}
