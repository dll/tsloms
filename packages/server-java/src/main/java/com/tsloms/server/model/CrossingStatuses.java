// 路口状态常量。
package com.tsloms.server.model;

public final class CrossingStatuses {
    public static final String NORMAL = "normal";
    public static final String ABNORMAL = "abnormal";
    public static final String OFFLINE = "offline";
    public static final String MAINTAIN = "maintain";
    public static final String FLASHING = "flashing";
    public static final String MONITOR = "monitor";

    private CrossingStatuses() {
    }

    /** 按故障比率/绿灯比率聚合路口状态（对齐 Go 版 ComputeCrossingStatus）。 */
    public static String compute(double faultRatio, double greenRatio) {
        if (faultRatio >= 1.0) {
            return OFFLINE;
        }
        if (faultRatio > 0) {
            return ABNORMAL;
        }
        return NORMAL;
    }
}
