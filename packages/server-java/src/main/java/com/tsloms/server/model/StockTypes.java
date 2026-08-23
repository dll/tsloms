// 库存变动类型常量（对齐 Go 版 model.StockType*）。
package com.tsloms.server.model;

public final class StockTypes {
    public static final String IN = "in";         // 采购入库
    public static final String USE = "use";       // 领用出库（维修/巡检）
    public static final String RETURN = "return"; // 退库
    public static final String GAIN = "gain";     // 盘盈
    public static final String LOSS = "loss";     // 盘亏/报废
    public static final String ADJUST = "adjust"; // 手动调整

    private StockTypes() {
    }
}
