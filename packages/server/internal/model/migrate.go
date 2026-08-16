package model

import "gorm.io/gorm"

// migrateWorkOrderActiveUnique 创建并维护 work_orders.fault_id 的部分唯一索引
// 保证「同一故障至多存在一条活跃工单（pending/processing）」，防止并发复核/自动派单重复建单（M1）。
// 允许历史工单（completed/rejected）与新的活跃工单共存（fault 复现会新建 FaultRecord，故历史单互不影响）。
// 迁移前清理：对同一 fault_id 已存在的多条活跃工单，保留最新（id 最大），其余置为 rejected 并注明来源，
// 以非破坏方式移出活跃范围后，再创建唯一索引。
// 幂等：索引已存在则跳过；由 AutoMigrate 在表结构就绪后调用。
func migrateWorkOrderActiveUnique(db *gorm.DB) error {
	const idx = "idx_wo_fault_active"
	var count int64
	db.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx).Scan(&count)
	if count > 0 {
		return nil // 已存在，幂等跳过
	}

	// 清理：同一 fault_id 若已有多条活跃工单，仅保留最新一条，其余置 rejected
	if err := db.Exec(`
		UPDATE work_orders
		SET status = 'rejected', result = COALESCE(result,'') || ' [系统迁移清理:重复自动派单]'
		WHERE status IN ('pending','processing')
		  AND id NOT IN (
			SELECT MAX(id) FROM work_orders
			WHERE status IN ('pending','processing')
			GROUP BY fault_id
		  )
	`).Error; err != nil {
		return err
	}

	// 创建部分唯一索引：活跃工单在 fault_id 上唯一；排除 fault_id=0（未关联故障的占位/历史数据），
	// 否则未关联的 fault 工单会因索引冲突无法入库（fault_id 为 uint 非 NULL，默认 0）。
	return db.Exec(
		"CREATE UNIQUE INDEX IF NOT EXISTS " + idx +
			" ON work_orders(fault_id) WHERE status IN ('pending','processing') AND fault_id > 0",
	).Error
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

	// 数据迁移：创建 work_orders 活跃工单部分唯一索引（先清理重复再建索引，见 migrateWorkOrderActiveUnique）
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
