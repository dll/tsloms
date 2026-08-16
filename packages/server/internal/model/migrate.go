package model

import "gorm.io/gorm"

// migrateWorkOrderActiveUnique 创建并维护 work_orders.fault_active_scope 唯一索引（M1 并发防重建单）。
//
// 背景：MySQL 不支持 SQLite/Postgres 的“部分/过滤索引”（CREATE ... WHERE），无法直接对
// “活跃工单(pending/processing)”建部分唯一。改用可空派生列 fault_active_scope 模拟：
//   - 活跃工单：fault_active_scope = fault_id（唯一索引保证同 fault 至多一条活跃单）
//   - 非活跃（completed/rejected）：fault_active_scope = NULL（NULL 不参与唯一，允许多条历史单）
//
// 该函数在 AutoMigrate 之后调用：先为既有活跃工单回填 scope（并清理同 fault 重复活跃单），
// 再创建唯一索引 uk_wo_active_scope。幂等：索引已存在则跳过。
// 兼容 SQLite(测试) 与 MySQL(生产)：全部用 GORM 查询/更新与标准 CREATE UNIQUE INDEX，
// 不使用 sqlite_master / CREATE...WHERE 等方言语法。
func migrateWorkOrderActiveUnique(db *gorm.DB) error {
	const idx = "uk_wo_active_scope"
	if db.Migrator().HasIndex(&WorkOrder{}, idx) {
		return nil // 已存在，幂等跳过
	}

	// 1) 清理：同一 fault_id 若已有多条活跃工单，仅保留最新(id 最大)一条，其余置 rejected 并注明来源。
	//    用 GORM 查出每 fault 应保留的最大 id，再按 id NOT IN 更新（避开 MySQL 同表子查询 Error 1093）。
	var keepIDs []uint
	db.Model(&WorkOrder{}).
		Select("MAX(id)").
		Where("status IN ?", []string{WorkOrderStatusPending, WorkOrderStatusProcessing}).
		Group("fault_id").
		Pluck("MAX(id)", &keepIDs)
	if len(keepIDs) > 0 {
		if err := db.Model(&WorkOrder{}).
			Where("status IN ? AND id NOT IN ?", []string{WorkOrderStatusPending, WorkOrderStatusProcessing}, keepIDs).
			Updates(map[string]interface{}{
				"status":             WorkOrderStatusRejected,
				"fault_active_scope": nil,
				"result":             gorm.Expr("COALESCE(result,'') || ' [系统迁移清理:重复自动派单]'"),
			}).Error; err != nil {
			return err
		}
	}

	// 2) 为仍活跃的工单回填 scope=fault_id（新工单在 EnsureActiveWorkOrder/CreateWorkOrder 已写；此处补历史数据）
	if err := db.Model(&WorkOrder{}).
		Where("status IN ? AND fault_active_scope IS NULL", []string{WorkOrderStatusPending, WorkOrderStatusProcessing}).
		Updates(map[string]interface{}{"fault_active_scope": gorm.Expr("fault_id")}).Error; err != nil {
		return err
	}

	// 3) 创建唯一索引（fault_active_scope 上唯一；NULL 允许多个，不参与唯一）
	return db.Exec("CREATE UNIQUE INDEX " + idx + " ON work_orders(fault_active_scope)").Error
}

// AutoMigrate 自动迁移全部域模型
// 分段抽出自 db.go（C7：维护职责拆分），逻辑不变。
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&Device{},
		&PacketLog{},
		&FaultRecord{},
		&FaultEvidence{},
		&FaultCase{},
		&WorkOrder{},
		&User{},
		&Department{},
		&OperationLog{},
		&DeviceMedia{},
		&Feedback{},
		&AIConfig{},
		&AIUsage{},
		&AIPrediction{},
		&FirmwarePackage{},
		&FirmwareUpgradeRecord{},
		&Material{},
		&MaterialStock{},
		&Supplier{},
		&PurchaseOrder{},
		&PurchaseOrderItem{},
		&RepairExpense{},
		&AIReport{},
		&AIAdvice{},
		&Permission{},
		&Role{},
		&RolePermission{},
		&UserPermission{},
		&Notification{},
		&NotificationRead{},
	); err != nil {
		return err
	}
	// 数据迁移：旧版故障状态 active → occurred（四态模型引入后）
	db.Model(&FaultRecord{}).Where("status = ?", "active").Update("status", FaultStatusOccurred)

	// 数据迁移：创建 work_orders.fault_active_scope 唯一索引（先回填/清理再建索引，见 migrateWorkOrderActiveUnique）
	if err := migrateWorkOrderActiveUnique(db); err != nil {
		return err
	}

	// 数据合并：旧的设备耗材台账(device_materials)并入统一物料档案(materials)
	// 保留设备维度 device_hw_id，写入初始库存流水，迁移完成后删除旧表
	MigrateLegacyDeviceMaterials(db)

	// 初始化 RBAC 权限字典与内置角色（幂等）
	SeedRBAC(db)
	return nil
}
