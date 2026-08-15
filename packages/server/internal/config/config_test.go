package config

import (
	"os"
	"testing"
)

// setEnv 设置环境变量并在测试结束后恢复
func setEnv(t *testing.T, k, v string) {
	t.Helper()
	old, existed := os.LookupEnv(k)
	if err := os.Setenv(k, v); err != nil {
		t.Fatalf("设置 %s 失败: %v", k, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(k, old)
		} else {
			os.Unsetenv(k)
		}
	})
}

func TestLoadDefaults(t *testing.T) {
	// 关键默认值
	c := Load()
	if c.ServerPort != "8093" {
		t.Errorf("ServerPort 默认 = %q, want 8093", c.ServerPort)
	}
	if c.AppEnv != "development" {
		t.Errorf("AppEnv 默认 = %q, want development", c.AppEnv)
	}
	if c.DBDriver != "mysql" {
		t.Errorf("DBDriver 默认 = %q, want mysql", c.DBDriver)
	}
	if c.DBName != "tsloms" {
		t.Errorf("DBName 默认 = %q, want tsloms", c.DBName)
	}
	if c.RedisDB != 1 {
		t.Errorf("RedisDB 默认 = %d, want 1", c.RedisDB)
	}
	if c.JWTSecret != "tsloms-secret-key" {
		t.Errorf("JWTSecret 默认 = %q", c.JWTSecret)
	}
	if c.OfflineAfterMin != 6 {
		t.Errorf("OfflineAfterMin 默认 = %d, want 6", c.OfflineAfterMin)
	}
	if c.AITextModel != "glm-4-flash" {
		t.Errorf("AITextModel 默认 = %q", c.AITextModel)
	}
	if c.AdminInitPwd != "" {
		t.Errorf("AdminInitPwd 默认应为空, got %q", c.AdminInitPwd)
	}
}

func TestLoadEnvOverrides(t *testing.T) {
	setEnv(t, "SERVER_PORT", "9100")
	setEnv(t, "APP_ENV", "production")
	setEnv(t, "DB_DRIVER", "sqlite")
	setEnv(t, "DB_HOST", "10.0.0.5")
	setEnv(t, "DB_PORT", "3307")
	setEnv(t, "DB_USER", "app")
	setEnv(t, "DB_PASSWORD", "s3cret")
	setEnv(t, "DB_NAME", "mydb")
	setEnv(t, "REDIS_ADDR", "10.0.0.6:6380")
	setEnv(t, "REDIS_PASS", "rpass")
	setEnv(t, "REDIS_DB", "3")
	setEnv(t, "JWT_SECRET", "a-very-long-secret-value-1234567890")
	setEnv(t, "MQTT_BROKER", "ssl://10.0.0.7:8883")
	setEnv(t, "MQTT_USERNAME", "muser")
	setEnv(t, "MQTT_PASSWORD", "mpass")
	setEnv(t, "MQTT_CLIENT_ID", "cid")
	setEnv(t, "MQTT_TOPIC_PREFIX", "pre")
	setEnv(t, "ALLOWED_ORIGINS", "https://a,https://b")
	setEnv(t, "OFFLINE_AFTER_MIN", "15")
	setEnv(t, "MEDIA_DIR", "/data/media")
	setEnv(t, "MEDIA_URL_PREFIX", "/m")
	setEnv(t, "AI_API_KEY", "key1")
	setEnv(t, "AI_TEXT_MODEL", "deepseek")
	setEnv(t, "AI_VISION_MODEL", "glm-4v-plus")
	setEnv(t, "ADMIN_INIT_PASSWORD", "Init@Pass2026")

	c := Load()
	if c.ServerPort != "9100" || c.AppEnv != "production" || c.DBDriver != "sqlite" {
		t.Errorf("ServerPort/AppEnv/DBDriver 覆盖失败: %+v", c)
	}
	if c.DBHost != "10.0.0.5" || c.DBPort != "3307" || c.DBUser != "app" || c.DBPassword != "s3cret" || c.DBName != "mydb" {
		t.Errorf("DB 配置覆盖失败: %+v", c)
	}
	if c.RedisAddr != "10.0.0.6:6380" || c.RedisPass != "rpass" || c.RedisDB != 3 {
		t.Errorf("Redis 配置覆盖失败: %+v", c)
	}
	if c.JWTSecret != "a-very-long-secret-value-1234567890" {
		t.Errorf("JWT 覆盖失败: %q", c.JWTSecret)
	}
	if c.MQTTBroker != "ssl://10.0.0.7:8883" || c.MQTTUsername != "muser" || c.MQTTPassword != "mpass" || c.MQTTClientID != "cid" || c.MQTTTopicPrefix != "pre" {
		t.Errorf("MQTT 配置覆盖失败: %+v", c)
	}
	if c.AllowedOrigins != "https://a,https://b" || c.OfflineAfterMin != 15 {
		t.Errorf("Origins/Offline 覆盖失败: %+v", c)
	}
	if c.MediaDir != "/data/media" || c.MediaURLPrefix != "/m" {
		t.Errorf("Media 配置覆盖失败: %+v", c)
	}
	if c.AIAPIKey != "key1" || c.AITextModel != "deepseek" || c.AIVisionModel != "glm-4v-plus" {
		t.Errorf("AI 配置覆盖失败: %+v", c)
	}
	if c.AdminInitPwd != "Init@Pass2026" {
		t.Errorf("AdminInitPwd 覆盖失败: %q", c.AdminInitPwd)
	}
}

func TestIsProduction(t *testing.T) {
	c := Load()
	setEnv(t, "APP_ENV", "production")
	if !c.IsProduction() {
		// 重新加载以体现环境
		if !Load().IsProduction() {
			t.Error("production 下 IsProduction 应为 true")
		}
	}
}

func TestIsTest(t *testing.T) {
	setEnv(t, "APP_ENV", "testing")
	if !Load().IsTest() {
		t.Error("testing 下 IsTest 应为 true")
	}
	setEnv(t, "APP_ENV", "test")
	if !Load().IsTest() {
		t.Error("test 下 IsTest 应为 true")
	}
	setEnv(t, "APP_ENV", "production")
	if Load().IsTest() {
		t.Error("production 下 IsTest 应为 false")
	}
}

func TestGetEnvIntInvalid(t *testing.T) {
	// 非法整数值应回退默认
	setEnv(t, "REDIS_DB", "not-a-number")
	if c := Load(); c.RedisDB != 1 {
		t.Errorf("非法 REDIS_DB 应回退默认 1, got %d", c.RedisDB)
	}
}
