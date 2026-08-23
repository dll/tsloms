// 业务单号生成：{prefix}{yyyyMMdd}{4位序号}（对齐 Go 版 NextBizNoCol）。
package com.tsloms.server.inventory;

import java.time.Instant;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;
import java.util.function.Function;

public final class BizNo {

    private static final DateTimeFormatter DAY =
            DateTimeFormatter.ofPattern("yyyyMMdd").withZone(ZoneId.systemDefault());

    /** 按前缀统计当日已有单数 +1，补零 4 位。 */
    public static String next(Function<String, Long> countByPrefix, String prefix) {
        String base = prefix + DAY.format(Instant.now());
        long count = countByPrefix.apply(base);
        return base + String.format("%04d", count + 1);
    }

    private BizNo() {
    }
}
