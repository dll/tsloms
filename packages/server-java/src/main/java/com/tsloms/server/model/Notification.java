// 站内通知与广播已读实体（对齐 Go 版 notification.go）。
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
@Table(name = "notifications", indexes = @Index(name = "idx_notif_user", columnList = "user_id"))
public class Notification {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    /** 接收用户（0=全员广播）。 */
    @Column(name = "user_id", nullable = false)
    public Long userId;

    /** report/alert/system。 */
    @Column(name = "type", nullable = false, length = 16)
    public String type = "system";

    @Column(name = "title", nullable = false, length = 128)
    public String title;

    @Column(name = "content", length = 1024)
    public String content;

    /** 前端跳转路径。 */
    @Column(name = "link", length = 256)
    public String link;

    @Column(name = "biz_type", length = 32)
    public String bizType;

    @Column(name = "biz_id", nullable = false)
    public Long bizId;

    @Column(name = "is_read", nullable = false)
    public boolean isRead;

    @Column(name = "created_at", nullable = false)
    public Instant createdAt;

    @PrePersist
    void onCreate() {
        if (createdAt == null) {
            createdAt = Instant.now();
        }
    }
}
