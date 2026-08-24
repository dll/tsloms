// NotificationRepository ：Notification 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.Notification;
import org.springframework.data.jpa.repository.JpaRepository;

public interface NotificationRepository extends JpaRepository<Notification, Long> {
}
