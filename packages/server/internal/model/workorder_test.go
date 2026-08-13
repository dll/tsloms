package model

import (
	"strings"
	"testing"
	"time"
)

func TestNextOrderNo_Sequential(t *testing.T) {
	db := InitTestDB()
	db.Create(&WorkOrder{OrderNo: "WO202608140001", Status: WorkOrderStatusPending})
	db.Create(&WorkOrder{OrderNo: "WO202608140002", Status: WorkOrderStatusPending})

	no := NextOrderNo(db)
	prefix := "WO" + time.Now().Format("20060102")
	if !strings.HasPrefix(no, prefix) {
		t.Errorf("工单编号 %s 前缀应为 %s", no, prefix)
	}
	if !strings.HasSuffix(no, "0003") {
		t.Errorf("工单编号 %s 序列应为 0003", no)
	}
}

func TestNextOrderNo_NewDateStartsAt0001(t *testing.T) {
	db := InitTestDB()
	// 模拟昨天的一张工单，不应计入今天的序列
	suffix := time.Now().AddDate(0, 0, -1).Format("20060102")
	db.Create(&WorkOrder{OrderNo: "WO" + suffix + "0001", Status: WorkOrderStatusPending})

	no := NextOrderNo(db)
	if !strings.HasSuffix(no, "0001") {
		t.Errorf("昨日工单不应计今日序列, 实际 %s 期望后缀 0001", no)
	}
}

func TestWorkOrderConstants(t *testing.T) {
	if WorkOrderStatusPending != "pending" ||
		WorkOrderStatusProcessing != "processing" ||
		WorkOrderStatusCompleted != "completed" ||
		WorkOrderStatusRejected != "rejected" {
		t.Error("工单状态常量定义不完整")
	}
}

func TestUserRoleConstants(t *testing.T) {
	if RoleAdmin != "admin" || RoleOperator != "operator" || RoleViewer != "viewer" {
		t.Error("用户角色常量定义不完整")
	}
}
