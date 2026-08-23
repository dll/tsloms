// 故障记录实体：逐列对齐 Go 版 internal/model/fault.go（表 fault_records）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.Index;
import jakarta.persistence.Table;
import java.time.Instant;

/** 生命周期状态机：occurred → confirmed → dispatched → resolved。 */
@Entity
@Table(name = "fault_records", indexes = {
        @Index(name = "idx_fault_device_hw", columnList = "device_hw_id"),
        @Index(name = "idx_fault_last_eval", columnList = "last_evaluation_id"),
})
public class FaultRecord extends BaseEntity {

    /** 设备硬件 ID（uuid 字符串）。 */
    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    /** 错误码（-1 至 -14）。 */
    @Column(name = "err_code")
    public Short errCode;

    @Column(name = "fault_type", length = 32)
    public String faultType;

    /** 故障等级（critical/normal）。 */
    @Column(name = "fault_level", length = 16)
    public String faultLevel;

    /** 故障时灯组状态。 */
    @Column(name = "led_state")
    public Short ledState;

    // uint16 用 Integer 承载（无符号，避免符号位问题）
    @Column(name = "current_r")
    public Integer currentR;

    @Column(name = "current_y")
    public Integer currentY;

    @Column(name = "current_g")
    public Integer currentG;

    @Column(name = "first_seen", nullable = false)
    public Instant firstSeen;

    @Column(name = "last_seen", nullable = false)
    public Instant lastSeen;

    @Column(name = "status", nullable = false, length = 16)
    public String status = "occurred";

    @Column(name = "owner_id")
    public Long ownerId;

    @Column(name = "repairer_id")
    public Long repairerId;

    @Column(name = "confirmed_at")
    public Instant confirmedAt;

    @Column(name = "dispatched_at")
    public Instant dispatchedAt;

    @Column(name = "resolved_at")
    public Instant resolvedAt;

    @Column(name = "work_order_id")
    public Long workOrderId;

    // ---- 智能多源故障识别研判引擎新增可空字段 ----
    @Column(name = "confidence")
    public Double confidence;

    @Column(name = "recognition_source", length = 24)
    public String recognitionSource = "rule";

    @Column(name = "recognition_status", length = 24)
    public String recognitionStatus = "confirmed";

    @Column(name = "is_false_positive")
    public Boolean isFalsePositive;

    @Column(name = "evidence_count", nullable = false)
    public Integer evidenceCount = 0;

    @Column(name = "last_evaluation_id", length = 40)
    public String lastEvaluationId;

    @Column(name = "reviewed_at")
    public Instant reviewedAt;

    /** 是否仍处于进行中（未解决）。 */
    public boolean isActive() {
        return !"resolved".equals(status);
    }
}
