package handler

import (
	"encoding/json"
	"strconv"
	"strings"

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

// SuggestDeviceCopilotAPI 设备 Copilot：依据前端提交字段给出填写/配置建议
func SuggestDeviceCopilotAPI(c *gin.Context) {
	var input map[string]any
	if err := c.ShouldBindJSON(&input); err != nil {
		input = map[string]any{}
	}
	uid := userIDFromCtx(c)
	res, err := ai.SuggestDeviceCopilot(uid, input)
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpRead, "ai/advice/device", "AI设备填写建议")
	ok(c, gin.H{"result": res})
}

// SuggestWorkOrderCreateAPI 建单 Copilot：基于关联故障推荐建单要素
// body: {fault_id}
func SuggestWorkOrderCreateAPI(c *gin.Context) {
	var req struct {
		FaultID uint `json:"fault_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "fault_id 必填")
		return
	}
	uid := userIDFromCtx(c)
	res, err := ai.SuggestWorkOrderCreate(uid, req.FaultID)
	if err != nil {
		notFound(c, "故障不存在")
		return
	}
	recordOperation(c, model.OpRead, "ai/advice/workorder/create", "AI建单建议")
	ok(c, gin.H{"result": res})
}

// SuggestPurchaseCopilotAPI 采购 Copilot：合理性校验 + 供应商建议
// body: {items:[{material_name,quantity,price}], supplier_id}
func SuggestPurchaseCopilotAPI(c *gin.Context) {
	var req struct {
		Items      []ai.PurchaseLine `json:"items"`
		SupplierID uint              `json:"supplier_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	uid := userIDFromCtx(c)
	res := ai.SuggestPurchaseCopilot(uid, req.Items, req.SupplierID)
	recordOperation(c, model.OpRead, "ai/advice/purchase", "AI采购建议")
	ok(c, gin.H{"result": res})
}

// ============================================================
// L5 AI 自然语言交互：识别意图 → 执行工具（查询/命令）→ 结构化回答
// ============================================================

// NLInteractAPI 顶部 AI 助手入口。body: {"text": "用户自然语言"}
// 查询类只读，命令类（建故障/建工单）为写操作，读权限由路由 RequirePerm 控制。
func NLInteractAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	var req struct {
		Text string `json:"text" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Text) == "" {
		badRequest(c, "请输入要查询或操作的内容(text)")
		return
	}
	ans := ai.InterpretNL(uid, req.Text)
	opType := model.OpRead
	opDesc := "AI自然语言查询"
	if ans.DidWrite {
		opType = model.OpCreate
		opDesc = "AI自然语言命令(建单/建故障)"
	}
	recordOperation(c, opType, "ai/nl/interact", opDesc)
	ok(c, gin.H{"result": ans})
}

// ============================================================
// L6 AI 自主决策：决策建议中心 + 一键采纳（半自动执行）
// ============================================================

// DecisionCenterAPI 运维健康评分 + 决策建议。只读聚合，不产生写操作。
func DecisionCenterAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	res, err := ai.DecisionCenter(uid)
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpRead, "ai/decision/center", "AI决策建议中心")
	ok(c, gin.H{"result": res})
}

// AdoptDecisionAPI 一键采纳建议并执行（当前支持备件采购 → 生成采购草稿单）。写操作。
// body: {"category":"备件采购","title":"...","supplier_id":0,"items":[{"material_name":"...","quantity":10,"price":0}]}
func AdoptDecisionAPI(c *gin.Context) {
	uid := userIDFromCtx(c)
	var req struct {
		Category   string            `json:"category" binding:"required"`
		Title      string            `json:"title"`
		SupplierID uint              `json:"supplier_id"`
		Items      []ai.PurchaseLine `json:"items"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	orderNo, err := ai.AdoptDecisionApply(uid, req.Category, req.Title, req.SupplierID, req.Items)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	recordOperation(c, model.OpCreate, "ai/decision/adopt", "AI一键采纳：生成采购单 "+orderNo)
	ok(c, gin.H{"order_no": orderNo, "message": "已生成采购草稿单 " + orderNo})
}
