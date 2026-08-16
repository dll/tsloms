package model

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// MigrateLegacyDeviceMaterials 将旧 device_materials 表数据合并进新的 materials 表
// 幂等：同名物料编码已存在则跳过仅绑定设备；迁移完成后删除旧表释放空间
// 抽自 db.go（C7：维护职责拆分），逻辑不变。
func MigrateLegacyDeviceMaterials(db *gorm.DB) error {
	// 旧表不存在（全新库）则静默跳过
	if !db.Migrator().HasTable("device_materials") {
		return nil
	}
	var legacyCount int64
	if err := db.Table("device_materials").Count(&legacyCount).Error; err != nil {
		return err
	}
	if legacyCount == 0 {
		// 无数据：直接删除旧表
		return db.Migrator().DropTable("device_materials")
	}

	type legacyRow struct {
		ID         uint
		DeviceHwID uint32
		Name       string
		PartNo     string
		Spec       string
		Quantity   int
		Unit       string
		Threshold  int
		Note       string
		CreatedAt  time.Time
	}
	var rows []legacyRow
	if err := db.Table("device_materials").Find(&rows).Error; err != nil {
		return err
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		for _, r := range rows {
			code := r.PartNo
			if code == "" {
				code = r.Name
			}
			if code == "" {
				continue
			}
			// 同名物料已存在则跳过，仅补充设备绑定
			var exists Material
			err := tx.Where("code = ?", code).First(&exists).Error
			if err == nil {
				if exists.DeviceHwID == nil && r.DeviceHwID > 0 {
					hw := r.DeviceHwID
					if err := tx.Model(&exists).Update("device_hw_id", &hw).Error; err != nil {
						return err
					}
				}
				continue
			} else if err != gorm.ErrRecordNotFound {
				return err
			}

			cat := "其他"
			switch {
			case containsAny(r.Name, "灯", "LED", "泡"):
				cat = "灯泡"
			case containsAny(r.Name, "电源", "驱动"):
				cat = "电源"
			case containsAny(r.Name, "控制", "模块"):
				cat = "控制器"
			case containsAny(r.Name, "线", "缆"):
				cat = "线缆"
			}

			var hwPtr *uint32
			if r.DeviceHwID > 0 {
				hw := r.DeviceHwID
				hwPtr = &hw
			}
			m := Material{
				Code: code, Name: r.Name, Category: cat, Spec: r.Spec,
				Unit: r.Unit, Stock: r.Quantity, Threshold: r.Threshold,
				DeviceHwID: hwPtr, Note: r.Note, Status: "active",
				CreatedAt: r.CreatedAt, UpdatedAt: time.Now(),
			}
			if err := tx.Create(&m).Error; err != nil {
				return err
			}
			if m.Stock > 0 {
				if err := tx.Create(&MaterialStock{
					MaterialID: m.ID, MaterialName: m.Name, Type: StockTypeIn,
					Quantity: m.Stock,
					RefType:  "adjust", Operator: "system", Note: "旧耗材台账合并初始库存",
				}).Error; err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 事务外删除旧表，避免事务内嵌套迁移导致连接死锁
	return db.Migrator().DropTable("device_materials")
}

// containsAny 判断 s 是否包含任意子串（大小写不敏感）
func containsAny(s string, subs ...string) bool {
	ls := strings.ToLower(s)
	for _, sub := range subs {
		if strings.Contains(ls, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}
