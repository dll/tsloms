// 故障码/灯态常量与研判映射：对齐 Go 版 internal/faultcode。
package com.tsloms.server.mqtt.protocol;

import java.util.Map;

public final class FaultCodes {

    public static final byte ERR_OK = 0;          // 无故障
    public static final byte ERR_R_OFF = -1;      // 红灯断路/全灭
    public static final byte ERR_Y_OFF = -2;
    public static final byte ERR_G_OFF = -3;
    public static final byte ERR_RY_ON = -4;      // 红+黄同亮
    public static final byte ERR_RG_ON = -5;
    public static final byte ERR_YG_ON = -6;
    public static final byte ERR_RYG_ON = -7;     // 三色同亮
    public static final byte ERR_R_ON_TIMEOUT = -8;
    public static final byte ERR_Y_ON_TIMEOUT = -9;
    public static final byte ERR_G_ON_TIMEOUT = -10;
    public static final byte ERR_R_DIM = -11;     // 缺亮
    public static final byte ERR_Y_DIM = -12;
    public static final byte ERR_G_DIM = -13;
    public static final byte ERR_POWER_LOSS = -14; // 断电

    /** 灯组状态。 */
    public static final byte STATE_R = 0;
    public static final byte STATE_Y = 1;
    public static final byte STATE_G = 2;
    public static final byte STATE_NONE = -1;

    private static final Map<Byte, String> TYPE_MAP = Map.ofEntries(
            Map.entry(ERR_R_OFF, "lamp_off"),
            Map.entry(ERR_Y_OFF, "lamp_off"),
            Map.entry(ERR_G_OFF, "lamp_off"),
            Map.entry(ERR_RY_ON, "abnormal_on"),
            Map.entry(ERR_RG_ON, "abnormal_on"),
            Map.entry(ERR_YG_ON, "abnormal_on"),
            Map.entry(ERR_RYG_ON, "abnormal_on"),
            Map.entry(ERR_R_ON_TIMEOUT, "timeout"),
            Map.entry(ERR_Y_ON_TIMEOUT, "timeout"),
            Map.entry(ERR_G_ON_TIMEOUT, "timeout"),
            Map.entry(ERR_R_DIM, "dim"),
            Map.entry(ERR_Y_DIM, "dim"),
            Map.entry(ERR_G_DIM, "dim"),
            Map.entry(ERR_POWER_LOSS, "power_loss"));

    /** critical 集合：灯灭三态/异常同亮四态/断电（对齐 Go 版 faultcode 测试口径）。 */
    private static final java.util.Set<Byte> CRITICAL = java.util.Set.of(
            ERR_R_OFF, ERR_Y_OFF, ERR_G_OFF,
            ERR_RY_ON, ERR_RG_ON, ERR_YG_ON, ERR_RYG_ON,
            ERR_POWER_LOSS);

    private FaultCodes() {
    }

    /** 错误码 → 故障类型分类（对齐 Go 版 FaultTypeFromErrCode；未知归 unknown）。 */
    public static String faultTypeFromErrCode(byte errCode) {
        return TYPE_MAP.getOrDefault(errCode, "unknown");
    }

    /** 错误码 → 故障等级（对齐 Go 版 FaultLevelFromErrCode；critical 集合外一律 normal）。 */
    public static String faultLevelFromErrCode(byte errCode) {
        return CRITICAL.contains(errCode) ? "critical" : "normal";
    }
}
