// 操作日志实体：列名对齐 Go 版 operation_logs MySQL 表。
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
@Table(name = "operation_logs", indexes = {
        @Index(name = "idx_oplog_target", columnList = "target"),
})
public class OperationLog {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 操作人用户 ID。 */
    @Column(name = "user_id")
    public Long userId;

    /** 操作人用户名。 */
    @Column(name = "username", length = 64)
    public String username;

    /** 操作类型（对齐 Go 版列名 action）。 */
    @Column(name = "action", length = 64)
    public String action;

    /** 目标对象标识（如 work-order/12）。 */
    @Column(name = "target", length = 128)
    public String target;

    /** 操作来源 IP。 */
    @Column(name = "ip", length = 64)
    public String ip;

    /** 操作说明。 */
    @Column(name = "detail", columnDefinition = "text")
    public String detail;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
