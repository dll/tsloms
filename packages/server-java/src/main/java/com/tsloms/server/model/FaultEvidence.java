// 多源证据实体：对齐 Go 版 FaultEvidence（表 fault_evidence）。
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

/** 每条证据带来源细分；误报过滤(未生成故障)的证据也经 evaluation_id 关联批次。 */
@Entity
@Table(name = "fault_evidence", indexes = {
        @Index(name = "idx_evi_fault", columnList = "fault_id"),
        @Index(name = "idx_evi_eval", columnList = "evaluation_id"),
        @Index(name = "idx_evi_hw_time", columnList = "device_hw_id"),
})
public class FaultEvidence {

    // 证据来源常量（对齐 Go 版 EvSource*）
    public static final String SRC_FIRMWARE = "firmware";
    public static final String SRC_CURRENT = "current";
    public static final String SRC_LED_STATE = "led_state";
    public static final String SRC_CITIZEN = "citizen";
    public static final String SRC_PHOTO_EVIDENCE = "photo_evidence";
    public static final String SRC_VIDEO_MONITOR = "video_monitor";

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 故障上报记录 ID（误报过滤的证据可为空）。 */
    @Column(name = "fault_id")
    public Long faultId;

    /** 研判批次号。 */
    @Column(name = "evaluation_id", nullable = false, length = 40)
    public String evaluationId;

    @Column(name = "device_hw_id", nullable = false, length = 64)
    public String deviceHwId;

    /** firmware/current/led_state/citizen/photo_evidence/video_monitor。 */
    @Column(name = "source_type", nullable = false, length = 24)
    public String sourceType;

    @Column(name = "err_code")
    public Short errCode;

    @Column(name = "led_state")
    public Short ledState;

    @Column(name = "current_r")
    public Integer currentR;

    @Column(name = "current_y")
    public Integer currentY;

    @Column(name = "current_g")
    public Integer currentG;

    @Column(name = "raw_data", columnDefinition = "text")
    public String rawData;

    @Column(name = "ref_media_id")
    public Long refMediaId;

    @Column(name = "ref_feedback_id")
    public Long refFeedbackId;

    @Column(name = "captured_at", nullable = false)
    public Instant capturedAt;

    @Column(name = "confidence", nullable = false)
    public Double confidence = 0.0;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        Instant now = Instant.now();
        if (createdAt == null) {
            createdAt = now;
        }
        if (capturedAt == null) {
            capturedAt = now;
        }
    }
}
