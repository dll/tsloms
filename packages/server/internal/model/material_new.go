package model

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ===== 库存管理-物料档案 =====

// Material 物料档案（独立于设备，一处库存多处领用）
type Material struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	Code       string     `json:"code" gorm:"size:32;uniqueIndex;comment:物料编码"`
	Name       string     `json:"name" gorm:"size:64;index;comment:物料名称"`
	Category   string     `json:"category" gorm:"size:32;index;comment:物料分类(灯泡/电源/控制器/线缆/其它)"`
	Spec       string     `json:"spec" gorm:"size:128;comment:规格参数"`
	Unit       string     `json:"unit" gorm:"size:16;comment:单位(个/支/套/米)"`
	UnitPrice  float64    `json:"unit_price" gorm:"type:decimal(10,2);default:0;comment:单价(元)"`
	Stock      int        `json:"stock" gorm:"default:0;comment:当前库存数量"`
	Threshold  int        `json:"threshold" gorm:"default:0;comment:库存预警阈值"`
	DeviceHwID *uint32    `json:"device_hw_id" gorm:"index;comment:绑定设备ID(可空,设备耗材才填)"`
	SupplierID *uint      `json:"supplier_id" gorm:"index;comment:默认供应商ID"`
	Note       string     `json:"note" gorm:"type:text;comment:备注"`
	Status     string     `json:"status" gorm:"size:16;default:active;comment:状态(active/disabled)"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (Material) TableName() string { return "materials" }

// 库存变动类型
const (
	StockTypeIn      = "in"      // 采购入库
	StockTypeUse     = "use"     // 领用出库(维修/工单)
	StockTypeReturn  = "return"  // 退库
	StockTypeGain    = "gain"    // 盘盈
	StockTypeLoss    = "loss"    // 盘亏/报废
	StockTypeAdjust  = "adjust"  // 手动调整
)

// MaterialStock 物料出入库流水
type MaterialStock struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	MaterialID  uint      `json:"material_id" gorm:"index;comment:物料ID"`
	MaterialName string   `json:"material_name" gorm:"size:64;comment:物料名称(冗余)"`
	Type        string    `json:"type" gorm:"size:16;index;comment:类型(in/use/return/gain/loss/adjust)"`
	Quantity    int       `json:"quantity" gorm:"comment:变动数量(正负,出库为负)"`
	Price       float64   `json:"price" gorm:"type:decimal(10,2);default:0;comment:单价(元)"`
	Amount      float64   `json:"amount" gorm:"type:decimal(12,2);default:0;comment:金额(元)"`
	RefType     string    `json:"ref_type" gorm:"size:24;comment:关联类型(purchase/repair/adjust)"`
	RefID       uint      `json:"ref_id" gorm:"index;comment:关联单ID"`
	WorkOrderID *uint     `json:"work_order_id" gorm:"index;comment:关联工单ID"`
	Operator    string    `json:"operator" gorm:"size:64;comment:操作人"`
	Note        string    `json:"note" gorm:"size:255;comment:备注"`
	CreatedAt   time.Time `json:"created_at"`
}

// TableName 指定表名
func (MaterialStock) TableName() string { return "material_stocks" }

// ===== 供应商 =====

// Supplier 供应商档案
type Supplier struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:64;index;comment:供应商名称"`
	Contact   string    `json:"contact" gorm:"size:32;comment:联系人"`
	Phone     string    `json:"phone" gorm:"size:32;comment:联系电话"`
	Address   string    `json:"address" gorm:"size:255;comment:地址"`
	Email     string    `json:"email" gorm:"size:64;comment:邮箱"`
	Status    string    `json:"status" gorm:"size:16;default:active;comment:状态(active/disabled)"`
	Note      string    `json:"note" gorm:"type:text;comment:备注"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName 指定表名
func (Supplier) TableName() string { return "suppliers" }

// ===== 采购单 =====

// PurchaseOrder 采购单
type PurchaseOrder struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	OrderNo      string     `json:"order_no" gorm:"size:32;uniqueIndex;comment:采购单号(PO{yyyyMMdd}{seq})"`
	SupplierID   uint       `json:"supplier_id" gorm:"index;comment:供应商ID"`
	Status       string     `json:"status" gorm:"size:16;default:draft;comment:状态(draft/partial/completed/cancelled)"`
	TotalAmount  float64    `json:"total_amount" gorm:"type:decimal(12,2);default:0;comment:采购总金额(元)"`
	ReceivedAt   *time.Time `json:"received_at" gorm:"comment:入库时间"`
	Operator     string     `json:"operator" gorm:"size:64;comment:经办人"`
	Note         string     `json:"note" gorm:"type:text;comment:备注"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (PurchaseOrder) TableName() string { return "purchase_orders" }

// 采购单状态
const (
	PurchaseStatusDraft     = "draft"     // 草稿
	PurchaseStatusPartial   = "partial"   // 部分入库
	PurchaseStatusCompleted = "completed" // 已完成(全部入库)
	PurchaseStatusCancelled = "cancelled" // 已取消
)

// PurchaseOrderItem 采购单明细
type PurchaseOrderItem struct {
	ID            uint    `json:"id" gorm:"primaryKey"`
	OrderID       uint    `json:"order_id" gorm:"index;comment:采购单ID"`
	MaterialID    uint    `json:"material_id" gorm:"index;comment:物料ID"`
	MaterialName  string  `json:"material_name" gorm:"size:64;comment:物料名称(冗余)"`
	Quantity      int     `json:"quantity" gorm:"comment:采购数量"`
	Price         float64 `json:"price" gorm:"type:decimal(10,2);default:0;comment:单价(元)"`
	Amount        float64 `json:"amount" gorm:"type:decimal(12,2);default:0;comment:小计(元)"`
	ReceivedQty   int     `json:"received_qty" gorm:"default:0;comment:已入库数量"`
}

// TableName 指定表名
func (PurchaseOrderItem) TableName() string { return "purchase_order_items" }

// ===== 维修费用单 =====

// 费用类型
const (
	ExpenseTypeMaterial = "material" // 耗材
	ExpenseTypeLabor    = "labor"    // 人工
	ExpenseTypeTraffic  = "traffic"  // 交通
	ExpenseTypeOther    = "other"    // 其它
)

// RepairExpense 维修费用单（关联工单/设备）
type RepairExpense struct {
	ID          uint       `json:"id" gorm:"primaryKey"`
	ExpenseNo   string     `json:"expense_no" gorm:"size:32;uniqueIndex;comment:费用单号(FE{yyyyMMdd}{seq})"`
	WorkOrderID *uint      `json:"work_order_id" gorm:"index;comment:关联工单ID"`
	DeviceHwID  uint32     `json:"device_hw_id" gorm:"index;comment:设备硬件ID"`
	Type        string     `json:"type" gorm:"size:16;index;comment:费用类型(material/labor/traffic/other)"`
	Amount      float64    `json:"amount" gorm:"type:decimal(12,2);default:0;comment:费用金额(元)"`
	Description string     `json:"description" gorm:"size:255;comment:费用说明"`
	WorkDate    *time.Time `json:"work_date" gorm:"comment:发生日期"`
	Operator    string     `json:"operator" gorm:"size:64;comment:经办人"`
	Confirmed   bool       `json:"confirmed" gorm:"default:false;comment:是否已确认入账"`
	Note        string     `json:"note" gorm:"type:text;comment:备注"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// TableName 指定表名
func (RepairExpense) TableName() string { return "repair_expenses" }

// NextBizNo 生成业务单号：{prefix}{yyyyMMdd}{4位自增序号}
// 基于当日前缀已有单数 + 1，保证同日内序号连续且唯一
// 默认查询 order_no 列
func NextBizNo(db *gorm.DB, tableName, prefix string) string {
	return NextBizNoCol(db, tableName, "order_no", prefix)
}

// NextBizNoCol 生成业务单号，可指定单号列名（如 expense_no）
func NextBizNoCol(db *gorm.DB, tableName, column, prefix string) string {
	today := time.Now().Format("20060102")
	base := prefix + today
	var count int64
	db.Table(tableName).Where(column+" LIKE ?", base+"%").Count(&count)
	return fmt.Sprintf("%s%04d", base, count+1)
}
