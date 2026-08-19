package service

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/tsloms/server/internal/faultcode"
	"github.com/tsloms/server/internal/logger"
	"github.com/tsloms/server/internal/model"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ============================================================================
// PatrolTaskService —— 自动巡检任务执行（P1）
// ----------------------------------------------------------------------------
// 与既有 PatrolService（AI 主动巡检：日报/异常检测）互不干扰：
//   - 本服务基于独立 patrol_tasks / patrol_records 表，独立调度；
//   - 不抢 PatrolService 已有的 AI 日报协程（二者职责不同，各自独立）。
//
// 巡检模式：
//   - area   : 按空间区域(区县 area_id)筛选关联设备；
//   - street : 按街道 street_id 筛选设备；
//   - random : 从全部设备随机抽 target_count 台；
//   - selfcheck : 信号灯自检：对目标设备采集灯态/errCode 判定 normal/abnormal；
//   - ai     : 从 AI 预测(AIPrediction)高危设备接入硬件数据。
//
// 红线保持：仅【消费】既有 MQTT 状态(devices.online_status/last_checkin_at)、
//   FaultRecord(活跃故障 err_code)、AIPrediction 产物，不修改其逻辑。
// ============================================================================

// patrolSelfCheckSnapshot 单台设备自检快照
type patrolSelfCheckSnapshot struct {
	DeviceID   uint     `json:"device_id"`
	HwID       string   `json:"hw_id"`
	Online     bool     `json:"online"`
	ErrCode    int8     `json:"err_code"`
	FaultType  string   `json:"fault_type"`
	FaultLevel string   `json:"fault_level"`
	Lat        *float64 `json:"lat"`
	Lng        *float64 `json:"lng"`
}

// PatrolTaskService 自动巡检任务服务
type PatrolTaskService struct {
	logger *zap.Logger
}

// NewPatrolTaskService 创建自动巡检服务
func NewPatrolTaskService() *PatrolTaskService {
	return &PatrolTaskService{logger: logger.Get()}
}

// ------------------------------------ 任务筛选 ------------------------------------

// selectTargetDevices 按任务模式筛选目标设备（返回设备集合 + 巡检类型标签 + 摘要）。
// 只读查询既有数据，不做任何修改。
func (s *PatrolTaskService) selectTargetDevices(task *model.PatrolTask) ([]model.Device, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	devMode := task.Mode
	// 若不明确指定（如空白），以任务模式为准；none 视为全部设备兜底
	q := model.DB.Model(&model.Device{})
	switch devMode {
	case model.PatrolModeArea:
		if task.AreaID != nil {
			// 空间区域：按区县（devices.district_id）或泛指区域挂接的街道/社区
			q = q.Where("district_id = ? OR province_id = ? OR city_id = ?", *task.AreaID, *task.AreaID, *task.AreaID)
		}
	case model.PatrolModeStreet:
		if task.StreetID != nil {
			q = q.Where("street_id = ?", *task.StreetID)
		} else {
			// 未指定街道 → 无目标
			return nil, nil
		}
	case model.PatrolModeRandom:
		// 随机抽检：target_count 台
		if task.TargetCount <= 0 {
			return nil, fmt.Errorf("random 模式需要 target_count>0")
		}
	case model.PatrolModeSelfCheck, model.PatrolModeAI:
		// 自检/AI 需要按任务限定范围（area/street）或全量；由限定条件过滤
	default:
		// 未知模式 → 全部设备
		devMode = ""
	}
	if devMode != "" && devMode != model.PatrolModeRandom {
		// area/street/selfcheck/ai 均可能挂接 street_id 作进一步范围
		if task.StreetID != nil {
			q = q.Where("street_id = ?", *task.StreetID)
		}
		if task.AreaID != nil && (devMode == model.PatrolModeArea || devMode == model.PatrolModeSelfCheck || devMode == model.PatrolModeAI) {
			q = q.Where("district_id = ? OR province_id = ? OR city_id = ?", *task.AreaID, *task.AreaID, *task.AreaID)
		}
	}
	var devices []model.Device
	q.Find(&devices)
	if devMode == model.PatrolModeRandom {
		// 随机抽检：洗牌后取前 target_count
		if len(devices) > task.TargetCount {
			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			r.Shuffle(len(devices), func(i, j int) { devices[i], devices[j] = devices[j], devices[i] })
			devices = devices[:task.TargetCount]
		}
	}
	if devMode == model.PatrolModeAI {
		// AI 硬件：仅保留 AI 预测为 high/critical 的设备
		devices = filterHighRiskAIDevices(devices)
	}
	return devices, nil
}

// filterHighRiskAIDevices 从候选设备中保留 AI 预测为 high/critical 风险者。
// 复用 AIPrediction 表产物（不改其逻辑）。
func filterHighRiskAIDevices(candidates []model.Device) []model.Device {
	if len(candidates) == 0 {
		return nil
	}
	hwSet := map[string]bool{}
	for _, d := range candidates {
		hwSet[d.HwID] = true
	}
	hwIDs := make([]string, 0, len(hwSet))
	for hw := range hwSet {
		hwIDs = append(hwIDs, hw)
	}
	var preds []model.AIPrediction
	model.DB.Where("device_hw_id IN ? AND risk_level IN ?", hwIDs, []string{"high", "critical"}).
		Select("device_hw_id").Distinct().Find(&preds)
	riskHW := map[string]bool{}
	for _, p := range preds {
		riskHW[p.DeviceHwID] = true
	}
	out := make([]model.Device, 0, len(riskHW))
	for _, d := range candidates {
		if riskHW[d.HwID] {
			out = append(out, d)
		}
	}
	return out
}

// ------------------------------------ 巡检执行 ------------------------------------

// RunTask 执行一次巡检任务。返回执行摘要（新增记录数/异常数/时间）。
//
// 设计：同一任务可重复执行（RunCount+1, last_run_at 更新）；每次执行都落巡检记录，
// 排行随时间聚合。任务状态由 planned → running → done 流转。
func (s *PatrolTaskService) RunTask(taskID uint, patrolBy string) (created int, abnormal int, err error) {
	if model.DB == nil {
		return 0, 0, fmt.Errorf("数据库未初始化")
	}
	var task model.PatrolTask
	if err := model.DB.First(&task, taskID).Error; err != nil {
		return 0, 0, err
	}

	// 状态置 running
	model.DB.Model(&task).Updates(map[string]interface{}{"status": model.PatrolStatusRunning})

	devices, err := s.selectTargetDevices(&task)
	if err != nil {
		model.DB.Model(&task).Updates(map[string]interface{}{"status": model.PatrolStatusDone})
		return 0, 0, err
	}

	now := time.Now()
	// 巡检类型标签：任务类型 + random 时为 random
	ptype := task.Mode
	if ptype == "" {
		ptype = model.PatrolModeRandom
	}

	for _, d := range devices {
		rec, isAbnormal := s.buildRecord(&task, &d, ptype, patrolBy, now)
		if rec == nil {
			continue
		}
		if err := model.DB.Create(rec).Error; err != nil {
			s.logger.Warn("巡检记录落库失败", zap.Error(err), zap.String("hw", d.HwID))
			continue
		}
		created++
		if isAbnormal {
			abnormal++
		}
	}

	// 状态置 done、累积执行次数
	model.DB.Model(&task).Updates(map[string]interface{}{
		"status":      model.PatrolStatusDone,
		"last_run_at": now,
		"run_count":   task.RunCount + 1,
	})
	return created, abnormal, nil
}

// BuildRecordForSelfCheck 供即时自检(handler)复用单设备判定逻辑。
// 用临时任务(mode=selfcheck)驱动 buildRecord，返回记录与是否异常。
func (s *PatrolTaskService) BuildRecordForSelfCheck(task *model.PatrolTask, d *model.Device, patrolBy string, now time.Time) (*model.PatrolRecord, bool) {
	return s.buildRecord(task, d, model.PatrolModeSelfCheck, patrolBy, now)
}

// buildRecord 依据设备现状构造一条巡检记录。isAbnormal 标识判定结果。
func (s *PatrolTaskService) buildRecord(task *model.PatrolTask, d *model.Device, ptype, patrolBy string, now time.Time) (*model.PatrolRecord, bool) {
	if d == nil {
		return nil, false
	}
	rec := &model.PatrolRecord{
		TaskID:      &task.ID,
		DeviceID:    &d.ID,
		DeviceHwID:  d.HwID,
		CrossingID:  d.CrossingID,
		PatrolType:  ptype,
		CheckResult: model.PatrolResultNormal,
		PatrolBy:    patrolBy,
		PatrolAt:    now,
		Lat:         d.Lat,
		Lng:         d.Lng,
	}

	// 活跃故障（消费 FaultRecord 产物，不改其逻辑）
	abnormal := s.hasActiveFault(d.HwID)
	detail := "设备在线，无活跃故障"
	if !d.OnlineStatus {
		abnormal = true
		detail = "设备离线/掉线"
	} else if abnormal {
		detail = "存在活跃故障(err_code 非空)"
	}

	// 信号灯自检：采集灯态/errCode → 判定
	if ptype == model.PatrolModeSelfCheck || ptype == model.PatrolModeAI {
		snap := s.collectSelfCheck(d)
		abnormal = abnormal || snap.ErrCode != faultcode.LEDErrOK
		if snap.ErrCode != faultcode.LEDErrOK {
			detail = fmt.Sprintf("自检异常: err_code=%d (%s %s)", snap.ErrCode, snap.FaultType, snap.FaultLevel)
		} else if !abnormal {
			detail = "信号灯自检正常"
		}
		sb, _ := json.Marshal(snap)
		rec.SelfCheckResult = string(sb)
	}

	if abnormal {
		rec.CheckResult = model.PatrolResultAbnormal
	}
	rec.CheckDetail = detail
	return rec, abnormal
}

// collectSelfCheck 采集单台设备的信号灯自检快照（灯态/errCode 来自活跃故障与在线状态）。
func (s *PatrolTaskService) collectSelfCheck(d *model.Device) patrolSelfCheckSnapshot {
	snap := patrolSelfCheckSnapshot{
		DeviceID: d.ID, HwID: d.HwID, Online: d.OnlineStatus,
		ErrCode: faultcode.LEDErrOK, FaultType: "ok", FaultLevel: "normal",
		Lat: d.Lat, Lng: d.Lng,
	}
	// 取最近活跃故障的 err_code 作为自检码
	var fault model.FaultRecord
	if err := model.DB.Where("device_hw_id = ? AND status IN ?",
		d.HwID, []string{model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched}).
		Order("last_seen DESC").First(&fault).Error; err == nil {
		snap.ErrCode = fault.ErrCode
		snap.FaultType = faultcode.FaultTypeFromErrCode(fault.ErrCode)
		snap.FaultLevel = faultcode.FaultLevelFromErrCode(fault.ErrCode)
	}
	return snap
}

// hasActiveFault 判断设备是否存在活跃故障（未解决）。
func (s *PatrolTaskService) hasActiveFault(hwID string) bool {
	var cnt int64
	model.DB.Model(&model.FaultRecord{}).
		Where("device_hw_id = ? AND status IN ?",
			hwID, []string{model.FaultStatusOccurred, model.FaultStatusConfirmed, model.FaultStatusDispatched}).
		Count(&cnt)
	return cnt > 0
}

// ------------------------------------ 排行聚合 ------------------------------------

// Ranking 聚合巡检排行：按巡检人 人次/异常数。dimension=patrol_by 或 device。
func (s *PatrolTaskService) Ranking(dimension string, limit int) ([]model.PatrolRankingItem, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	const aggSel = "COUNT(*) AS cnt, COALESCE(SUM(CASE WHEN check_result = ? THEN 1 ELSE 0 END),0) AS abnormal, MAX(patrol_at) AS last_at"
	out := make([]model.PatrolRankingItem, 0)

	if dimension == "device" {
		// 按设备：device_hw_id 原生列映射到具类型字段
		var rows []struct {
			DeviceHwID string
			Cnt        int
			Abnormal   int
			LastAt     string
		}
		if err := model.DB.Model(&model.PatrolRecord{}).
			Select("device_hw_id, "+aggSel, model.PatrolResultAbnormal).
			Group("device_hw_id").
			Order("cnt DESC").
			Limit(limit).
			Scan(&rows).Error; err != nil {
			return nil, err
		}
		for _, r := range rows {
			out = append(out, buildRankingItem(r.DeviceHwID, r.Cnt, r.Abnormal, r.LastAt))
		}
		return out, nil
	}

	// 按巡检人：patrol_by 原生列映射到字符串字段
	var rows []struct {
		PatrolBy string
		Cnt      int
		Abnormal int
		LastAt   string
	}
	if err := model.DB.Model(&model.PatrolRecord{}).
		Select("patrol_by, "+aggSel, model.PatrolResultAbnormal).
		Group("patrol_by").
		Order("cnt DESC").
		Limit(limit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, r := range rows {
		key := r.PatrolBy
		if key == "" {
			key = "unknown"
		}
		out = append(out, buildRankingItem(key, r.Cnt, r.Abnormal, r.LastAt))
	}
	return out, nil
}

// buildRankingItem 组装排行条目（含异常率与最后巡检时间解析）。
func buildRankingItem(key string, cnt, abnormal int, lastAt string) model.PatrolRankingItem {
	item := model.PatrolRankingItem{
		Key:         key,
		PatrolCount: cnt,
		AbnormalCnt: abnormal,
	}
	if lastAt != "" && lastAt != "0000-00-00 00:00:00" {
		if t, err := parsePatrolTime(lastAt); err == nil {
			item.LastPatrolAt = &t
		}
	}
	if cnt > 0 {
		item.AbnormalRate = float64(abnormal) / float64(cnt)
	}
	return item
}

// parsePatrolTime 解析数据库返回的时间字符串（SQLite/MySQL 兼容）。
func parsePatrolTime(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02 15:04:05Z07:00", s); err == nil {
		return t, nil
	}
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04:05.999999",
		time.RFC3339,
		time.RFC3339Nano,
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unparsable time: %s", s)
}

// CreateTask 创建巡检任务（校验模式/范围合法）。
func (s *PatrolTaskService) CreateTask(t *model.PatrolTask) (*model.PatrolTask, error) {
	if model.DB == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	if t.Name == "" {
		return nil, fmt.Errorf("任务名称必填")
	}
	mode := strings.TrimSpace(strings.ToLower(t.Mode))
	if mode == "" {
		return nil, fmt.Errorf("巡检模式必填")
	}
	switch mode {
	case model.PatrolModeArea, model.PatrolModeStreet, model.PatrolModeRandom, model.PatrolModeSelfCheck, model.PatrolModeAI:
	default:
		return nil, fmt.Errorf("不支持的巡检模式: %s", mode)
	}
	t.Mode = mode
	if t.Status == "" {
		t.Status = model.PatrolStatusPlanned
	}
	if err := model.DB.Create(t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

// ListTasks 分页查询巡检任务。
func (s *PatrolTaskService) ListTasks(page, pageSize uint) ([]model.PatrolTask, int64, error) {
	if model.DB == nil {
		return nil, 0, fmt.Errorf("数据库未初始化")
	}
	q := model.DB.Model(&model.PatrolTask{})
	var total int64
	q.Count(&total)
	var tasks []model.PatrolTask
	q.Order("id DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&tasks)
	return tasks, total, nil
}

// GetTask 单任务详情。
func (s *PatrolTaskService) GetTask(id uint, db *gorm.DB) (*model.PatrolTask, error) {
	if db == nil {
		db = model.DB
	}
	var task model.PatrolTask
	if err := db.First(&task, id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

// UpdateTask 更新任务（路径：状态/指派人/时段/抽检数）。仅改可空字段之外的业务字段。
func (s *PatrolTaskService) UpdateTask(id uint, patch map[string]interface{}) error {
	if model.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	// 白名单字段，防注入
	allowed := map[string]bool{
		"name": true, "mode": true, "area_id": true, "street_id": true,
		"time_window": true, "target_count": true, "status": true,
		"assignee_id": true, "remark": true,
	}
	upd := map[string]interface{}{}
	for k, v := range patch {
		if allowed[k] {
			upd[k] = v
		}
	}
	if len(upd) == 0 {
		return fmt.Errorf("无可更新字段")
	}
	return model.DB.Model(&model.PatrolTask{}).Where("id = ?", id).Updates(upd).Error
}

// DeleteTask 删除任务与关联记录（级联清理避免孤儿）。
func (s *PatrolTaskService) DeleteTask(id uint) error {
	if model.DB == nil {
		return fmt.Errorf("数据库未初始化")
	}
	return model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("task_id = ?", id).Delete(&model.PatrolRecord{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.PatrolTask{}, id).Error
	})
}

// ListRecords 分页查询巡检记录（过滤：task/device/result/type/巡检人/时间）。
func (s *PatrolTaskService) ListRecords(page, pageSize uint, f map[string]string) ([]model.PatrolRecord, int64, error) {
	if model.DB == nil {
		return nil, 0, fmt.Errorf("数据库未初始化")
	}
	q := model.DB.Model(&model.PatrolRecord{})
	if v := f["task_id"]; v != "" {
		if id, err := parseUint(v); err == nil {
			q = q.Where("task_id = ?", id)
		}
	}
	if v := f["device_hw_id"]; v != "" {
		q = q.Where("device_hw_id = ?", v)
	}
	if v := f["check_result"]; v != "" {
		q = q.Where("check_result = ?", v)
	}
	if v := f["patrol_type"]; v != "" {
		q = q.Where("patrol_type = ?", v)
	}
	if v := f["patrol_by"]; v != "" {
		q = q.Where("patrol_by = ?", v)
	}
	if v := f["start_time"]; v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("patrol_at >= ?", t)
		}
	}
	if v := f["end_time"]; v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			q = q.Where("patrol_at <= ?", t.Add(24*time.Hour))
		}
	}
	var total int64
	q.Count(&total)
	var list []model.PatrolRecord
	q.Order("patrol_at DESC").Offset(int((page - 1) * pageSize)).Limit(int(pageSize)).Find(&list)
	return list, total, nil
}

// parseUint 解析字符串为 uint（本地小工具，避免依赖 handler）
func parseUint(s string) (uint, error) {
	var v uint
	_, err := fmt.Sscanf(s, "%d", &v)
	return v, err
}
