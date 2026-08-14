package handler

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/ai"
	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/model"
)

// maskKey 脱敏 API Key（仅显示前4后4）
func maskKey(k string) string {
	if len(k) <= 8 {
		return "****"
	}
	return k[:4] + "..." + k[len(k)-4:]
}

// GetAIConfig 读取 AI 配置（API Key 脱敏）
func GetAIConfig(c *gin.Context) {
	cfg := model.GetAIConfig()
	ok(c, gin.H{
		"provider": cfg.Provider, "text_model": cfg.TextModel,
		"vision_model": cfg.VisionModel, "enabled": cfg.Enabled,
		"day_token_limit": cfg.DayTokenLimit, "day_call_limit": cfg.DayCallLimit,
		"api_key_masked": maskKey(cfg.APIKey),
		"has_key":        cfg.APIKey != "",
	})
}

// UpdateAIConfig 更新 AI 配置（仅管理员）
func UpdateAIConfig(c *gin.Context) {
	var req struct {
		Provider      string `json:"provider"`
		TextModel     string `json:"text_model"`
		VisionModel   string `json:"vision_model"`
		APIKey        string `json:"api_key"`
		Enabled       *bool  `json:"enabled"`
		DayTokenLimit int64  `json:"day_token_limit"`
		DayCallLimit  int    `json:"day_call_limit"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}

	cfg := model.GetAIConfig()
	if cfg.ID == 0 {
		cfg.ID = 1
	}
	if req.Provider != "" { cfg.Provider = req.Provider }
	if req.TextModel != "" { cfg.TextModel = req.TextModel }
	if req.VisionModel != "" { cfg.VisionModel = req.VisionModel }
	if req.APIKey != "" && req.APIKey != maskKey(cfg.APIKey) { cfg.APIKey = req.APIKey }
	if req.Enabled != nil { cfg.Enabled = *req.Enabled }
	if req.DayTokenLimit > 0 { cfg.DayTokenLimit = req.DayTokenLimit }
	if req.DayCallLimit > 0 { cfg.DayCallLimit = req.DayCallLimit }

	if err := model.DB.Save(cfg).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "ai/config", "更新AI配置(额度/模型)")
	ok(c, gin.H{"message": "AI 配置已更新"})
}

// MyAIUsage 查询当前用户今日额度使用
func MyAIUsage(c *gin.Context) {
	uid := userIDFromCtx(c)
	tokens, calls := model.TodayAIConsumed(uid)
	cfg := model.GetAIConfig()
	ok(c, gin.H{
		"today_tokens": tokens, "today_calls": calls,
		"day_token_limit": cfg.DayTokenLimit, "day_call_limit": cfg.DayCallLimit,
		"enabled": cfg.Enabled,
	})
}

// ResetAIUsage 重置 AI 额度（仅管理员）：清空今日使用流水
func ResetAIUsage(c *gin.Context) {
	start := time.Now().Truncate(24 * time.Hour)
	model.DB.Where("created_at >= ?", start).Delete(&model.AIUsage{})
	recordOperation(c, model.OpUpdate, "ai/usage", "重置今日AI额度")
	ok(c, gin.H{"message": "今日 AI 额度已重置"})
}

// AIUsagePage AI 额度使用流水（管理员查看）
func AIUsagePage(c *gin.Context) {
	page, _ := parseUint(c.DefaultQuery("page", "1"))
	pageSize, _ := parseUint(c.DefaultQuery("page_size", "20"))
	if page == 0 { page = 1 }
	if pageSize > 100 { pageSize = 100 }

	var list []model.AIUsage
	var total int64
	q := model.DB.Model(&model.AIUsage{})
	if u := c.Query("username"); u != "" {
		// 关联用户查询（简化按 user_id 模糊）
		var users []model.User
		model.DB.Where("username LIKE ?", "%"+u+"%").Find(&users)
		ids := make([]uint, 0, len(users))
		for _, us := range users { ids = append(ids, us.ID) }
		if len(ids) > 0 { q = q.Where("user_id IN ?", ids) } else { q = q.Where("1=0") }
	}
	q.Count(&total)
	q.Order("created_at DESC").Offset(int((page-1)*pageSize)).Limit(int(pageSize)).Find(&list)

	names := map[uint]string{}
	var uids []uint
	for _, l := range list { uids = append(uids, l.UserID) }
	var users []model.User
	if len(uids) > 0 {
		model.DB.Where("id IN ?", uids).Find(&users)
		for _, us := range users { names[us.ID] = us.Username }
	}

	out := make([]gin.H, 0, len(list))
	actions := map[string]string{"predict": "故障预测", "diagnose": "故障诊断", "lifecycle": "生命周期", "": "-"}
	for _, l := range list {
		out = append(out, gin.H{
			"id": l.ID, "user": names[l.UserID], "action": actions[l.Action],
			"model": l.Model, "tokens": l.Tokens, "ok": l.OK,
			"error": l.Error, "created_at": l.CreatedAt,
		})
	}
	ok(c, gin.H{"list": out, "total": total})
}

// RunPrediction 运行全量设备故障预测（规则引擎），返回预测清单 + 地图着色数据
func RunPrediction(c *gin.Context) {
	var devices []model.Device
	model.DB.Find(&devices)
	batchID := time.Now().Format("200601021504")

	results := make([]gin.H, 0, len(devices))
	riskCount := map[string]int{"low": 0, "medium": 0, "high": 0, "critical": 0}
	for _, d := range devices {
		p := ai.RunRulePrediction(&d, batchID)
		riskCount[p.RiskLevel]++
		results = append(results, gin.H{
			"device_hw_id":  p.DeviceHwID,
			"intersection":  p.Intersection,
			"lat":           d.Lat, "lng": d.Lng,
			"health_score":  p.HealthScore,
			"risk_level":    p.RiskLevel,
			"risk_label":    ai.RiskLabel(p.RiskLevel),
			"predict_type":  p.PredictType,
			"remain_days":   p.RemainDays,
			"confidence":    p.Confidence,
			"factors":       p.Factors,
			"plan":          p.Plan,
			"source":        p.Source,
			"online":        d.OnlineStatus,
		})
	}
	recordOperation(c, model.OpRead, "ai/predict", fmt.Sprintf("运行全量故障预测(%d台)", len(devices)))
	ok(c, gin.H{"batch_id": batchID, "list": results, "count": len(devices), "risk_count": riskCount})
}

// EnhancePredictionPlan 对单条预测生成 LLM 增强预案（消耗额度）
func EnhancePredictionPlan(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "预测ID无效")
		return
	}
	var pred model.AIPrediction
	if err := model.DB.First(&pred, id).Error; err != nil {
		notFound(c, "预测记录不存在")
		return
	}
	uid := userIDFromCtx(c)
	client := ai.NewLLMClient(nil)
	prompt := fmt.Sprintf(
		"你是交通信号灯运维专家。设备#%d（路口：%s）健康分%d/100，风险等级%s，"+
			"预测故障类型：%s，剩余寿命约%d天。风险因子：%s。\n"+
			"请用中文给出 ≤200字的具体应对预案，包括：检修优先级、排查步骤、需准备的备件、预计耗时。",
		pred.DeviceHwID, pred.Intersection, pred.HealthScore, ai.RiskLabel(pred.RiskLevel),
		pred.PredictType, pred.RemainDays, pred.Factors,
	)
	plan, tokens, err := client.Ask(uid, "predict", prompt)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	model.DB.Model(&pred).Updates(map[string]interface{}{"plan": plan, "source": "LLM增强"})
	ok(c, gin.H{"plan": plan, "source": "LLM增强", "tokens_used": tokens})
}

// DiagnoseFeedbackAPI AI 故障诊断：根据反馈ID（含图片/文字）诊断
func DiagnoseFeedbackAPI(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "反馈ID无效")
		return
	}
	var fb model.Feedback
	if err := model.DB.First(&fb, id).Error; err != nil {
		notFound(c, "反馈不存在")
		return
	}
	uid := userIDFromCtx(c)

	// 解析本地图片：反馈图片URL（/tsloms/media/...）→ 本地文件路径
	mediaDir := mediaRootDir()
	images := resolveFeedbackImages(fb.ImageURL, mediaDir)

	res := ai.DiagnoseFeedback(uid, &fb, mediaDir, images)
	recordOperation(c, model.OpRead, "ai/diagnose/"+strconv.Itoa(int(id)), "AI故障诊断反馈:"+fb.Title)
	ok(c, gin.H{"result": res})
}

// BuildLifecycleAPI AI 全流程溯源：设备生命周期时间线 + LLM画像
func BuildLifecycleAPI(c *gin.Context) {
	hw, err := strconv.ParseUint(c.Param("hwid"), 10, 32)
	if err != nil {
		badRequest(c, "设备ID无效")
		return
	}
	var dev model.Device
	if err := model.DB.Where("hw_id = ?", uint32(hw)).First(&dev).Error; err != nil {
		notFound(c, "设备不存在")
		return
	}
	uid := userIDFromCtx(c)
	res := ai.BuildLifecycle(uid, &dev)
	recordOperation(c, model.OpRead, "ai/lifecycle/"+c.Param("hwid"), "AI生命周期溯源设备#"+c.Param("hwid"))
	ok(c, gin.H{"result": res})
}

// AIPredictions 查询历史预测记录（按批次）
func AIPredictions(c *gin.Context) {
	batch := c.Query("batch_id")
	var list []model.AIPrediction
	var total int64
	q := model.DB.Model(&model.AIPrediction{})
	if batch != "" { q = q.Where("batch_id = ?", batch) }
	q.Count(&total)
	q.Order("health_score ASC").Limit(500).Find(&list)

	out := make([]gin.H, 0, len(list))
	for _, p := range list {
		out = append(out, gin.H{
			"id": p.ID, "device_hw_id": p.DeviceHwID, "batch_id": p.BatchID,
			"health_score": p.HealthScore, "risk_level": p.RiskLevel,
			"risk_label": ai.RiskLabel(p.RiskLevel), "predict_type": p.PredictType,
			"remain_days": p.RemainDays, "confidence": p.Confidence,
			"factors": p.Factors, "plan": p.Plan, "source": p.Source,
			"created_at": p.CreatedAt,
		})
	}
	ok(c, gin.H{"list": out, "total": total})
}

// ---- 内部辅助 ----

func userIDFromCtx(c *gin.Context) uint {
	if v, exists := c.Get("user_id"); exists {
		if id, ok := v.(uint); ok { return id }
	}
	return 0
}

// mediaRootDir 媒体根目录（本地文件解析用）
func mediaRootDir() string {
	cfg := config.Get()
	if cfg.MediaDir != "" { return cfg.MediaDir }
	return ""
}

// resolveFeedbackImages 把反馈图片 URL(/tsloms/media/xxx) 解析为本地文件路径（目录中查找匹配文件）
func resolveFeedbackImages(imageURL, mediaDir string) []string {
	if imageURL == "" || mediaDir == "" {
		return nil
	}
	// URL 形如 /tsloms/media/<filename>
	name := imageURL
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	if name == "" || strings.Contains(name, "..") {
		return nil
	}
	path := filepath.Join(mediaDir, name)
	return []string{path}
}
