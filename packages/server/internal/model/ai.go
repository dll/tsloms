package model

import (
	"time"
)

// AIConfig AI 分析配置（单行，id=1）
// 保存 LLM 提供商、模型、每日额度上限、开关
type AIConfig struct {
	ID             uint      `json:"id" gorm:"primaryKey"`
	Provider       string    `json:"provider" gorm:"size:32;default:zhipu;comment:LLM提供商(zhipu/deepseek)"`
	TextModel      string    `json:"text_model" gorm:"size:64;default:glm-4-flash;comment:文本模型"`
	VisionModel    string    `json:"vision_model" gorm:"size:64;default:glm-4v;comment:多模态模型"`
	APIKey         string    `json:"api_key" gorm:"size:256;comment:API密钥"` // 标记为敏感，视图层脱敏
	Enabled        bool      `json:"enabled" gorm:"default:true;comment:是否启用LLM"`
	DayTokenLimit  int64     `json:"day_token_limit" gorm:"default:1000000;comment:每日token上限"`
	DayCallLimit   int       `json:"day_call_limit" gorm:"default:200;comment:每日调用次数上限"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AIUsage AI 调用额度流水：每次 LLM 调用记录消耗
type AIUsage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"index;comment:发起用户"`
	Action    string    `json:"action" gorm:"size:32;comment:动作(predict/diagnose/lifecycle)"`
	Model     string    `json:"model" gorm:"size:64;comment:使用的模型"`
	TokensIn  int       `json:"tokens_in" gorm:"comment:输入token"`
	TokensOut int       `json:"tokens_out" gorm:"comment:输出token"`
	Tokens    int       `json:"tokens" gorm:"comment:总token"`
	OK        bool      `json:"ok" gorm:"comment:是否成功"`
	Error     string    `json:"error" gorm:"size:256;comment:错误信息"`
	CreatedAt time.Time `json:"created_at"`
}

// AIPrediction AI 故障预测结果：每个设备一次预测
type AIPrediction struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	DeviceHwID    uint32    `json:"device_hw_id" gorm:"index:idx_pred_dev,unique,priority:1;comment:设备硬件ID"`
	Intersection  string    `json:"intersection" gorm:"size:128;comment:路口位置"`
	BatchID       string    `json:"batch_id" gorm:"size:40;index;comment:批次ID(YYYYMMDDHHMM)"`
	HealthScore   int       `json:"health_score" gorm:"comment:健康分0-100"`
	RiskLevel     string    `json:"risk_level" gorm:"size:16;comment:风险等级(low/medium/high/critical)"`
	PredictType   string    `json:"predict_type" gorm:"size:32;comment:预测故障类型"`
	RemainDays    int       `json:"remain_days" gorm:"comment:剩余寿命预估(天)"`
	Confidence    float64   `json:"confidence" gorm:"comment:置信度0-1"`
	Factors       string    `json:"factors" gorm:"size:512;comment:风险因子(JSON)"`
	Plan          string    `json:"plan" gorm:"size:1024;comment:应对预案文本(LLM生成或规则兜底)"`
	Source        string    `json:"source" gorm:"size:16;comment:规则/LLM"`
	CreatedAt     time.Time `json:"created_at"`
}

// TableName
func (AIUsage) TableName() string     { return "ai_usage" }
func (AIPrediction) TableName() string { return "ai_predictions" }
func (AIConfig) TableName() string     { return "ai_config" }

// GetAIConfig 读取 AI 配置，不存在时返回默认值
func GetAIConfig() *AIConfig {
	cfg := &AIConfig{}
	if err := DB.First(cfg).Error; err != nil {
		// 返回默认配置
		cfg.Provider = "zhipu"
		cfg.TextModel = "glm-4-flash"
		cfg.VisionModel = "glm-4v"
		cfg.Enabled = true
		cfg.DayTokenLimit = 1000000
		cfg.DayCallLimit = 200
	}
	return cfg
}

// TodayAIConsumed 查询今日某用户的 token/调用消耗
func TodayAIConsumed(userID uint) (tokens int64, calls int) {
	start := time.Now().Truncate(24 * time.Hour)
	DB.Model(&AIUsage{}).
		Where("user_id = ? AND created_at >= ?", userID, start).
		Select("COALESCE(SUM(tokens),0)").Scan(&tokens)
	var cnt int64
	DB.Model(&AIUsage{}).
		Where("user_id = ? AND created_at >= ?", userID, start).
		Count(&cnt)
	return tokens, int(cnt)
}

// SeedAIConfig 若 ai_config 为空，则用环境变量初始化 AI 配置
// 只写入环境变量给定的 key（若环境变量为空则不覆盖已有 key）
func SeedAIConfig(apiKey, textModel, visionModel string) {
	var count int64
	DB.Model(&AIConfig{}).Count(&count)
	if count > 0 {
		// 仅在环境变量提供了 key 且当前无 key 时补充
		if apiKey != "" {
			cfg := GetAIConfig()
			if cfg.APIKey == "" {
				DB.Model(&AIConfig{}).Where("id = ?", cfg.ID).Update("api_key", apiKey)
			}
		}
		return
	}
	cfg := &AIConfig{
		ID: 1, Provider: "zhipu",
		TextModel:    def(textModel, "glm-4-flash"),
		VisionModel:  def(visionModel, "glm-4v"),
		APIKey:       apiKey,
		Enabled:      true,
		DayTokenLimit: 1000000,
		DayCallLimit:  200,
	}
	DB.Create(cfg)
}

func def(v, d string) string {
	if v == "" { return d }
	return v
}
