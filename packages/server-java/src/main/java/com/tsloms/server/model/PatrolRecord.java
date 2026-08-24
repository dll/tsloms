// 巡检记录实体（表 patrol_records）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Index;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import java.time.Instant;

@Entity
@Table(name = "patrol_records", indexes = {
        @Index(name = "idx_precord_task", columnList = "task_id"),
        @Index(name = "idx_precord_device_hw", columnList = "device_hw_id"),
        @Index(name = "idx_precord_type", columnList = "patrol_type"),
        @Index(name = "idx_precord_result", columnList = "check_result"),
        @Index(name = "idx_precord_patrol_at", columnList = "patrol_at"),
})
public class PatrolRecord {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "task_id")
    public Long taskId;

    @Column(name = "device_id")
    public Long deviceId;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    @Column(name = "crossing_id")
    public Long crossingId;

    @Column(name = "patrol_type", nullable = false, length = 16)
    public String patrolType;

    /** normal/abnormal。 */
    @Column(name = "check_result", nullable = false, length = 16)
    public String checkResult;

    @Column(name = "check_detail", length = 512)
    public String checkDetail;

    /** 自检采集结果 JSON（灯态/errCode/电流）。 */
    @Column(name = "self_check_result", columnDefinition = "text")
    public String selfCheckResult;

    /** 证据 JSON（巡查照片/多源等）。 */
    @Column(name = "evidences", columnDefinition = "text")
    public String evidences;

    @Column(name = "patrol_by", nullable = false, length = 64)
    public String patrolBy;

    @Column(name = "patrol_at", nullable = false)
    public Instant patrolAt;

    @Column(name = "lat")
    public Double lat;

    @Column(name = "lng")
    public Double lng;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
