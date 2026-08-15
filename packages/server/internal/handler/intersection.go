package handler

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ListIntersections 路口维度设备统计列表
// 按 intersection 分组，返回每个路口的设备总数、在线数、故障数、经纬度（取该路口首台设备）
func ListIntersections(c *gin.Context) {
	// 查询所有设备，按路口在 Go 层聚合（兼容 SQLite/MySQL）
	var devices []model.Device
	model.DB.Select("id, hw_id, intersection, lat, lng, online_status").
		Where("intersection <> ''").
		Find(&devices)

	// 聚合统计
	type Agg struct {
		Total   int
		Online  int
		Fault   int
		Lat     *float64
		Lng     *float64
	}
	agg := map[string]*Agg{}

	// 查询活跃故障按设备分组（一次取回）
	faultDevices := map[uint32]bool{}
	var faults []model.FaultRecord
	model.DB.Select("device_hw_id").Where("status IN ?", []string{
		model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched,
	}).Find(&faults)
	for _, f := range faults {
		faultDevices[f.DeviceHwID] = true
	}

	for _, d := range devices {
		a := agg[d.Intersection]
		if a == nil {
			a = &Agg{Lat: d.Lat, Lng: d.Lng}
			agg[d.Intersection] = a
		}
		a.Total++
		if d.OnlineStatus {
			a.Online++
		}
		if faultDevices[d.HwID] {
			a.Fault++
		}
	}

	// 组装返回结果
	type row struct {
		Intersection string   `json:"intersection"`
		DeviceTotal  int      `json:"device_total"`
		Online       int      `json:"online"`
		Offline      int      `json:"offline"`
		Fault        int      `json:"fault"`
		Lat          *float64 `json:"lat"`
		Lng          *float64 `json:"lng"`
	}
	result := make([]row, 0, len(agg))
	for name, a := range agg {
		result = append(result, row{
			Intersection: name,
			DeviceTotal:  a.Total,
			Online:       a.Online,
			Offline:      a.Total - a.Online,
			Fault:        a.Fault,
			Lat:          a.Lat,
			Lng:          a.Lng,
		})
	}

	// 按设备数降序
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].DeviceTotal > result[i].DeviceTotal {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	ok(c, gin.H{"list": result, "total": len(result)})
}

// RenameIntersection 重命名路口（批量更新该路口下所有设备的 intersection）
// 仅运维/管理员
func RenameIntersection(c *gin.Context) {
	var req struct {
		Old string `json:"old" binding:"required"`
		New string `json:"new" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "old 与 new 必填")
		return
	}
	if req.Old == req.New {
		badRequest(c, "新路口名不能与旧名相同")
		return
	}
	res := model.DB.Model(&model.Device{}).Where("intersection = ?", req.Old).Update("intersection", req.New)
	if res.Error != nil {
		serverError(c, res.Error)
		return
	}
	recordOperation(c, model.OpUpdate, "intersection/"+req.Old, fmt.Sprintf("重命名路口 %s → %s（%d 台设备）", req.Old, req.New, res.RowsAffected))
	ok(c, gin.H{"message": "路口已重命名", "affected": res.RowsAffected})
}

// SetIntersectionLocation 设置路口经纬度（该路口下所有设备同步 lat/lng，供地图打点）
func SetIntersectionLocation(c *gin.Context) {
	var req struct {
		Intersection string   `json:"intersection" binding:"required"`
		Lat          float64  `json:"lat" binding:"required"`
		Lng          float64  `json:"lng" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "路口名与经纬度必填")
		return
	}
	if req.Lat < -90 || req.Lat > 90 || req.Lng < -180 || req.Lng > 180 {
		badRequest(c, "经纬度范围非法")
		return
	}
	res := model.DB.Model(&model.Device{}).
		Where("intersection = ?", req.Intersection).
		Updates(map[string]interface{}{"lat": req.Lat, "lng": req.Lng})
	if res.Error != nil {
		serverError(c, res.Error)
		return
	}
	recordOperation(c, model.OpUpdate, "intersection/"+req.Intersection, fmt.Sprintf("设置路口 %s 经纬度（%d 台设备）", req.Intersection, res.RowsAffected))
	ok(c, gin.H{"message": "路口经纬度已设置", "affected": res.RowsAffected})
}

// ClearIntersection 清空路口（将该路口下设备的 intersection 置空，设备回到未分配）
// 仅管理员
func ClearIntersection(c *gin.Context) {
	intersection := c.Query("intersection")
	if intersection == "" {
		badRequest(c, "intersection 必填")
		return
	}
	res := model.DB.Model(&model.Device{}).Where("intersection = ?", intersection).Update("intersection", "")
	if res.Error != nil {
		serverError(c, res.Error)
		return
	}
	recordOperation(c, model.OpDelete, "intersection/"+intersection, fmt.Sprintf("清空路口 %s（%d 台设备）", intersection, res.RowsAffected))
	ok(c, gin.H{"message": "路口已清空", "affected": res.RowsAffected})
}
