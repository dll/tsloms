package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// P0-4 路口/行政区划
// ----------------------------------------------------------------------------
// GET    /crossings                     路口列表（分页+过滤 road/area/status）
// POST   /crossings                     新增路口
// GET    /crossings/:id                 路口详情
// PUT    /crossings/:id                 编辑路口（含经纬度/区划）
// DELETE /crossings/:id                 删除路口（解除设备挂接）
// GET    /crossings/:id/devices         路口下设备（地图下钻/聚合下游）
// GET    /areas/tree                    行政区划树
// POST   /areas                         新增区划（area:manage）
// PUT    /areas/:id                     编辑区划
// DELETE /areas/:id                     删除区划
//
// 设备挂接区划/路口：扩展已有 PUT /devices/:id 与 POST /devices 接收 crossing_id/区划字段。
// ============================================================================

// crossingView 路口视图：附带区划完整名称
func crossingView(x *model.Crossing) gin.H {
	v := gin.H{
		"id": x.ID, "point_no": x.PointNo, "name": x.Name, "type": x.Type,
		"province_id": x.ProvinceID, "city_id": x.CityID, "district_id": x.DistrictID,
		"street_id": x.StreetID, "community_id": x.CommunityID, "road_id": x.RoadID,
		"road_name": x.RoadName, "lat": x.Lat, "lng": x.Lng, "status": x.Status,
		"fault_ratio": x.FaultRatio, "green_ratio": x.GreenRatio, "remark": x.Remark,
		"created_at": x.CreatedAt, "updated_at": x.UpdatedAt,
	}
	// 区划完整路径名称（省→市→区→街道→社区）
	v["area_full_name"] = crossingAreaFullName(x)
	return v
}

// crossingAreaFullName 拼接路口所属区划完整名称
func crossingAreaFullName(x *model.Crossing) string {
	if x == nil {
		return ""
	}
	var names []string
	for _, id := range []*uint{x.ProvinceID, x.CityID, x.DistrictID, x.StreetID, x.CommunityID} {
		if id == nil {
			continue
		}
		var a model.Area
		if model.DB.First(&a, *id).Error == nil {
			if a.Name != "" {
				names = append(names, a.Name)
			}
		}
	}
	if x.RoadName != "" {
		names = append(names, x.RoadName)
	}
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "/"
		}
		out += n
	}
	return out
}

// ListCrossings GET /crossings
func ListCrossings(c *gin.Context) {
	page, pageSize := paginate(c)
	q := model.DB.Model(&model.Crossing{})

	if kw := c.Query("keyword"); kw != "" {
		like := "%" + kw + "%"
		q = q.Where("name LIKE ? OR road_name LIKE ? OR point_no LIKE ?", like, like, like)
	}
	if road := c.Query("road_id"); road != "" {
		if v, err := parseUint(road); err == nil {
			q = q.Where("road_id = ?", v)
		}
	}
	if street := c.Query("street_id"); street != "" {
		if v, err := parseUint(street); err == nil {
			q = q.Where("street_id = ?", v)
		}
	}
	if status := c.Query("status"); status != "" {
		q = q.Where("status = ?", status)
	}

	var total int64
	q.Count(&total)

	var list []model.Crossing
	q.Order("created_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)
	view := make([]gin.H, 0, len(list))
	for i := range list {
		view = append(view, crossingView(&list[i]))
	}
	ok(c, gin.H{"list": view, "total": total, "page": page, "page_size": pageSize})
}

// GetCrossing GET /crossings/:id
func GetCrossing(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "路口ID无效")
		return
	}
	var x model.Crossing
	if err := model.DB.First(&x, id).Error; err != nil {
		notFound(c, "路口不存在")
		return
	}
	ok(c, gin.H{"crossing": crossingView(&x)})
}

// crossingRequest 路口请求体
type crossingRequest struct {
	PointNo     string   `json:"point_no"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	ProvinceID  *uint    `json:"province_id"`
	CityID      *uint    `json:"city_id"`
	DistrictID  *uint    `json:"district_id"`
	StreetID    *uint    `json:"street_id"`
	CommunityID *uint    `json:"community_id"`
	RoadID      *uint    `json:"road_id"`
	RoadName    string   `json:"road_name"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	Status      string   `json:"status"`
	Remark      string   `json:"remark"`
}

func applyCrossing(x *model.Crossing, req *crossingRequest) {
	if req.PointNo != "" {
		x.PointNo = req.PointNo
	}
	if req.Name != "" {
		x.Name = req.Name
	}
	if req.Type != "" {
		x.Type = req.Type
	}
	if req.ProvinceID != nil {
		x.ProvinceID = req.ProvinceID
	}
	if req.CityID != nil {
		x.CityID = req.CityID
	}
	if req.DistrictID != nil {
		x.DistrictID = req.DistrictID
	}
	if req.StreetID != nil {
		x.StreetID = req.StreetID
	}
	if req.CommunityID != nil {
		x.CommunityID = req.CommunityID
	}
	if req.RoadID != nil {
		x.RoadID = req.RoadID
	}
	if req.RoadName != "" {
		x.RoadName = req.RoadName
	}
	if req.Lat != nil {
		x.Lat = req.Lat
	}
	if req.Lng != nil {
		x.Lng = req.Lng
	}
	if req.Status != "" {
		x.Status = req.Status
	}
	if req.Remark != "" {
		x.Remark = req.Remark
	}
}

// CreateCrossing POST /crossings
func CreateCrossing(c *gin.Context) {
	var req crossingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if req.Name == "" {
		badRequest(c, "路口名称必填")
		return
	}
	// 同名路口查重（应用层保证，兼容 SQLite/MySQL）
	var cnt int64
	model.DB.Model(&model.Crossing{}).Where("name = ?", req.Name).Count(&cnt)
	if cnt > 0 {
		badRequest(c, "路口名称已存在")
		return
	}
	x := model.Crossing{Status: model.CrossingStatusNormal}
	applyCrossing(&x, &req)
	if err := model.DB.Create(&x).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, "crossing/"+utoa(x.ID), "新增路口 "+x.Name)
	ok(c, gin.H{"crossing": crossingView(&x), "message": "路口已新增"})
}

// UpdateCrossing PUT /crossings/:id
func UpdateCrossing(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "路口ID无效")
		return
	}
	var req crossingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var x model.Crossing
	if err := model.DB.First(&x, id).Error; err != nil {
		notFound(c, "路口不存在")
		return
	}
	applyCrossing(&x, &req)
	if err := model.DB.Model(&x).Updates(map[string]interface{}{
		"point_no": x.PointNo, "name": x.Name, "type": x.Type,
		"province_id": x.ProvinceID, "city_id": x.CityID, "district_id": x.DistrictID,
		"street_id": x.StreetID, "community_id": x.CommunityID, "road_id": x.RoadID,
		"road_name": x.RoadName, "lat": x.Lat, "lng": x.Lng, "status": x.Status, "remark": x.Remark,
	}).Error; err != nil {
		serverError(c, err)
		return
	}
	model.DB.First(&x, id)
	recordOperation(c, model.OpUpdate, "crossing/"+utoa(x.ID), "更新路口 "+x.Name)
	ok(c, gin.H{"crossing": crossingView(&x), "message": "路口已更新"})
}

// DeleteCrossing DELETE /crossings/:id
func DeleteCrossing(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "路口ID无效")
		return
	}
	var x model.Crossing
	if err := model.DB.First(&x, id).Error; err != nil {
		notFound(c, "路口不存在")
		return
	}
	if err := model.DB.Delete(&x).Error; err != nil {
		serverError(c, err)
		return
	}
	// 解除该路口下设备挂接
	model.DB.Model(&model.Device{}).Where("crossing_id = ?", id).Update("crossing_id", nil)
	recordOperation(c, model.OpDelete, "crossing/"+utoa(id), "删除路口 "+x.Name)
	ok(c, gin.H{"message": "路口已删除"})
}

// GetCrossingDevices GET /crossings/:id/devices —— 路口下设备（地图下钻）
func GetCrossingDevices(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "路口ID无效")
		return
	}
	var x model.Crossing
	if err := model.DB.First(&x, id).Error; err != nil {
		notFound(c, "路口不存在")
		return
	}
	var devices []model.Device
	model.DB.Where("crossing_id = ?", id).Find(&devices)
	ok(c, gin.H{"crossing": crossingView(&x), "devices": devices})
}

// ============================================================================
// 行政区划
// ============================================================================

// areaNode 区划树节点
type areaNode struct {
	ID       uint        `json:"id"`
	Code     string      `json:"code"`
	Name     string      `json:"name"`
	Type     string      `json:"type"`
	FullName string      `json:"full_name"`
	Children []*areaNode `json:"children,omitempty"`
}

// loadAreaTreeRaw 一次性载入全部区划并按 parent_id 组装树
func loadAreaTreeRaw(filterType string) []*areaNode {
	var all []model.Area
	q := model.DB.Model(&model.Area{})
	if filterType != "" {
		q = q.Where("area_type = ?", filterType)
	}
	q.Order("area_sort ASC, id ASC").Find(&all)

	nodes := map[uint]*areaNode{}
	for _, a := range all {
		if _, ok := nodes[a.ID]; !ok {
			nodes[a.ID] = &areaNode{ID: a.ID, Code: a.Code, Name: a.Name, Type: a.AreaType, FullName: a.FullName}
		}
	}
	roots := []*areaNode{}
	for _, a := range all {
		n := nodes[a.ID]
		if a.ParentID != nil {
			if p, ok := nodes[*a.ParentID]; ok {
				p.Children = append(p.Children, n)
				continue
			}
		}
		roots = append(roots, n)
	}
	return roots
}

// ListAreasTree GET /areas/tree?level=&type=
func ListAreasTree(c *gin.Context) {
	typeFilter := c.Query("type")
	roots := loadAreaTreeRaw(typeFilter)
	ok(c, gin.H{"tree": roots, "total": areaTreeCount(roots)})
}

func areaTreeCount(roots []*areaNode) int {
	n := 0
	var walk func(nodes []*areaNode)
	walk = func(nodes []*areaNode) {
		for _, x := range nodes {
			n++
			if len(x.Children) > 0 {
				walk(x.Children)
			}
		}
	}
	walk(roots)
	return n
}

// CreateArea POST /areas
func CreateArea(c *gin.Context) {
	var req model.Area
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if req.Name == "" {
		badRequest(c, "区划名称必填")
		return
	}
	if req.AreaType == "" {
		badRequest(c, "区划层级(area_type)必填")
		return
	}
	if err := model.DB.Create(&req).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpCreate, "area/"+utoa(req.ID), "新增区划 "+req.Name)
	ok(c, gin.H{"area": req, "message": "区划已新增"})
}

// UpdateArea PUT /areas/:id
func UpdateArea(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "区划ID无效")
		return
	}
	var req model.Area
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	var a model.Area
	if err := model.DB.First(&a, id).Error; err != nil {
		notFound(c, "区划不存在")
		return
	}
	updates := map[string]interface{}{
		"name": req.Name, "code": req.Code, "area_type": req.AreaType,
		"area_sort": req.AreaSort, "remark": req.Remark, "full_name": req.FullName,
		"updated_at": time.Now(),
	}
	if err := model.DB.Model(&a).Updates(updates).Error; err != nil {
		serverError(c, err)
		return
	}
	model.DB.First(&a, id)
	recordOperation(c, model.OpUpdate, "area/"+utoa(id), "更新区划 "+a.Name)
	ok(c, gin.H{"area": a, "message": "区划已更新"})
}

// DeleteArea DELETE /areas/:id
func DeleteArea(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "区划ID无效")
		return
	}
	var a model.Area
	if err := model.DB.First(&a, id).Error; err != nil {
		notFound(c, "区划不存在")
		return
	}
	// 防御：存在下级区划时禁止删除
	var child int64
	model.DB.Model(&model.Area{}).Where("parent_id = ?", id).Count(&child)
	if child > 0 {
		badRequest(c, "该区划存在下级区划，请先删除下级")
		return
	}
	if err := model.DB.Delete(&a).Error; err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, "area/"+utoa(id), "删除区划 "+a.Name)
	ok(c, gin.H{"message": "区划已删除"})
}
