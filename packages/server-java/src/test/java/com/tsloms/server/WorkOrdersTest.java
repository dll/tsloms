// 工单工具类单测：SLA 超时计算与工单编号生成。
package com.tsloms.server;

import static org.assertj.core.api.Assertions.assertThat;

import com.tsloms.server.model.WorkOrder;
import com.tsloms.server.repository.WorkOrderRepository;
import com.tsloms.server.workorder.WorkOrders;
import java.time.Instant;
import java.time.temporal.ChronoUnit;
import org.junit.jupiter.api.Test;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.boot.test.autoconfigure.orm.jpa.DataJpaTest;
import org.springframework.context.annotation.Import;

@DataJpaTest
@Import(WorkOrders.class)
class WorkOrdersTest {

    @Autowired WorkOrderRepository orders;

    private WorkOrder wo(String status, long hoursAgo) {
        WorkOrder w = new WorkOrder();
        w.orderNo = "WO-T-" + System.nanoTime();
        w.faultId = 1L;
        w.deviceHwId = "hw";
        w.status = status;
        w.createdAt = Instant.now().minus(hoursAgo, ChronoUnit.HOURS);
        return orders.save(w);
    }

    @Test
    void 超时计算_各状态() {
        // pending 超 24h SLA：48h 前 → 超 ~24h
        double overduePending = WorkOrders.overdueHours(wo("pending", 48));
        assertThat(overduePending).isGreaterThan(23.9).isLessThan(24.1);
        // processing 48h 前 → 未超（SLA 48h 边界内）
        assertThat(WorkOrders.overdueHours(wo("processing", 10))).isZero();
        // completed 不参与超时判定
        assertThat(WorkOrders.overdueHours(wo("completed", 999))).isZero();
    }

    @Test
    void 工单编号_日期前缀四位序号递增() {
        String no1 = WorkOrders.nextOrderNo(orders);
        orders.save(newOrder(no1));
        String no2 = WorkOrders.nextOrderNo(orders);
        assertThat(no1).startsWith("WO").hasSize(14);
        assertThat(no2).startsWith("WO").hasSize(14);
        int seq1 = Integer.parseInt(no1.substring(12));
        int seq2 = Integer.parseInt(no2.substring(12));
        assertThat(seq2).isEqualTo(seq1 + 1);
    }

    private WorkOrder newOrder(String orderNo) {
        WorkOrder w = new WorkOrder();
        w.orderNo = orderNo;
        w.faultId = 2L;
        w.deviceHwId = "hw";
        w.status = "pending";
        return w;
    }
}
