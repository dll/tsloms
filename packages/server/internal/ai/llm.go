package ai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/tsloms/server/internal/model"
)

// LLMClient 智谱 LLM 网关
type LLMClient struct {
	cfg      *model.AIConfig
	http     *http.Client
	endpoint string
}

// NewLLMClient 创建 LLM 客户端（使用配置中的 key/模型）
func NewLLMClient(cfg *model.AIConfig) *LLMClient {
	if cfg == nil {
		cfg = model.GetAIConfig()
	}
	return &LLMClient{
		cfg:      cfg,
		http:     &http.Client{Timeout: 60 * time.Second},
		endpoint: "https://open.bigmodel.cn/api/paas/v4/chat/completions",
	}
}

// chatMsg 请求消息
type chatMsg struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string 或 []contentPart（多模态）
}

type contentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL string `json:"url"`
}

type chatReq struct {
	Model    string    `json:"model"`
	Messages []chatMsg `json:"messages"`
	MaxTokens int      `json:"max_tokens,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
}

type chatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

// quotaResult 额度检查结果
type quotaResult struct {
	allowed  bool
	reason   string
}

// checkQuota 检查该用户今日额度是否超额
func (c *LLMClient) checkQuota(userID uint) quotaResult {
	if !c.cfg.Enabled {
		return quotaResult{false, "AI 功能已停用（管理员关闭）"}
	}
	tokens, calls := model.TodayAIConsumed(userID)
	if c.cfg.DayCallLimit > 0 && calls >= c.cfg.DayCallLimit {
		return quotaResult{false, fmt.Sprintf("今日 AI 调用次数已达上限(%d次)，请明日再试", c.cfg.DayCallLimit)}
	}
	if c.cfg.DayTokenLimit > 0 && tokens >= c.cfg.DayTokenLimit {
		return quotaResult{false, fmt.Sprintf("今日 AI token 额度已用完(%d)，请管理员调整额度", c.cfg.DayTokenLimit)}
	}
	return quotaResult{true, ""}
}

// Ask 文本对话（GLM-4/GLM-4-Flash）
func (c *LLMClient) Ask(userID uint, action, prompt string) (string, int, error) {
	q := c.checkQuota(userID)
	if !q.allowed {
		return "", 0, errors.New(q.reason)
	}
	return c.chat(userID, action, c.cfg.TextModel, []chatMsg{{Role: "user", Content: prompt}})
}

// AskVision 多模态对话（GLM-4V），imageDataURLs 为 base64 图片 data URL 列表
func (c *LLMClient) AskVision(userID uint, action, prompt string, imageDataURLs []string) (string, int, error) {
	q := c.checkQuota(userID)
	if !q.allowed {
		return "", 0, errors.New(q.reason)
	}
	parts := []contentPart{{Type: "text", Text: prompt}}
	for _, u := range imageDataURLs {
		parts = append(parts, contentPart{Type: "image_url", ImageURL: &imageURL{URL: u}})
	}
	return c.chat(userID, action, c.cfg.VisionModel, []chatMsg{{Role: "user", Content: parts}})
}

// chat 底层调用并记账
func (c *LLMClient) chat(userID uint, action, modelName string, msgs []chatMsg) (string, int, error) {
	body, _ := json.Marshal(chatReq{
		Model: modelName, Messages: msgs,
		MaxTokens: 1500, Temperature: 0.4,
	})
	req, err := http.NewRequest("POST", c.endpoint, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		c.record(userID, action, modelName, 0, false, err.Error())
		return "", 0, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var cr chatResp
	if err := json.Unmarshal(raw, &cr); err != nil {
		c.record(userID, action, modelName, 0, false, "响应解析失败")
		return "", 0, errors.New("LLM 响应解析失败")
	}
	if cr.Error != nil {
		c.record(userID, action, modelName, 0, false, cr.Error.Message)
		return "", 0, errors.New("LLM 错误: " + cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		c.record(userID, action, modelName, 0, false, "无生成内容")
		return "", 0, errors.New("LLM 无返回内容")
	}
	content := cr.Choices[0].Message.Content
	total := cr.Usage.TotalTokens
	if total == 0 {
		total = cr.Usage.PromptTokens + cr.Usage.CompletionTokens
	}
	c.record(userID, action, modelName, total, true, "")
	return content, total, nil
}

// record 记录额度消耗流水
func (c *LLMClient) record(userID uint, action, modelName string, tokens int, ok bool, errMsg string) {
	u := model.AIUsage{
		UserID: userID, Action: action, Model: modelName,
		Tokens: tokens, OK: ok, Error: errMsg,
	}
	model.DB.Create(&u)
}
