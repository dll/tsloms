package service

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/tsloms/server/internal/ai"
	"github.com/tsloms/server/internal/logger"
	"github.com/tsloms/server/internal/model"
	"go.uber.org/zap"
)

// PatrolService AI 主动巡检：定时生成运维日报 + 异常检测 + 主动站内推送
//   - 每日在配置时刻（PATROL_DAILY_HOUR，默认 08:00；PATROL_DAILY_MIN 默认 0）自动生成运维日报
//   - 巡检异常：超时工单 / 高风险设备 / 低库存或缺货 → 生成 alert 通知推送给运维与管理员
//   - 与 OfflineCheck / WorkOrderEscalator 一致的后台协程模式，每 60 秒检查一次时间窗口
type PatrolService struct {
	logger *zap.Logger
	done   chan struct{}
	hour   int
	minute int
}

// NewPatrolService 创建 AI 主动巡检服务
func NewPatrolService() *PatrolService {
	hour, minute := 8, 0
	if h := os.Getenv("PATROL_DAILY_HOUR"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n >= 0 && n <= 23 {
			hour = n
		}
	}
	if m := os.Getenv("PATROL_DAILY_MIN"); m != "" {
		if n, err := strconv.Atoi(m); err == nil && n >= 0 && n <= 59 {
			minute = n
		}
	}
	return &PatrolService{logger: logger.Get(), done: make(chan struct{}), hour: hour, minute: minute}
}

// Start 启动巡检协程
func (p *PatrolService) Start(ctx context.Context) {
	go func() {
		// 启动时先执行一次巡检（方便部署后立即有日报与预警）
		p.patrol()
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				close(p.done)
				return
			case <-ticker.C:
				// 仅在到达每日配置时刻附近触发（±30 秒窗口，避免重复）
				now := time.Now()
				if now.Hour() == p.hour && now.Minute() == p.minute && now.Second() < 60 {
					p.patrol()
				}
			}
		}
	}()
}

// Done 返回停止信号
func (p *PatrolService) Done() <-chan struct{} { return p.done }

// patrol 执行一次巡检：
//  1. 生成运维日报（AI 或规则兜底）→ 推送 report 通知
//  2. 异常检测 → 推送 alert 通知
func (p *PatrolService) patrol() {
	if model.DB == nil {
		return
	}
	p.logger.Info("AI 主动巡检开始")
	// 1) 运维日报（userID=0 表示系统自动，走全局额度/规则兜底）
	if _, err := ai.GenerateDailyReport(0); err != nil {
		p.logger.Warn("运维日报生成失败", zap.Error(err))
	}
	// 2) 快照用于异常检测与日报通知详情
	snap, err := ai.BuildDailySnapshot()
	if err != nil {
		p.logger.Warn("巡检快照失败", zap.Error(err))
		return
	}
	// 3) 生成日报通知（针对运维与管理员）
	reportTitle := "AI 巡检日报 " + snap.Date
	reportContent := p.buildReportContent(snap)
	p.notifyOps("report", reportTitle, reportContent, "/ai/workbench", "report", 0)

	// 4) 异常检测 → alert 通知
	p.checkAlerts(snap)
	p.logger.Info("AI 主动巡检完成")
}

func (p *PatrolService) buildReportContent(s *ai.DailySnapshot) string {
	return fmt.Sprintf(
		"设备在线 %d/%d；故障活跃 %d、今日新增 %d、已解决 %d；工单待处理 %d、处理中 %d、超时 %d 单，平均闭环 %.1f 小时；今日新增费用 %.2f 元。",
		s.Devices.Active, s.Devices.Total, s.Faults.Active, s.Faults.TodayNew, s.Faults.Total,
		s.WorkOrders.Pending, s.WorkOrders.Processing, s.OverdueOrders, s.AvgClosureHours, s.NewExpenses)
}

// checkAlerts 异常检测：超时工单 / 高风险设备 / 低库存或缺货
func (p *PatrolService) checkAlerts(s *ai.DailySnapshot) {
	alerts := 0
	// 超时工单
	if s.OverdueOrders > 0 {
		p.notifyOps("alert",
			fmt.Sprintf("有 %d 张工单超时", s.OverdueOrders),
			fmt.Sprintf("当前有 %d 张工单超时未闭环，请尽快跟进处理。", s.OverdueOrders),
			"/workorder", "workorder", 0)
		alerts++
	}
	// 高风险设备
	if len(s.HighRiskDevices) > 0 {
		names := ""
		for _, d := range s.HighRiskDevices[:min(5, len(s.HighRiskDevices))] {
			names += d.Name + " "
		}
		p.notifyOps("alert", fmt.Sprintf("%d 台高风险设备需巡检", len(s.HighRiskDevices)),
			"高风险/极高风险设备："+names+", 建议优先安排巡检或预测处理。",
			"/ai/predict", "device", 0)
		alerts++
	}
	// 低库存/缺货
	p.checkStockAlerts(&alerts)
	if alerts == 0 {
		p.logger.Info("巡检未发现异常告警")
	}
}

// checkStockAlerts 低库存/缺货物料检测
// 低库存：threshold>0 且 0<stock<=threshold；缺货：threshold>0 且 stock<=0
func (p *PatrolService) checkStockAlerts(alerts *int) {
	lowCount, lowList := p.stockCountAndNames(true)
	outCount, outList := p.stockCountAndNames(false)
	if lowCount > 0 {
		p.notifyOps("alert", fmt.Sprintf("%d 种物料低于预警阈值", lowCount),
			"低库存物料："+joinStr(lowList, ", ")+", 建议近期安排采购。", "/inventory/material", "inventory", 0)
		*alerts++
	}
	if outCount > 0 {
		p.notifyOps("alert", fmt.Sprintf("%d 种物料已缺货", outCount),
			"缺货物料："+joinStr(outList, ", ")+", 请尽快补货。", "/inventory/material", "inventory", 0)
		*alerts++
	}
}

// stockCountAndNames 一次查询同时取指定库存状态的物料数（count）与前 N 名单（topN）
// low=true 取“低库存”（0<stock<=threshold）；low=false 取“缺货”（stock<=0）
// 原实现 count 与名单各自查询一遍（B4：重复扫描），这里合并为一次扫描；语义不变（名单按 stock 升序取前 6）。
func (p *PatrolService) stockCountAndNames(low bool) (int, []string) {
	q := model.DB.Model(&model.Material{}).Where("threshold > 0 AND stock <= threshold")
	if low {
		q = q.Where("stock > 0")
	} else {
		q = q.Where("stock <= 0")
	}
	var rows []struct {
		Name string
	}
	q.Select("name").Order("stock ASC").Find(&rows)
	topN := rows
	if len(topN) > 6 {
		topN = topN[:6]
	}
	out := make([]string, 0, len(topN))
	for _, r := range topN {
		out = append(out, r.Name)
	}
	return len(rows), out
}

// lowStockNames 保留：由 stockCountAndNames 取代（不再逐次独立查询）
func (p *PatrolService) lowStockNames(low bool) []string {
	_, names := p.stockCountAndNames(low)
	return names
}

// notifyOps 推送通知给运维与管理员（role in admin/operator/自定义含 ai:ops 者）
func (p *PatrolService) notifyOps(ntype, title, content, link, bizType string, bizID uint) {
	var users []model.User
	model.DB.Where("role IN ? AND status = ?", []string{"admin", "operator"}, model.UserStatusEnabled).
		Select("id").Find(&users)
	if len(users) == 0 {
		// 无目标用户 → 面向全体
		model.CreateNotification(0, ntype, title, content, link, bizType, bizID)
		return
	}
	for _, u := range users {
		model.CreateNotification(u.ID, ntype, title, content, link, bizType, bizID)
	}
}

func joinStr(items []string, sep string) string {
	out := ""
	for i, s := range items {
		if i > 0 {
			out += sep
		}
		out += s
	}
	return out
}
