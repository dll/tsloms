package ai

import (
	"time"

	"github.com/tsloms/server/internal/model"
)

// BuildDeviceFacts 从数据库聚合单台设备的预测输入
func BuildDeviceFacts(dev *model.Device) DeviceFacts {
	f := DeviceFacts{
		HwID:         dev.HwID,
		Intersection: dev.Intersection,
		AgeDays:      NowAgeDays(dev.InstalledAt),
		Online:       dev.OnlineStatus,
	}

	// 历史活跃故障数
	var faultCount int64
	model.DB.Model(&model.FaultRecord{}).
		Where("device_hw_id = ?", dev.HwID).Count(&faultCount)
	f.FaultCount = int(faultCount)

	// 近30天故障数 + 类型
	since30 := time.Now().AddDate(0, 0, -30)
	var recent []model.FaultRecord
	model.DB.Where("device_hw_id = ? AND created_at >= ?", dev.HwID, since30).
		Order("created_at DESC").Limit(20).Find(&recent)
	f.RecentFaults = len(recent)
	seen := map[string]bool{}
	for _, r := range recent {
		// 取最近一次故障类型
		if !seen[r.FaultType] {
			seen[r.FaultType] = true
			f.FaultTypes = append(f.FaultTypes, r.FaultType)
		}
	}

	// 电流统计：取该设备最近报文里的电流均值/最大
	var pack model.PacketLog
	model.DB.Where("device_hw_id = ? AND parsed_content LIKE ?", dev.HwID, "%current%").
		Order("received_at DESC").Limit(50).Find(&pack)
	// 简化：统计 fault_records 里的电流
	if len(recent) > 0 {
		var rSum, ySum, gSum uint64
		var mx uint16
		for _, r := range recent {
			rSum += uint64(r.CurrentR)
			ySum += uint64(r.CurrentY)
			gSum += uint64(r.CurrentG)
			if r.CurrentR > mx {
				mx = r.CurrentR
			}
			if r.CurrentY > mx {
				mx = r.CurrentY
			}
			if r.CurrentG > mx {
				mx = r.CurrentG
			}
		}
		n := uint64(len(recent))
		f.AvgCurrentR = float64(rSum) / float64(n)
		f.AvgCurrentY = float64(ySum) / float64(n)
		f.AvgCurrentG = float64(gSum) / float64(n)
		f.MaxCurrent = float64(mx)
	}

	// 近30天离线次数（按报文间隔推算：最近30天报文数少则视为离线频繁）
	var packetCount int64
	model.DB.Model(&model.PacketLog{}).
		Where("device_hw_id = ? AND received_at >= ?", dev.HwID, since30).Count(&packetCount)
	// 粗略：30天应至少签到若干次，若几乎无报文且当前离线视为离线异常
	if packetCount == 0 && !dev.OnlineStatus {
		f.OfflineCount = 3
	} else if packetCount < 5 {
		f.OfflineCount = int((30 - packetCount) / 6)
		if f.OfflineCount < 0 {
			f.OfflineCount = 0
		}
	}

	// 关联异常媒体/反馈
	var fb int64
	model.DB.Model(&model.Feedback{}).
		Where("device_hw_id = ? AND status NOT IN (?)", dev.HwID, []string{"closed"}).Count(&fb)
	f.HasMediaAnomaly = fb > 0

	return f
}

// RunRulePrediction 对给定设备运行规则预测，保存到 ai_predictions（按设备+批次覆盖）
func RunRulePrediction(dev *model.Device, batchID string) Prediction {
	p := PredictDevice(BuildDeviceFacts(dev))
	// 同一设备+批次幂等：先删旧记录再插入
	model.DB.Where("device_hw_id = ? AND batch_id = ?", dev.HwID, batchID).Delete(&model.AIPrediction{})
	model.DB.Create(&model.AIPrediction{
		DeviceHwID:   p.DeviceHwID,
		Intersection: p.Intersection,
		BatchID:      batchID,
		HealthScore:  p.HealthScore,
		RiskLevel:    p.RiskLevel,
		PredictType:  p.PredictType,
		RemainDays:   p.RemainDays,
		Confidence:   p.Confidence,
		Factors:      jsonFactors(p.Factors),
		Plan:         p.Plan,
		Source:       p.Source,
	})
	return p
}
