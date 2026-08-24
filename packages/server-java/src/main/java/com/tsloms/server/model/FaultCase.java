// 识别案例库实体：对齐 Go 版 FaultCase（表 fault_case）。
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

/** 每条案例沉淀一次判定的证据摘要 + 期望判定 + 关闭阈值，用于案例学习。 */
@Entity
@Table(name = "fault_case", indexes = {
        @Index(name = "idx_fcase_type", columnList = "fault_type"),
        @Index(name = "idx_fcase_device", columnList = "device_hw_id"),
})
public class FaultCase {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 基准故障类型。 */
    @Column(name = "fault_type", nullable = false, length = 32)
    public String faultType;

    @Column(name = "fault_level", length = 16)
    public String faultLevel;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    /** 输入证据指纹（哈希签名，用于快速检索）。 */
    @Column(name = "input_signature", length = 128)
    public String inputSignature;

    @Column(name = "evidence_summary", columnDefinition = "text")
    public String evidenceSummary;

    /** 关闭阈值：真实故障类型/等级（expected=normal 表示误报）。 */
    @Column(name = "expected_result", nullable = false, length = 32)
    public String expectedResult;

    /** 引擎原始判定。 */
    @Column(name = "judged_result", length = 32)
    public String judgedResult;

    @Column(name = "judge_confidence")
    public Double judgeConfidence;

    @Column(name = "is_correct")
    public Boolean isCorrect;

    @Column(name = "source_evaluation_id", length = 40)
    public String sourceEvaluationId;

    /** seed/confirmed/training/test。 */
    @Column(name = "status", nullable = false, length = 16)
    public String status = "seed";

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
