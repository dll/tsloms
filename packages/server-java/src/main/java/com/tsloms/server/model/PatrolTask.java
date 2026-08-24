// 巡检任务实体（表 patrol_tasks）。
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

@Entity
@Table(name = "patrol_tasks", indexes = {
        @Index(name = "idx_ptask_mode", columnList = "mode"),
        @Index(name = "idx_ptask_status", columnList = "status"),
        @Index(name = "idx_ptask_assignee", columnList = "assignee_id"),
})
public class PatrolTask {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 任务名称。 */
    @Column(name = "name", nullable = false, length = 128)
    public String name;

    /** area/street/random/selfcheck/ai。 */
    @Column(name = "mode", nullable = false, length = 16)
    public String mode;

    @Column(name = "area_id")
    public Long areaId;

    @Column(name = "street_id")
    public Long streetId;

    /** 时间窗（如 08:00-10:00，可空）。 */
    @Column(name = "time_window", length = 32)
    public String timeWindow;

    /** 目标数量（random 模式）。 */
    @Column(name = "target_count", nullable = false)
    public Integer targetCount = 0;

    @Column(name = "status", nullable = false, length = 16)
    public String status = "planned";

    @Column(name = "assignee_id")
    public Long assigneeId;

    @Column(name = "created_by", nullable = false)
    public Long createdBy;

    @Column(name = "last_run_at")
    public Instant lastRunAt;

    @Column(name = "run_count", nullable = false)
    public Integer runCount = 0;

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
