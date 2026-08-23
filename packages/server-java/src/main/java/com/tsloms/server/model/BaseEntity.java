// 实体公共基类：主键与创建时间，语义对齐 GORM 模型（ID primaryKey + CreatedAt 自动填充）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.MappedSuperclass;
import jakarta.persistence.PrePersist;
import java.time.Instant;

@MappedSuperclass
public abstract class BaseEntity {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /**
     * 创建时间：为空时在落库前自动填充（GORM autoCreateTime 同义——
     * 显式赋值则保留，便于测试回填历史数据）。
     */
    @Column(name = "created_at", nullable = false, updatable = false)
    public Instant createdAt;

    public Long getId() {
        return id;
    }

    public Instant getCreatedAt() {
        return createdAt;
    }

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
