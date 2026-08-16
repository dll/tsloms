package model

import "gorm.io/gorm"

// AutoMigrate 自动迁移全部域模型
// 分段抽出自 db.go（C7：维护职责拆分），逻辑不变。
func AutoMigrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&Device{},
		&PacketLog{},
		&FaultRecord{},
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

	// 数据合并：旧的设备耗材台账(device_materials)并入统一物料档案(materials)
	// 保留设备维度 device_hw_id，写入初始库存流水，迁移完成后删除旧表
	MigrateLegacyDeviceMaterials(db)

	// 初始化 RBAC 权限字典与内置角色（幂等）
	SeedRBAC(db)
	return nil
}
