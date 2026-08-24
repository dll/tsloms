// 意见反馈实体：对齐 Go 版 Feedback（表 feedbacks）。
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

/** 市民/移动端/后台提交的设备路灯问题反馈，可关联生成工单。 */
@Entity
@Table(name = "feedbacks", indexes = {
        @Index(name = "idx_feedback_device", columnList = "device_hw_id"),
})
public class Feedback {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "device_hw_id", length = 64)
    public String deviceHwId;

    /** 路口位置。 */
    @Column(name = "intersection", length = 128)
    public String intersection;

    @Column(name = "title", nullable = false, length = 128)
    public String title;

    @Column(name = "content", columnDefinition = "text")
    public String content;

    @Column(name = "reporter", length = 64)
    public String reporter;

    @Column(name = "contact", length = 64)
    public String contact;

    @Column(name = "status", nullable = false, length = 16)
    public String status = "open";

    /** 关联生成的工单 ID。 */
    @Column(name = "work_order_id")
    public Long workOrderId;

    @Column(name = "image_url", length = 512)
    public String imageUrl;

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
