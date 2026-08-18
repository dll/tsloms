package handler

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/tsloms/server/internal/model"
	"github.com/tsloms/server/internal/service"
)

// ============================================================================
// P1 自动巡检：REST 处理器（独立命名空间 /patrol/*）
// ----------------------------------------------------------------------------
// - GET  /patrol/tasks              巡检任务列表
// - POST /patrol/tasks              创建巡检任务
// - POST /patrol/tasks/:id/run      触发执行（area/street/random/selfcheck/ai）
// - GET  /patrol/tasks/:id          任务详情
// - PUT  /patrol/tasks/:id          更新任务
// - DELETE /patrol/tasks/:id        删除任务（级联记录）
// - GET  /patrol/records            巡检记录（分页+过滤）
// - GET  /patrol/ranking            巡检排行（按巡检人/设备）
// - POST /patrol/selfcheck          信号灯自检：对一组设备直接判定（可无任务）
//
// 所有实现委托 service.PatrolTaskService；仅消费既有数据产物，不修改其逻辑。
// ============================================================================

func patrolSvc() *service.PatrolTaskService {
	return service.NewPatrolTaskService()
}

// ListPatrolTasks GET /patrol/tasks
func ListPatrolTasks(c *gin.Context) {
	page, pageSize := paginate(c)
	tasks, total, err := patrolSvc().ListTasks(page, pageSize)
	if err != nil {
		serverError(c, err)
		return
	}
	view := make([]gin.H, 0, len(tasks))
	for i := range tasks {
		view = append(view, patrolTaskView(&tasks[i]))
	}
	ok(c, gin.H{"list": view, "total": total, "page": page, "page_size": pageSize})
}

// CreatePatrolTask POST /patrol/tasks
func CreatePatrolTask(c *gin.Context) {
	var req struct {
		Name        string `json:"name" binding:"required"`
		Mode        string `json:"mode" binding:"required"`
		AreaID      *uint  `json:"area_id"`
		StreetID    *uint  `json:"street_id"`
		TimeWindow  string `json:"time_window"`
		TargetCount int    `json:"target_count"`
		AssigneeID  *uint  `json:"assignee_id"`
		Remark      string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误: "+err.Error())
		return
	}
	task := &model.PatrolTask{
		Name:        req.Name,
		Mode:        req.Mode,
		AreaID:      req.AreaID,
		StreetID:    req.StreetID,
		TimeWindow:  req.TimeWindow,
		TargetCount: req.TargetCount,
		AssigneeID:  req.AssigneeID,
		Remark:      req.Remark,
		CreatedBy:   c.GetUint("user_id"),
		Status:      model.PatrolStatusPlanned,
	}
	task, err := patrolSvc().CreateTask(task)
	if err != nil {
		badRequest(c, err.Error())
		return
	}
	recordOperation(c, model.OpCreate, "patrol/task", "创建巡检任务 "+task.Name)
	ok(c, gin.H{"task": patrolTaskView(task)})
}

// RunPatrolTask POST /patrol/tasks/:id/run
func RunPatrolTask(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "任务ID无效")
		return
	}
	patrolBy := c.GetString("username")
	if patrolBy == "" {
		patrolBy = c.GetString("op_username")
	}
	if patrolBy == "" {
		patrolBy = "system"
	}
	created, abnormal, err := patrolSvc().RunTask(id, patrolBy)
	if err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpUpdate, "patrol/task/run", "执行巡检任务")
	ok(c, gin.H{
		"task_id":   id,
		"created":   created,
		"abnormal":  abnormal,
		"message":   "巡检执行完成",
		"patrol_by": patrolBy,
	})
}

// GetPatrolTask GET /patrol/tasks/:id
func GetPatrolTask(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "任务ID无效")
		return
	}
	task, err := patrolSvc().GetTask(id, nil)
	if err != nil {
		notFound(c, "巡检任务不存在")
		return
	}
	ok(c, gin.H{"task": patrolTaskView(task)})
}

// UpdatePatrolTask PUT /patrol/tasks/:id
func UpdatePatrolTask(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "任务ID无效")
		return
	}
	var patch map[string]interface{}
	if err := c.ShouldBindJSON(&patch); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if err := patrolSvc().UpdateTask(id, patch); err != nil {
		serverError(c, err)
		return
	}
	task, _ := patrolSvc().GetTask(id, nil)
	recordOperation(c, model.OpUpdate, "patrol/task", "更新巡检任务")
	ok(c, gin.H{"task": patrolTaskView(task)})
}

// DeletePatrolTask DELETE /patrol/tasks/:id
func DeletePatrolTask(c *gin.Context) {
	id, err := parseUint(c.Param("id"))
	if err != nil {
		badRequest(c, "任务ID无效")
		return
	}
	if err := patrolSvc().DeleteTask(id); err != nil {
		serverError(c, err)
		return
	}
	recordOperation(c, model.OpDelete, "patrol/task", "删除巡检任务")
	ok(c, gin.H{"message": "已删除"})
}

// ListPatrolRecords GET /patrol/records
func ListPatrolRecords(c *gin.Context) {
	page, pageSize := paginate(c)
	f := map[string]string{
		"task_id":      c.Query("task_id"),
		"device_hw_id": c.Query("device_hw_id"),
		"check_result": c.Query("check_result"),
		"patrol_type":  c.Query("patrol_type"),
		"patrol_by":    c.Query("patrol_by"),
		"start_time":   c.Query("start_time"),
		"end_time":     c.Query("end_time"),
	}
	list, total, err := patrolSvc().ListRecords(page, pageSize, f)
	if err != nil {
		serverError(c, err)
		return
	}
	view := make([]gin.H, 0, len(list))
	for i := range list {
		view = append(view, patrolRecordView(&list[i]))
	}
	ok(c, gin.H{"list": view, "total": total, "page": page, "page_size": pageSize})
}

// GetPatrolRanking GET /patrol/ranking
// 维度：默认 by 巡检人；?dimension=device 按设备；?limit=N 控制条数。
func GetPatrolRanking(c *gin.Context) {
	dim := c.DefaultQuery("dimension", "by")
	limit := 20
	if v := c.Query("limit"); v != "" {
		if n, err := parseUint(v); err == nil && n > 0 {
			limit = int(n)
		}
	}
	items, err := patrolSvc().Ranking(dim, limit)
	if err != nil {
		serverError(c, err)
		return
	}
	ok(c, gin.H{"list": items, "dimension": dim})
}

// PostPatrolSelfCheck POST /patrol/selfcheck
// 信号灯自检：对一组设备直接采集判定（无需预建任务）。
// 请求: {device_ids?: [..] | device_hw_ids?: [..], validate?: bool}
// 响应: {list: [{hw_id, online, err_code, result, detail}]}
func PostPatrolSelfCheck(c *gin.Context) {
	var req struct {
		DeviceIDs   []uint   `json:"device_ids"`
		DeviceHwIDs []string `json:"device_hw_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c, "参数错误")
		return
	}
	if len(req.DeviceIDs) == 0 && len(req.DeviceHwIDs) == 0 {
		badRequest(c, "请指定 device_ids 或 device_hw_ids")
		return
	}

	svc := patrolSvc()
	patrolBy := c.GetString("username")
	if patrolBy == "" {
		patrolBy = "system"
	}
	now := time.Now()

	out := make([]gin.H, 0)
	created, abnormal := 0, 0
	// 按 device_id 自检（无需任务）
	if len(req.DeviceIDs) > 0 {
		var devices []model.Device
		model.DB.Where("id IN ?", req.DeviceIDs).Find(&devices)
		// 复用 RunTask 的路由：构造一个临时任务（mode=selfcheck）
		task := &model.PatrolTask{Name: "即时自检", Mode: model.PatrolModeSelfCheck, Status: model.PatrolStatusDone}
		for _, d := range devices {
			rec, isAbnormal := svc.BuildRecordForSelfCheck(task, &d, patrolBy, now)
			if rec == nil {
				continue
			}
			model.DB.Create(rec)
			created++
			if isAbnormal {
				abnormal++
			}
			out = append(out, gin.H{
				"hw_id": d.HwID, "online": d.OnlineStatus,
				"result": rec.CheckResult, "detail": rec.CheckDetail,
				"selfcheck_result": rec.SelfCheckResult,
			})
		}
	}
	// 按 hw_id 自检
	if len(req.DeviceHwIDs) > 0 {
		var devices []model.Device
		model.DB.Where("hw_id IN ?", req.DeviceHwIDs).Find(&devices)
		task := &model.PatrolTask{Name: "即时自检", Mode: model.PatrolModeSelfCheck, Status: model.PatrolStatusDone}
		for _, d := range devices {
			rec, isAbnormal := svc.BuildRecordForSelfCheck(task, &d, patrolBy, now)
			if rec == nil {
				continue
			}
			model.DB.Create(rec)
			created++
			if isAbnormal {
				abnormal++
			}
			out = append(out, gin.H{
				"hw_id": d.HwID, "online": d.OnlineStatus,
				"result": rec.CheckResult, "detail": rec.CheckDetail,
				"selfcheck_result": rec.SelfCheckResult,
			})
		}
	}

	recordOperation(c, model.OpCreate, "patrol/selfcheck", "信号灯自检")
	ok(c, gin.H{"list": out, "created": created, "abnormal": abnormal})
}

// ---- 视图辅助 ----

// patrolTaskView 任务携带冗余字段便于前端展示。
func patrolTaskView(t *model.PatrolTask) gin.H {
	return gin.H{
		"id": t.ID, "name": t.Name, "mode": t.Mode,
		"area_id": t.AreaID, "street_id": t.StreetID,
		"time_window": t.TimeWindow, "target_count": t.TargetCount,
		"status": t.Status, "assignee_id": t.AssigneeID,
		"created_by": t.CreatedBy, "run_count": t.RunCount,
		"last_run_at": t.LastRunAt, "remark": t.Remark,
		"created_at": t.CreatedAt, "updated_at": t.UpdatedAt,
	}
}

// patrolRecordView 记录附带 路口名/设备路口 冗余展示。
func patrolRecordView(r *model.PatrolRecord) gin.H {
	v := gin.H{
		"id": r.ID, "task_id": r.TaskID, "device_id": r.DeviceID,
		"device_hw_id": r.DeviceHwID, "crossing_id": r.CrossingID,
		"patrol_type": r.PatrolType, "check_result": r.CheckResult,
		"check_detail": r.CheckDetail, "selfcheck_result": r.SelfCheckResult,
		"evidences": r.Evidences, "patrol_by": r.PatrolBy,
		"patrol_at": r.PatrolAt, "lat": r.Lat, "lng": r.Lng,
		"created_at": r.CreatedAt,
	}
	if r.CrossingID != nil {
		var crossing model.Crossing
		if model.DB.First(&crossing, *r.CrossingID).Error == nil {
			v["crossing_name"] = crossing.Name
		}
	}
	var device model.Device
	if model.DB.Where("hw_id = ?", r.DeviceHwID).First(&device).Error == nil {
		v["intersection"] = device.Intersection
	}
	return v
}
