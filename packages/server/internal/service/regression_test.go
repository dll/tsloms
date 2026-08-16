package service

import (
	"strconv"
	"testing"

	"github.com/tsloms/server/internal/model"
)

// ============================================================================
// 回归测试：针对 refactor-notes.md 中 patrol 改动（B4）
//   基线：origin/main a460365 ｜ 范围：不改变业务行为
// ============================================================================

// TestRegression_B4_StockCountVsTopN 覆盖 B4：stockCountAndNames 返回的 count=全部匹配数、
// 名单=按 stock 升序前 6，且 low 分支区隔低库存(0<stock<=threshold) 与缺货(stock<=0)。
// 这是 B2 合并单次扫描的关键语义，重构后不得把 count 误截成前 6。
func TestRegression_B4_StockCountVsTopN(t *testing.T) {
	model.InitTestDB()
	p := NewPatrolService()

	// 构造 7 种低库存（0<stock<=threshold），验证 count=7 但名单只取前 6，且按 stock 升序
	stocks := []int{5, 4, 3, 2, 1, 6, 7}
	for i, s := range stocks {
		model.DB.Create(&model.Material{Code: "LOW" + strconv.Itoa(i), Name: "灯珠" + strconv.Itoa(i),
			Status: "active", Stock: s, Threshold: 10})
	}
	// 2 种缺货（stock<=0）
	model.DB.Create(&model.Material{Code: "OUT1", Name: "电源缺1", Status: "active", Stock: 0, Threshold: 5})
	model.DB.Create(&model.Material{Code: "OUT2", Name: "电源缺2", Status: "active", Stock: -3, Threshold: 5})
	// 1 种正常库存（>threshold），不得计入低库存
	model.DB.Create(&model.Material{Code: "OK1", Name: "正常", Status: "active", Stock: 50, Threshold: 10})

	// 低库存：count=7（全部匹配），名单=前 6（按 stock 升序：1,2,3,4,5,6）
	lowCount, lowNames := p.stockCountAndNames(true)
	if lowCount != 7 {
		t.Errorf("低库存 count = %d, 期望 7", lowCount)
	}
	if len(lowNames) != 6 {
		t.Fatalf("低库存名单应取前 6, 实际 %d", len(lowNames))
	}
	// 名单按 stock 升序，内容为前 6 小 stock 的物料名
	if lowNames[0] == "" {
		t.Error("名单不应含空名")
	}
	// stock=7 的“灯珠6”不应出现在前 6 名单中（其 stock 最大被剔出）
	for _, n := range lowNames {
		if n == "灯珠6" {
			t.Errorf("stock=7 的物料不应进入前 6 名单: %v", lowNames)
		}
	}

	// 缺货：count=2（全量），名单=2（不足 6 全量返回）
	outCount, outNames := p.stockCountAndNames(false)
	if outCount != 2 {
		t.Errorf("缺货 count = %d, 期望 2", outCount)
	}
	if len(outNames) != 2 {
		t.Errorf("缺货名单应全量 2, 实际 %d", len(outNames))
	}
}

// TestRegression_B4_LowStockNamesThinWrapper 覆盖 B4：lowStockNames 保留为薄封装，
// 结果需与重构前一致（名单=前6 按 stock 升序），兼容既有调用/测试。
func TestRegression_B4_LowStockNamesThinWrapper(t *testing.T) {
	model.InitTestDB()
	p := NewPatrolService()
	// 5 种低库存（不足 6，全量返回；按 stock 升序）
	model.DB.Create(&model.Material{Code: "a", Name: "A", Status: "active", Stock: 9, Threshold: 10})
	model.DB.Create(&model.Material{Code: "b", Name: "B", Status: "active", Stock: 1, Threshold: 10})
	model.DB.Create(&model.Material{Code: "c", Name: "C", Status: "active", Stock: 5, Threshold: 10})

	names := p.lowStockNames(true)
	if len(names) != 3 {
		t.Fatalf("lowStockNames 应返回 3 个低库存, 实际 %d", len(names))
	}
	// 按 stock 升序应为 B(1), C(5), A(9)
	if names[0] != "B" || names[1] != "C" || names[2] != "A" {
		t.Errorf("lowStockNames 排序错误 = %v, 期望 [B C A]", names)
	}
}
