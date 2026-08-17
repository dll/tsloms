package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P0-5 地图分级渐变着色 + 下钻
// ----------------------------------------------------------------------------
// GET /map/crossing-data   路口聚合数据（全量轻量，供 Cesium 按故障比例/绿占比着色）
//                         返回每个路口：id/name/lat/lng/status/device_total/
//                         fault_ratio(故障设备/全部)/green_ratio(正常设备/全部)/level
// GET /map/road-data       道路级聚合（由路口再上卷，整条路一段一色）【可选】
// GET /crossings/:id/devices + GET /devices 供“道路→路口→具体故障设备点(红)”下钻。
//
// 分级算法（对齐需求）：
//   - 全部设备 => fault_ratio==0 && green_ratio>0 ：正常（绿）
//   - 故障比例由 0→1 递增：由绿→黄→红 渐变（前端据此插值着色）
//   - 全部故障/断电（fault_ratio>=1）：全红（停电/线路）→ offline
//
// 聚合为实时计算（路口设备数少），并把结果冗余回写 crossings.status/fault_ratio/green_ratio，
// 供列表/地图一次拉取（对齐参考项目 a 的 /getMapData 轻量全量接口）。
// ============================================================================

// crossingPoly 单路口聚合计算结果
type crossingPoly struct {
	ID          uint     `json:"id"`
	PointNo     string   `json:"point_no"`
	Name        string   `json:"name"`
	RoadName    string   `json:"road_name"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	Status      string   `json:"status"`
	DeviceTotal int      `json:"device_total"`
	FaultCount  int      `json:"fault_count"`
	OnlineCount int      `json:"online_count"`
	FaultRatio  float64  `json:"fault_ratio"` // 故障设备/全部
	GreenRatio  float64  `json:"green_ratio"` // 正常(在线且无故障)/全部
	Level       string   `json:"level"`       // 由比例导出的着色档：green/yellow/red
	AreaPath    string   `json:"area_path"`   // 区划完整名称
}

// deriveColorLevel 由故障/绿占比推导着色档：
//   - fault_ratio>=1 → red（全部红/停电/线路）
//   - fault_ratio==0 → green（全绿/正常）
//   - 0<fault_ratio<1 → 由绿→黄→红渐变；按阈值二分：<0.34 yellow_low, <0.67 yellow, else red
//
// 首版返回三档（green/yellow/red），前端可依据 fault_ratio 数值做连续插值（绿→黄→红）。
func deriveColorLevel(faultRatio float64) string {
	if faultRatio <= 0 {
		return "green"
	}
	if faultRatio >= 1 {
		return "red"
	}
	if faultRatio < 0.34 {
		return "yellow_low"
	}
	if faultRatio < 0.67 {
		return "yellow"
	}
	return "orange"
}

// computeCrossingPoly 计算单个路口的聚合：关联设备 + 活跃故障/离线
// faultCount = 存在活跃故障(未解决) 或 离线 的设备数；greenRatio = 在线且无故障设备占比。
func computeCrossingPoly(x *model.Crossing) crossingPoly {
	p := crossingPoly{
		ID: x.ID, PointNo: x.PointNo, Name: x.Name, RoadName: x.RoadName,
		Lat: x.Lat, Lng: x.Lng, AreaPath: crossingAreaFullName(x),
	}

	var devices []model.Device
	model.DB.Where("crossing_id = ?", x.ID).Find(&devices)
	p.DeviceTotal = len(devices)

	// 活跃故障设备集合
	faultSet := map[uint32]bool{}
	var hwIDs []uint32
	for _, d := range devices {
		hwIDs = append(hwIDs, d.HwID)
	}
	if len(hwIDs) > 0 {
		var faults []model.FaultRecord
		model.DB.Select("device_hw_id").Where("device_hw_id IN ? AND status IN ?",
			hwIDs, []string{model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched}).
			Find(&faults)
		for _, f := range faults {
			faultSet[f.DeviceHwID] = true
		}
	}

	for _, d := range devices {
		if d.OnlineStatus && !faultSet[d.HwID] {
			p.OnlineCount++
			p.GreenRatio += 1
		}
		if faultSet[d.HwID] || !d.OnlineStatus {
			p.FaultCount++
		}
	}
	if p.DeviceTotal > 0 {
		p.GreenRatio = p.GreenRatio / float64(p.DeviceTotal)
		p.FaultRatio = float64(p.FaultCount) / float64(p.DeviceTotal)
	}
	p.Level = deriveColorLevel(p.FaultRatio)
	p.Status = model.ComputeCrossingStatus(p.FaultRatio, p.GreenRatio)

	// 回写缓存（非阻塞；失败不阻断返回）
	now := time.Now()
	model.DB.Model(&model.Crossing{}).Where("id = ?", x.ID).Updates(map[string]interface{}{
		"status": p.Status, "fault_ratio": p.FaultRatio, "green_ratio": p.GreenRatio, "updated_at": now,
	})
	return p
}

// RecomputeCrossingCache 刷新全部路口聚合缓存（列表/地图一次拉取）。测试可用。
func RecomputeCrossingCache() {
	var crossings []model.Crossing
	model.DB.Find(&crossings)
	for i := range crossings {
		computeCrossingPoly(&crossings[i])
	}
}

// GetCrossingMapData GET /map/crossing-data
// 返回全部路口聚合（实时计算 + 回写缓存）。query 可选 road_id/street_id 过滤。
func GetCrossingMapData(c *gin.Context) {
	q := model.DB.Model(&model.Crossing{})
	if roadID := c.Query("road_id"); roadID != "" {
		if v, err := parseUint(roadID); err == nil {
			q = q.Where("road_id = ?", v)
		}
	}
	if streetID := c.Query("street_id"); streetID != "" {
		if v, err := parseUint(streetID); err == nil {
			q = q.Where("street_id = ?", v)
		}
	}
	var crossings []model.Crossing
	q.Find(&crossings)

	out := make([]crossingPoly, 0, len(crossings))
	for i := range crossings {
		out = append(out, computeCrossingPoly(&crossings[i]))
	}
	ok(c, gin.H{"list": out, "total": len(out)})
}

// roadAgg 道路级聚合（一段一色）
type roadAgg struct {
	RoadID      *uint   `json:"road_id,omitempty"`
	RoadName    string  `json:"road_name"`
	CrossingCnt int     `json:"crossing_count"`
	DeviceTotal int     `json:"device_total"`
	FaultTotal  int     `json:"fault_total"`
	FaultRatio  float64 `json:"fault_ratio"`
	GreenRatio  float64 `json:"green_ratio"`
	Level       string  `json:"level"`
	Crossings   []uint  `json:"crossing_ids"`
}

// GetRoadMapData GET /map/road-data —— 道路级聚合（由路口上卷）
func GetRoadMapData(c *gin.Context) {
	var crossings []model.Crossing
	model.DB.Where("road_name <> ''").Order("road_name ASC").Find(&crossings)
	roadMap := map[string]*roadAgg{}
	for i := range crossings {
		p := computeCrossingPoly(&crossings[i])
		name := p.RoadName
		if name == "" {
			name = "(未归类)"
		}
		r, ok := roadMap[name]
		if !ok {
			r = &roadAgg{RoadName: name}
			roadMap[name] = r
		}
		r.CrossingCnt++
		r.CrossingIdsAppend(crossings[i].ID)
		r.DeviceTotal += p.DeviceTotal
		r.FaultTotal += p.FaultCount
	}
	out := make([]roadAgg, 0, len(roadMap))
	for _, r := range roadMap {
		if r.DeviceTotal > 0 {
			r.FaultRatio = float64(r.FaultTotal) / float64(r.DeviceTotal)
			r.GreenRatio = 1 - r.FaultRatio
		}
		r.Level = deriveColorLevel(r.FaultRatio)
		out = append(out, *r)
	}
	ok(c, gin.H{"list": out, "total": len(out)})
}

// CrossingIdsAppend 追加路口 ID（辅助）
func (r *roadAgg) CrossingIdsAppend(id uint) {
	r.Crossings = append(r.Crossings, id)
}
