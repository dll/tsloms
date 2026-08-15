package handler

import (
	"encoding/json"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/ai"
	"github.com/tsloms/server/internal/model"
)

// ============================================================
// AI 原生增强接口：库存/成本分析 + 各模块运维报告 + 核心流程建议
// ============================================================

// AnalyzeInventoryAPI 库存健康 AI 分析
func AnalyzeInventoryAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	res, err := ai.AnalyzeInventory(uid)
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpRead, "ai/analyze/inventory", "AI库存健康分析")
	ok(c, gin.H{"result": res})
}

// AnalyzeCostAPI 维修成本 AI 归因分析
func AnalyzeCostAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	days := 90
	if d := c.Query("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 {
			days = n
		}
	}
	res, err := ai.AnalyzeCost(uid, days)
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpRead, "ai/analyze/cost", "AI成本归因分析")
	ok(c, gin.H{"result": res})
}

// GenerateReportAPI 生成运维日报 / 指定模块报告
// body: module(daily/inventory/cost/fault/workorder/device), period(day/week/month)
func GenerateReportAPI(c *gin.Context) {
	var req struct {
		Module string `json:"module" binding:"required"`
		Period string `json:"period"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误（module 必填）")
		return
	}
	uid := userIDFromCtx(c)
	if req.Period == "" {
		req.Period = "day"
	}
	var out string
	var err error
	if req.Module == "daily" {
		out, err = ai.GenerateDailyReport(uid)
	} else {
		out, err = ai.GenerateModuleReport(uid, req.Module, req.Period)
	}
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	// 反序列化回结构化返回
	var data map[string]any
	if jerr := json.Unmarshal([]byte(out), &data); jerr != nil {
		serverError(c, jerr)
		return
	}
	recordOperation(c, model.OpRead, "ai/report/"+req.Module, "生成运维报告:"+req.Module)
	ok(c, gin.H{"result": data})
}

// ListReportsAPI 查询历史报告
func ListReportsAPI(c *gin.Context) {
	module := c.Query("module")
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	list := ai.ListReports(module, limit)
	ok(c, gin.H{"list": list, "total": len(list)})
}

// SuggestFaultAdviceAPI 故障级 AI 建议（确认/派单辅助）
func SuggestFaultAdviceAPI(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "故障ID无效")
		return
	}
	uid := userIDFromCtx(c)
	res, err := ai.SuggestFaultAdvice(uid, id)
	if err != nil {
		notFound(c, "故障不存在")
		return
	}
	recordOperation(c, model.OpRead, "ai/advice/fault/"+strconv.Itoa(int(id)), "AI故障处置建议")
	ok(c, gin.H{"result": res})
}

// SuggestWorkOrderAdviceAPI 工单 Copilot（处理协助/维修小结）
// query: stage(copilot/summary)
func SuggestWorkOrderAdviceAPI(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "工单ID无效")
		return
	}
	stage := c.DefaultQuery("stage", "copilot")
	uid := userIDFromCtx(c)
	res, err := ai.SuggestWorkOrderAdvice(uid, id, stage)
	if err != nil {
		notFound(c, "工单不存在")
		return
	}
	recordOperation(c, model.OpRead, "ai/advice/workorder/"+strconv.Itoa(int(id)), "AI工单Copilot:"+stage)
	ok(c, gin.H{"result": res})
}

// ListAdvicesAPI 查询流程建议历史
func ListAdvicesAPI(c *gin.Context) {
	bizType := c.Query("biz_type")
	var bizID uint
	if v := c.Query("biz_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			bizID = uint(n)
		}
	}
	limit := 20
	if l := c.Query("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}
	list := ai.ListAdvices(bizType, bizID, limit)
	ok(c, gin.H{"list": list, "total": len(list)})
}
