package service

import (
	"context"
	"time"

	"github.com/tsloms/server/internal/model"
	"go.uber.org/zap"
)

// WorkOrderEscalator 工单超时升级检测
// 业务规则：
//   - pending 超过 SLA（24h）未派单 → 自动升级为 processing，提示管理员尽快派单
//   - processing 超过 SLA（48h）未完成 → 由前端/看板标记"超时"，提醒管理员介入（不改状态）
//
// 设计成与 OfflineCheck 一致的后台协程模式，每 15 分钟扫描一次。
type WorkOrderEscalator struct {
	logger *zap.Logger
	done   chan struct{}
}

// NewWorkOrderEscalator 创建工单超时升级器
func NewWorkOrderEscalator() *WorkOrderEscalator {
	logger, _ := zap.NewProduction()
	return &WorkOrderEscalator{
		logger: logger,
		done:   make(chan struct{}),
	}
}

// Start 启动后台超时升级协程
func (e *WorkOrderEscalator) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(15 * time.Minute)
		defer ticker.Stop()
		e.runOnce()
		for {
			select {
			case <-ctx.Done():
				close(e.done)
				return
			case <-ticker.C:
				e.runOnce()
			}
		}
	}()
}

// Done 返回停止信号（测试/优雅退出用）
func (e *WorkOrderEscalator) Done() <-chan struct{} {
	return e.done
}

// runOnce 执行一次超时升级扫描：
//   - 将超时未派单的 pending 工单自动转为 processing（视为已自动升级，待指派）
//   - 处理中超时工单保持 processing（仅由看板/列表标注超时预警）
func (e *WorkOrderEscalator) runOnce() {
	if model.DB == nil {
		return
	}
	now := time.Now()
	// 1) pending 超时 → 自动升级为 processing
	pendingThreshold := now.Add(-time.Duration(model.WorkOrderPendingSLASeconds) * time.Second)
	var pendings []model.WorkOrder
	model.DB.Where("status = ? AND created_at < ?", model.WorkOrderStatusPending, pendingThreshold).
		Find(&pendings)
	upgraded := 0
	for _, wo := range pendings {
		res := model.DB.Model(&wo).Updates(map[string]interface{}{
			"status": model.WorkOrderStatusProcessing,
			"result": "（系统自动升级）待处理超时，自动转为处理中，请尽快派单。",
		})
		if res.Error == nil && res.RowsAffected > 0 {
			upgraded++
		}
	}
	if upgraded > 0 {
		e.logger.Warn("工单自动升级", zap.Int("pending_overdue_auto_upgraded", upgraded))
	}

	// 2) processing 超时统计（供日志预警，状态保持不变）
	procThreshold := now.Add(-time.Duration(model.WorkOrderProcessingSLASeconds) * time.Second)
	var procOverdue int64
	model.DB.Model(&model.WorkOrder{}).
		Where("status = ? AND created_at < ?", model.WorkOrderStatusProcessing, procThreshold).
		Count(&procOverdue)
	if procOverdue > 0 {
		e.logger.Warn("处理中超时工单", zap.Int64("processing_overdue", procOverdue),
			zap.String("hint", "建议管理员介入协调，尽快闭环"))
	}
}
