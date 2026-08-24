// AI 预测/用量最小实体（映射 Go 版 ai_predictions/ai_usage，供看板聚合）。
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
import java.time.Instant;

@Entity
@Table(name = "ai_predictions", indexes = @Index(name = "idx_apred_batch", columnList = "batch_id"))
public class AIPrediction {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "device_hw_id", length = 64)
    public String deviceHwId;

    @Column(name = "intersection", length = 128)
    public String intersection;

    @Column(name = "batch_id", length = 40)
    public String batchId;

    @Column(name = "health_score", nullable = false)
    public Integer healthScore;

    @Column(name = "risk_level", length = 16)
    public String riskLevel;

    @Column(name = "predict_type", length = 32)
    public String predictType;

    @Column(name = "remain_days", nullable = false)
    public Integer remainDays;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
