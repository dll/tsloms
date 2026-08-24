// AI 用量实体（表 ai_usage）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.PrePersist;
import jakarta.persistence.Table;
import java.time.Instant;

@Entity
@Table(name = "ai_usage")
public class AIUsage {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "user_id", nullable = false)
    public Long userId;

    /** 动作（analyze/report/suggest…）。 */
    @Column(name = "action", nullable = false, length = 32)
    public String action;

    @Column(name = "tokens", nullable = false)
    public Integer tokens = 0;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
