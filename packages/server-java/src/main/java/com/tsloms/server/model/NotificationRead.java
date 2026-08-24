// 广播通知按用户已读记录（表 notification_reads）。
package com.tsloms.server.model;

import jakarta.persistence.Column;
import jakarta.persistence.Entity;
import jakarta.persistence.GeneratedValue;
import jakarta.persistence.GenerationType;
import jakarta.persistence.Id;
import jakarta.persistence.Table;
import jakarta.persistence.UniqueConstraint;
import java.time.Instant;

@Entity
@Table(name = "notification_reads",
        uniqueConstraints = @UniqueConstraint(
                name = "uk_nr_notif_user",
                columnNames = {"notification_id", "user_id"}))
public class NotificationRead {

    @Id
    @GeneratedValue(strategy = GenerationType.IDENTITY)
    public Long id;

    @Column(name = "notification_id", nullable = false)
    public Long notificationId;

    @Column(name = "user_id", nullable = false)
    public Long userId;

    @Column(name = "read_at", nullable = false)
    public Instant readAt;
}
