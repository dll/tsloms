package service

import (
	"context"
	"time"

	"github.com/tsloms/server/internal/config"
	"github.com/tsloms/server/internal/logger"
	"github.com/tsloms/server/internal/model"
	"go.uber.org/zap"
)

// OfflineCheck 设备离线判定
// 规则（PRD §4.1）：超过签到周期（checkinMin）的 N 倍未收到签到包，标记设备离线。
// 默认 offlineAfterMin=6（= 2分钟签到周期 × 3倍）。
// 实际判断依据：设备最后签到时间 last_checkin_at，超过阈值即置 online_status=false。
type OfflineCheck struct {
	logger  *zap.Logger
	timeout time.Duration
	done    chan struct{}
}

// NewOfflineCheck 创建离线检测器
func NewOfflineCheck(cfg *config.Config) *OfflineCheck {
	timeout := time.Duration(cfg.OfflineAfterMin) * time.Minute
	if timeout <= 0 {
		timeout = 6 * time.Minute
	}
	return &OfflineCheck{
		logger:  logger.Get(),
		timeout: timeout,
		done:    make(chan struct{}),
	}
}

// Start 启动后台离线检测（go 协程）
func (o *OfflineCheck) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		o.runOnce()
		for {
			select {
			case <-ctx.Done():
				close(o.done)
				return
			case <-ticker.C:
				o.runOnce()
			}
		}
	}()
}

// Done 返回停止信号（测试/优雅退出用）
func (o *OfflineCheck) Done() <-chan struct{} {
	return o.done
}

// runOnce 执行一次离线判定
func (o *OfflineCheck) runOnce() {
	if model.DB == nil {
		return
	}
	threshold := time.Now().Add(-o.timeout)
	// 将超时且当前仍为在线的设备置为离线
	result := model.DB.Model(&model.Device{}).
		// 兼容迁移前/测试中仅有 last_checkin_at、尚未回填 access_status 的历史设备：
		// 有有效签到时间即可视为曾接入；仅真正从未签到的预登记设备不参与离线判定。
		Where("lifecycle_status <> ? AND online_status = ? AND last_checkin_at IS NOT NULL AND last_checkin_at < ?", "retired", true, threshold).
		Updates(map[string]interface{}{"online_status": false, "access_status": "offline"})
	if result.Error != nil {
		o.logger.Error("离线检测更新失败", zap.Error(result.Error))
		return
	}
	if result.RowsAffected > 0 {
		o.logger.Info("设备离线判定", zap.Int64("count", result.RowsAffected),
			zap.Duration("timeout", o.timeout))
	}
}
