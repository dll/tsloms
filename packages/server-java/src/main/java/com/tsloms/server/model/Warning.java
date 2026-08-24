// 预警记录实体：对齐 Go 版 Warning（表 warnings）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.PreUpdate;
import jakarta.persistence.Table;
import java.time.Instant;

/** 故障识别-生成链路的前端/通知通知记录；可被转工单或忽略。 */
@Entity
@Table(name = "warnings", indexes = {
        @Index(name = "idx_warn_device_hw", columnList = "device_hw_id"),
        @Index(name = "idx_warn_crossing", columnList = "crossing_id"),
        @Index(name = "idx_warn_code", columnList = "warning_code"),
        @Index(name = "idx_warn_level", columnList = "level"),
        @Index(name = "idx_warn_source", columnList = "source"),
        @Index(name = "idx_warn_deal_state", columnList = "deal_state"),
        @Index(name = "idx_warn_status", columnList = "status"),
        @Index(name = "idx_warn_fault_id", columnList = "fault_id"),
        @Index(name = "idx_warn_occurred", columnList = "occurred_at"),
})
public class Warning {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    @Column(name = "crossing_id")
    public Long crossingId;

    @Column(name = "equipment_uuid", length = 64)
    public String equipmentUuid;

    /** 告警数字码（识别引擎 errCode -1~-14，0=通用）。 */
    @Column(name = "warning_code", nullable = false)
    public Integer warningCode;

    @Column(name = "warning_label", length = 64)
    public String warningLabel;

    @Column(name = "level", nullable = false, length = 16)
    public String level = "warning";

    @Column(name = "func", length = 64)
    public String func;

    @Column(name = "source", nullable = false, length = 24)
    public String source = "fault";

    @Column(name = "deal_state", nullable = false, length = 16)
    public String dealState = "unhandled";

    @Column(name = "status", nullable = false, length = 16)
    public String status = "untransferred";

    /** 源故障 ID（来源对故障）。 */
    @Column(name = "fault_id")
    public Long faultId;

    /** 转工单后的工单 ID。 */
    @Column(name = "work_order_id")
    public Long workOrderId;

    @Column(name = "ignore_reason", length = 255)
    public String ignoreReason;

    @Column(name = "occurred_at", nullable = false)
    public Instant occurredAt;

    @Column(name = "resolved_at")
    public Instant resolvedAt;

    @Column(name = "remark", length = 255)
    public String remark;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @Column(name = "updated_at", nullable = false)
    public Instant updatedAt;

    @PrePersist
    void onCreate() {
        Instant now = Instant.now();
        if (createdAt == null) {
            createdAt = now;
        }
        if (updatedAt == null) {
            updatedAt = now;
        }
        if (occurredAt == null) {
            occurredAt = now;
        }
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = Instant.now();
    }
}
