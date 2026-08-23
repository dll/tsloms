// 工单实体：逐列对齐 Go 版 internal/model/workorder.go（表 work_orders）。
//
// FaultActiveScope 唯一约束说明：活跃工单(active=pending/processing)时为 fault_id，
// 完结/驳回为 NULL；唯一索引 uk_wo_active_scope 由迁移脚本维护（Java 双跑期不建表）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Index;
import jakarta.persistence.Table;
import java.time.Instant;

@Entity
@Table(name = "work_orders", indexes = {
        @Index(name = "idx_wo_fault_id", columnList = "fault_id"),
        @Index(name = "idx_wo_device_hw", columnList = "device_hw_id"),
        @Index(name = "idx_wo_fas", columnList = "fault_active_scope"),
})
public class WorkOrder extends BaseEntity {

    /** 工单编号 WO{yyyyMMdd}{seq}。 */
    @Column(name = "order_no", nullable = false, length = 32)
    public String orderNo;

    @Column(name = "fault_id", nullable = false)
    public Long faultId;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    @Column(name = "status", nullable = false, length = 16)
    public String status = "pending";

    /** 活跃工单唯一约束载体；非活跃为 NULL。 */
    @Column(name = "fault_active_scope")
    public Long faultActiveScope;

    @Column(name = "assignee_id")
    public Long assigneeId;

    /** 维修结果说明（text）。 */
    @Column(name = "result", columnDefinition = "text")
    public String result;

    @Column(name = "closed_at")
    public Instant closedAt;
}
