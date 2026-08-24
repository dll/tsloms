// 预警忽略规则实体：对齐 Go 版 WarningRule（表 warning_rules）。
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

/** 路口/设备/码值/等级组合的自动忽略配置，支持永久与时间段生效。 */
@Entity
@Table(name = "warning_rules", indexes = {
        @Index(name = "idx_wrule_crossing", columnList = "crossing_id"),
        @Index(name = "idx_wrule_device", columnList = "device_hw_id"),
        @Index(name = "idx_wrule_code", columnList = "warning_code"),
})
public class WarningRule {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "name", length = 64)
    public String name;

    /** 空=全部路口。 */
    @Column(name = "crossing_id")
    public Long crossingId;

    /** 空=全部设备。 */
    @Column(name = "device_hw_id", length = 64)
    public String deviceHwId;

    /** 空=全部预警码。 */
    @Column(name = "warning_code")
    public Integer warningCode;

    /** 空=全部等级。 */
    @Column(name = "level", length = 16)
    public String level;

    /** permanent/period。 */
    @Column(name = "effective_type", nullable = false, length = 16)
    public String effectiveType = "permanent";

    /** HH:mm（period 模式）。 */
    @Column(name = "start_time", length = 8)
    public String startTime;

    @Column(name = "end_time", length = 8)
    public String endTime;

    @Column(name = "action", nullable = false, length = 16)
    public String action = "ignore";

    @Column(name = "enabled", nullable = false)
    public boolean enabled = true;

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
    }

    @PreUpdate
    void onUpdate() {
        updatedAt = Instant.now();
    }
}
