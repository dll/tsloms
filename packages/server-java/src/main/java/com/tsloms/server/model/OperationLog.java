// 操作日志实体：对齐 Go 版 internal/model/operation_log.go（表 operation_logs）。
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
    @Column(name = "user_id", nullable = false)
    public Long userId;

    /** 操作人用户名。 */
    @Column(name = "username", nullable = false, length = 64)
    public String username;

    /** 操作类型（login/create/update/delete/dispatch）。 */
    @Column(name = "op_type", nullable = false, length = 16)
    public String opType;

    /** 目标对象标识（如 work-order/12）。 */
    @Column(name = "target", nullable = false, length = 128)
    public String target;

    /** 操作说明。 */
    @Column(name = "detail", length = 255)
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
