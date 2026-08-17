package config

import (
	"os"
	"strconv"
	"sync"
)

// Config 全局配置结构体
// 通过环境变量读取，sync.Once 缓存进程内单例
type Config struct {
	ServerPort      string // 后端服务端口
	AppEnv          string // 运行环境（development/production/test）
	DBDriver        string // 数据库驱动（mysql/sqlite）
	DBHost          string // MySQL 主机
	DBPort          string // MySQL 端口
	DBUser          string // 数据库用户名
	DBPassword      string // 数据库密码
	DBName          string // 数据库名（tsloms，与 EQS 的 eqs 隔离）
	RedisAddr       string // Redis 地址（与 EQS 共享实例）
	RedisPass       string // Redis 密码
	RedisDB         int    // Redis DB 索引（TSLOMS 使用 1，EQS 使用 0）
	JWTSecret       string // JWT 签名密钥
	MQTTBroker      string // MQTT Broker 地址
	MQTTUsername    string // MQTT 用户名
	MQTTPassword    string // MQTT 密码
	MQTTClientID    string // MQTT 客户端 ID
	MQTTTopicPrefix string // MQTT Topic 前缀
	AllowedOrigins  string // CORS 生产白名单（逗号分隔，可为空）
	OfflineAfterMin int    // 设备离线判定阈值（分钟，默认6=3倍签到周期）
	MediaDir        string // 媒体文件存储目录（默认 ./uploads/media）
	MediaURLPrefix  string // 媒体对外访问前缀（默认 /media）
	AIAPIKey        string // AI LLM API Key（智谱/DeepSeek，可选，UI可改）
	AITextModel     string // AI 文本模型（默认 glm-4-flash）
	AIVisionModel   string // AI 多模态模型（默认 glm-4v）
	AdminInitPwd    string // 首次初始化管理员密码（ADMIN_INIT_PASSWORD，空则生成随机）
	SuperAdminPwd   string // 超级管理员初始密码（SUPER_ADMIN_PASSWORD，空则生成随机并打印一次）
	EnabledModules  string // 可选模块启用列表（ENABLED_MODULES，逗号分隔，空=仅核心模块）
	AMapWebKey      string // 高德 Web 服务 Key（POI/地名搜索，AMAP_WEB_KEY，可选）
}

// Load 从环境变量构造完整配置（每次调用都会重新解析环境变量）
func Load() *Config {
	return &Config{
		ServerPort:      getEnv("SERVER_PORT", "8093"),
		AppEnv:          getEnv("APP_ENV", "development"),
		DBDriver:        getEnv("DB_DRIVER", "mysql"),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBPort:          getEnv("DB_PORT", "3306"),
		DBUser:          getEnv("DB_USER", "root"),
		DBPassword:      getEnv("DB_PASSWORD", "root"),
		DBName:          getEnv("DB_NAME", "tsloms"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPass:       getEnv("REDIS_PASS", ""),
		RedisDB:         getEnvInt("REDIS_DB", 1),
		JWTSecret:       getEnv("JWT_SECRET", "tsloms-secret-key"),
		MQTTBroker:      getEnv("MQTT_BROKER", "tcp://localhost:1883"),
		MQTTUsername:    getEnv("MQTT_USERNAME", "tsloms"),
		MQTTPassword:    getEnv("MQTT_PASSWORD", ""),
		MQTTClientID:    getEnv("MQTT_CLIENT_ID", "tsloms-server"),
		MQTTTopicPrefix: getEnv("MQTT_TOPIC_PREFIX", "trafficLight"),
		AllowedOrigins:  getEnv("ALLOWED_ORIGINS", ""),
		OfflineAfterMin: getEnvInt("OFFLINE_AFTER_MIN", 6),
		MediaDir:        getEnv("MEDIA_DIR", ""),
		MediaURLPrefix:  getEnv("MEDIA_URL_PREFIX", "/media"),
		AIAPIKey:        getEnv("AI_API_KEY", ""),
		AITextModel:     getEnv("AI_TEXT_MODEL", "glm-4-flash"),
		AIVisionModel:   getEnv("AI_VISION_MODEL", "glm-4v"),
		AdminInitPwd:    getEnv("ADMIN_INIT_PASSWORD", ""),
		SuperAdminPwd:   getEnv("SUPER_ADMIN_PASSWORD", ""),
		EnabledModules:  getEnv("ENABLED_MODULES", ""),
		AMapWebKey:      getEnv("AMAP_WEB_KEY", ""),
	}
}

// getEnv 读取环境变量，未设置时返回默认值
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt 读取整型环境变量，未设置或解析失败时返回默认值
func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

var (
	cachedOnce sync.Once
	cachedCfg  *Config
)

// Get 返回进程内缓存的配置（首次调用时从环境变量构造一次，之后复用同一实例）。
// 配置在进程生命周期内恒定，热路径应使用 Get 而非每次 Load。
func Get() *Config {
	cachedOnce.Do(func() {
		cachedCfg = Load()
	})
	return cachedCfg
}

// ResetCache 清空配置缓存（测试使用；生产代码不应调用）
func ResetCache() {
	cachedOnce = sync.Once{}
	cachedCfg = nil
}

// IsProduction 是否生产环境
func (c *Config) IsProduction() bool {
	return c.AppEnv == "production"
}

// IsTest 是否测试环境
func (c *Config) IsTest() bool {
	return c.AppEnv == "test" || c.AppEnv == "testing"
}
