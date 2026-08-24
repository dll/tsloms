// NotificationReadRepository ：NotificationRead 数据访问。
package com.tsloms.server.repository;

import com.tsloms.server.model.NotificationRead;
import java.util.List;
import java.util.Optional;
import org.springframework.data.jpa.repository.JpaRepository;

public interface NotificationReadRepository extends JpaRepository<NotificationRead, Long> {
    Optional<NotificationRead> findByNotificationIdAndUserId(Long notificationId, Long userId);
}
