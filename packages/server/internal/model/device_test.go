package model

import (
	"testing"
	"time"
)

func TestDecodeSwVer(t *testing.T) {
	// 构造: major=1, minor=2, year(2000+n)=26, month=6, day=10, build=3
	// bit[31:28]=1, bit[27:24]=2, bit[23:18]=26, bit[17:14]=6, bit[13:8]=10, bit[7:0]=3
	var v uint32
	v |= 1 << 28
	v |= 2 << 24
	v |= 26 << 18
	v |= 6 << 14
	v |= 10 << 8
	v |= 3

	info := DecodeSwVer(v)
	if info.Major != 1 || info.Minor != 2 {
		t.Errorf("版本主/次 = %d.%d, 期望 1.2", info.Major, info.Minor)
	}
	if info.Year != 26 {
		t.Errorf("year = %d, 期望 26(2000+26=2026)", info.Year)
	}
	if info.Month != 6 || info.Day != 10 {
		t.Errorf("month/day = %d/%d, 期望 6/10", info.Month, info.Day)
	}
	if info.Build != 3 {
		t.Errorf("build = %d, 期望 3", info.Build)
	}
}

func TestDecodeConfVer(t *testing.T) {
	// 0x26030801 = 26年03月08日01版 (协议 PDF 示例)
	info := DecodeConfVer(0x26030801)
	if info.Year != 0x26 || info.Month != 0x03 || info.Day != 0x08 || info.Build != 0x01 {
		t.Errorf("confVer 解码 = %02x/%02x/%02x/%02x, 期望 26/03/08/01",
			info.Year, info.Month, info.Day, info.Build)
	}
}

func TestDevice_OfflineThreshold(t *testing.T) {
	db := InitTestDB()
	oldTime := time.Now().Add(-10 * time.Minute)
	recent := time.Now()
	db.Create(&Device{HwID: 1, OnlineStatus: true, LastCheckinAt: &oldTime})
	db.Create(&Device{HwID: 2, OnlineStatus: true, LastCheckinAt: &recent})

	// 模拟离线检测：6分钟阈值
	threshold := time.Now().Add(-6 * time.Minute)
	res := db.Model(&Device{}).
		Where("online_status = ? AND last_checkin_at IS NOT NULL AND last_checkin_at < ?", true, threshold).
		Update("online_status", false)
	if res.Error != nil {
		t.Fatalf("更新失败: %v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Errorf("应置 1 台离线, 实际 %d", res.RowsAffected)
	}

	var d1, d2 Device
	db.First(&d1, "hw_id = ?", 1)
	db.First(&d2, "hw_id = ?", 2)
	if d1.OnlineStatus {
		t.Error("设备1(10分钟前签到)应离线")
	}
	if !d2.OnlineStatus {
		t.Error("设备2(刚签到)应在线")
	}
}
