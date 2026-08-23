// 工单业务工具：SLA 超时计算 + 工单编号生成（对齐 Go 版 model/workorder.go 同名函数）。
package com.tsloms.server.workorder;

import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.WorkOrderRepository;
import java.time.Duration;
import java.time.Instant;
import java.time.ZoneId;
import java.time.format.DateTimeFormatter;

public final class WorkOrders {

    /** SLA：pending 24h / processing 48h。 */
    public static final Duration PENDING_SLA = Duration.ofHours(24);
    public static final Duration PROCESSING_SLA = Duration.ofHours(48);

    private WorkOrders() {
    }

    /**
     * 计算工单超时小时数（>0 表示超时，保留 1 位小数）。
     * pending/processing 才参与判定（对齐 Go 版 WorkOrderOverdueHours）。
     */
    public static double overdueHours(WorkOrder wo) {
        Duration sla = switch (wo.status) {
            case "pending" -> PENDING_SLA;
            case "processing" -> PROCESSING_SLA;
            default -> null;
        };
        if (sla == null || wo.createdAt == null) {
            return 0;
        }
        double overdue = (Duration.between(wo.createdAt, Instant.now()).toMillis()
                - sla.toMillis()) / 1000.0;
        if (overdue <= 0) {
            return 0;
        }
        // 保留 1 位小数（Go: float64(int(overdue/60)) / 60.0 → 分钟截断）
        return Math.floor(overdue / 60.0) / 60.0;
    }

    /**
     * 生成工单编号：WO{yyyyMMdd}{当日序号+1 补零4位}（对齐 Go 版 NextOrderNo）。
     */
    public static String nextOrderNo(WorkOrderRepository repo) {
        String today = DateTimeFormatter.ofPattern("yyyyMMdd")
                .withZone(ZoneId.systemDefault()).format(Instant.now());
        String prefix = "WO" + today;
        long sameDay = repo.countByOrderNoStartingWith(prefix);
        return prefix + String.format("%04d", sameDay + 1);
    }
}
