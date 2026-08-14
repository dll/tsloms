package handler

import (
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
	model.DB.Select("device_hw_id").Where("status = ?", "active").Find(&faults)
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
