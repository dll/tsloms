// 硬件 ID 规范化与别名匹配：对齐 Go 版 model/hardware_id.go + recognition.LedUUID。
package com.tsloms.server.mqtt.protocol;

import java.util.ArrayList;
import java.util.List;
import java.util.Locale;

public final class HardwareIds {

    /** 协议 uint32 → 8 位大写十六进制历史编码（对齐 recognition.LedUUID）。 */
    public static String ledUuid(long hwId) {
        return String.format("%08X", hwId);
    }

    /** 协议值 → LA+8 位表示（仅展示/预登记匹配用，不改写历史 ID）。 */
    public static String laFromProtocol(long value) {
        return String.format("LA%08X", value);
    }

    /** 规范化：大写去空白。 */
    public static String normalize(String value) {
        return value == null ? "" : value.trim().toUpperCase(Locale.ROOT);
    }

    /** 是否平台支持的 ID 格式（8 位十六进制 或 LA+8 位字母数字）。 */
    public static boolean isValid(String value) {
        return aliases(value).size() > 0;
    }

    /**
     * 别名集合：LA+8 位 ↔ 8 位十六进制互配（对齐 Go 版 HardwareIDAliases）。
     */
    public static List<String> aliases(String value) {
        String v = normalize(value);
        List<String> out = new ArrayList<>();
        if (v.length() == 10 && v.startsWith("LA")) {
            out.add(v);
            out.add(v.substring(2));
            return out;
        }
        if (isLegacy(v)) {
            out.add(v);
            String padded = v.length() < 8 ? "0".repeat(8 - v.length()) + v : v;
            if (!padded.equals(v)) {
                out.add(padded);
            }
            out.add("LA" + padded);
            return out;
        }
        out.add(v);
        return out;
    }

    private static boolean isLegacy(String v) {
        if (v.isEmpty() || v.length() > 8) {
            return false;
        }
        for (char c : v.toCharArray()) {
            boolean hex = (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F');
            if (!hex) {
                return false;
            }
        }
        return true;
    }

    private HardwareIds() {
    }
}
